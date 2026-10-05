# XZJI

垂直资源站。基于 WordPress 的自研主题，业务能力尽量用 WP 核心与成熟插件，主题只负责本站的外观和少量业务胶水。

> **2026-10-05 方向变更**：此前的 v1 是一套完全自研的程序（Go + Nuxt 4 + Vue 3，M0–M6 已完成并上线 www.xzji.cc）。实机跑下来开发量太大、没有生态兜底、运维复杂，改为「把你的程序最大程度转成 WordPress 主题，不再自研 WP 已有的功能」。
> 方案见 [docs/wp-theme-plan.md](docs/wp-theme-plan.md)，v1 代码封存在 `archive/v1` 分支与 `v1.0-standalone` 标签，v1 的全部文档在 [docs/archive/](docs/archive/)。

## 现状

首版（内容 + 下载 + 支付）**待开工**，当前仓库里还没有主题代码。

| 里程碑 | 状态 |
|---|---|
| M1-0 插件调研（定「下载 + 支付 + 解锁」走插件还是自研） | 待开始 |
| M1-1 ~ M1-7 环境、内容、前台页面、用户与下载、支付、SEO 与缓存、上线 | 待开始 |

## 首版做什么

| 方向 | 明细 |
|---|---|
| 内容 | 分类、标签、平台；文章与资源；资源扩展信息（版本、大小、更新日志、截图、属性表）；封面图 |
| 下载 | 每篇资源多个下载地址（网盘名、链接、提取码、解压密码），按条件可见 |
| 支付 | 单篇购买 → 订单 → 彩虹易支付 → 回调验签 → 解锁下载地址 |

用户注册登录、评论、媒体库、回收站、搜索、后台管理都由 WordPress 原生提供，不写代码。会员体系、余额钱包留到后续里程碑。

## 目录

```
theme/xzji/              WordPress 主题（首版唯一产物，待建）
design/                  设计稿：6 个页面 + xzji.css 设计变量与组件样式——主题的视觉基准
docs/
  wp-theme-plan.md        现行技术方案
  archive/                v1 自研方案的全部文档（已封存）
CLAUDE.md                 开发约定
```

### v1 存量（只读，不构建、不维护）

`server/`（Go 后端）、`web/`（Nuxt 前台）、`admin/`（Vue 后台）、`api/`（接口契约）、`packages/`、`deploy/`、`dist/`、`Taskfile.yml`、`pnpm-workspace.yaml`、`package.json`。

这些东西已经冻结，新开发一律在 `theme/` 下。需要翻 v1 的代码或跑 v1 的构建时，切到 `archive/v1` 分支。

## 文档

| 文档 | 内容 |
|---|---|
| [docs/wp-theme-plan.md](docs/wp-theme-plan.md) | 现行方案：方向变更理由、首版范围、技术选型、功能归属对照表、内容模型、主题结构、支付、部署、开发顺序、验收 |
| [docs/archive/phase1-tech-plan.md](docs/archive/phase1-tech-plan.md) | v1 技术方案（Go + Nuxt 自研） |
| [docs/archive/deploy-guide.md](docs/archive/deploy-guide.md) | v1 上线手册（1Panel、宝塔、纯命令行三条路线） |
| [docs/archive/m5-seo-performance.md](docs/archive/m5-seo-performance.md) | v1 的 SEO 与性能阶段说明与实测数据 |
| [docs/archive/m6-launch-prep.md](docs/archive/m6-launch-prep.md) | v1 的上线准备阶段说明 |
| [CLAUDE.md](CLAUDE.md) | 主题开发约定 |

## 设计基准

`design/` 下是六个页面的静态稿（首页与资源详情、分类、用户中心、会员、登录）和一份 698 行的 `xzji.css`，视觉语言参照子比主题。主题开发时这份 CSS 原样迁入，逐页对照还原，配色、圆角、阴影、渐变、间距都以它的 CSS 变量为准。
