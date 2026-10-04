# XZJI 开发约定

垂直资源站程序：Go 后端（接口 + 内嵌后台页面）+ Nuxt 前台（服务端渲染）+ Vue 后台。
方案与业务规则以 `docs/phase1-tech-plan.md` 为准；前台视觉以 `design/` 下的设计稿和 `design/xzji.css` 为准。

## 目录

```
api/             接口契约（public.yaml 前台与 App，admin.yaml 后台）——接口的唯一依据
server/          Go 后端
  cmd/xzji/        入口：serve / migrate / version
  db/migrations/   goose 迁移 SQL（嵌入程序）
  db/queries/      sqlc 查询 SQL
  internal/
    module/<名称>/   业务模块：model / repo / service 及测试
    httpapi/         路由、中间件、public 与 admin 接口实现、后台页面托管
    platform/        基础设施（db 等）
    apigen/ dbgen/   生成代码，禁止手改
web/             Nuxt 4 前台（app/ 为源码目录）
admin/           Vue 3 + Element Plus 后台，构建后嵌入后端程序
packages/api-client/  从契约生成的 TS 类型与请求客户端，web 和 admin 共用
deploy/          Nginx、systemd、建库 SQL 等部署样例
```

## 常用命令（go-task）

- `task dev:server` / `task dev:web` / `task dev:admin`：启动三端（8080 / 3000 / 5173）
- `task gen`：重新生成接口代码与数据库访问代码
- `task migrate -- status`、`task migrate:create -- add_users`
- `task test`：go vet + go test + 前端类型检查
- `task build`：构建后台并嵌入后端，构建后端与前台
- 首次环境：`task tools`、`task install`，复制 `server/config.example.yaml` 为 `server/config.yaml`
- 集成测试连接 `server/.env.test` 中的 `XZJI_TEST_DB_DSN`（库名必须以 `_test` 结尾，测试会清空它），复制 `.env.test.example` 后修改
- 创建管理员：`cd server && go run ./cmd/xzji admin create -email 你的邮箱`
- 支付：开发环境在 `config.yaml` 设 `pay.mock: true`，微信、支付宝走模拟渠道，支付弹窗出现“模拟支付成功”按钮；真实渠道参数在后台“支付设置”填写，私钥用 `secret_key_file` 加密存库

## 必须遵守的规则

**接口**
- 新增或修改接口，先改 `api/*.yaml`，再 `task gen`，最后写实现。不允许绕过契约手写路由。
- 需要登录的接口在契约里声明 `security`，`task gen:api` 会据此生成 `SecuredOperations`，由路由统一拦截未登录请求；不要在 handler 里自己判断。
- 前台接口（`/api/v1`）只能做向后兼容的变更；不兼容变更走新版本路径。
- 错误统一返回 `{"error":{"code","message"}}`，在 `httpapi/apierr` 定义业务错误；内部错误不向客户端暴露细节。

**数据库**
- 表结构只能通过新增迁移文件修改，已经发布的迁移文件不再改动。
- 查询写在 `db/queries/*.sql` 由 sqlc 生成；后台多条件筛选可用 squirrel 拼接，必须参数绑定。不用 ORM。
- 金额一律 BIGINT 存"分"；时间存 UTC（连接已强制 UTC）；字符集 utf8mb4。
- 余额只能通过写流水变动，同一事务内先写 `wallet_ledger` 再改 `wallets.balance`（用 `wallet.Service.ChangeTx`）。
- 跨模块的事务（如支付成功后发放权益）由 service 提供接收 `*sql.Tx` 的方法，加锁顺序固定为订单 → 钱包 → 会员/已购，避免死锁。
- 订单状态更新必须带原状态条件（`WHERE id=? AND status='pending'`），以影响行数判断是否成功，保证幂等。

**模块边界**
- 业务模块之间只通过 service 调用，不直接读写其他模块的表。
- handler 只做参数转换和调用 service，业务逻辑放在 service。

**前台**
- 服务端渲染只渲染公开内容，调用接口时不带用户身份；个人数据、隐藏内容、下载地址在浏览器端请求接口获取。
- 付费内容永远不能出现在服务端渲染的 HTML 里。
- 页面样式用 Tailwind 实现；配色、圆角、阴影、渐变等设计变量以 `design/xzji.css` 为参照映射到 Tailwind 主题，组件外观对照设计稿还原。

**测试**
- 访问规则、价格计算、会员时长与升级折算、钱包流水必须有表驱动单元测试。
- 涉及金额的代码测试不通过不合并。

**代码风格**
- 注释、提交信息、文档用中文；技术名词、命令、配置键名保持原文。
- 注释精炼，只说明用途和不显而易见的原因。
- 改动提交前运行 `task test`，改过契约或 SQL 再运行 `task check:gen`。
