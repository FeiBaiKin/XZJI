> **历史文档（v1 自研方案）**：本文描述的是已封存的 Go + Nuxt 方案，2026-10-05 起不再维护，原样保留以备考。
> 现行方案见 [../wp-theme-plan.md](../wp-theme-plan.md)；v1 代码见 `archive/v1` 分支与 `v1.0-standalone` 标签。

# M5 SEO 与性能：开发说明

> 日期：2026-10-04，M4 完成后编写
> 范围以 `docs/phase1-tech-plan.md` 第 3.6、3.7、9、11 节为准，本文是拆解后的执行说明。两者冲突时以总文档为准，并在总文档里更正。

## 1. 目标与验收

M5 的验收标准：**达到总文档第 9 节的全部指标，并把实测结果补充进第 9 节**。

| 指标 | 目标 |
|---|---|
| 后端常驻内存 | < 100MB（不含 MySQL） |
| 前台 Nuxt 常驻内存 | < 200MB |
| 命中页面缓存时的前台响应 | < 10ms |
| 命中缓存时的后端公开接口 | < 5ms |
| 未命中缓存时的详情页服务端渲染 | < 100ms（前后端同机） |
| 首屏 JS（gzip 后） | < 100KB |
| Lighthouse 移动端性能评分 | ≥ 90 |

要做的六件事：浏览量上报、后端公开接口缓存、页面缓存和清除通知、sitemap 与 robots、结构化数据、图片优化；最后灌 1 万篇测试数据压测并跑 Lighthouse。

## 2. 开工前必读

**项目规则**（完整版见根目录 `CLAUDE.md`，以下是本次最容易踩的）：

- 新增或修改接口：先改 `api/*.yaml`，再 `task gen`，最后写实现。`internal/apigen`、`internal/dbgen`、`packages/api-client/src/gen` 是生成代码，禁止手改。
- 表结构只能新增迁移文件（下一个是 `00006_*.sql`），已发布的迁移不能改。SQL 写在 `server/db/queries/*.sql` 由 sqlc 生成。
- 服务端渲染（SSR）调用接口时不带用户身份；付费内容、下载地址永远不能出现在 HTML 里。页面缓存正是建立在这一点上，任何改动都不能破坏它。
- 业务模块之间只通过 service 调用；handler 只做参数转换。
- 注释、提交信息、文档用中文，技术名词保持原文。注释只说用途和不显而易见的原因。
- 提交前运行 `task test`；改过契约或 SQL 再运行 `task check:gen`。

**本机环境**：

- 三端：后端 `task dev:server`（8080）、前台 `task dev:web`（3000）、后台 `task dev:admin`（5173）。Windows 上开发服务器要绑定 `127.0.0.1`，Node 会把 `localhost` 解析成 `::1`。
- 集成测试需要 `server/.env.test` 里的 `XZJI_TEST_DB_DSN`。用 `task test:server` 跑；手动跑时用 `export XZJI_TEST_DB_DSN="$(grep '^XZJI_TEST_DB_DSN=' .env.test | cut -d= -f2-)"`，不要 `source .env.test`（DSN 里的括号会报错，还会把密码打到输出里）。
- 测试账号和密码在 `server/.env.test`，不要写进仓库或对话。
- 开发库 `xzji_dev` 有 15 篇演示内容；`config.yaml` 开着模拟支付（`pay.mock: true`）。

**已知的坑**：

- 组件里不要在 setup 阶段或 `watch(..., { immediate: true })` 里调用 `window`、`addEventListener` 等浏览器 API，SSR 会直接报错（M4 的支付弹窗踩过）。浏览器端逻辑放进 `onMounted`。
- Tailwind 里 `[&_td]:text-xxx` 这种父级任意变体的优先级高于单元格自己的颜色类；给表格设默认色请放在 `<tbody class="...">` 上。
- `posts.updated_at` 带 `ON UPDATE CURRENT_TIMESTAMP`。任何只改计数的 UPDATE 都必须写 `updated_at = updated_at`，否则“最近更新”和 sitemap 的 lastmod 会乱跳（见 4.2）。

## 3. 现状

| 功能 | 现状 |
|---|---|
| SSR 不带身份 | 已做到：`web/app/composables/useApi.ts` 服务端走 `apiInternalBase`、不带 Cookie |
| 推广 Cookie | 已只在浏览器端写（`web/app/middleware/ref.global.ts`），不会让 SSR 响应带 Set-Cookie |
| 前台详情接口 | `GetPost` 只返回已发布内容，与身份无关，可以缓存 |
| 前台 `web/server/` | 不存在，sitemap、robots、清除接口都要新建 |
| 后端缓存 | 没有，`internal/platform/` 下没有 cache 包 |
| 浏览量上报 | 契约里没有 `/posts/{id}/view` |
| 图片缩略图 | `media.variants` 列已在 00003 迁移里，从未写入 |
| canonical、JSON-LD | 都没有 |
| 前台站点地址配置 | 没有（只有接口地址），sitemap 和 canonical 需要 |

## 4. 任务拆解（按顺序做）

### 4.1 前台站点地址与 canonical

- `web/nuxt.config.ts` 的 `runtimeConfig.public` 增加 `siteUrl`（环境变量 `NUXT_PUBLIC_SITE_URL`，开发默认 `http://localhost:3000`，末尾不带 `/`）；`deploy/systemd/xzji-web.service` 补上这个环境变量。
- 在 `web/app/app.vue` 或默认布局里统一输出 `<link rel="canonical">` 和 `og:url`：`siteUrl + route.path`，**不带查询参数**（`?ref=` 推广参数不能进 canonical）。分页用的是路径（`/category/x/page/2/`），canonical 指向自身。
- 验收：查看首页、详情、分类第 2 页的网页源代码，canonical 都是绝对地址且没有查询参数。

### 4.2 浏览量上报（总文档 3.6 第 5 条）

- **先修 M3 遗留**：`server/db/queries/access.sql` 的 `SetPostCommentCount` 改为 `UPDATE posts SET comment_count = ?, updated_at = updated_at WHERE id = ?`。
- 契约：`api/public.yaml` 增加 `POST /posts/{id}/view`，operationId `recordPostView`，不需要登录，成功返回 204。
- SQL：`posts.sql` 增加 `AddPostViews`：`UPDATE posts SET view_count = view_count + ?, updated_at = updated_at WHERE id = ?`。
- 后端放在 `content` 模块（posts 表归它）：
  - 内存里按内容 ID 累加（`map[int64]int64` + 互斥锁）；同一 IP 对同一内容 30 分钟内只计一次，用现有的 `ratelimit.Limiter` 做去重。
  - 每分钟批量写库：交换出当前的 map 再逐条执行 `AddPostViews`；写库失败的计数放回去下次重试。
  - 在 `server/internal/app/app.go` 的 `Serve` 里用 `scheduler.Every` 注册；**进程退出前再刷一次**（ctx 取消后、`srv.Shutdown` 之后调用 Flush）。
- 前台：`web/app/pages/post.vue` 在 `onMounted` 里调用一次（浏览器端请求，命中页面缓存时 SSR 不执行，所以只能在这里统计）。
- 浏览量变化**不触发**缓存清除，靠 10 分钟兜底过期更新显示。
- 测试：单元测试覆盖累加、IP 去重、Flush 后清零、写库失败放回；集成测试覆盖上报后 Flush 一次，`view_count` 增加且 `updated_at` 不变。

### 4.3 后端公开接口缓存（总文档 3.7）

- 新建 `server/internal/platform/cache`：定义接口（Get / Set / Purge 全部），先实现进程内 LRU + TTL（容量约 2000 条、TTL 10 分钟，可放配置）。接口抽象是为以后换 Redis（总文档第 14 节）。
- 用 Gin 中间件在 `/api/v1` 上缓存，只缓存同时满足以下条件的请求：
  1. GET 请求，路径在白名单内：`/site/home`、`/categories`、`/posts`、`/posts/{id}`、`/posts/{id}/related`、`/tags`、`/tags/{slug}`、`/platforms`、`/membership/levels`、`/membership/plans`，以及 4.5 新增的 sitemap 数据接口；
  2. **匿名请求**：没有前台会话 Cookie（`xzji_session`），也没有 `Authorization` 头。SSR 请求本来就是匿名的，这样既能命中，又从根上排除了个人数据被缓存的可能；
  3. 响应状态码为 200。
- 缓存键用完整的路径加查询串。命中时直接写回缓存的状态码、Content-Type 和响应体。
- **禁止缓存**：`/posts/{id}/unlocked`、`/posts/{id}/comments`（登录用户能看到自己待审核的评论）、`/auth/*`、`/me/*`、`/orders*`、`/pay/*` 以及所有写请求。
- 测试：匿名命中、带 Cookie 不命中、带 Bearer 不命中、非白名单不缓存、非 200 不缓存、Purge 后失效。

### 4.4 清除通知与前台页面缓存（总文档 3.7、8）

**后端 `frontsync` 模块**（总文档 3.3 已规划）：

- 配置（`server/internal/config`，同步更新 `config.example.yaml` 和 `deploy/config.prod.example.yaml`）：
  - `frontend.purge_urls`：前台服务器地址列表，如 `[http://127.0.0.1:3000]`；
  - `frontend.purge_secret`：HMAC 密钥，生产必须非空（加到 `validate()`）。
- `Purge()` 依次执行：**先清后端缓存，再通知各前台服务器**。顺序不能反，否则前台会用后端的旧数据重新渲染并缓存。
- 通知：对每个地址 `POST {url}/_internal/purge`，请求头 `X-Purge-Timestamp`（Unix 秒）和 `X-Purge-Signature`（`hex(HMAC-SHA256(secret, timestamp))`）。异步发送、带超时，失败只记日志（前台有 10 分钟兜底）。1 秒内的多次 Purge 合并成一次。
- 触发时机：
  - **后台任意写请求成功后**清整站缓存：在 `server/internal/httpapi/router.go` 给 `/admin-api/v1` 加一个中间件，非 GET 且响应状态码小于 400 时调用 `Purge()`（`/auth/*`、`/me/*` 除外）。这样内容、分类、标签、首页设置、评论审核、会员等级与套餐都覆盖到了，不用逐个 handler 加；
  - 前台发表评论且直接审核通过时（评论数变化）；
  - 定时发布：每分钟检查是否有 `published_at` 刚到期（落在上次检查到现在之间）的内容，有就 Purge。

**前台**：

- `web/server/routes/_internal/purge.post.ts`：校验时间戳在 ±5 分钟内、签名用常量时间比较，通过后清空 Nitro 的页面缓存存储，返回 204；否则 401。密钥用环境变量 `NUXT_PURGE_SECRET`（`runtimeConfig.purgeSecret`，不能放进 `public`），`deploy/systemd/xzji-web.service` 补上。
- `web/nuxt.config.ts` 的 `routeRules` 给公开页面开路由级缓存（Nitro `cache`，`maxAge: 600`）：`/`、`/{id}.html`、`/category/**`、`/tag/**`、`/vip`；`/search` 用短缓存（如 60 秒）。`/user/**`、`/login`、`/forgot`、`/pay/**` 保持 `ssr: false` 不缓存。
- 必须确认：只缓存 200 响应（404、500 不缓存）；缓存的响应里没有 `Set-Cookie`；缓存键区分查询串（搜索页）。用 `curl -I` 连续请求两次对比响应头验证。
- Nginx：`deploy/nginx/split-front.conf` 和 `single.conf` 给 `/_internal/` 加访问限制，只允许后端服务器的地址访问（分机部署时）。
- 测试：后端 HMAC 签名的单元测试；前台用 curl 验证签名错误返回 401、过期时间戳返回 401、正确签名返回 204 且下一次请求重新渲染。

### 4.5 sitemap.xml 与 robots.txt

- 契约：`api/public.yaml` 增加 `GET /seo/sitemap`，返回已发布内容的 `{id, updatedAt}`、全部分类的 `slug`、有内容的标签的 `slug`。数据由后端新建的 `seo` 模块组织（总文档 3.3 已规划），内容数据通过 content 的 service 取，不直接查别的模块的表。1 万篇量级一次返回即可；超过 5 万条 URL 时再拆分页和 sitemap 索引（先在代码里留注释说明上限）。
- 前台 `web/server/routes/sitemap.xml.ts`：用 `siteUrl` 拼绝对地址，输出首页、`/vip`、所有分类、标签、内容详情；内容带 `<lastmod>`。同样走 10 分钟缓存。
- 前台 `web/server/routes/robots.txt.ts`：`Disallow: /user/`、`/login`、`/forgot`、`/pay/`；最后一行 `Sitemap: {siteUrl}/sitemap.xml`。
- 验收：sitemap 能通过 XML 校验，里面没有草稿、回收站和定时未发布的内容。

### 4.6 结构化数据（JSON-LD）

用 `useHead` 输出 `<script type="application/ld+json">`：

- 首页：`WebSite`，带 `potentialAction` 的 `SearchAction`（`{siteUrl}/search?q={search_term_string}`）。
- 详情页：文章用 `Article`（headline、datePublished、dateModified、author、image）；资源用 `SoftwareApplication`（name、softwareVersion、operatingSystem 取平台、applicationCategory、有价格时带 `offers`，价格单位换算成元）。两种都再加一份 `BreadcrumbList`，和页面面包屑一致。
- 分类、标签页：`BreadcrumbList`。
- **只能用公开字段**，不能把隐藏块内容放进去。
- 验收：用 Google 富媒体搜索测试或 schema.org 校验器检查源代码里的 JSON-LD 无错误。

### 4.7 图片优化

- 上传时生成缩略图并写入 `media.variants`（JSON，记录各尺寸的路径和宽高）：封面生成 480、960 宽两档，软件图标生成 96、192 两档；原图宽度不够时不放大。
- **编码格式需要先问用户**（见第 6 节）。约束：后端要保持 `CGO_ENABLED=0` 单文件部署，所以不能用依赖 libwebp 的 cgo 库。
- 已有图片要能补生成：在 `server/cmd/xzji` 增加子命令，如 `xzji media rebuild-variants`。
- 接口：`api/public.yaml` 的 `Image` 增加可选的 `variants`（向后兼容）；前台 `PostCover`、`AppTile`、`HomeBanner` 用 `srcset` + `sizes` 输出。首页第一张横幅保留 `loading="eager"` 并加 `fetchpriority="high"`，其余图片保持懒加载、带宽高（避免布局偏移）。

### 4.8 测试数据、压测与 Lighthouse

- **测试数据**：写一个只在开发环境用的灌数据工具，放在 `server/tools/seed/`（不进正式程序），向 `xzji_dev` 写入 1 万篇内容（文章、资源混合，带分类、标签、隐藏块、下载地址）。工具开头必须检查库名不是生产库。
- **压测要用生产构建**：前台 `task build:web` 后 `node web/.output/server/index.mjs`，后端用 `task build:server` 的产物。开发模式的数据没有意义。
- **工具**：`go install github.com/rakyll/hey@latest`。示例：
  - 前台命中缓存：先访问一次预热，再 `hey -n 5000 -c 50 http://127.0.0.1:3000/`；
  - 后端命中缓存：`hey -n 5000 -c 50 http://127.0.0.1:8080/api/v1/posts?pageSize=20`；
  - 详情页未命中缓存：Purge 后用不同的内容 ID 依次请求，统计平均耗时。
- **内存**：压测期间和压测后记录两个进程的常驻内存（Windows 用任务管理器或 `Get-Process` 的 WorkingSet；Linux 用 `ps -o rss`）。
- **首屏 JS**：统计首页实际加载的 `_nuxt/*.js` gzip 后的总大小（浏览器开发者工具网络面板，或 `nuxi analyze`）。超标时优先排查是否把后台用的依赖、`uqr` 等只在弹窗里用的库打进了首屏，必要时改成异步组件。
- **Lighthouse**：移动端、生产构建下跑首页和一个资源详情页，各跑 3 次取中位数。
- **把实测数据、测试环境（CPU、内存、数据量）、测试命令写进总文档第 9 节**，没达标的项写明原因和优化措施。

## 5. 测试要求

- 新增的纯逻辑（HMAC 签名校验、缓存中间件的命中规则、浏览量累加与去重）都要有表驱动单元测试。
- 集成测试（`server/internal/app/`，参照 `trade_integration_test.go` 的写法）至少覆盖：后台写操作触发 Purge（注入一个记录调用的假 Purger）、匿名请求命中缓存而带 Cookie 的不命中、浏览量 Flush 后入库且 `updated_at` 不变、sitemap 不含未发布内容。
- 前台改动跑 `pnpm -r typecheck`；页面效果用浏览器实际打开验证（用户偏好本机 Chrome 浏览器 MCP，连不上时先问用户再改用其他浏览器）。

## 6. 需要先问用户的决定

开始对应任务前先问，不要自行决定：

1. **缩略图格式**：
   - 方案 A：用 `golang.org/x/image/draw` 缩放后输出 JPEG（照片）和 PNG（透明图），纯 Go、零新依赖；
   - 方案 B：WebP（总文档原计划），候选库 `github.com/gen2brain/webp`（libwebp 编译成 WASM、经 wazero 运行，不需要 cgo）。引入前要验证它支持有损压缩，并测一下对后端常驻内存的影响（目标 < 100MB）。
2. **`/vip` 是否缓存**：会员页的套餐信息是公开的，缓存后后台改价最多 1 秒内经清除通知生效；如果用户希望改价立即可见且不依赖通知，可以不缓存这一页。
3. 引入任何新的第三方依赖（Go 或 npm）前，说明用途、版本和发布时间。pnpm 有发布时间观察期策略，不要接受它自动写入的 `minimumReleaseAgeExclude`。

## 7. M4 遗留：支付沙箱与小额真钱测试

M4 的验收（总文档第 11 节）要求“沙箱环境走通全部支付流程，再用小额真钱各测一遍”。代码已完成，只差渠道凭证。用户提供凭证后再做，步骤：

1. `server/config.yaml` 把 `pay.mock` 改为 `false` 并重启后端。
2. 后台“支付设置”填写接口地址、前台地址和渠道参数，勾选支付宝的“沙箱环境”。保存时会实际创建一次客户端，参数错了会直接提示。
3. 回调地址必须能从公网访问。本地没有公网时，前台轮询订单状态会顺带向渠道查询，也能确认到账，但回调这条链路需要部署到有公网的环境再测一次。
4. 依次测：电脑扫码（微信 Native）、电脑网站支付（支付宝）、手机浏览器（支付宝手机网站、微信 H5，后者需要商户单独开通）、继续支付时切换渠道、超时关闭、后台查询渠道补单。
5. 小额真钱测试后，在后台对测试订单做退款记录，并在支付平台原路退款。

## 8. 完成标准

- 第 4 节全部完成，第 9 节指标实测达标并写回总文档。
- `task test`、`task check:gen` 通过，前后台生产构建通过。
- 提交信息格式参照已有提交：标题“实现 SEO 与性能（M5）”，正文按模块列出要点。
