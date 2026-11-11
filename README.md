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

## 对外访问链路

公网入口**不在 k8s 内**，而在阿里云 worker 节点的宿主机 nginx 上。完整链路：

```
121.40.235.227:8000
  → 宿主机 nginx (listen 8000, /etc/nginx/sites-available/entry.conf)
  → proxy_pass 127.0.0.1:18043      （SSH 隧道，已在 aliyun 上常驻）
  → NAT VPS 的 k3s NodePort 30443
  → Service yulin-server:8080
  → Pod
```

这个结构不是自愿的：NodePort 30443 在公网**不可达**（云安全组未放行），
只能靠 aliyun → NAT VPS 的 SSH 隧道穿进去。所以对外端口实际是云主机上的 **8000**，
而"30443"只是隧道内的一跳。

验证方式：

```bash
curl http://121.40.235.227:8000/health
curl http://121.40.235.227:8000/ready
```

> 如果本机设置了 `HTTP_PROXY`，测试 POST 时可能被代理卡住（GET 正常、POST 超时），
> 加 `--noproxy '*'` 排除干扰。这个坑排查时会误判成服务端问题。

## 原 im-toy 服务的处置（2026-11-11）

im-toy 曾占用 `8000 → 30443` 这条链路，现由本项目接管。处置方式是**缩容而非删除**：

| 对象 | 状态 | 回滚方式 |
| --- | --- | --- |
| `deploy/im-toy-backend` | 缩容到 0 副本，对象保留 | `kubectl scale deploy/im-toy-backend -n apps --replicas=1` |
| `deploy/im-toy-nginx` | 缩容到 0 副本，对象保留 | 同上，并重新 apply 备份的 Service |
| `svc/im-toy-nginx` | **已删除**（释放 30443 端口） | `kubectl apply -f deploy/backup/im-toy-svc-im-toy-nginx.yaml` |
| PVC `im-toy-public` | 保留 | 未动 |
| MySQL 库 `im_chat` | 保留 | 未动 |

回滚完整步骤：先 apply 备份 Service 会与 yulin 的 30443 冲突，
必须先把 yulin 的 Service 改回 ClusterIP，顺序不能反。
所有原始清单快照在 `deploy/backup/`。

## CI/CD

推送到 `master` 自动部署，配置见 `.gitea/workflows/deploy.yaml`，实际工作在
`/root/deploy-yulin-remote.sh`（部署机上）。

```
git push master
  → Gitea Actions runner（k3s 内 runner-0）
  → SSH 到 NAT VPS
  → git clone 走 127.0.0.1:30300（Gitea NodePort，绕开 dind 网络）
  → deploy-yulin-remote.sh：
      1. go vet + go test          静态检查与单测门禁
      2. 交叉编译 linux/amd64 静态二进制
      3. docker build（sha tag + latest）
      4. ctr import 到 k3s containerd
      5. mysqldump 备份 yulin 库
      6. kubectl set image + rollout status（超时自动回滚）
      7. 公网 /health 与 /ready 验证（失败自动回滚）
```

### 为什么不直接用多阶段 Dockerfile

部署机的 `docker` 拉不到 `docker.io`（DNS 被污染，解析到无效 IP），
`golang:1.26-alpine` 永远拉不下来。所以编译放在宿主机（有 go1.26.1），
镜像阶段只 `COPY` 产物 —— 即 `deploy/Dockerfile.prebuilt`。

### 与 blog-v3 流水线的三处差异

| 项 | blog-v3 | 本项目 | 原因 |
| --- | --- | --- | --- |
| 镜像 tag | `latest` | git 短 sha + `latest` | `latest` + `IfNotPresent` 会复用旧 digest，表现为“部署成功但代码没变” |
| 发布失败 | 需人工介入 | 自动 `rollout undo` | 无人值守场景下，失败必须自己退回去 |
| 数据库备份 | root 账号 | 项目专用账号，从 Secret 现取 | 备份不需要 root 权限，脚本里也不该出现密码 |

### 需要的 secrets

| 名称 | 用途 | 配置方式 |
| --- | --- | --- |
| `DEPLOY_SSH_KEY` | SSH 到部署机的私钥（对应 `yulin-ci-deploy` 公钥） | 仓库 Settings → Actions → Secrets |
| `GITHUB_TOKEN` | clone 私有仓库 | Gitea 内置，无需配置 |

部署密钥是**每项目独立**的（`blog-ci-deploy`、`im-toy-ci-deploy`、`yulin-ci-deploy` 各自一对），
任何一方泄露可以单独从 `natvps:/root/.ssh/authorized_keys` 吊销，不影响其他项目。

### 手动操作

```bash
# 手动触发一次部署（不改代码）
#   Gitea 仓库页 → Actions → deploy-yulin → Run workflow

# 本地直接跑远程脚本（排障用，跳过 runner）
ssh natvps "bash /root/deploy-yulin-remote.sh /root/workspace/yulin-src"

# 回滚到上一版本
kubectl rollout undo deploy/yulin-server -n apps

# 查看某次部署对应的镜像
kubectl get deploy yulin-server -n apps -o jsonpath='{.spec.template.spec.containers[0].image}'
```

## 已知取舍

| 取舍 | 现状 | 理由 |
| --- | --- | --- |
| 中间件复用 `infra` 共享实例 | MySQL 用库 + 账号隔离，Redis 用 DB 号 + 键前缀隔离 | 集群内存紧张（NAT VPS 8C8G / worker 2C1.6G），再起一套 MySQL 会触发 OOM |
| Redis 共享口令 | `yulin-secret` 中存的是实例级密码 | 共享实例无法做到每应用独立口令；要真正隔离需独立 Redis 实例或 Redis ACL |
| 单副本 | `replicas: 1` | 骨架阶段优先可观测；水平扩展需先解决会话状态与迁移锁 |
| 无 CI | 手动 build + import | 下一步接 Gitea Actions，runner 已在 `infra/runner-0` 就绪 |
