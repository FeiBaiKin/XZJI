# XZJI 开发约定

垂直资源站，基于 WordPress 的自研主题。方案以 `docs/wp-theme-plan.md` 为准；视觉以 `design/` 下的设计稿和 `design/xzji.css` 为准。

## 两条硬原则

1. **能交给 WordPress 的，一律不自研。** WP 核心有的（用户、评论、媒体、分类法、回收站、区块编辑器、REST API、后台）直接用；成熟插件有的（SEO、缓存、SMTP、备份、安全、图片优化）装插件。
2. **主题只做两件事**：本站的外观，以及 WP 生态里确实没有、又不值得为它装一整套插件的最小业务胶水。

动手写代码前先按这个顺序问一遍：WP 核心有没有 → 成熟插件有没有 → 主题写最小实现。跳过前两步直接开写，就是 v1 走过的弯路。

## 目录

```
theme/xzji/      WordPress 主题（首版唯一产物）
design/          设计稿与 design/xzji.css——主题的视觉基准，只读参考
docs/            方案与文档
  wp-theme-plan.md  现行方案
  archive/          v1 自研方案的全部文档（已封存）
```

**v1 存量**：`server/`（Go）、`web/`（Nuxt）、`admin/`（Vue）、`api/`、`packages/`、`deploy/`、`dist/`、`Taskfile.yml`、`pnpm-workspace.yaml`、`package.json`。这是已冻结的 Go + Nuxt 自研版本，**只读参考，不构建、不维护、不改动**；完整代码在 `archive/v1` 分支。

## 主题开发

- **经典主题（PHP 模板），不引入构建工具**。CSS 和 JS 是原生文件，改完即生效；不装 Node 工具链，不用 Tailwind。
- 模板层级用 WP 约定：`front-page.php`、`single-resource.php`、`category.php`、`taxonomy-platform.php`、`search.php`、`404.php`，公共片段放 `template-parts/`。
- 逻辑放 `inc/`，`functions.php` 只做引导；模板文件只做展示，不写业务逻辑。
- 前缀：函数、钩子、选项、文章类型、分类法、meta key 一律带 `xzji_` 前缀，避免与插件冲突。
- 文本域统一 `xzji`，面向用户的字符串用 `__()` / `esc_html__()` 包起来。

**输出与安全**

- 所有输出转义：文本 `esc_html()`、属性 `esc_attr()`、URL `esc_url()`、允许 HTML 处 `wp_kses_post()`。不允许直接 `echo` 变量。
- 表单和所有改数据的请求带 `wp_nonce_field()` / `check_admin_referer()`。
- 入库前校验：`sanitize_text_field()`、`absint()`、`sanitize_key()`，按字段类型选。
- 数据库查询用 `$wpdb->prepare()`，不拼接 SQL；能用 `WP_Query` / `get_posts()` 就不手写 SQL。
- **受保护内容不能出现在未解锁访客的 HTML 里**。付费墙在服务端判断——`the_content` 过滤里求值短代码，无权时只输出占位符和购买按钮，不输出内容本身。不允许用 CSS 或 JS 隐藏来充当付费墙。改完必须**查看网页源代码**确认，只看页面表现不算验证通过。

**缓存**

- 不要假设页面每次都重新渲染。付费解锁依赖登录态，缓存插件必须配成登录用户不命中页面缓存。
- 内容变更后用 `clean_post_cache()` 或缓存插件的清理钩子，别指望改完页面立刻更新。

**字段与内容**

- ACF 字段组用 PHP 注册（`acf_add_local_field_group`），定义进版本库，不从后台导出 JSON。
- 短代码：`[xzji_hide]` 登录后可见、`[xzji_paid]` 购买后可见。
- 资源用自定义文章类型 `resource`，文章用内置 `post`，两者共用 `category` 与 `post_tag`；平台是仅挂 `resource` 的分类法 `platform`。

**支付**

- 金额一律用「分」存储，展示时格式化。
- 回调必须验签；订单状态更新带原状态条件（`WHERE ... AND status = 'pending'`），以影响行数判断是否成功，重复回调直接返回成功、不重复发放。
- 订单 30 分钟过期；补单以易支付 `api.php` 查单为准。

## 视觉

- 配色、圆角、阴影、渐变、间距一律以 `design/xzji.css` 顶部那段 CSS 变量为准，不凭感觉调。
- `design/xzji.css`、`design/xzji.js`、`design/icons.svg` 迁入主题后**原样不动**；主题自己的补充样式单独放 `assets/css/theme.css`，保持基准文件干净。
- 每个页面与 `design/` 对应页面逐页比对。

## 代码风格

- 注释、提交信息、文档用中文；技术名词、命令、配置键名、WP 函数名保持原文。
- 注释精炼，只说明用途和不显而易见的原因。
- 遵循 WordPress PHP 编码标准：函数名全小写下划线、Yoda 条件、大括号不换行。
- 提交前自查：`php -l` 过语法、输出是否都转义、付费内容有没有泄漏到源码里。

## 测试与验收

- 在真实 WordPress 环境里走通，不用 mock 代替实际验证。
- 支付相关改动必须用 1 分钱真钱走一次完整流程（下单 → 支付 → 回调 → 解锁），验证步骤写进提交说明。
- 涉及金额的代码，测试不通过不合并。
