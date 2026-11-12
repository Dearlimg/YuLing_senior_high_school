# 核心 Web 功能与验收说明

## 页面

| 路径 | 功能 |
| --- | --- |
| `/` | 校园摄影首页、实时统计、四个核心模块、最近校友与照片 |
| `/register?identity=student` | 在读报到；不带参数时为校友登记；卡片实时预览 |
| `/alumni/:id` | 公开校友卡、复制链接，入口连接个人中心 |
| `/login`、`/me` | 登录、旧卡绑定账号、个人中心编辑与退出 |
| `/directory` | 关键词、届别、班级和实际院校筛选、列表/班级视图、分页 |
| `/universities` | 实际大学榜单、比例条、校名搜索、通讯录反查 |
| `/moments` | 响应式照片墙、分类、登录上传/撤下、本机一键瘦身与预览 |
| `/forum` | 登录发帖/回复、单张配图、搜索、最新/热度排序、总榜/近 7 天榜 |
| `/privacy` | 基本资料公开说明、联系方式隐私与内容规则 |

页面由 Go 同源提供，支持直接访问和刷新详情路由，未知 API 不回退到 HTML。
前端采用 ES modules，`internal/web/static` 内的源码即发布资源，不需要 npm 安装或打包。

## 接口

统一格式：`{"code":0,"msg":"ok","data":{...}}`。错误使用 HTTP 400/401/403/404/409/413/429/500/503 和非零业务码。

| 方法 | 路径（均以 `/api/v1` 开头） | 说明 |
| --- | --- | --- |
| GET | `/meta`、`/stats` | 演示标识与公开统计 |
| GET | `/directory-meta`、`/classes` | 届别/总数、分页班级组 |
| GET | `/alumni` | `keyword`、`graduation_year`、`university`、`class_name`、`city`、`page`、`page_size` |
| GET | `/alumni/:id` | 公开字段白名单详情 |
| POST | `/alumni`、`/auth/register` | JSON 账号/密码及登记字段，事务创建账号和校友卡，返回 Token、到期时间、校友卡 ID；旧卡另传 `alumni_id`、`edit_code` |
| POST | `/auth/login`、`/auth/logout` | 账号密码登录 / 撤销当前 Bearer Token |
| GET/PUT | `/me`、`/me/alumni` | 登录获取本人资料 / 编辑自己的校友卡 |
| POST | `/alumni/:id/unlock` | JSON `edit_code`；仅用于尚未绑定账号的旧卡；绑定后编辑码失效 |
| PUT | `/alumni/:id` | 仅兼容未绑定账号的旧卡；新卡使用 `/me/alumni` |
| GET | `/universities` | `items[{name,count}]`、实际就读统计人数 `total` |
| GET/POST | `/photos` | 分页/分类列表；登录 multipart 上传 `image`、`caption`、`category`；上传人从当前校友卡获取 |
| GET | `/photos/:id/image` | 有效照片的二进制；撤下后立即 404，不缓存 |
| DELETE | `/photos/:id` | 登录后撤下自己的照片；旧照片另需 JSON `delete_code`；后台写审计 |
| GET/POST | `/posts` | 分页/搜索/排序；登录后 multipart `content`、可选 `image`，署名与届别从校友卡获取 |
| GET | `/posts/ranking` | `period=all/week`，有效回复数降序的前 10 名，Redis Sorted Set 加速 |
| GET | `/posts/:id/image` | 有效帖子配图 |
| GET/POST | `/posts/:id/replies` | 分页正序回复；登录 POST JSON `content`，署名从校友卡获取 |
| GET | `/admin/:kind/:id` | 管理员按 ID 查询，包括下架内容；仅返回管理摘要 |
| PUT | `/admin/:kind/:id` | `kind=alumni/photo/post/reply`；JSON `action=hidden/active`、必填 `reason` |
| GET | `/admin/audit` | 分页审计记录 |

分页默认 20，每页上限 100。班级名 `__unassigned__` 表示未填写班级。创建接口支持 `Idempotency-Key`，
前端每个表单使用随机请求标识，数据库通过唯一索引阻止重复写入。重复请求返回 409，不会重复创建；注册响应丢失时可以使用已创建的账号密码登录。旧管理码尚未提供自动找回能力。

## 数据与权限

- `app_user`、`alumni`、`photo`、`post`、`reply`、`audit_log` 均由 GORM 模型及显式迁移命令管理。原有资料保留。
- 在读生仅保存理想大学；实际大学聚合只选择 `identity=alumni`。切换身份是本人明确的编辑动作。
- 常见学校别名与全半角/空白标准化在写入前完成；榜单与大学反查使用同一标准名。
- 密码保存 bcrypt 哈希，随机 Bearer Token 的 SHA-256 哈希在 Redis 保存 7 天，退出立即撤销。
- 旧编辑码/删除码来自 192 位安全随机数，数据库只保存 SHA-256 哈希；校验使用常数时间比较。
- 手机号、微信、邮箱仅在本人登录或未绑定旧卡凭码验证后返回。尚未实现校友认证，因此普通访客均不能查看联系方式。
- 公开帖子与照片先发后审；管理员可下架/恢复，照片凭删除码撤下后不能通过普通审核恢复。
- 公开接口的帖子、回复、相册、人数均排除下架内容；回复创建事务锁定父帖，防止对已下架帖子新增回复。
- 浏览器及服务端均校验图片；最长边压缩至 2048px，重新编码 JPEG 移除元数据，单张入库上限 512KB。
- 上传图片上限 2MB，原图可在浏览器瘦身（最多 25MB / 4000 万像素）；服务端限制解码为 1600 万像素及并发，避免压缩炸弹。
- 普通写操作默认同 IP/路由 10 分钟 20 次；登录/注册与编辑码验证每分钟 10 次，Redis 不可用时拒绝写入。
- `TRUSTED_PROXIES` 只填写真实代理 IP/CIDR，默认不信任伪造转发头。正式部署应配置入口代理，否则所有请求可能按代理 IP 限流。
- `ADMIN_TOKEN` 至少 32 字符，从 Secret 注入，所有管理接口使用 `Authorization: Bearer <token>`；留空时禁用。
- GORM 日志不输出绑定参数，HTTP 不记录请求体。公开列表在服务端 Redis 缓存 60～80 秒，写成功更换版本；账号/本人数据/图片不缓存，HTTP API 禁止浏览器缓存。模块和 CSS 使用内容版本路径，发布后自动更新缓存。

## 运行与部署

1. 备份现有 MySQL 数据库。
2. 为迁移进程注入与正式服务一致的 MySQL 环境变量，运行 `go run ./cmd/migrate`，或新镜像的 `/usr/local/bin/server migrate`。
3. 完成迁移后发布新服务。前端已编入二进制，标准 Dockerfile 与离线 Dockerfile 无需额外复制资源。
4. 检查 `/health`、`/ready`、首页，以及真实数据库下的注册、登录、旧卡绑定、本人修改、照片/帖子写入与榜单缓存。

提交代码不会在本次会话中自动发布。仓库现有流水线推送 `master` 会部署；本次新增表结构要求先迁移，
因此发布前必须更新部署机的执行流程或按上述步骤先跑迁移，不能仅依赖旧服务的 `/ready` 判断业务表已齐全。
新服务在启动时检查核心表和字段，缺失时拒绝启动并提示迁移，避免旧表结构下对外提供半成品接口。

## 验收边界

`go test ./...` 包含业务规则测试、GORM SQL/事务测试（sqlmock）和 HTTP 路由/隐私/限流测试；
sqlmock 验证真实 GORM 生成的语句与事务编排，但不替代 MySQL 服务端集成。浏览器操作使用明确的本地演示模式。
本机未配置真实 MySQL/Redis，正式持久化、生产 Redis 和 HTTPS 入口需部署前独立联调。

仍需后续运营/产品配置：校友认证、验证码服务、完整内容审核台、个人资料删除申请联系渠道，以及学校授权摄影。
当前提供先发后审与管理 API，不声称已接入外部内容审核或验证码平台。


## 帖子卡片流与详情

校友贴吧采用响应式瀑布流；点击卡片或热议榜打开图文详情，保留搜索、排序和分页。
详情公开浏览，登录后可评论、点赞及取消点赞。点赞使用 `PUT /api/v1/posts/:id/likes`
和 `{ "liked": true|false }`，按用户与帖子唯一且可重试；`GET` 同路径读取本人状态。
`GET /api/v1/posts/:id` 返回公开详情及互动计数。直接访问 `/forum?post=编号` 可打开对应详情。
上线前须执行 `go run ./cmd/migrate` 创建 `content_like`；启动会检查表结构。


## 回复任意评论

发评论时可传 `parent_id`（省略或 0 表示直接评论帖子）。列表返回 `parent_id`、
`parent_nickname`；每条评论均提供回复按钮和取消回复入口。回复目标可为任意层级评论，
必须在同一帖子且仍可见，事务内校验，禁止跨帖回复。目标隐藏后保留后续讨论，
但不公开被隐藏目标的署名。上线前迁移 `reply.parent_id` 字段。
