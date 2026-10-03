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
| 状态管理 | Pinia | Vue 官方推荐 |
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
  rank                   INT UNIQUE          数字越大等级越高
  discount_rate          INT                 80 表示 8 折，100 表示不打折
  daily_download_limit   INT                 0 表示不限
  badge                  VARCHAR(255)
  status / sort

membership_plans         售卖套餐
  id / level_id
  name                   VARCHAR(32)         月卡 / 年卡 / 永久
  duration_days          INT                 0 表示永久
  price / original_price BIGINT
  status / sort

user_memberships         用户当前的会员状态（同一时间只有一个等级）
  user_id                BIGINT PK
  level_id               BIGINT
  expires_at             DATETIME NULL       NULL 表示永久
  updated_at
```

- **同等级续费**：在原到期时间上叠加时长。
- **升级到更高等级**：按剩余价值折算，规则见第 13 节。
- **到期处理**：定时任务每小时扫描一次。判断权限时也会同时校验 `expires_at`，不依赖定时任务是否及时执行。

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
  pay_method             VARCHAR(16)         wechat / alipay / balance
  status                 VARCHAR(16)         pending / paid / closed / refunded
  expires_at             DATETIME            30 分钟
  paid_at                DATETIME NULL
  created_at / updated_at

payments                 第三方支付记录
  id / order_id(INDEX)
  channel                VARCHAR(16)
  trade_no               VARCHAR(64)         UNIQUE(channel, trade_no)
  amount                 BIGINT
  raw_notify             TEXT                回调原文，排查问题用
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
4. **补单**：定时任务每分钟主动查询"已发起支付、但还没收到回调"的订单，防止回调丢失。
5. **超时关闭**：定时任务关闭过期未付的订单。
6. **前台确认结果**：支付页轮询订单状态接口，确认已支付后刷新解锁内容。

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

- Nuxt 服务端调用接口时带上 `X-Internal-Key` 请求头（前后端共享的密钥）。
- 后端据此识别出这是前台服务器发来的请求，不对它做按 IP 限流（否则所有访客的渲染请求都来自同一个 IP，会被误判为攻击）。
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

| 指标 | 目标 |
|---|---|
| 后端常驻内存 | < 100MB（不含 MySQL） |
| 前台 Nuxt 常驻内存 | < 200MB |
| 最低配置 | 分机部署时每台 1 核 1G；单机部署 1 核 2G |
| 命中页面缓存时的前台响应 | < 10ms |
| 命中缓存时的后端公开接口 | < 5ms |
| 未命中缓存时的详情页服务端渲染 | < 100ms（前后端在同一机房） |
| 首屏 JS（gzip 后） | < 100KB |
| Lighthouse 移动端性能评分 | ≥ 90 |

M5 阶段用压测工具和 Lighthouse 实际测量，并把结果补充到本文档。

## 10. 部署

| 部分 | 产物 | 运行方式 |
|---|---|---|
| 后端 | 一个可执行文件（嵌入后台页面和迁移 SQL）+ `config.yaml` + `data/`（上传文件） | systemd 守护；启动时自动执行数据库迁移 |
| 前台 | Nuxt 构建产物 `.output/` + `.env` 配置（接口地址、内部密钥、清除缓存的密钥） | Node 运行，由 systemd 或 PM2 守护 |
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
| M6 上线 | 按分机方式部署，配好备份脚本和生产环境配置 | 正式站点上线运行 |

前台设计风格已于 2026-10-03 确定：以 `design/index.html`（首页与资源详情）为基准，其余页面见 `design/` 目录，视觉语言参照子比主题。前台页面从 M2 开始，按这份设计稿里的配色、圆角、组件来实现。

## 12. 测试策略

- **单元测试（必写，表驱动）**：访问规则判定、价格计算、会员时长和升级折算、钱包流水。
- **集成测试（连接本机测试库 `xzji_test`）**：
  - 支付回调的幂等性和重复回调；
  - 回调金额不符；
  - 余额并发扣款；
  - 订单超时关闭。
- **契约一致性**：后端由 oapi-codegen 的严格模式在编译期保证；前端由生成的类型在编译期保证。
- **端到端测试（M4 起）**：用 Playwright 自动走完主流程：注册 → 购买 → 解锁 → 下载。
- **每个里程碑结束时**：前后台手工走一遍主流程。

涉及金额的代码，测试不通过不合并。

## 13. 业务规则（已确认）

| 事项 | 规则 |
|---|---|
| 会员升级 | 按剩余价值折算，详见下文 |
| "评论可见"的判定标准 | 评论审核通过后才解锁 |
| 退款 | 不做线上退款，由后台人工处理并写入流水 |
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
- 折算结果写入订单快照，方便以后对账。

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
