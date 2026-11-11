# YuLing Senior High School

榆林高中校友录 —— Go 单体服务。当前阶段目标是**基础设施与工程骨架**：
分层清晰、配置可注入、凭据不进仓库、能一条命令部署到自建 k3s。

## 目录结构

```
.
├── cmd/server/            # 唯一入口：装配依赖 + 启停生命周期
├── internal/
│   ├── conf/              # 配置加载与校验（只读环境变量）
│   ├── controller/        # HTTP 层：解析入参 → 调 logic → 写响应
│   ├── logic/             # 业务层：规则校验、DTO 装配
│   ├── dao/               # 数据层：唯一能碰 gorm / redis 的地方
│   ├── model/             # 持久化实体
│   └── router/            # 路由注册
├── pkg/
│   ├── db/                # MySQL / Redis 客户端初始化
│   └── resp/              # 统一响应与业务错误码
├── deploy/
│   ├── k8s/               # ConfigMap / Secret 模板 / Deployment+Service
│   └── sql/               # 建表脚本
├── Dockerfile             # 多阶段构建，产出 ~20MB 非 root 镜像
└── Makefile               # 开发与部署入口
```

## 分层契约

依赖方向严格单向：`controller → logic → dao → model`。反向依赖出现即说明逻辑放错了位置。

| 层 | 可以做 | 不可以做 |
| --- | --- | --- |
| controller | 参数绑定、错误码翻译、写响应 | 写业务规则、拼 SQL、直接持有 `gorm.DB` |
| logic | 领域校验、编排多个 dao、装配 VO | 感知 gin、直接执行 SQL |
| dao | 使用 gorm / redis、返回领域错误 | 返回 HTTP 语义、做"该不该允许"的判断 |

`logic` 不依赖 gin 这一点是刻意设计的：单测可以直接构造 `context.Background()` 调用，
不必起 HTTP 服务、不必造 request/response。

## 配置与凭据

配置**只**来自环境变量，代码不读配置文件。K8s 下由两类对象通过 `envFrom` 注入：

| 来源 | 内容 | 举例 |
| --- | --- | --- |
| ConfigMap `yulin-config` | 泄露不造成直接损失的配置 | `MYSQL_HOST`、`MYSQL_DATABASE`、`REDIS_DB`、`LOG_LEVEL` |
| Secret `yulin-secret` | 泄露等同凭据泄露的值 | `MYSQL_PASSWORD`、`REDIS_PASSWORD` |

**Secret 真值不进仓库。** 仓库里只有 `01-secret.example.yaml` 模板，
`.gitignore` 已忽略 `deploy/k8s/secret.yaml` 与 `.env*`。创建方式见下文。

> 这两个来源的 key **不允许重名**。`envFrom` 同时挂载时 Secret 覆盖 ConfigMap，
> 重名会导致"改了 ConfigMap 但服务读到旧值"这类极难排查的问题。

### 完整配置项

| 变量 | 来源 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `APP_ENV` | ConfigMap | `dev` | `prod` 时启用 JSON 日志、Gin release 模式、强制校验 DB 密码 |
| `SERVER_PORT` | ConfigMap | `8080` | HTTP 监听端口 |
| `LOG_LEVEL` | ConfigMap | `info` | `debug`/`info`/`warn`/`error` |
| `SHUTDOWN_TIMEOUT` | ConfigMap | `15s` | 优雅退出等待上限 |
| `MYSQL_HOST` | ConfigMap | `127.0.0.1` | 集群内用 `mysql.infra.svc.cluster.local` |
| `MYSQL_PORT` | ConfigMap | `3306` | |
| `MYSQL_DATABASE` | ConfigMap | `yulin` | |
| `MYSQL_USER` | ConfigMap | `yulin` | 专用账号，非 root |
| `MYSQL_PASSWORD` | **Secret** | — | 生产环境缺失即启动失败 |
| `MYSQL_PARAMS` | ConfigMap | `charset=utf8mb4&parseTime=True&loc=Local&timeout=5s` | |
| `MYSQL_MAX_OPEN_CONNS` | ConfigMap | `20` | |
| `MYSQL_MAX_IDLE_CONNS` | ConfigMap | `10` | |
| `MYSQL_CONN_MAX_LIFETIME` | ConfigMap | `1h` | |
| `REDIS_ADDR` | ConfigMap | `127.0.0.1:6379` | |
| `REDIS_PASSWORD` | **Secret** | — | 共享实例，值同 `infra-secret` |
| `REDIS_DB` | ConfigMap | `1` | 避开其他业务占用的 DB 0 |
| `REDIS_KEY_PREFIX` | ConfigMap | `yulin:` | 由 dao 层显式拼接 |
| `REDIS_POOL_SIZE` | ConfigMap | `10` | |

## 本地开发

```bash
# 1. 把集群中间件映射到本地（两个终端各开一个）
kubectl port-forward -n infra svc/mysql 13306:3306
kubectl port-forward -n infra svc/redis 16379:6379

# 2. 导环境变量
cp .env.example .env   # 填好本地密码
set -a && . ./.env && set +a

# 3. 起服务
make run

# 4. 验证
curl localhost:8080/health
curl localhost:8080/ready
```

## 接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/health` | 存活探针。**不检查任何外部依赖** |
| GET | `/ready` | 就绪探针。检查 MySQL + Redis |
| GET | `/api/v1/alumni` | 校友列表，支持 `graduation_year`/`city`/`keyword`/`page`/`page_size` |
| GET | `/api/v1/alumni/:id` | 校友详情 |
| POST | `/api/v1/alumni` | 新增校友 |

> `/health` 与 `/ready` 语义必须分开。让 liveness 依赖数据库，等于把 MySQL 抖动
> 翻译成 Pod 批量重启，故障范围从"查不了数据"扩大成"整个服务不可用"。

## 部署

```bash
# 1. 建库表（库与账号已预先创建）
kubectl exec -i -n infra deploy/mysql -- \
  mysql -uyulin -p"$MYSQL_PASSWORD" yulin < deploy/sql/001_init.sql

# 2. 创建 Secret（密码不回显、不落地文件）
make secret

# 3. 构建镜像并导入 k3s containerd（在构建机上）
docker build -t yulin-server:latest .
docker save yulin-server:latest | ssh natvps 'ctr -n k8s.io images import -'

# 4. 应用清单
make deploy

# 5. 验证
kubectl get pods -n apps -l app=yulin-server
kubectl logs -f deploy/yulin-server -n apps
```

无私有 registry，因此 `imagePullPolicy: IfNotPresent` + `nodeSelector` 固定到导入镜像的节点。
后续接入 Gitea 的容器 registry 后，可以去掉 `nodeSelector` 并把镜像换成
`121.40.235.227:3000/gao/yulin-server:<tag>`，实现多节点与版本可追溯。

## 已知取舍

| 取舍 | 现状 | 理由 |
| --- | --- | --- |
| 中间件复用 `infra` 共享实例 | MySQL 用库 + 账号隔离，Redis 用 DB 号 + 键前缀隔离 | 集群内存紧张（NAT VPS 8C8G / worker 2C1.6G），再起一套 MySQL 会触发 OOM |
| Redis 共享口令 | `yulin-secret` 中存的是实例级密码 | 共享实例无法做到每应用独立口令；要真正隔离需独立 Redis 实例或 Redis ACL |
| 单副本 | `replicas: 1` | 骨架阶段优先可观测；水平扩展需先解决会话状态与迁移锁 |
| 无 CI | 手动 build + import | 下一步接 Gitea Actions，runner 已在 `infra/runner-0` 就绪 |
