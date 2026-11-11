# 核心 Web 功能与验收说明

## 页面

| 路径 | 功能 |
| --- | --- |
| `/` | 校园摄影首页、实时统计、四个核心模块、最近校友与照片 |
| `/register?identity=student` | 在读报到；不带参数时为校友登记；卡片实时预览 |
| `/alumni/:id` | 公开校友卡，复制链接，凭编辑码验证/修改本人资料 |
| `/directory` | 关键词、届别、班级和实际院校筛选、列表/班级视图、分页 |
| `/universities` | 实际大学榜单、比例条、校名搜索、通讯录反查 |
| `/moments` | 响应式照片墙、分类、上传/压缩、删除码、预览 |
| `/forum` | 发帖、单张配图、分页、正序单层回复 |
| `/privacy` | 基本资料公开说明、联系方式隐私与内容规则 |

页面由 Go 同源提供，支持直接访问和刷新详情路由，未知 API 不回退到 HTML。
前端采用 ES modules，`internal/web/static` 内的源码即发布资源，不需要 npm 安装或打包。

## 接口

统一格式：`{"code":0,"msg":"ok","data":{...}}`。错误使用 HTTP 400/403/404/409/429/500/503 和非零业务码。

| 方法 | 路径（均以 `/api/v1` 开头） | 说明 |
| --- | --- | --- |
| GET | `/meta`、`/stats` | 演示标识与公开统计 |
| GET | `/directory-meta`、`/classes` | 届别/总数、分页班级组 |
| GET | `/alumni` | `keyword`、`graduation_year`、`university`、`class_name`、`city`、`page`、`page_size` |
| GET | `/alumni/:id` | 公开字段白名单详情 |
| POST | `/alumni` | JSON 登记；返回 `id`、仅一次的 `edit_code` |
| POST | `/alumni/:id/unlock` | JSON `edit_code`；正确时返回本人可编辑字段，禁止缓存 |
| PUT | `/alumni/:id` | 完整登记字段和 `edit_code`；支持清空字段与关闭隐私开关 |
| GET | `/universities` | `items[{name,count}]`、实际就读统计人数 `total` |
| GET/POST | `/photos` | 分页/分类列表；multipart 上传 `image`、`caption`、`uploader`、`category`；创建返回 `delete_code` |
| GET | `/photos/:id/image` | 有效照片的二进制；撤下后立即 404，不缓存 |
| DELETE | `/photos/:id` | JSON `delete_code`；前端要求勾选确认，后台写审计 |
| GET/POST | `/posts` | 分页列表；multipart `nickname`、`graduation_year`、`content`、可选 `image` |
| GET | `/posts/:id/image` | 有效帖子配图 |
| GET/POST | `/posts/:id/replies` | 分页正序回复；POST JSON `nickname`、`content` |
| GET | `/admin/:kind/:id` | 管理员按 ID 查询，包括下架内容；仅返回管理摘要 |
| PUT | `/admin/:kind/:id` | `kind=alumni/photo/post/reply`；JSON `action=hidden/active`、必填 `reason` |
| GET | `/admin/audit` | 分页审计记录 |

分页默认 20，每页上限 100。班级名 `__unassigned__` 表示未填写班级。创建接口支持 `Idempotency-Key`，
前端每个表单使用随机请求标识，数据库通过唯一索引阻止重复写入。重复请求返回 409，不会再次返回原编辑码/删除码；
若创建响应丢失，需由维护者核验处理，尚未提供码找回能力。

## 数据与权限

- `alumni`、`photo`、`post`、`reply`、`audit_log` 均由 GORM 模型及显式迁移命令管理。原有资料保留。
- 在读生仅保存理想大学；实际大学聚合只选择 `identity=alumni`。切换身份是本人明确的编辑动作。
- 常见学校别名与全半角/空白标准化在写入前完成；榜单与大学反查使用同一标准名。
- 编辑码/删除码来自 192 位安全随机数，数据库只保存 SHA-256 哈希；校验使用常数时间比较。
- 手机号、微信、邮箱只在本人凭码验证后返回。尚未实现校友认证，因此普通访客均不能查看联系方式。
- 公开帖子与照片先发后审；管理员可下架/恢复，照片凭删除码撤下后不能通过普通审核恢复。
- 公开接口的帖子、回复、相册、人数均排除下架内容；回复创建事务锁定父帖，防止对已下架帖子新增回复。
- 浏览器及服务端均校验图片；最长边压缩至 2048px，重新编码 JPEG 移除元数据，单张压缩后上限 3MB。
- 上传原图上限 10MB；服务端额外限制解码像素数与并发，避免压缩炸弹。
- 普通写操作默认同 IP/路由 10 分钟 20 次；编辑码验证每分钟 10 次，Redis 不可用时拒绝写入。
- `TRUSTED_PROXIES` 只填写真实代理 IP/CIDR，默认不信任伪造转发头。正式部署应配置入口代理，否则所有请求可能按代理 IP 限流。
- `ADMIN_TOKEN` 至少 32 字符，从 Secret 注入，所有管理接口使用 `Authorization: Bearer <token>`；留空时禁用。
- GORM 日志不输出绑定参数，HTTP 不记录请求体。API 禁止缓存，模块和 CSS 使用内容版本路径，发布后自动更新缓存。

## 运行与部署

1. 备份现有 MySQL 数据库。
2. 为迁移进程注入与正式服务一致的 MySQL 环境变量，运行 `go run ./cmd/migrate`，或新镜像的 `/usr/local/bin/server migrate`。
3. 完成迁移后发布新服务。前端已编入二进制，标准 Dockerfile 与离线 Dockerfile 无需额外复制资源。
4. 检查 `/health`、`/ready`、首页，以及真实数据库下的登记、凭码修改、照片/帖子写入。

提交代码不会在本次会话中自动发布。仓库现有流水线推送 `master` 会部署；本次新增表结构要求先迁移，
因此发布前必须更新部署机的执行流程或按上述步骤先跑迁移，不能仅依赖旧服务的 `/ready` 判断业务表已齐全。
新服务在启动时检查核心表和字段，缺失时拒绝启动并提示迁移，避免旧表结构下对外提供半成品接口。

## 验收边界

`go test ./...` 包含业务规则测试、GORM SQL/事务测试（sqlmock）和 HTTP 路由/隐私/限流测试；
sqlmock 验证真实 GORM 生成的语句与事务编排，但不替代 MySQL 服务端集成。浏览器操作使用明确的本地演示模式。
本机未配置真实 MySQL/Redis，正式持久化、生产 Redis 和 HTTPS 入口需部署前独立联调。

仍需后续运营/产品配置：校友认证、验证码服务、完整内容审核台、个人资料删除申请联系渠道，以及学校授权摄影。
当前提供先发后审与管理 API，不声称已接入外部内容审核或验证码平台。
