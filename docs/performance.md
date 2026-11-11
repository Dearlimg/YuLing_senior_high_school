# 性能压测报告

日期：2026-11-11　目标：`yulin-server:85f0fab`（单副本，NodePort 30443）

## 测试方法

| 项 | 设置 |
| --- | --- |
| 压测工具 | `tools/loadtest`（自建，Go 交叉编译，零外部依赖） |
| 压测位置 | 部署机本地 `127.0.0.1:30443`，**绕开 nginx 与 SSH 隧道** |
| 数据量 | 5001 条校友档案、29 个届别 |
| 并发 | 默认 50，对照组 10 |
| 预热 | 1–2s，避免 TCP 握手与首次查询污染样本 |

不压公网 8000 的原因：那条链路是 `nginx → SSH 隧道 → NodePort`，测出来的是隧道能力，
不是服务能力。隧道抖动会完全淹没服务本身的性能特征。

## 结果

| 接口 | 查询类型 | QPS | P50 | P95 | P99 |
| --- | --- | --- | --- | --- | --- |
| `GET /health` | 纯内存 | **15,147** | 1.7 ms | 10.4 ms | 22.5 ms |
| `GET /api/v1/alumni/:id` | 主键查询 | **3,294** | 11.7 ms | 38.3 ms | 50.4 ms |
| `GET /api/v1/alumni` | COUNT + 排序分页 | **170** | 270 ms | 530 ms | 675 ms |
| `GET /api/v1/alumni` 深分页 | 同上，OFFSET 4000 | 145 | 312 ms | 594 ms | 809 ms |
| `GET /api/v1/directory-meta` | GROUP BY 聚合 | 145 | 309 ms | 625 ms | 862 ms |
| `GET /api/v1/stats` | 多表 COUNT | 169 | 284 ms | 456 ms | 600 ms |

写接口（30 次并发提交同一表单）：

```
状态码  409:20  429:14
```

- `409`：`create_key` 唯一索引拦下重复提交，幂等保护有效
- `429`：第 21 次起触发 20 次/10 分钟限流

## 结论

### 1. 框架层不是瓶颈

`/health` 15,147 QPS、P99 22.5ms。Gin + 中间件 + 序列化的开销可以忽略。

### 2. 主键查询正常，聚合查询塌陷

`/alumni/:id` 3,294 QPS，而任何带 `COUNT`/`GROUP BY` 的接口只有 145~170 QPS，
相差 **20 倍**。这是明确的信号：问题在聚合，不在传输或路由。

### 3. 排序索引缺失（已修复）

原始执行计划：

```
列表 SELECT: type=ALL  key=NULL  rows=4981  Extra=Using where; Using filesort
```

`ORDER BY graduation_year DESC, id DESC` 用不上 `idx_year_class`（第二列是 `class_name`），
优化器放弃索引，退化成全表扫描 + filesort。

补索引 `(deleted_at, graduation_year, id)` 后：

```
type=index  key=idx_feed  rows=20  Extra=Using where; Backward index scan
```

扫描行数 4981 → 20，filesort 消失。**但 QPS 没有提升（170 → 158）**——
这个反直觉的结果恰好证明：filesort 不是瓶颈，`COUNT(*)` 才是。

### 4. COUNT(*) 是真正的吞吐上限

```
COUNT(*) WHERE deleted_at IS NULL   19.5 ms   （5001 行）
SELECT ... LIMIT 20                  9.7 ms   → 加索引后 <1 ms
```

单次列表请求要执行 COUNT + SELECT 两条 SQL。索引把 SELECT 优化到 1ms 以内，
但 COUNT 仍是 19.5ms，占总耗时 95%。即使数据全在 buffer pool（命中率 99.86%），
纯内存扫 5001 个索引条目也不该要 19.5ms——这台机器的单核性能受虚拟化 steal 影响
（steal time 9.6%，load average 8.5，同机还跑着 Kafka/Hadoop/YARN）。

**关键判断：COUNT 是 O(n) 操作，与数据量线性相关。** 当前 5000 行已经限制住 170 QPS，
涨到 10 万行会直接不可用。这是架构问题，换机器解决不了。

### 5. 存储层存在极端长尾

| I/O 类型 | 平均等待 |
| --- | --- |
| `innodb_data_file` | 1123 ms |
| `innodb_log_file` | 1338 ms |
| `innodb_dblwr_file` | 2456 ms |

配合 buffer pool 命中率 99.86% 看，这些是偶发但极慢的落盘操作（累计 11 天统计）。
后果是 P99 长尾明显：列表接口 P50 270ms、P99 675ms，抖动幅度 2.5 倍。
共享磁盘上还有 Kafka、MinIO、Hadoop 在写，MySQL 拿不到稳定的 I/O 带宽。

### 6. 限流与幂等符合设计

- 重复提交返回 409，不是 429——说明幂等检查在限流之前，语义正确
- 限流按 IP + 路径分桶，20 次/10 分钟，第 21 次返回 429

## 优化建议（按收益/成本排序）

| 优先级 | 措施 | 预期收益 | 成本 |
| --- | --- | --- | --- |
| P0 | COUNT 结果缓存到 Redis（TTL 30–60s） | QPS 170 → 1500+，因为 SELECT 已降到 <1ms | 低，改 logic 层一个方法 |
| P0 | 或翻页时不重复 COUNT，只在第 1 页算总数 | 翻页请求直接受益 | 低，属接口契约调整 |
| P1 | 深分页改 keyset（`WHERE (graduation_year,id) < (?,?)`） | 深页不再随 OFFSET 线性劣化 | 中，需前端配合 |
| P1 | 调大 `innodb_buffer_pool_size`（128M → 512M） | 减少偶发落盘，改善 P99 | 低，但影响共享实例上其他业务 |
| P2 | MySQL 换独立磁盘或独立实例 | 根治 I/O 长尾 | 高 |
| P2 | 为 `/stats`、`/directory-meta` 增加 Redis 缓存 | 这两个接口同样被聚合拖累 | 中 |

## 优化后复测（v0.1.1，Redis cache aside 上线）

同样的数据量（5001 条）、同样的并发（50）、同样的压测位置：

| 接口 | 优化前 QPS | 优化后 QPS | 提升 | P50 前 | P50 后 |
| --- | --- | --- | --- | --- | --- |
| `/api/v1/alumni` 列表 | 170 | **1,669** | 9.8× | 270 ms | 27 ms |
| 深分页 `page=200` | 145 | **1,799** | 12.4× | 312 ms | 24 ms |
| `/api/v1/stats` | 169 | **4,383** | 26× | 284 ms | 9.7 ms |

三点值得记录：

1. **深分页比第一页还快**（1799 vs 1669）。缓存命中后 `OFFSET 4000` 这个动作根本没发生，
   说明 OFFSET 的线性劣化被彻底绕过，不需要急着改 keyset 分页。
2. **`/stats` 提升最猛（26×）**，因为它的原始实现是多次跨表 `COUNT`，一次命中缓存等于省掉全部聚合。
3. **剩余延迟的构成变了**。优化后 P50 27ms，其中 Redis 往返 + JSON 序列化占了大头，
   数据库不再是主导。想继续压，下一步要看 Redis 网络栈而不是 MySQL。

这也反过来验证了原始判断：瓶颈确实是聚合查询，而不是框架或传输层。
如果当时误判成"机器太慢"去升级配置，钱花了问题还在。

## 部署机环境依赖（务必记录）

CI 的 `go build` 需要拉依赖，而这台机器的网络有两个坑：

| 问题 | 现象 | 处理 |
| --- | --- | --- |
| `docker.io` DNS 污染 | 解析到 `157.240.18.18`（无效 IP） | 镜像改用本地预编译（`Dockerfile.prebuilt`） |
| `goproxy.cn` 只有 AAAA 可达 | `dial tcp [240e:...]:443: network is unreachable` | `/etc/hosts` 固定 IPv4 `115.227.35.44 goproxy.cn` |

第二条的成因：机器没有全局 IPv6 路由（只有 link-local），但 Go 的 resolver 优先选了 AAAA 记录
并且没有回退。IPv4 本身完全正常（`curl -4` 返回 200）。

**换机器或重装后必须重新加这条 hosts，否则任何新增依赖都会让 CI 在 `go mod download` 阶段失败。**
更彻底的做法是 `go mod vendor` 把依赖固化进仓库，目前没有采用（仓库体积换网络确定性，取舍留给后续）。

## 复现方式

```bash
# 工具在 tools/loadtest，交叉编译后拷到部署机
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/loadtest ./tools/loadtest

# 列表接口
/root/loadtest -url 'http://127.0.0.1:30443/api/v1/alumni?page=1&page_size=20' -c 50 -d 10s

# 固定请求数模式（用于观察状态码分布，例如限流边界）
/root/loadtest -url 'http://127.0.0.1:30443/api/v1/alumni' -method POST \
  -H 'Content-Type: application/json' -body '{"name":"x","graduation_year":2015}' -c 5 -n 30
```
