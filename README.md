# XZJI

脱离 WordPress 等 CMS 的现代资源站程序。Go 后端 + Nuxt 4 前台（服务端渲染）+ Vue 3 后台一体设计，面向 1000 ~ 1 万篇内容的垂直资源站。

第一阶段功能已完整实现（内容管理、访问控制、会员与交易、SEO 与性能、部署运维），可正式上线运营。

## 特点

- **快**：后端单文件部署，常驻内存 20–29MB；前台服务端渲染加路由级页面缓存；公开内容与个人内容分离，付费内容不会出现在服务端渲染的 HTML 里。
- **业务灵活**：免费、付费、会员、评论可见、隐藏块等访问规则，以及会员等级、套餐、价格全部在后台配置。
- **不被绑死**：自有数据结构，迁移 SQL 内嵌程序、启动自动升级；接口用 OpenAPI 契约生成前后端代码，漏实现或类型不符会直接编译报错，前台、App、小程序共用一套接口。

## 功能

| 模块 | 内容 |
|---|---|
| 内容 | 分类、标签、平台、文章与资源（含网盘链接、购买价格、访问条件）；TipTap 编辑器与隐藏内容块；图片上传、WebP 缩略图多尺寸；软删除与回收站 |
| 访问控制 | 规则判定（免费 / 单独购买 / 会员等级 / 评论可见 / 隐藏块）、解锁接口、下载计数与限流、评论与审核、链接失效反馈 |
| 用户 | 邮箱注册登录、找回密码、会话管理（HttpOnly Cookie + CSRF）、个人中心（订单、已购、钱包流水、资料） |
| 会员与交易 | 会员等级与套餐、升级按剩余价值折算、订单（30 分钟过期）、余额钱包与流水、支付宝与微信支付（gopay）、支付回调幂等、补单与超时任务、后台退款 |
| SEO 与性能 | sitemap.xml、robots.txt、结构化数据、canonical、页面缓存与内容变更清除通知、浏览量上报、接口响应缓存 |
| 后台 | 用户与会员管理、内容管理、评论与反馈、订单与退款、钱包流水、站点设置、首页设置、支付设置（私钥加密存库）、操作日志 |

## 技术栈

| 端 | 选型 |
|---|---|
| 后端 | Go 1.27、Gin、oapi-codegen（契约生成）、sqlc + 手写 SQL、goose（迁移）、gopay（支付）、argon2id、bluemonday |
| 前台 | Nuxt 4（SSR）、Tailwind CSS 4、openapi-fetch、uqr（支付二维码） |
| 后台 | Vue 3、Vite、TypeScript、Element Plus、TipTap，构建产物嵌入后端程序 |
| 数据库 | MySQL 8.0+ / MariaDB 10.6+ |
| 工程 | pnpm workspace、Taskfile（go-task） |

## 目录结构

```
api/                  接口契约（public.yaml 前台与 App，admin.yaml 后台）——接口的唯一依据
server/               Go 后端
  cmd/xzji/             入口：serve / migrate / admin / media / version
  db/migrations/        goose 迁移 SQL（嵌入程序）
  db/queries/           sqlc 查询 SQL
  internal/
    module/<名称>/        业务模块：model / repo / service 及测试
    httpapi/              路由、中间件、public 与 admin 接口实现、后台页面托管
    platform/             基础设施（db、cache、mail 等）
    apigen/ dbgen/        生成代码，禁止手改
web/                  Nuxt 4 前台（app/ 为源码目录）
admin/                Vue 3 + Element Plus 后台，构建后嵌入后端程序
packages/api-client/  从契约生成的 TS 类型与请求客户端，web 和 admin 共用
design/               前台设计稿与设计变量（xzji.css）
deploy/               Nginx、systemd、建库 SQL、备份恢复脚本等部署样例
docs/                 技术方案、各阶段开发说明、上线手册
dist/                 task release 生成的部署包
```

## 快速开始

### 环境要求

- Go 1.27+
- Node.js 22.12+ 与 pnpm 12
- MySQL 8.0+ 或 MariaDB 10.6+
- [go-task](https://taskfile.dev/)（`task` 命令）

### 步骤

```bash
# 1. 安装代码生成工具（oapi-codegen、sqlc）和前端依赖
task tools
task install
```

```bash
# 2. 建库（开发库，参考 deploy/sql/create-database.sql 按需改库名）
mysql -uroot -p -e "CREATE DATABASE xzji_dev CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"
```

```bash
# 3. 复制配置并改 db.dsn 里的数据库账号密码
cp server/config.example.yaml server/config.yaml
```

```bash
# 4. 启动三端（各开一个终端）
task dev:server   # 后端 http://127.0.0.1:8080
task dev:web      # 前台 http://localhost:3000
task dev:admin    # 后台 http://localhost:5173/admin/
```

后端启动时自动执行数据库迁移。首次使用需要创建管理员：

```bash
cd server && go run ./cmd/xzji admin create -email 你的邮箱
```

开发环境在 `server/config.yaml` 里保持 `pay.mock: true`，支付走模拟渠道，支付弹窗会出现"模拟支付成功"按钮；验证码邮件走 `mail.driver: log`，直接看后端日志。真实渠道参数在后台「支付设置」里填写。

### 集成测试

复制 `server/.env.test.example` 为 `server/.env.test`，把 `XZJI_TEST_DB_DSN` 指向一个**库名以 `_test` 结尾**的库（测试会清空它），然后运行：

```bash
task test
```

## 常用命令

| 命令 | 用途 |
|---|---|
| `task dev:server` / `dev:web` / `dev:admin` | 启动三端 |
| `task gen` | 由契约和 SQL 重新生成代码（改过 `api/*.yaml` 或 `db/queries/*.sql` 后执行） |
| `task migrate -- status` | 查看迁移状态；`task migrate:create -- add_users` 新建迁移 |
| `task test` | go vet + go test + 前端类型检查 |
| `task check:gen` | 检查生成代码与契约、SQL 是否一致（提交前跑） |
| `task build` | 构建后台并嵌入后端，构建后端与前台 |
| `task release` | 打包 Linux 部署产物到 `dist/`（各带 `.sha256`） |

后端程序自身的命令：

```bash
cd server && go run ./cmd/xzji help
```

## 部署

上线操作见 [docs/deploy-guide.md](docs/deploy-guide.md)，覆盖 1Panel、宝塔、纯命令行三条路线，含域名与 Nginx 规划、备份恢复、升级流程和上线检查清单。

拓扑上是单机（1 核 2G）或分机（后台服务器 + 前台服务器）两种。后端编译成单个可执行文件、内嵌后台页面，前台由 Node 运行 Nuxt 产物，都用 systemd 或面板守护，Nginx 按 `www` / `api` / `admin` / `img` 域名反代。

## 文档

| 文档 | 内容 |
|---|---|
| [docs/phase1-tech-plan.md](docs/phase1-tech-plan.md) | 第一阶段技术方案：架构、数据库设计、业务规则、性能目标、开发顺序 |
| [docs/m5-seo-performance.md](docs/m5-seo-performance.md) | SEO 与性能阶段的任务拆解与实测结果 |
| [docs/m6-launch-prep.md](docs/m6-launch-prep.md) | 上线准备阶段的任务拆解 |
| [docs/deploy-guide.md](docs/deploy-guide.md) | 上线手册：部署、运维、备份恢复、升级 |
| [CLAUDE.md](CLAUDE.md) | 开发约定：接口、数据库、模块边界、代码风格 |

## 开发约定摘要

- 新增或修改接口，先改 `api/*.yaml`，再 `task gen`，最后写实现；不绕过契约手写路由。
- 表结构只能通过新增迁移文件修改，已发布的迁移不再改动；查询写在 `db/queries/*.sql` 由 sqlc 生成。
- 金额一律用 BIGINT 存"分"，时间存 UTC；余额只能通过写流水变动。
- 业务模块之间只通过 service 调用，不直接读写其他模块的表。
- 注释、提交信息、文档用中文。改过契约或 SQL 先跑 `task check:gen`，提交前跑 `task test`。

完整约定见 [CLAUDE.md](CLAUDE.md)。
