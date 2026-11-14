# 真实访客统计与隐藏看板 · 设计文档

| 项目 | 内容 |
| --- | --- |
| 日期 | 2026-11-14 |
| 状态 | 已评审，进入实现 |
| 关联 | PRD 第 10 章（数据埋点）；2026-11-14 访问统计排查（191 IP 中 ~180 为扫描器） |

## 1. 背景

运营需要知道"今天多少真人访问了校友之家"。2026-11-14 用入口 nginx 日志事后统计发现：应用日志无客户端 IP，独立 IP 的 95% 是全网扫描器（Censys/zgrab/伪造 UA），真实访客只能靠人工逐条甄别，无法形成日常可用的指标。

## 2. 现状

- 链路：公网 → aliyun nginx:8000（已传 `X-Real-IP`/`X-Forwarded-For`）→ SSH 隧道 → k3s NodePort → yulin-server。
- 已有 `growth_event` 体系：前端 `growth.js` 维护匿名浏览器 ID（localStorage `yulin-visitor`，90 天），`track()` 上报白名单增长事件；管理端 `/admin?view=growth` 有增长看板，`adminGuard` 支持 `Authorization: Bearer ADMIN_TOKEN`。
- `requestLogging` 中间件只记 method/status/duration，不含 IP。
- Redis（DB1，`yulin:` 前缀）与 MySQL 均在线；demo 模式无 Redis/MySQL，走 MemoryStore。

## 3. 目标

1. 准确记录**真实访客** PV / UV：以浏览器主动上报为准，扫描器不执行 JS，天然排除。
2. 保留**全量流量底账**（含爬虫）：中间件侧 Redis 计数，用于对照 bot 占比与风控参考。
3. 提供**隐藏看板页**：独立路由、不挂任何导航/页脚入口、密码（ADMIN_TOKEN）访问，刷新后需重新输入。
4. 隐私合规：不落明文 IP / UA / 完整 referer；只存哈希与设备分类。

非目标：不做事件级自定义埋点平台、不做地域解析、不做实时秒级大屏。

## 4. 差值

| 能力 | 现状 | 目标 |
| --- | --- | --- |
| 页面浏览采集 | 无 | SPA 每次路由渲染上报 pageview |
| UV 口径 | 无 | 主口径 = 匿名浏览器 ID；对照口径 = IP+UA 哈希 |
| 流量底账 | 仅 nginx 日志 | Redis 按天计数（请求数 + IP 集合） |
| 数据展示 | 无 | 隐藏看板：今日/累计/14 日趋势/Top 页面/明细 |
| 访问控制 | 已有 adminGuard | 复用，看板 API 挂 `/api/v1/admin/visitors` |

## 5. 方案

### 5.1 口径定义

- **PV**：一次页面渲染 = 1 次 pageview 上报（SPA 内切换计数，刷新计数）。
- **UV（主口径）**：当日 `COUNT(DISTINCT visitor_hash)`。同一浏览器 90 天内为同一访客；清缓存/换浏览器算新访客，属可接受误差。
- **独立 IP（对照口径）**：当日 `COUNT(DISTINCT ip_hash)`，`ip_hash = sha256(ClientIP + "|" + UA)`，无盐哈希（脱敏且可跨天对照）。
- **底账**：中间件对每个 HTTP 请求计数，IP 集合按天留存 72 小时后过期。

### 5.2 数据模型

新表 `visit_event`（GORM AutoMigrate，跟随现有迁移流程）：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | uint64 PK | |
| event_key | char(64) unique | 前端 requestKey()，幂等防重 |
| visitor_hash | char(64) index | sha256(匿名浏览器 ID) |
| ip_hash | char(64) | sha256(IP+UA)，不落明文 |
| path | varchar(128) index | 仅 pathname，不含 query（防敏感参数） |
| source | varchar(48) | direct / 外部 referer 主域 / 内部导航 |
| device | varchar(16) | mobile / desktop（UA 粗分类，不存原文） |
| created_at | datetime index | |

Redis 键（带 `yulin:` 前缀，TTL 72h）：
- `visit:req:YYYYMMDD`：全量请求计数（INCR）
- `visit:ip:YYYYMMDD`：全量独立 IP 哈希集合（SADD）

### 5.3 接口

| 接口 | 鉴权 | 说明 |
| --- | --- | --- |
| POST `/api/v1/track/pageview` | 公开 + writeGuard 独立宽松桶（120 次/10min，同 growth/events） | body：event_key、visitor_id、path、source；服务端补 ip_hash/device/时间 |
| GET `/api/v1/admin/visitors` | adminGuard（Bearer ADMIN_TOKEN 或管理员会话） | 返回今日/昨日/累计 PV UV、14 日趋势、7 日 Top 页面、最近 30 条明细、当日底账对照 |

校验：event_key/visitor_id 限 `[a-zA-Z0-9_-]{1,64}`；path 必须 `/` 开头、≤128 字符、不含 `?`；source 白名单化（direct/internal/合法域名，否则归 other）。上报失败前端静默，不影响浏览。

### 5.4 前端

- `app.js render()` 成功后上报 pageview（`/ops-board` 自身不上报，避免看板自污染）；复用 `growth.js` 的 `visitorID()`。
- 新页面 `/ops-board`：不注册到导航、页脚、`data-nav`；`web.go` 的 isPage 白名单加入该路径保证刷新可恢复。
- 看板密码门：与 `/admin` 的 bootstrap-gate 同模式——输入 ADMIN_TOKEN，仅存活于 JS 内存，刷新重输；错误 token 显示 401 提示。
- 趋势图用纯 HTML/CSS 条形图（CSP `script-src 'self'` 不允许外链图表库，不引依赖）。

### 5.5 关键取舍

1. **不做每日批处理**：当前量级（日 PV 百级）直接 MySQL 实时写入 + SQL 聚合足够，省掉定时任务与一致性问题；量大了再加物化日表。
2. **beacon 为主、中间件为辅**：真人 UV 只信 beacon；中间件底账含爬虫，仅作对照展示，不混入主指标。
3. **复用 adminGuard 而非新密码体系**：看板密码就是 ADMIN_TOKEN，不引入新的 Secret 与配置面。
4. **隐藏 ≠ 安全**：路由不公开只是减少曝光，真正的访问控制由 adminGuard 密码校验承担。

## 6. 验收

| 场景 | 通过标准 |
| --- | --- |
| 正常浏览 | 打开首页→切换通讯录→看板今日 PV+2、UV 不变 |
| 幂等 | 同一 event_key 重复上报不产生重复行 |
| 鉴权 | 无 token / 错误 token 访问看板 API 返回 401；正确 token 返回数据 |
| 隐藏性 | 导航/页脚无 `/ops-board` 链接；直接访问该路由需密码 |
| 隐私 | visit_event 全表无明文 IP/UA；path 不含 query |
| 底账 | curl 发起请求（无 JS）只增加底账计数，不增加 UV |
| demo 模式 | 无 Redis/MySQL 时服务正常启动，统计降级不报错 |
