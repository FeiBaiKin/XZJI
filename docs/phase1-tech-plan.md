# XZJI 第一阶段技术方案

> 状态：草案，业务规则已确认，整体待过目
> 日期：2026-10-03

## 1. 定位与目标

XZJI 是一个垂直的资源站程序。前台、业务和后台一体设计，不依赖 WordPress，也不做通用 CMS 的扩展层（插件、钩子、自定义内容类型）。

工程上前后端分离：
- **后端**：Go 编写，负责接口和后台管理；
- **前台**：Nuxt 编写，负责服务端渲染页面。

两者可以部署在同一台服务器，也可以分别部署在不同服务器上。

第一阶段目标：功能足以支撑一个 1000~1 万篇内容的资源站正式运营。

三个核心卖点，所有技术决策都优先服务于它们：

| 卖点 | 落地方式 |
|---|---|
| 性能 | 后端单文件、内存占用低；前台服务端渲染加页面缓存；公开内容和个人内容分开 |
| 业务逻辑灵活 | 访问规则、会员权益全部在后台配置 |
| 不被绑死 | 自有数据结构；内置迁移和一键升级；数据可导出；接口标准化，前台、App、小程序共用一套接口 |

## 2. 技术选型

### 2.1 后端

| 用途 | 选择 | 理由 |
|---|---|---|
| 语言 | Go 1.27 | 单文件部署，常驻内存小；静态类型让编译器兜住 AI 的错误；编译型便于日后授权保护 |
| Web 框架 | Gin | 生态最大，AI 生成质量稳定 |
| 接口代码生成 | oapi-codegen | 从接口契约生成 Gin 接口和数据结构；漏实现接口或类型不对时，编译直接报错 |
| 数据库 | MySQL 8.0+ / MariaDB 10.6+ | 站长熟悉，宝塔默认提供；开发环境使用本机 MariaDB 12 |
| 数据访问 | sqlc + 手写 SQL | SQL 显式可见，性能透明；生成类型安全的代码；后台多条件筛选用 squirrel 拼接 |
| 迁移 | goose | 迁移 SQL 嵌入程序，启动时自动升级表结构 |
| 缓存 | 进程内 LRU | 第一阶段不强制使用 Redis |
| 支付 | gopay | 成熟的微信支付 v3 和支付宝 Go SDK |
| 密码哈希 | argon2id | 当前推荐的密码哈希算法 |
| HTML 过滤 | bluemonday | 防 XSS |
| 日志 | log/slog | 标准库自带，支持结构化日志 |

### 2.2 前台

| 用途 | 选择 | 理由 |
|---|---|---|
| 框架 | Nuxt 4 | 首屏服务端渲染，保证 SEO；之后站内跳转不再整页刷新；支持路由级页面缓存 |
| 样式 | Tailwind CSS | 构建时按需生成，产物小；设计变量以 `design/xzji.css` 为参照映射到 Tailwind 主题 |
| 状态管理 | 不引入状态库 | 站点状态简单，Nuxt 自带的 `useState` 与 composable 已经够用（2026-10-05 修正：原计划用 Pinia，实际未引入） |
| 接口请求 | openapi-typescript + openapi-fetch | 从契约生成类型；接口改了，前端编译就会报错 |

### 2.3 后台

| 用途 | 选择 | 理由 |
|---|---|---|
| 框架 | Vue 3 + Vite + TypeScript | 和前台同属 Vue 生态 |
| 组件库 | Element Plus | 成熟，AI 熟悉 |
| 编辑器 | TipTap | 方便做"隐藏内容块"这类自定义块 |

后台不需要 SEO，做成纯单页应用。构建产物嵌入后端程序，随后端一起部署。

### 2.4 工程

| 用途 | 选择 |
|---|---|
| 前端多包管理 | pnpm workspace（前台、后台、共享的接口客户端） |
| 常用命令 | Taskfile（go-task，Windows 下可用） |

### 2.5 明确不用的东西

| 不用 | 原因 |
|---|---|
| 纯 SPA 前台 | 百度几乎无法收录，对资源站是致命问题 |
| ORM（GORM 等） | 隐式查询和 N+1 问题难以察觉；涉及金额的代码需要 SQL 一眼可见 |
| 数据库全文索引做搜索 | MariaDB 不支持中文 ngram 分词，两种数据库行为不一致。1 万篇量级下，对标题和摘要做 LIKE 查询只要毫秒级，再加结果缓存就够用。搜索做成接口，日后可替换为嵌入式搜索引擎（如 Bleve） |
| Redis（第一阶段） | 单个后端实例用不上，接口预留 |
| JWT 存在 localStorage | 一旦有 XSS 就能被直接窃取。网页端使用 HttpOnly Cookie |

## 3. 整体架构

### 3.1 部署拓扑

```
                       ┌────────── 前台服务器 ──────────┐
 浏览器 ── www.域名 ──▶ │ Nginx → Nuxt（服务端渲染 + 页面缓存）│
   │                   └──────────────┬────────────────┘
   │                                  │ 服务端渲染时调用公开接口（不带用户身份）
   │                                  ▼
   │                   ┌────────── 后端服务器 ──────────┐
   ├── api.域名 ──────▶ │ Nginx → Go 程序：前台接口 /api/v1 │
   ├── admin.域名 ────▶ │          后台页面 + /admin-api/v1 │
   └── img.域名 ──────▶ │ Nginx 直接提供上传的图片           │
                       │ MySQL                           │
                       └─────────────────────────────────┘
```

- **分机部署**：前台服务器和后端服务器各自独立。建议放在同一地域或同一机房，因为服务端渲染时每个页面都要调用接口，跨地域会增加延迟（页面缓存可以抵消大部分影响）。
- **单机部署**：同一个 Nginx 承接四个域名，Nuxt 通过 `127.0.0.1` 调用接口。
- 两种部署方式只是配置不同，代码不需要改。
- 浏览器访问个人数据时直接请求 `api.域名`，不经过前台服务器中转。

### 3.2 仓库结构

使用单仓库（monorepo）管理全部代码：

```
XZJI/
├── api/
│   ├── public.yaml        前台和 App 使用的接口契约（带版本号，保持向后兼容）
│   └── admin.yaml         后台接口契约
├── server/                Go 后端
├── web/                   Nuxt 前台
├── admin/                 Vue 后台（构建产物嵌入后端程序）
├── packages/
│   └── api-client/        从契约生成的 TS 类型和请求客户端，web 和 admin 共用
├── deploy/                Nginx 和 systemd 配置样例（单机、分机两套）
├── docs/
├── pnpm-workspace.yaml
├── Taskfile.yml
└── CLAUDE.md              AI 开发约定
```

### 3.3 后端目录

后端采用模块化单体：只有一个进程，内部按业务划分模块。模块之间只通过 service 接口互相调用，不直接读写对方的表。

```
server/
├── cmd/xzji/              程序入口（serve / migrate / admin 子命令）
├── internal/
│   ├── app/               依赖组装、启动与关闭
│   ├── config/
│   ├── platform/          基础设施：db / cache / mail / storage / scheduler
│   ├── module/            业务模块，每个模块内含 model / repo / service 及测试
│   │   ├── user/          用户、会话、认证
│   │   ├── content/       文章、资源、分类、标签、隐藏块、下载地址
│   │   ├── access/        访问规则判定、价格计算
│   │   ├── membership/    会员等级、套餐、开通与续期
│   │   ├── order/         订单
│   │   ├── payment/       支付渠道适配、回调、补单
│   │   ├── wallet/        余额与流水
│   │   ├── comment/       评论
│   │   ├── media/         图片上传与处理
│   │   ├── seo/           sitemap 数据
│   │   ├── setting/       站点设置
│   │   └── frontsync/     通知前台清除缓存
│   ├── httpapi/
│   │   ├── middleware/    会话、CSRF、跨域、限流、日志、异常恢复
│   │   ├── public/        实现 public.yaml 生成的接口
│   │   ├── admin/         实现 admin.yaml 生成的接口
│   │   ├── adminui/       托管嵌入程序的后台页面
│   │   └── apierr/        统一错误响应
│   ├── apigen/            oapi-codegen 生成的代码（禁止手改）
│   └── dbgen/             sqlc 生成的代码（禁止手改）
├── db/
│   ├── migrations/        goose 迁移 SQL
│   └── queries/           sqlc 查询 SQL
└── config.example.yaml
```

### 3.4 前台目录

```
web/
├── app/                   源码目录（Nuxt 4 约定）
│   ├── pages/             页面路由
│   ├── components/
│   └── composables/       接口调用、登录状态等复用逻辑
├── server/                Nuxt 服务端路由：清除缓存、sitemap.xml、robots.txt
└── nuxt.config.ts
```

### 3.5 接口规范

**契约优先**：先写 `api/*.yaml`，再生成两端代码，最后才写实现。接口的增减和修改只能从契约开始，这是防止 AI 在前后端写出不一致代码的核心手段。

| 约定 | 内容 |
|---|---|
| 路径 | 前台和 App 用 `/api/v1/*`，后台用 `/admin-api/v1/*` |
| 版本 | 前台接口出现不兼容变更时升级到 `/api/v2`，旧版本保留一段时间，保证已安装的旧版 App 还能用；后台接口随后端一起发布，不做版本兼容 |
| 成功响应 | 直接返回数据，不额外包一层 `{code, data}` |
| 错误响应 | 用 HTTP 状态码加统一的错误体：`{"error": {"code": "ORDER_EXPIRED", "message": "订单已过期"}}`。前端根据 `code` 判断错误类型，`message` 直接展示给用户 |
| 分页 | 请求参数 `page`、`page_size`；响应 `{"items": [], "total": 0, "page": 1, "page_size": 20}` |
| 金额 | 整数，单位为分；由前端负责格式化显示 |
| 时间 | RFC 3339 格式，UTC 时区 |
| 接口文档 | 由契约自动渲染，供以后开发 App 和小程序时查阅 |

### 3.6 关键设计：公开内容和个人内容分开

这是性能和安全的核心设计，前后端分离后同样适用，而且实现起来更自然。

1. **服务端渲染只渲染公开内容**。Nuxt 在服务端调用接口时不携带用户身份，所以渲染结果对所有访客都一样：
   - 隐藏块只显示一个占位符；
   - 下载地址不出现在页面里；
   - 页头的用户区域留空。
2. **因此整个页面可以在前台服务器上缓存**，所有访客共用，命中缓存时既不用渲染，也不用请求后端。
3. **页面在浏览器里加载完成后**，前端直接请求 `api.域名` 获取个人部分：`/api/v1/auth/session`（登录状态，未登录时也返回 200，避免匿名访问产生大量 401）、`/api/v1/posts/{id}/unlocked`（已解锁的隐藏块和下载地址）。这些接口每次都实时判断权限。
4. **只在浏览器端渲染的页面**：用户中心、订单、支付这些页面不需要 SEO，不做服务端渲染，减轻前台服务器的压力。
5. **浏览量**：由浏览器调用 `POST /api/v1/posts/{id}/view` 上报（命中页面缓存时服务端根本不会执行，没法在服务端统计）。后端先在内存里累加，每分钟批量写一次数据库。

付费内容永远不会出现在缓存或公开的 HTML 里，按 F12 也看不到。

### 3.7 缓存与失效

| 层 | 缓存内容 | 失效方式 |
|---|---|---|
| 前台 Nuxt | 公开页面的 HTML（路由级缓存，存在内存里） | 收到后端的清除通知后清空；同时设置 10 分钟兜底过期 |
| 后端 Go | 公开接口的响应数据 | 内容、评论或设置发生变更时清空 |

- **清除通知**：后端调用每台前台服务器的 `POST /_internal/purge`。请求带 HMAC 签名和时间戳，防止被伪造或重放。前台服务器的地址列表写在后端配置里，以后增加前台服务器时，只需把地址加进列表。
- **通知失败时**（比如前台服务器正好重启），最多 10 分钟后缓存自动过期，内容不会一直停在旧版本。
- **清空范围**：一律清空整站缓存。1 万篇这个量级重建成本很低，换来实现简单、不出错。

### 3.8 URL 设计

前台页面由 Nuxt 提供：

| 页面 | URL | 渲染方式 |
|---|---|---|
| 首页 | `/` | 服务端渲染 + 缓存 |
| 内容详情（文章、资源统一） | `/{id}.html` | 服务端渲染 + 缓存 |
| 分类 | `/category/{slug}/`，分页 `/category/{slug}/page/{n}/` | 服务端渲染 + 缓存 |
| 标签 | `/tag/{slug}/`，分页 `/tag/{slug}/page/{n}/` | 服务端渲染 + 缓存 |
| 搜索 | `/search?q=` | 服务端渲染，短时缓存 |
| 用户中心 | `/user/*` | 仅浏览器端渲染 |
| sitemap | `/sitemap.xml` | Nuxt 服务端路由，数据从后端获取 |

## 4. 数据库设计

**约定：**
- 金额一律用 BIGINT 存"分"；
- 时间用 DATETIME 存 UTC；
- 主键用 BIGINT 自增，对外展示的订单号单独用 `order_no`；
- 字符集用 utf8mb4。

### 4.1 用户

```
users                    用户
  id                     BIGINT PK
  email                  VARCHAR(191) UNIQUE
  password_hash          VARCHAR(255)        argon2id
  nickname               VARCHAR(64)
  avatar_media_id        BIGINT NULL
  role                   VARCHAR(16)         user / admin
  status                 VARCHAR(16)         active / banned
  inviter_id             BIGINT NULL         邀请人（为第二阶段分佣预留，从第一天开始记录）
  email_verified_at      DATETIME NULL
  last_login_at          DATETIME NULL
  created_at / updated_at

sessions                 登录会话
  id                     CHAR(64) PK         令牌的 SHA-256；客户端只持有原始令牌
  user_id                BIGINT INDEX
  client                 VARCHAR(16)         web / admin / app
  ttl_seconds            INT                 会话时长：勾选自动登录和 App 为 30 天，未勾选为 1 天，admin 为 12 小时
  ip / user_agent
  expires_at             DATETIME            剩余不足一半时续期；admin 不续期
  created_at

email_codes              邮箱验证码
  id                     BIGINT PK
  email                  VARCHAR(191) INDEX
  purpose                VARCHAR(16)         register / reset_password
  code_hash              VARCHAR(64)
  attempts               INT                 校验失败次数，达到 5 次即作废
  ip                     VARCHAR(45)
  expires_at             DATETIME            5 分钟
  used_at                DATETIME NULL
  created_at
```

**邀请关系从第一天就记录**：访问带 `?ref=用户ID` 的链接时写入 Cookie，注册时存入 `inviter_id`。第一阶段不做分佣，但如果现在不记录，以后这些用户的邀请关系就找不回来了。

### 4.2 内容

```
categories               分类（文章和资源共用一棵分类树，最多两级）
  id                     BIGINT PK
  parent_id              BIGINT NULL
  name / slug(UNIQUE)
  icon                   VARCHAR(32)         图标名，对应前台 icons.svg
  color                  VARCHAR(16)         渐变色：blue / red / orange / green / purple / gold / vip
  description            VARCHAR(255)
  sort                   INT                 越小越靠前；顶级分类按此顺序出现在导航和首页入口
  seo_title / seo_description

tags                     标签
  id / name(UNIQUE) / slug(UNIQUE)          slug 可以是中文，留空时由名称生成

posts                    内容主表（文章和资源共用）
  id                     BIGINT PK
  type                   VARCHAR(16)         article / resource
  title                  VARCHAR(255)
  summary                VARCHAR(500)
  summary_auto           TINYINT             摘要由正文自动截取，修改正文时重新生成
  content_html           MEDIUMTEXT          公开版正文，隐藏块只保留占位符；本站图片地址存为 @media/ 前缀
  toc                    JSON                正文目录，保存时从二、三级标题提取
  cover_media_id         BIGINT NULL         16:10 封面图
  category_id            BIGINT INDEX
  author_id              BIGINT
  status                 VARCHAR(16)         draft / published / trash
  pinned                 TINYINT             置顶
  paid_rule              JSON                付费设置，见 4.4（M3 加入）
  view_count / comment_count  INT
  seo_title / seo_keywords / seo_description
  published_at           DATETIME INDEX      晚于当前时间即为定时发布
  created_at / updated_at

post_tags                内容与标签的关联
  post_id / tag_id       联合主键
  sort                   INT                 标签在这篇内容里的顺序

post_hidden_blocks       隐藏内容块（保存时从正文中拆出单独存放）
  id                     BIGINT PK
  post_id                BIGINT INDEX
  seq                    INT                 对应正文中占位符的序号
  cond                   VARCHAR(32)         解锁条件，见 4.4
  content_html           MEDIUMTEXT

resources                资源扩展信息（与 posts 一对一，仅 type=resource）
  post_id                BIGINT PK
  platform               VARCHAR(32)         可选值在后台配置：安卓 / Windows / macOS / 源码 / 主题 等
  version                VARCHAR(64)
  file_size              VARCHAR(32)
  icon_media_id          BIGINT NULL         方形软件图标；没有封面图时，前台按图标主色配底色显示
  attrs                  JSON                自定义属性 [{name, value}]（语言、官网、系统要求等），前台以表格展示
  changelog              JSON                更新日志 [{version, date, note}]，新的在前，前台以时间线展示
  download_cond          VARCHAR(32)         下载区的解锁条件（M3 加入）

resource_links           下载地址
  id                     BIGINT PK
  post_id                BIGINT INDEX
  name                   VARCHAR(64)         如"百度网盘""夸克网盘"
  url                    VARCHAR(1024)
  extract_code           VARCHAR(32)
  unzip_password         VARCHAR(64)
  sort                   INT
  status                 VARCHAR(16)         ok / broken

link_reports             下载地址失效反馈
  id / link_id / user_id
  reason                 VARCHAR(255)
  status                 VARCHAR(16)         pending / resolved / ignored
  created_at

download_logs            下载记录（用于统计和每日次数限制）
  id / user_id / post_id / link_id
  is_member_free         TINYINT             是否靠会员身份免费下载（只有这类下载计入每日次数）
  ip
  created_at             INDEX(user_id, created_at)
```

### 4.3 媒体、评论、设置、审计

```
media                    上传文件
  id / user_id
  path                   VARCHAR(255)        年/月/随机文件名；对外地址 = 配置中的 media.base_url + path
  mime / size / width / height
  color                  CHAR(7)             主色，上传时提取，用于给软件图标配底色
  variants               JSON                各尺寸的 WebP 缩略图路径（M5 生成）
  created_at

comments                 评论
  id / post_id(INDEX) / user_id / parent_id
  content                TEXT
  status                 VARCHAR(16)         pending / approved / spam
  ip / user_agent
  created_at

settings                 站点设置
  key                    VARCHAR(64) PK
  value                  JSON
  updated_at

admin_logs               后台操作日志（调整余额、修改订单等敏感操作必须记录）
  id / admin_id
  action / target_type / target_id
  detail                 JSON
  ip
  created_at
```

- 首页横幅（最多 5 张轮播）和公告存在 `settings` 的 `home` 键，在后台“首页设置”修改。
- 上传只接受 JPEG、PNG、GIF、WebP，按文件内容判断类型，单边不超过 10000 像素，大小上限为 `media.max_size_mb`。开发环境由后端在 `/uploads/` 下直接提供文件（`media.serve: true`），生产由 Nginx 提供。

### 4.4 访问规则

**每篇内容有一份付费设置**，存在 `posts.paid_rule`：

```json
{
  "price": 990,
  "member_free_rank": 2,
  "member_discount": true
}
```

| 字段 | 含义 |
|---|---|
| `price` | 单独购买的价格（分）；0 表示不能单独购买 |
| `member_free_rank` | 会员等级达到这个 rank 即可免费；0 表示会员也不免费 |
| `member_discount` | 等级不够免费的会员，是否享受该等级的折扣 |

**每个隐藏块、每个资源的下载区，各自有一个解锁条件**（`cond`）：

| 条件 | 含义 |
|---|---|
| `public` | 公开 |
| `login` | 登录后可见 |
| `comment` | 评论审核通过后可见 |
| `paid` | 已购买本篇，或者会员等级达到 `member_free_rank` |
| `member:N` | 会员等级 rank ≥ N 才可见，不能单独购买 |

- 购买一次，就解锁这篇内容里所有条件为 `paid` 的部分。
- 管理员和作者本人始终可见。
- 判定逻辑和价格计算集中在 `access` 模块，配有完整的表驱动单元测试。

### 4.5 会员

```
membership_levels        会员等级
  id / name
  description            VARCHAR(64)         一句话介绍，前台会员页显示（M4 加入）
  rank                   INT UNIQUE          数字越大等级越高
  discount_rate          INT                 80 表示 8 折，100 表示不打折
  daily_download_limit   INT                 0 表示不限
  perks                  JSON                自定义权益说明；折扣和每日下载次数由前台自动生成，高等级自动包含低等级的自定义权益（M4 加入）
  status

membership_plans         售卖套餐
  id / level_id
  name                   VARCHAR(32)         月卡 / 年卡 / 永久
  duration_days          INT                 0 表示永久
  price / original_price BIGINT               original_price 为划线价，0 表示不显示
  badge                  VARCHAR(16)         角标，如“最多人选”
  status / sort                              同一等级只能有一个启用的永久套餐（永久升级补差价的依据）

user_memberships         用户当前的会员状态（同一时间只有一个等级）
  user_id                BIGINT PK
  level_id               BIGINT
  expires_at             DATETIME NULL       NULL 表示永久
  updated_at
```

- **同等级续费**：在原到期时间上叠加时长。
- **升级到更高等级**：按剩余价值折算，规则见第 13 节。
- **到期处理**：定时任务每小时删除已过期的会员状态，让会员人数等统计准确。判断权限时也会同时校验 `expires_at`，不依赖定时任务是否及时执行。

### 4.6 订单、支付、余额

```
orders                   订单
  id                     BIGINT PK
  order_no               VARCHAR(32) UNIQUE  时间戳 + 随机数
  user_id                BIGINT INDEX
  type                   VARCHAR(16)         post / membership / recharge
  target_id              BIGINT              内容 ID 或套餐 ID；充值订单为 0
  title                  VARCHAR(255)
  snapshot               JSON                下单时的商品快照（价格、规则、升级折算结果）
  amount                 BIGINT              原价
  discount_amount        BIGINT
  pay_amount             BIGINT              应付金额
  pay_method             VARCHAR(16)         wechat / alipay / balance；空表示还没发起支付，补单任务只查已发起第三方支付的订单
  status                 VARCHAR(16)         pending / paid / closed / refunded
  expires_at             DATETIME            30 分钟
  paid_at                DATETIME NULL
  refund_method          VARCHAR(16)         balance 退回余额 / original 原路退回（只记录）
  refund_remark / refunded_at
  created_at / updated_at

payments                 第三方支付记录
  id / order_id(INDEX)
  channel                VARCHAR(16)
  trade_no               VARCHAR(64)         UNIQUE(channel, trade_no)
  amount                 BIGINT
  status                 VARCHAR(16)         success 已入账 / duplicate 订单已由其他交易支付，需要人工原路退款
  raw_notify             TEXT                回调或查询的原文，排查问题用
  created_at

user_purchases           已购权益
  user_id / post_id      联合唯一
  order_id
  created_at

wallets                  钱包
  user_id                BIGINT PK
  balance                BIGINT
  updated_at

wallet_ledger            余额流水（只增不改，是余额的唯一依据）
  id                     BIGINT PK
  user_id                BIGINT INDEX
  account                VARCHAR(16)         balance；第二阶段增加 commission
  amount                 BIGINT              正数为入账，负数为出账
  balance_after          BIGINT
  biz_type               VARCHAR(32)         recharge / purchase / refund / admin_adjust
  biz_id                 BIGINT
  remark                 VARCHAR(255)
  created_at
```

**余额规则：**
- 余额的每一次变动，都必须在同一个事务里先写流水、再改 `wallets.balance`。任何地方都不允许直接修改余额。
- 充值得到的余额只能消费，不能提现。第二阶段的分佣收益记在 `commission` 账户，单独结算和提现。
- 不能用余额充值余额。

## 5. 关键流程

### 5.1 第三方支付

1. **下单**：创建订单（状态 pending，30 分钟后过期），保存商品快照和应付金额。金额一律由服务端计算，不信任前端传来的任何价格。
2. **调起支付**：调用支付渠道，返回二维码或跳转链接。
   - 支付完成后跳回的地址：`www.域名`；
   - 异步回调地址：`api.域名/api/v1/pay/notify/{channel}`。
3. **接收回调**：
   1. 验签；
   2. 核对回调金额是否等于订单的应付金额；
   3. 开启一个事务，依次执行：
      - `UPDATE orders SET status='paid' ... WHERE id=? AND status='pending'`，影响行数为 1 才继续，保证重复回调只处理一次；
      - 写入 payments 记录；
      - 发放权益；
      - 提交事务。
4. **补单**：定时任务每分钟主动查询"已发起支付、但还没收到回调"的订单（最近 2 小时内发起的），防止回调丢失。
5. **超时关闭**：定时任务关闭过期未付的订单；关闭前先查询一次渠道，已到账的改为确认支付，否则同时关闭渠道交易。
6. **前台确认结果**：支付页轮询订单状态接口，后端每次（同一订单每 3 秒最多一次）顺便查询渠道，确认已支付后刷新解锁内容。

补充规则（M4 确认）：
- 已超时关闭的订单收到渠道到账通知时同样改为已支付并发放权益：钱已经收到。
- 订单已支付后又收到另一笔交易的到账通知（换渠道两边都付了），记为重复支付，不再入账，后台可筛选出来人工原路退款。
- 发放权益在同一事务里进行。余额支付时发放失败（如会员状态在下单后变了、同一篇内容重复购买）整单回滚不扣钱；第三方已到账时发放失败只记入订单快照，后台标出“权益未发放”人工处理。
- 开发环境可开启模拟支付（`pay.mock`），微信、支付宝不连真实渠道，生产环境禁止开启。

不同订单类型发放的权益：

| 订单类型 | 发放内容 |
|---|---|
| post | 写入 `user_purchases` |
| membership | 开通或续期会员 |
| recharge | 余额入账（写流水） |

### 5.2 余额支付

在一个事务里依次执行：锁定钱包行（`SELECT ... FOR UPDATE`）→ 校验余额 → 写流水 → 扣余额 → 订单改为 paid → 发放权益。

### 5.3 解锁内容与下载

1. 前端请求 `/api/v1/posts/{id}/unlocked`。
2. 服务端逐个判断隐藏块和下载区的解锁条件，只返回当前用户有权查看的部分。
3. 获取下载地址时：
   - 如果这次是靠会员身份免费下载，检查当天次数是否已用完；
   - 写入 `download_logs`；
   - 返回网盘地址、提取码和解压密码。
4. 没有权限时，返回解锁方式：需要登录、需要评论、价格多少、需要哪个会员等级。前端据此显示对应的操作按钮。

## 6. 第一阶段的支付渠道

| 渠道 | 产品 | 场景 |
|---|---|---|
| 微信支付 | Native 扫码支付 | 电脑端 |
| 微信支付 | H5 支付 | 手机浏览器（需要单独申请开通） |
| 支付宝 | 电脑网站支付 | 电脑端 |
| 支付宝 | 手机网站支付 | 手机浏览器 |

开通条件以官方要求为准。在微信内打开网页时的支付（JSAPI）需要公众号，放到第二阶段，与微信登录一起做。

支付参数（商户号、AppID、密钥、证书）在后台“支付设置”里填写，保存时用填写的参数实际创建一次客户端校验。应用私钥、商户私钥和 APIv3 密钥用 `secret_key_file`（首次启动自动生成）加密后存入站点设置，接口不回显；只拿到数据库备份解不出明文，密钥文件需要和数据库备份分开保管。微信支持“微信支付公钥”和自动下载平台证书两种验签方式，支付宝支持公钥和公钥证书两种模式。

## 7. 认证与跨域

### 7.1 三类客户端

| 客户端 | 身份凭证 | 说明 |
|---|---|---|
| 前台网页 | HttpOnly Cookie，作用于主域（`.域名`） | `www` 和 `api` 是同一主域下的子域名，浏览器会自动带上 Cookie |
| 后台 | HttpOnly Cookie，只作用于 `admin.域名`，`SameSite=Strict` | 与前台的会话完全隔离；前台出问题也拿不到后台权限 |
| App / 小程序（后续） | `Authorization: Bearer <令牌>` | 登录接口在响应体里返回令牌；和网页共用 sessions 表 |

### 7.2 跨域与 CSRF

- **CORS**：
  - 只允许配置中列出的前台域名访问，且必须精确匹配，不用通配符；
  - 允许携带 Cookie。
- **CSRF**：
  - 前端首次发起写请求前调用 `GET /auth/csrf` 取得令牌，后端同时把同一个令牌写进 HttpOnly 的 `xzji_csrf` Cookie；
  - 前端把令牌放进 `X-CSRF-Token` 请求头；攻击页面跨域读不到这个接口的响应，也就伪造不出请求头；
  - 后端校验请求头与 Cookie 是否一致，同时校验 `Origin` 是否在白名单内（后台要求 `Origin` 与 Host 相同）；
  - 使用 Bearer 令牌的请求不需要 CSRF 校验。

### 7.3 服务端渲染请求的识别

- 后端对公开 GET 接口不按 IP 限流，所以前台服务器服务端渲染时发出的请求本来就不会被误伤，不需要额外的“内部请求”密钥机制（原方案里的 `X-Internal-Key` 已取消）。
- 防刷改在 Nginx：api 域名加 `limit_req`（见 `deploy/nginx/single.conf`），限制浏览器端直连接口的频率；前台服务器的服务端渲染走内网直连后端 8080，不经过这层 Nginx，不受影响。
- 服务端渲染请求只能调用公开接口，后端不接受它代表某个用户的身份。
- 后端只信任来自自身 Nginx 的 `X-Real-IP`，客户端自己传来的 `X-Forwarded-For` 一律忽略。

## 8. 安全要点

- **密码**：用 argon2id 哈希；登录失败按 IP 和账号分别限流。
- **邮箱验证码**：5 分钟有效，同一个验证码最多输错 5 次；同一邮箱同一用途 60 秒内只能发一次，同一邮箱、同一 IP 每天各最多 10 次；找回密码时邮箱未注册也提示发送成功，不暴露注册情况。
- **登录**：同一账号 15 分钟内失败 5 次、同一 IP 失败 20 次后暂时拒绝；邮箱不存在时同样做一次密码哈希计算，避免通过响应时间判断邮箱是否注册；后台登录对非管理员账号返回与密码错误相同的提示。
- **XSS**：评论只允许纯文本；后台正文用 bluemonday 白名单过滤。
- **SQL 注入**：所有查询使用参数绑定。
- **上传**：
  - 按文件内容判断真实类型；
  - 限制文件大小；
  - 只允许图片；
  - 使用随机文件名；
  - 图片由 Nginx 以静态文件方式提供，上传目录禁止执行脚本。
- **支付**：回调必须验签并核对金额；价格只在服务端计算；订单状态更新保证幂等。
- **后台**：
  - 使用独立域名，可以在 Nginx 上限制只允许指定 IP 访问；
  - 会话 12 小时过期；
  - 敏感操作写入 `admin_logs`；
  - 第二阶段增加两步验证。
- **前台服务器的清除缓存接口**：校验 HMAC 签名，时间戳超过 5 分钟的请求直接拒绝。
- **付费内容**：永远不出现在服务端渲染的 HTML 里（见 3.6）。

## 9. 性能目标

| 指标 | 目标 | M5 实测 | 结论 |
|---|---|---|---|
| 后端常驻内存 | < 100MB（不含 MySQL） | 29.1MB（压测后；空载 20.1MB） | 达标 |
| 前台 Nuxt 常驻内存 | < 200MB | 107.4MB | 达标 |
| 最低配置 | 分机部署时每台 1 核 1G；单机部署 1 核 2G | — | 按此配置部署 |
| 命中页面缓存时的前台响应 | < 10ms | 并发 1：0.6ms；并发 10：3.9ms；并发 50：18.8ms | 低并发达标，高并发见 9.3 |
| 命中缓存时的后端公开接口 | < 5ms | 并发 1：0.2ms；并发 10：1.0ms；并发 50：5.0ms | 达标 |
| 未命中缓存时的详情页服务端渲染 | < 100ms（前后端在同一机房） | 平均 25.2ms，中位 24.3ms，P90 35.7ms | 达标 |
| 首屏 JS（gzip 后） | < 100KB | 100.7KB（首页 20 个 chunk） | 超 0.7KB，见 9.3 |
| Lighthouse 移动端性能评分 | ≥ 90 | 首页 97、资源详情页 96（各跑 3 次取中位数） | 达标 |

### 9.1 测试环境与数据量

- 机器：Windows 11 IoT Enterprise LTSC（26100），i7-12700H（14 核 20 线程），15.7GB 内存，NVMe SSD
- 版本：Go 1.27.1、Node 24.21.0、MariaDB 12.3，数据库与前后端同机
- 数据量：`xzji_dev` 灌入 10015 篇内容（已发布且到发布时间的 9716 篇）、13222 条下载地址、19847 条内容标签关联、5004 个隐藏块、16 张图片（含各档 WebP 缩略图），由 `server/tools/seed` 生成
- 压测用生产构建：后端 `task build:server` 的 `server/bin/xzji.exe`，前台 `task build:web` 后 `node web/.output/server/index.mjs`，都不是开发模式
- Lighthouse 经一层 gzip 代理访问前台（模拟 `deploy/nginx/*.conf` 里的 `gzip on`）：Nitro 自身不压缩响应，直接访问 `.output` 会按未压缩体积计分，首页只有 80 分。这一层代理只是把 Nginx 的职责补上，生产部署本来就有

### 9.2 测试命令

```
# 后端接口命中缓存（先访问一次预热）
hey -n 5000 -c 50 "http://127.0.0.1:8080/api/v1/posts?pageSize=20"

# 前台首页命中缓存（先访问一次预热）
hey -n 3000 -c 50 http://127.0.0.1:3000/

# 详情页未命中缓存：先清空页面缓存，再依次请求 30 个不同的内容 ID，统计 time_total
curl -X POST -H "X-Purge-Timestamp: <ts>" -H "X-Purge-Signature: <sig>" http://127.0.0.1:3000/_internal/purge
curl -w "%{time_total}\n" http://127.0.0.1:3000/<id>.html

# 常驻内存（Windows）
Get-Process xzji | Select-Object WorkingSet64
Get-CimInstance Win32_Process -Filter "Name='node.exe'" | Where-Object { $_.CommandLine -like '*output/server/index.mjs*' }

# Lighthouse 移动端（默认 mobile 仿真 + 4 倍 CPU 降速 + 慢速 4G），首页和资源详情页各 3 次
npx lighthouse http://127.0.0.1:3100/ --port=9222 --only-categories=performance --output=json --output-path=home.json
npx lighthouse http://127.0.0.1:3100/1.html --port=9222 --only-categories=performance --output=json --output-path=detail.json
```

### 9.3 结论与偏差说明

- **命中页面缓存的响应**：单连接 0.6ms、并发 10 时 3.9ms，都远低于 10ms；并发 50 时 18.8ms 是排队延迟——单进程 Node 在这台机器上的吞吐上限约 2600 req/s，50 个连接排队等待就产生了十几毫秒。这是容量问题，不是单请求耗时问题。生产环境由 Nginx 承接连接与静态资源，前台可用 `pm2 -i` 或多实例水平扩展，必要时把 Nginx 的 `proxy_cache` 放在 Nuxt 前面。
- **后端接口命中缓存**：同样受回环排队影响，并发 50 时的 5.0ms 只是刚过线；单连接的 0.2ms 才反映真实处理耗时。
- **首屏 JS 100.7KB**：已确认首屏 20 个 chunk 里没有 `uqr`、Element Plus 等只在弹窗或后台使用的依赖，全部是 Vue 3 + Vue Router + Nuxt 运行时代码加本项目代码，属于框架基线。再降需要减少运行时能力（例如换更轻的渲染方案），不为 0.7KB 动。
- **图片处理时的内存**：上传 3000×2000（600 万像素）图片时，逐档编码 WebP 会让后端瞬时峰值到 118MB（优化前 171MB）。已做两件事压下来：每个档位编码后立即回收临时内存、同一时刻只处理一张图片。这是低频管理操作的瞬时值，不进常驻内存，上传完成后进程马上回落到 20MB。`media.variants` 只写有缩小价值的档位，原图宽度不够的跳过，不做放大。
- **Lighthouse 的 80 分与 97 分之别**：直接用 `node .output/server/index.mjs` 访问时首页 80 分，因为 Nitro 不压缩响应，48KB 的 HTML 和 57KB 的 CSS 都按原样传输；补上 Nginx 的 gzip（`deploy/nginx/*.conf` 里已经开好）后首页升到 97、详情页 96。也就是说**部署时前台前面必须有开 gzip 的 Nginx**，这不是可选项。TBT 只有 80–110ms，说明问题从来不在 JS 执行，而在传输体积。

M5 阶段用压测工具和 Lighthouse 实际测量，并把结果补充到本文档。

## 10. 部署

| 部分 | 产物 | 运行方式 |
|---|---|---|
| 后端 | 一个可执行文件（嵌入后台页面和迁移 SQL）+ `config.yaml` + `data/`（上传文件） | systemd 守护；启动时自动执行数据库迁移 |
| 前台 | Nuxt 构建产物 `.output/` + 环境变量（接口地址、清除缓存的密钥，见 `deploy/web.env.example`） | Node 运行，由 systemd 或 PM2 守护 |
| Nginx | `deploy/` 下提供单机和分机两套配置样例 | 负责 HTTPS、gzip 和静态图片 |

- **后端命令行**：`xzji serve`、`xzji migrate`、`xzji admin create`。
- **备份**：每天用 mysqldump 备份数据库，同时打包 `data/uploads`。
- **暂不做**：Docker 镜像、安装向导、一键升级、授权系统，放到第三阶段产品化时再做。

## 11. 开发顺序

| 里程碑 | 内容 | 验收标准 |
|---|---|---|
| M0 工程骨架 | 初始化 git 和单仓库结构；搭好 Go、Nuxt、后台三个项目的骨架；打通契约到代码的生成链路；配置、日志、数据库连接、迁移；Taskfile、CLAUDE.md、Nginx 样例 | 三个项目都能启动；前台和后台都能用生成的客户端调通健康检查接口 |
| M1 用户 | 注册、登录、找回密码、会话；跨域和 CSRF；后台登录和管理员管理 | 前台在 `www` 域名下登录后，请求 `api` 时身份有效；后台会话与前台互相隔离 |
| M2 内容 | 分类、标签、文章和资源的增删改查；编辑器；图片上传；前台列表页和详情页（服务端渲染） | 后台发布的内容在前台正常显示，查看网页源代码能看到完整的公开内容 |
| M3 访问控制 | 规则判定、隐藏块、解锁和下载接口、评论、链接失效反馈 | 各种解锁条件都按预期生效；网页源代码中看不到任何受保护的内容 |
| M4 会员与交易 | 会员等级和套餐、订单、钱包、支付宝和微信支付、补单和超时任务 | 沙箱环境走通全部支付流程，再用小额真钱各测一遍 |
| M5 SEO 与性能 | sitemap、结构化数据、页面缓存和清除通知、浏览量上报、图片优化、压测 | 达到第 9 节的指标 |
| M6 上线准备 | 上线前加固、站点设置与协议页、部署包、备份恢复脚本、本机全新安装演练、上线手册 | 照 `docs/deploy-guide.md` 能从一台空服务器把站点装起来，且已在本机用生产构建从空库完整装过一遍 |
| M7 正式部署 | 在真实服务器上按分机方式部署，配好备份和监控，跑通支付沙箱与小额真钱 | 正式站点上线运行 |

M5 的任务拆解、设计约束和验收步骤见 `docs/m5-seo-performance.md`。
M6 的任务拆解见 `docs/m6-launch-prep.md`，操作步骤见 `docs/deploy-guide.md`。

**M6（上线准备）完成情况（2026-10-05）**：

- 加固：前台页面缓存换成有上限的 LRU 驱动（500 条 / 64MB），缓存键只保留页面用到的查询参数；后端缓存加 `cache.max_bytes`（32MB）；浏览量上报按 IP 限流、pending 有上限、刷库分批；顺带修掉「详情页缓存规则 `/*.html` 在 Nitro 里永远匹配不上、详情页从未进过缓存」这个 M5 遗留缺陷
- 站点信息：后台新增「站点设置」，前台页脚显示简介与备案号，新增 `/terms`、`/privacy` 两个协议页
- 部署：`task release` 生成后端（Linux amd64）和前台两个包，各带 `.sha256`；补齐生产配置样例、生产校验和 Nginx 样例
- 备份：`deploy/scripts/backup.sh`、`restore.sh`，本机演练过备份恢复
- 演练与手册：本机空库完整安装一遍（结果和偏差见 `docs/deploy-guide.md` 第 9 节），操作手册 `docs/deploy-guide.md`
- 本阶段不做：登录服务器、真实域名与证书、支付渠道对接、发信实测，这些都归 M7

前台设计风格已于 2026-10-03 确定：以 `design/index.html`（首页与资源详情）为基准，其余页面见 `design/` 目录，视觉语言参照子比主题。前台页面从 M2 开始，按这份设计稿里的配色、圆角、组件来实现。

## 12. 测试策略

- **单元测试（必写，表驱动）**：访问规则判定、价格计算、会员时长和升级折算、钱包流水。
- **集成测试（连接本机测试库 `xzji_test`）**：
  - 支付回调的幂等性和重复回调；
  - 回调金额不符；
  - 余额并发扣款；
  - 订单超时关闭。
- **契约一致性**：后端由 oapi-codegen 的严格模式在编译期保证；前端由生成的类型在编译期保证。
- **端到端验证（M4 起）**：用本机真实浏览器（经 chrome-devtools MCP 驱动）走完主流程：注册 → 购买 → 解锁 → 下载，不引入 Playwright。
- **每个里程碑结束时**：前后台手工走一遍主流程。

涉及金额的代码，测试不通过不合并。

## 13. 业务规则（已确认）

| 事项 | 规则 |
|---|---|
| 会员升级 | 按剩余价值折算，详见下文 |
| "评论可见"的判定标准 | 评论审核通过后才解锁 |
| 退款 | 不做线上退款，由后台人工处理并写入流水，详见下文 |
| 每日下载次数的计数范围 | 只统计"靠会员身份免费下载"的次数，已单独购买的和公开资源不计 |
| 订单过期时间 | 30 分钟 |
| 会员折扣价的取整方式 | 四舍五入到分，最低 1 分 |

**会员升级的折算规则：**
- **剩余价值** = 剩余天数 × 原等级的日均价。
- **日均价**统一取该等级年卡价格 ÷ 365；没有年卡的等级，取它时长最长的有限期套餐来算。不按用户当初实际付了多少钱来算，避免优惠活动让计算变得混乱。
- **升到有限期套餐**：剩余价值 ÷ 新等级的日均价，得到折算天数（向下取整），叠加到新套餐的时长上。
- **升到永久套餐**：剩余价值直接抵扣新套餐的价格。
- **原本就是永久会员**：只能升级到更高等级的永久套餐，补两者永久价的差价。
- **会员有效期内**不能购买比当前等级低的套餐。
- **同等级从有限期升到永久**：同样按剩余价值抵扣永久套餐的价格。
- **剩余天数**按整天向下取整；剩余价值四舍五入到分；抵扣后最少支付 1 分。
- **支付成功时**按当时的会员状态重新计算到期时间（例如下单后又开通了同等级会员，就按续费叠加），避免状态在 30 分钟内变化导致重复折算。
- 折算结果写入订单快照，方便以后对账。

**退款规则：**
- 只有已支付的订单可以退款，订单改为已退款，已购内容同时收回。
- 余额支付的订单退回余额（写退款流水）；第三方支付的订单由管理员选择退回余额，或者在支付平台原路退回后在后台记录。
- 充值订单只能原路退回，同时从余额里扣回这笔充值，余额不足时不能退。
- 会员订单不自动扣减会员时长，由管理员在用户管理里另行调整。

## 14. 为后续阶段预留的设计

| 后续功能 | 第一阶段的预留 |
|---|---|
| App / 小程序 | 接口带版本号并保持向后兼容；认证层同时支持 Bearer 令牌；sessions 表有 `client` 字段 |
| 推广分佣 | `users.inviter_id` 从第一天开始记录；流水表的 `account` 字段预留 `commission` |
| 签到积分 | 沿用流水账模式，届时单独建积分流水表 |
| 微信登录 | 认证层支持多种登录方式，届时新增 `user_identities` 表（provider + openid） |
| 多台前台服务器 | 清除缓存的通知地址支持配置多个 |
| 对象存储 / CDN | 存储层抽象成接口；图片地址由 `media.base_url` 配置，切换时不用改数据 |
| 多个后端实例 | 缓存和限流抽象成接口，届时切换为 Redis 实现 |
| 搜索升级 | 搜索抽象成接口，第一阶段用 LIKE 实现 |
| AI | 第一阶段不预留，第二阶段单独设计 |
