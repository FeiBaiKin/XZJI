# M6 上线准备：开发说明

> 日期：2026-10-05，M5 验收后编写
> 范围以 `docs/phase1-tech-plan.md` 第 8、10、11 节为准，本文是拆解后的执行说明。两者冲突时以总文档为准，并在总文档里更正。

## 1. 目标与边界

总文档的 M6 是“按分机方式部署，配好备份脚本和生产环境配置，正式站点上线运行”。用户已确认：

- **生产服务器和域名还没准备好**。本阶段只做上线准备，不登录任何服务器、不部署。等用户有了服务器，再单独安排正式部署。
- **用户自己用 1Panel 面板**，很多站长用宝塔面板。上线手册要覆盖 1Panel、宝塔、纯命令行三条路线，其中 1Panel 写得最细。
- **支付宝沙箱和小额真钱测试放到正式部署时再做**，本阶段不碰支付渠道，但手册里要把步骤写清楚。

本阶段做完的标志：照着手册，从一台空服务器就能把站点装起来；并且已经在本机用生产构建、从空库开始完整装过一遍。

要做的事（按顺序）：

1. M5 验收发现的三处加固，外加一处文档更正（第 4 节）；
2. 上线必须有的站点信息：备案号、用户协议、隐私政策（第 5 节）；
3. 部署包与生产配置（第 6 节）；
4. 备份与恢复脚本（第 7 节）；
5. 本机全新安装演练（第 8 节）；
6. 上线手册（第 9 节）。

## 2. 开工前必读

**项目规则**（完整版见根目录 `CLAUDE.md`，以下是本次最容易踩的）：

- 新增或修改接口：先改 `api/*.yaml`，再 `task gen`，最后写实现。`internal/apigen`、`internal/dbgen`、`packages/api-client/src/gen` 是生成代码，禁止手改。
- 表结构只能新增迁移文件（下一个是 `00006_*.sql`），已发布的迁移不能改。
- 服务端渲染不带用户身份；付费内容、下载地址永远不能出现在 HTML 里。页面缓存正是建立在这一点上。
- 业务模块之间只通过 service 调用；handler 只做参数转换。
- 注释、提交信息、文档用中文，技术名词保持原文。注释只说用途和不显而易见的原因。
- 提交前运行 `task test`；改过契约或 SQL 再运行 `task check:gen`。

**本机环境**：

- 三端：后端 `task dev:server`（8080）、前台 `task dev:web`（3000）、后台 `task dev:admin`（5173）。Windows 上服务要绑定 `127.0.0.1`，Node 会把 `localhost` 解析成 `::1`。
- 系统是 Windows 11，**没有 WSL、没有 Docker、没有 Linux 机器**。Shell 脚本只能在 Git Bash 里验证，systemd、面板相关步骤无法在本机实测。
- MariaDB 12.3 是 Windows 服务，必须用管理员权限启动。连不上数据库（`connection refused`）时请用户启动服务，**不要自己删表、重建或改数据目录**。`mysqldump`、`mysql` 在 `C:\Program Files\mariadb-12.3.3\bin`，root 本机免密。
- 集成测试用 `task test:server`，它会加载 `server/.env.test` 里的 `XZJI_TEST_DB_DSN`。手动跑时用 `export XZJI_TEST_DB_DSN="$(grep '^XZJI_TEST_DB_DSN=' .env.test | cut -d= -f2-)"`，不要 `source .env.test`。
- 数据库密码只在 `server/config.yaml`，测试账号在 `server/.env.test`，都不要写进仓库、文档或对话。
- 开发库 `xzji_dev` 里有 M5 灌入的 1 万篇压测数据。本阶段**不要往 `xzji_dev`、`xzji_test` 里恢复备份，也不要改动 `xzji_dev` 的数据**；演练用新建的临时库（见第 7、8 节）。

**已知的坑**：

- 组件里不要在 setup 阶段或 `watch(..., { immediate: true })` 里调用 `window`、`addEventListener` 等浏览器 API，SSR 会报错。浏览器端逻辑放进 `onMounted`。
- `posts.updated_at` 带 `ON UPDATE CURRENT_TIMESTAMP`，只改计数的 UPDATE 必须写 `updated_at = updated_at`。
- Nitro 打包后 `useStorage('cache').clear()` 是空操作（M5 修过，见 `web/server/routes/_internal/purge.post.ts`）。改动缓存存储时注意清除逻辑仍然有效，并在生产构建下验证，不能只看开发模式。
- pnpm 有发布时间观察期策略：不要接受它自动写入的 `minimumReleaseAgeExclude`，改为放宽版本范围让它选成熟版本。

## 3. 现状

| 项 | 现状 |
|---|---|
| 部署样例 | `deploy/nginx/`（single、split-back、split-front、xzji-proxy）、`deploy/systemd/`（server、web）、`deploy/sql/create-database.sql`、`deploy/config.prod.example.yaml` |
| 构建 | `task build`、`task build:server:linux` 已有；没有打包任务，产物需要手工拼 |
| 备份 | 没有任何脚本 |
| 站点设置 | 后台只有首页、评论、支付三个设置页。前台页脚简介写死在 `web/app/components/AppFooter.vue`，没有备案号；注册页的《用户协议》《隐私政策》链接是 `#`（`web/app/pages/login.vue`） |
| 站点名称 | 由前台环境变量 `NUXT_PUBLIC_SITE_NAME` 配置，本阶段不改 |
| 健康检查 | `GET /api/v1/health` 已有，可用于面板或外部监控 |
| 全新安装 | 从没走过：开发库是逐个迁移升级上来的 |

## 4. M5 遗留：上线前加固

### 4.1 前台页面缓存没有上限

生产构建里 Nitro 的 `cache` 存储默认用内存驱动，不淘汰；路由缓存的键包含完整查询串。用随机查询串（`/?x=1`、`/?x=2`……）连续请求，每个都会触发一次服务端渲染，并常驻内存 10 分钟，前台进程会被撑爆。

要求：

- 生产环境的页面缓存要有上限，可以按条数或字节，并在注释里写明取值依据（前台常驻内存目标 < 200MB）。
  - 可选方案：unstorage 的 `lru-cache` 驱动；
  - 或者在项目里用 `defineDriver` 写一个小的 LRU 驱动。
  - 先确认 Nitro 是否已内置所需依赖；需要新装包的话按第 11 节先问用户。
- 缓存键只保留页面真正用到的查询参数，其余参数（`utm_*`、`ref`、随机串）不影响缓存键。哪些页面用了哪些参数，以代码为准（分类、标签的分页，搜索的关键词和分页等）。
  - 一种做法：在 Nitro 中间件里规范化请求地址，再交给路由缓存。
  - 要确认这样做不会造成水合不一致。
  - 推广链接 `?ref=` 必须仍然有效：`ref.global.ts` 只在浏览器端读取，忽略查询串不影响它。
- 验证必须用生产构建（`node web/.output/server/index.mjs`）：
  - 用上千个不同的随机查询串请求首页，进程内存不持续增长；
  - 缓存条目不超过上限；
  - 清除通知仍能清空缓存；
  - `?ref=` 链接打开后浏览器里有 `xzji_ref` Cookie。

### 4.2 后端接口缓存只限条数，不限字节

`server/internal/platform/cache` 按条数淘汰（默认 2000 条），单条上限 1MB，最坏情况能占到 GB 级内存，而后端常驻内存目标是 < 100MB。

要求：

- 增加总字节上限，配置项名称自定，例如 `cache.max_bytes`，默认值按 100MB 的目标给出（如 32MB）。
- 淘汰时同时满足条数和字节两个上限。
- 同步更新 `config.example.yaml`、`deploy/config.prod.example.yaml`。
- 补表驱动单元测试。
- 可选：缓存键去掉契约里没有声明的查询参数。

### 4.3 浏览量上报可以被刷

`POST /posts/{id}/view` 不查库、不限流。用随机 ID 刷这个接口，`viewCounter.pending` 和限流器的键都会无限增长，每分钟刷库时还会对每个 ID 各执行一条 UPDATE。

要求：

- 按 IP 限流，例如每分钟 60 次；超出的请求仍返回 204，只是不计数。
- `pending` 设上限，超出后丢弃新 ID，并记一次警告日志（不要每条都记）。
- 刷库时合并成批量 UPDATE，或者限制单次条数，避免一次刷库打出成千上万条语句。
- 补单元测试。

### 4.4 文档与实现不符：`X-Internal-Key`

总文档 7.3 写的是“SSR 调接口时带 `X-Internal-Key`，后端据此不按 IP 限流”，第 10 节也说前台 `.env` 里有“内部密钥”。但代码里没有实现这个机制。实际情况是：后端没有对公开 GET 接口按 IP 限流，所以 SSR 请求本来就不会被误伤。

处理方式：

- 不实现 `X-Internal-Key`。
- 在 Nginx 样例里给 api 域名加 `limit_req`，用来防浏览器端的刷接口。SSR 走内网直连后端 8080，不经过这层 Nginx，不受影响。
- 按上述实际情况更正总文档 7.3 和第 10 节。

## 5. 上线必需的站点信息（法规）

大陆服务器上线要在页脚展示 ICP 备案号；网站有注册功能、收集邮箱，就必须有用户协议和隐私政策。

### 5.1 后台“站点设置”

参照现有的 `/settings/home`、`/settings/comment` 的实现方式（setting 模块、契约、后台页面），新增站点设置，包含以下字段：

- 页脚简介（替换 `AppFooter.vue` 里写死的那句）；
- ICP 备案号；
- 公安备案号（可空）；
- 用户协议正文；
- 隐私政策正文。

协议正文沿用现有内容编辑器和渲染方式，经过与内容正文相同的 XSS 净化。

后台编辑框旁写清楚隐私政策应包含的要点：

- 收集哪些信息：邮箱、IP、订单与支付记录、Cookie；
- 用途；
- 第三方：支付宝、微信支付；
- 保存期限；
- 用户的查询、更正、注销权利；
- 联系方式。

**不要替站长写法律条款正文。**

后台保存后，现有的 `purgeOnAdminWrite` 会自动清缓存，不需要额外处理。

### 5.2 前台

**页脚**：

- 显示 ICP 备案号，链接 `https://beian.miit.gov.cn/`，新窗口打开；
- 填了公安备案号时，显示并链接 `https://beian.mps.gov.cn/#/query/webSearch?code=<备案号中的数字>`；
- 简介改读设置。

**公开接口**：

- 新增一个返回这些字段的公开接口（例如 `GET /site/info`）；
- 加进后端缓存白名单；
- 页脚在 SSR 时读取。

**协议页**：

- 新增 `/terms`、`/privacy` 两个页面，SSR 并开页面缓存；
- 站长还没填写时显示“站长尚未填写”，同时输出 `noindex`。

**注册页**：两个协议链接指向这两个页面，新窗口打开。

**页面外观**：沿用现有组件和 `design/xzji.css` 的设计变量，参照内容详情页的正文排版，不另起风格。

## 6. 部署包与生产配置

### 6.1 打包任务 `task release`

在 `dist/` 下生成两个包。分机部署时分别上传到后端服务器和前台服务器，单机部署时两个都传。

| 包 | 内容 |
|---|---|
| `xzji-server-<版本>-linux-amd64.tar.gz` | `bin/xzji`（内嵌后台，可执行权限 0755）；`config.example.yaml`（取自 `deploy/config.prod.example.yaml`）；`scripts/`（备份、恢复）；`systemd/xzji-server.service`；`nginx/` 样例；`sql/create-database.sql` |
| `xzji-web-<版本>.tar.gz` | `.output/`；`web.env.example`；`systemd/xzji-web.service` |

要求：

- 每个包附带 `.sha256` 文件；版本号沿用 Taskfile 里的 `VERSION`。
- 打包逻辑必须能在 Windows 上运行，不依赖 Linux 专有命令。
  - 建议在 `server/tools/release` 用 Go 标准库的 `archive/tar` 加 `compress/gzip` 实现。
  - 二进制文件在 tar 头里要设 0755，否则在 Linux 上解压后无法执行。
- 确认 Windows 上构建的 `web/.output` 可以直接在 Linux 上运行：检查 `.output/server/node_modules` 里有没有 `.node` 原生模块或平台相关的二进制，把结论写进手册。

### 6.2 生产配置

**前台环境变量样例**：新增 `deploy/web.env.example`，列出前台的全部环境变量，与 `xzji-web.service` 保持一致。面板用“Node 项目”方式运行时，也照这份样例填写。

**后端配置样例**：复核 `deploy/config.prod.example.yaml`，确保分机部署要改的每一项都有注释，第 4.2 节新增的配置项也要写进去。

**生产环境校验**（`config.validate` 里 `IsProd()` 分支）补齐以下几项：

- `auth.cookie_secure` 必须为 true；
- `cors.allow_origins` 必须是 https 地址；
- `media.serve` 应为 false。

每一项是报错还是只打印警告由你判断，在注释里写明理由。不能弄坏现有测试。

### 6.3 Nginx 样例复核

- HTTPS 配置；
- gzip 覆盖 HTML、CSS、JS、JSON、SVG。M5 实测过：前台前面没有 gzip 时，Lighthouse 首页只有 80 分。
- `client_max_body_size` 与 `media.max_size_mb` 对齐；
- `/_internal/purge` 只允许后端访问（已有，确认分机样例也正确）；
- 安全响应头：
  - 全站加 `X-Content-Type-Options: nosniff`、`Referrer-Policy`；
  - 后台域名加 `X-Frame-Options: DENY`；
  - HSTS 先写成注释，并说明开启条件（确认全部子域名都已是 HTTPS 再开）。
- 图片域名只提供图片文件，不执行脚本，不提供目录列表；
- `/_nuxt/` 静态资源设长期缓存（`immutable`）；
- 第 4.4 节的 `limit_req`。

## 7. 备份与恢复

### 7.1 `deploy/scripts/backup.sh`

需要备份的内容：

- 数据库：`mysqldump --single-transaction --quick`；
- `data/uploads`：打包；
- `config.yaml` 和 `secret.key`：单独复制。`secret.key` 丢了，就要在后台重新填写所有支付参数。

要求：

- 备份文件按日期命名，保留天数可以用参数指定，过期的自动删除。
- 任何一步失败都返回非零退出码，并输出清楚的错误信息。
- 数据库连接参数从参数或环境变量读取，不要把密码打印到输出或日志里。
- 支持 `mysqldump` 不在宿主机上的情况。1Panel 的 MySQL 运行在 Docker 容器里，`mysqldump` 命令要做成可配置的，例如 `MYSQLDUMP="docker exec -i 1Panel-mysql-xxxx mysqldump"`。

### 7.2 `deploy/scripts/restore.sh`

- 把数据库恢复到指定库，同时恢复上传文件。
- 目标库必须显式传入，并要求二次确认（例如 `--yes` 参数）。
- 拒绝恢复到名字以 `_test` 结尾的库。

### 7.3 本机演练

在 Git Bash 里，对第 8 节的演练库做一次备份，恢复到另一个新建的临时库，再逐表比对行数，上传文件也要比对。演练结束后删除临时库。结果写进手册。**绝不能恢复到 `xzji_dev`、`xzji_test`。**

### 7.4 面板路线

1Panel 和宝塔的计划任务都自带“备份数据库”“备份目录”。手册里写清楚以下几点：

- 用面板备份时要包含哪些路径；
- `secret.key` 必须另外保存，并且不要和数据库备份放在同一个地方；
- 恢复的步骤。

## 8. 本机全新安装演练（生产构建）

目的：验证从空库开始能把站点完整装起来。

**准备**：

- 用 root 新建空库，例如 `xzji_rehearsal`，演练结束后删除。
- 用 `task build` 的产物，再写一份演练用配置。
  - 配置文件放在 git 忽略的位置（确认 `.gitignore` 覆盖到了；没覆盖就放仓库外的临时目录），不要提交。
  - 尽量按 `env: prod` 来填。本机没有 HTTPS、没有 SMTP，满足不了的项记录为偏差。
  - **不要为了演练放宽生产校验**，也不要把放宽的代码提交上去。

**依次走一遍**：

1. 启动后端，自动迁移建表，然后 `xzji migrate status`。
2. `xzji admin create` 创建管理员。
3. 后台：建分类、发布文章和资源、上传图片（检查是否生成了缩略图）、配会员套餐、填写站点设置。
4. 前台匿名访问：页面缓存能命中；页脚显示备案号；协议页正常。
5. 后台修改一篇内容，前台 2 秒内更新（验证清除通知和 HMAC 签名）。
6. 注册用户。生产模式下不能用模拟支付，改为后台给这个用户调整余额，再用余额购买内容、解锁、拿到下载地址。
7. 检查 `sitemap.xml` 和 `robots.txt` 里的地址是不是配置的站点地址。
8. 在这个库上做第 7.3 节的备份恢复演练。

有浏览器工具就在浏览器里实际点一遍；没有的话，用 curl 验证接口和 HTML，并在汇报里写明哪些步骤没在浏览器里验证过，验收时会补测。

## 9. 上线手册 `docs/deploy-guide.md`

读者是站长：会用面板，不一定熟悉命令行。手册要包含以下内容：

1. **部署拓扑与最低配置**：
   - 单机 1 核 2G，分机每台 1 核 1G；
   - 域名规划：www、api、admin、img；
   - 大陆服务器要先完成 ICP 备案才能解析访问。
2. **路线 A：1Panel**（用户本人在用，写得最细）。按以下顺序写：
   1. 安装 OpenResty 和 MySQL 应用；
   2. 创建数据库。1Panel 的 MySQL 跑在容器里，从宿主机连接时来源不是 localhost，所以账号的访问权限不能只给 localhost。
   3. 上传并解压部署包；
   4. 进程守护：用 systemd 还是面板自带的守护，选一种并说明理由；
   5. 创建反向代理网站并申请证书；
   6. 图片目录。OpenResty 也跑在容器里，只能读挂载进去的目录，要写清楚 `media.dir` 放在哪里、怎么让 OpenResty 读到。
   7. 配置计划任务备份。
3. **路线 B：宝塔**：网站与反向代理、MySQL、证书、进程守护（Go 项目、Node 项目或 Supervisor）、计划任务。
4. **路线 C：纯命令行**：systemd 加 Nginx，基于现有样例。
5. **首次启动**：
   - 后端配置项逐项说明；
   - 两个密钥用 `openssl rand -hex 32` 生成：`frontend.purge_secret` 与前台的 `NUXT_PURGE_SECRET` 必须一致；
   - 创建管理员；
   - 后台站点设置、支付设置。
6. **支付上线**：先沙箱、再小额真钱。把 `docs/m5-seo-performance.md` 第 7 节的步骤搬过来并更新，回调地址必须能从公网访问。
7. **上线检查清单**（可逐项勾选），至少包括：
   - `pay.mock` 已关闭；
   - HTTPS 已生效；
   - Cookie 主域、CORS 正确；
   - SMTP 发信测试通过；
   - 清除通知已打通；
   - sitemap、robots 能访问；
   - 备案号已显示；
   - 备份跑过一次，并且演练过恢复；
   - `secret.key` 已另外保存。
8. **日常运维**：
   - 升级流程：备份 → 替换后端 → 启动时自动迁移 → 替换前台 → 重启 → 确认缓存已清除；
   - 回滚：迁移只进不退，回滚只能靠恢复备份；
   - 看日志、日志轮转、证书续期、常见问题。

面板的菜单名称和路径以官方文档为准（1Panel：https://1panel.cn/docs/ ，宝塔：https://www.bt.cn/ ）。能联网就查证当前版本；查不到的地方写清楚要达到的目的，并标注“待实机核对”。**不要编造菜单名**。正式部署时会按手册实机走一遍，再修正。

## 10. 测试要求

- 第 4.2、4.3 节的逻辑写表驱动单元测试。
- 站点设置要有集成测试（`server/internal/app/`），至少覆盖：
  - 保存后公开接口能读到；
  - 保存会触发清除；
  - 正文经过 XSS 净化。
- 前台改动跑 `pnpm -r typecheck`；页面缓存相关改动必须在生产构建下验证。
- Shell 脚本在 Git Bash 下实际跑过（第 7.3 节），并用 `bash -n` 检查语法。

## 11. 需要先问用户的决定

开始对应任务前先问，不要自行决定：

1. 引入任何新的第三方依赖（Go 或 npm）前，说明用途、版本和发布时间。
2. 如果第 4.4 节的处理方式（不实现 `X-Internal-Key`、改为 Nginx 限流）你认为不妥，说明理由后再动。
3. 任何涉及真实服务器、域名、支付账号、发信账号的操作，本阶段都不做。

## 12. 完成标准与提交

- 第 4 至 9 节全部完成。
- `task test`、`task check:gen`、`task build`、`task release` 全部通过。
- 本机全新安装演练、备份恢复演练都走通，结果和偏差写进手册。
- 更新总文档：
  - 第 7.3、10 节（第 4.4 节的更正）；
  - 第 11 节：M6 一行拆成“上线准备（本阶段）”和“正式部署（有服务器后）”两部分，并记录本阶段完成情况。
- 分 4 次提交，每次提交前测试都要通过，提交正文按模块列要点：
  1. `上线前加固（M6）`：第 4 节；
  2. `新增站点设置与协议页面（M6）`：第 5 节；
  3. `新增部署包与备份脚本（M6）`：第 6、7 节；
  4. `完成上线演练与上线手册（M6）`：第 8、9 节，以及总文档更新。
- 全部完成后汇报四件事：
  - 做了什么；
  - 与本文的偏差及原因；
  - 哪些步骤没能实测；
  - 需要用户决定的问题。
