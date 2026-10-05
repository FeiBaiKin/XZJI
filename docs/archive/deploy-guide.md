> **历史文档（v1 自研方案）**：本文描述的是已封存的 Go + Nuxt 方案，2026-10-05 起不再维护，原样保留以备考。
> 现行方案见 [../wp-theme-plan.md](../wp-theme-plan.md)；v1 代码见 `archive/v1` 分支与 `v1.0-standalone` 标签。

# XZJI 上线手册

面向站长。会用面板就能照着做完，命令行的地方可以直接复制。

> 面板的菜单名称和路径以官方文档为准（1Panel：https://1panel.cn/docs/ ，宝塔：https://www.bt.cn/ ），
> 本文按 1Panel 1.x / 宝塔 Linux 面板 9.x 的常见布局书写，标注「待实机核对」的地方表示编写时
> 没能联网确认当前版本，首次部署时按面板实际界面走，走完把本文改准确。

## 1. 部署拓扑与最低配置

一台服务器（单机）或两台服务器（分机）：

| 角色 | 域名 | 最低配置 | 跑什么 |
|---|---|---|---|
| 单机 | www / api / admin / img 都在同一台 | 1 核 2G，40G 硬盘 | Nginx + MySQL + 后端 + 前台 |
| 后台服务器 | api / admin / img | 1 核 1G | Nginx + MySQL + 后端（含图片目录） |
| 前台服务器 | www | 1 核 1G | Nginx + 前台 Node 进程 |

内存参考：M5 实测后端常驻 20–29MB（图片处理瞬时峰值 118MB），前台常驻 107MB；本手册里的缓存上限
配置（后端 32MB、前台 64MB）已经算在内。1 核 2G 的单机方案够日常使用，装了 1Panel 之后要留出面板
自身和 MySQL 的开销，内存吃紧就加 swap。

域名规划：

- `www.example.com`：前台，主站；
- `api.example.com`：前台接口，浏览器和 App 直连，必须与 www 同主域，登录 Cookie 才共享；
- `admin.example.com`：后台，建议只放行自己的 IP；
- `img.example.com`：图片，直接由 Nginx 提供上传目录。

**大陆服务器要先完成 ICP 备案**，域名解析和 80/443 端口开放都要等备案通过。备案号上线后在后台
「站点设置」里填，前台页脚会自动显示。

## 2. 路线 A：1Panel

用户本人在用这条路线。1Panel 的 OpenResty 和 MySQL 都跑在容器里，这一点会带来两个坑（见 2.6、2.7），
务必看完。

### 2.1 安装应用

在「应用商店」里安装：

1. **OpenResty**（提供 Nginx）；
2. **MySQL**（8.0 或 5.7 都行，本站用不到 8.0 特有的功能）；
3. **Node.js**（版本 20 或 22，前台要用；面板里能装「Node.js 运行环境」）。

安装 MySQL 时面板会让你设置 root 密码，记下来。

### 2.2 创建数据库和账号

数据库在容器里，**从宿主机连过去时来源 IP 不是 localhost**，所以账号不能只授权给 localhost。

1. 打开「数据库 → MySQL」，新建数据库 `xzji`，字符集选 `utf8mb4`，排序规则 `utf8mb4_unicode_ci`；
2. 新建用户 `xzji`，主机选 **「所有主机」**（或 `%`），设置强密码，把这个库的权限给它。

面板里如果只能填「权限：指定数据库」，就选 `xzji`。

**待实机核对**：面板版本不同，主机一栏可能写成「本地」「任意」或一个 IP 输入框。要达到的效果是
`GRANT ALL PRIVILEGES ON xzji.* TO 'xzji'@'%'`，即 MySQL 容器的网桥来源也能连上。

如果面板没有提供「所有主机」选项，用面板自带的「终端」进入 MySQL 容器执行：

```bash
mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS xzji CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci; CREATE USER IF NOT EXISTS 'xzji'@'%' IDENTIFIED BY '把这里换成强密码'; GRANT ALL PRIVILEGES ON xzji.* TO 'xzji'@'%'; FLUSH PRIVILEGES;"
```

连接地址写 `127.0.0.1`，端口填面板给这个 MySQL 映射到宿主机的端口（在容器的端口映射里看，不是 3306）。

### 2.3 上传部署包

部署包由 `task release` 生成，放在 `dist/`：

- `xzji-server-<版本>-linux-amd64.tar.gz`
- `xzji-web-<版本>.tar.gz`

每个包旁边有同名 `.sha256`，上传后先校验，确认没传坏：

```bash
sha256sum -c xzji-server-<版本>-linux-amd64.tar.gz.sha256
sha256sum -c xzji-web-<版本>.tar.gz.sha256
```

解压到固定目录（下面的命令和 systemd 单元、Nginx 样例都按这两个路径写）：

```bash
mkdir -p /opt/xzji/server /opt/xzji/web
tar xzf xzji-server-<版本>-linux-amd64.tar.gz -C /opt/xzji/server
tar xzf xzji-web-<版本>.tar.gz -C /opt/xzji/web
chmod +x /opt/xzji/server/bin/xzji /opt/xzji/server/scripts/*.sh
```

用面板的文件管理器上传也行，解压后同样确认 `bin/xzji` 有执行权限。

创建运行服务的系统用户（不要用 root 跑）：

```bash
useradd -r -s /sbin/nologin xzji
mkdir -p /opt/xzji/server/data
chown -R xzji:xzji /opt/xzji
```

### 2.4 进程守护

**推荐用 systemd**：面板自带的守护够用，但 systemd 能在开机时先等 MySQL 起来，日志和重启策略也更清楚。
1Panel 也有「进程守护」功能（待实机核对菜单），效果类似，二选一即可。

把包里的单元文件装上去：

```bash
cp /opt/xzji/server/systemd/xzji-server.service /etc/systemd/system/
cp /opt/xzji/web/systemd/xzji-web.service /etc/systemd/system/
# 改成本机实际的用户、路径和环境变量后再启动
systemctl daemon-reload
systemctl enable --now xzji-server xzji-web
```

启动前先把配置写好（见第 5 节）。

### 2.5 反向代理与证书

在「网站 → 网站 → 创建网站」里建 **四个** 站点，类型都选 **反向代理**：

| 站点 | 代理到 | 备注 |
|---|---|---|
| www.example.com | `http://127.0.0.1:3000` | 前台 |
| api.example.com | `http://127.0.0.1:8080` | 接口 |
| admin.example.com | `http://127.0.0.1:8080` | 后台，建议加 IP 白名单 |
| img.example.com | 不放代理，见 2.6 | 图片目录直接由 OpenResty 提供 |

每个站点在面板里申请 Let's Encrypt 证书并开启 HTTPS。然后把 `deploy/nginx/single.conf`
里各站点块的其余配置补进去——1Panel 的网站配置是 Nginx 语法的，直接对着贴：

- `add_header` 那几行安全响应头；
- api 站点的 `limit_req`、`limit_req_status`、`error_page 429` 和 `@xzji_too_many`；
- api/admin 站点的 `client_max_body_size 20m`；
- www 站点的 `location /_internal/`（只允许 127.0.0.1，用于后端清缓存）。

**`/_internal/purge` 这段不能漏**：漏了后台改内容前台不会立刻更新，要等 10 分钟兜底过期。

gzip 和限流 zone 要写在 http 块，1Panel 里通常有一处全局的 Nginx 配置（待实机核对路径），把
`deploy/nginx/xzji-http.conf` 的内容贴进去。**gzip 不是可选项**：M5 实测前台前面没有 gzip 时
Lighthouse 首页只有 80 分。

### 2.6 图片目录

OpenResty 也跑在容器里，只能读挂载进去的目录。所以要达到的效果是：**OpenResty 容器能看到上传目录**。

做法：在 1Panel 建 img 站点时，把上传目录挂进 OpenResty 容器。

- 把 `media.dir` 设成容器能挂载的宿主目录，例如 `/opt/1panel/www/xzji-uploads`（面板的网站目录通常在
  `/opt/1panel/www` 下，**待实机核对**你机器上的实际前缀）；
- 在 1Panel 的「容器 → 容器列表 → OpenResty → 编辑」里加一条挂载：宿主 `/opt/1panel/www/xzji-uploads`
  → 容器内的同一个路径；
- img 站点的根目录设成容器内的那个路径，并加上 `deploy/nginx/single.conf` 里图片站点的配置
  （禁脚本、禁止访问隐藏文件、30 天缓存）。

`config.yaml` 里对应两行：

```yaml
media:
  dir: /opt/1panel/www/xzji-uploads   # 绝对路径，后端也写这里，两个容器共享
  base_url: https://img.example.com
  serve: false                         # 交给 OpenResty，不要开后端的文件服务
```

上传一张图片，访问 `https://img.example.com/2026/10/xxx.webp` 能打开就对了。

### 2.7 计划任务备份

「计划任务 → 创建计划任务」，建两条：

1. **备份数据库**：选 `xzji` 库，每天凌晨执行，保留最近 7 份；
2. **备份目录**：备份 `/opt/xzji/server/data/uploads`，同样每天执行。

再建一条 **Shell 脚本** 任务，每天跑一次包里的备份脚本，这样数据库、上传目录、配置和密钥会在同一份
备份里（详见第 6 节）。三种方式选一种即可，但**密钥文件 `secret.key` 必须另外保存一份**，不要和数据库
备份放在同一台机器上。

## 3. 路线 B：宝塔

宝塔在宿主机上直接装 Nginx 和 MySQL，比 1Panel 少一层容器，步骤更简单：

1. **网站**：建四个站点，www/api/admin 用「反向代理」指向 `127.0.0.1:3000` / `127.0.0.1:8080`，
   img 站点用「静态站点」指向上传目录；每个站点申请 Let's Encrypt 证书。
2. **MySQL**：建库 `xzji`（utf8mb4）和用户 `xzji`，访问权限选「所有人」或 `%` 都行（宝塔的 MySQL 在
   宿主机上，`127.0.0.1` 也能连）。
3. **配置文件**：把 `deploy/nginx/single.conf` 里各站点块的 `add_header`、`limit_req`、
   `client_max_body_size`、`/_internal/` 补进对应站点；把 `deploy/nginx/xzji-http.conf` 的内容贴到
   「软件商店 → Nginx → 配置修改」里的 http 块。
4. **进程守护**：用「软件商店 → 进程守护管理器」或「网站 → Node 项目」「Go 项目」都行；
   用 systemd 也可以，命令和第 2.4 节一样。绑定后端时：
   - 后端：启动命令 `/opt/xzji/server/bin/xzji serve -c /opt/xzji/server/config.yaml`，工作目录 `/opt/xzji/server`；
   - 前台：启动命令 `/usr/bin/node .output/server/index.mjs`，工作目录 `/opt/xzji/web`，环境变量照
     `deploy/web.env.example` 填。
5. **计划任务**：类型选「备份数据库」「备份目录」，路径和保留份数照第 2.7 节的思路填。

## 4. 路线 C：纯命令行

不带面板时，用包里的样例直接装：

```bash
# 1. 依赖
apt install -y nginx mariadb-server nodejs   # Debian/Ubuntu；CentOS 用 dnf

# 2. 建库建账号（把密码换掉）
mysql -u root < /opt/xzji/server/sql/create-database.sql

# 3. 放程序
mkdir -p /opt/xzji/server /opt/xzji/web
tar xzf xzji-server-<版本>-linux-amd64.tar.gz -C /opt/xzji/server
tar xzf xzji-web-<版本>.tar.gz -C /opt/xzji/web
useradd -r -s /sbin/nologin xzji && chown -R xzji:xzji /opt/xzji

# 4. 配置（见第 5 节）
cp /opt/xzji/server/config.example.yaml /opt/xzji/server/config.yaml
$EDITOR /opt/xzji/server/config.yaml

# 5. Nginx
cp /opt/xzji/server/nginx/*.conf /etc/nginx/snippets/    # xzji-proxy.conf 放这里
cp /opt/xzji/server/nginx/single.conf /etc/nginx/conf.d/xzji.conf
# xzji-http.conf 的内容贴进 /etc/nginx/nginx.conf 的 http 块
nginx -t && systemctl reload nginx

# 6. systemd
cp /opt/xzji/server/systemd/xzji-server.service /etc/systemd/system/
cp /opt/xzji/web/systemd/xzji-web.service /etc/systemd/system/
systemctl daemon-reload && systemctl enable --now xzji-server xzji-web
```

证书用 certbot 或 acme.sh 申请，路径填进 `single.conf` 的 `ssl_certificate` 两行。

## 5. 首次启动

### 5.1 后端配置

复制 `config.example.yaml` 为 `config.yaml`，逐项改（文件里有数据库密码，权限设成 600）：

| 配置项 | 怎么填 |
|---|---|
| `env` | `prod` |
| `timezone` | 服务器时区，如 `Asia/Shanghai` |
| `http.addr` | 单机 `127.0.0.1:8080`；分机改成 `0.0.0.0:8080` 并用防火墙限制来源 |
| `db.dsn` | `xzji:密码@tcp(127.0.0.1:端口)/xzji`，端口用 1Panel 映射出来的那个 |
| `log.format` | `json`（方便按字段检索） |
| `cors.allow_origins` | `https://www.example.com`，**必须是 https**，写错后端会拒绝启动 |
| `auth.cookie_domain` | `.example.com`（前面带点，www 与 api 共享登录状态） |
| `auth.cookie_secure` | `true`，生产环境必须开 |
| `mail.smtp.*` | 发信服务器的地址、端口、账号；密码建议用环境变量 `XZJI_SMTP_PASSWORD`，不要写在文件里 |
| `media.dir` | 上传目录，用绝对路径 |
| `media.base_url` | `https://img.example.com` |
| `media.serve` | `false` |
| `frontend.purge_urls` | 单机 `[http://127.0.0.1:3000]`；分机填前台服务器的内网地址 |
| `frontend.purge_secret` | 见 5.2 |
| `pay.mock` | `false` |
| `secret_key_file` | `data/secret.key`，首次启动自动生成 |

配置写错时后端会拒绝启动并打印原因，照提示改就行。`media.serve` 忘关只会打印一条告警，不影响启动。

### 5.2 两个密钥

```bash
openssl rand -hex 32
```

- 生成的值填进 `config.yaml` 的 `frontend.purge_secret`；
- 同一个值填进前台环境变量 `NUXT_PURGE_SECRET`。

两边必须**完全一致**，否则后台改内容后前台不会清缓存。

### 5.3 前台环境变量

照 `deploy/web.env.example` 逐项填。用 systemd 时改 `xzji-web.service` 里的 `Environment=`；
用宝塔「Node 项目」时填在项目设置的环境变量里。三个容易填错的：

- `NUXT_API_INTERNAL_BASE`：服务端渲染时调的接口地址，单机填 `http://127.0.0.1:8080/api/v1`，
  分机填后端服务器的内网地址；
- `NUXT_PUBLIC_API_BASE`：浏览器直连的接口地址，填 `https://api.example.com/api/v1`；
- `NUXT_PUBLIC_SITE_URL`：站点对外地址 `https://www.example.com`，末尾不要带 `/`，canonical、
  og:url 和 sitemap 都用它。

### 5.4 创建管理员

```bash
cd /opt/xzji/server
./bin/xzji admin create -email 你的邮箱
```

会提示输入两遍密码。命令行的其他用法：

```bash
./bin/xzji migrate status              # 看迁移执行到哪一版
./bin/xzji migrate up                  # 手动执行迁移（服务启动时也会自动执行）
./bin/xzji media rebuild-variants      # 给已有图片补生成缩略图
```

### 5.5 后台设置

登录 `https://admin.example.com`：

1. **站点设置**：页脚简介、ICP 备案号、公安备案号、用户协议和隐私政策正文。
   用户协议和隐私政策要自己写，页面上列了隐私政策该包含哪些内容；没填之前前台 `/terms`、`/privacy`
   显示「站长尚未填写」并且不收录；
2. **首页设置**：首页横幅和公告；
3. **会员与套餐**：先建等级（数字越大越高），再给等级建套餐；建了套餐前台会员页才会显示可购买；
4. **支付设置**：见第 6 节。

## 6. 支付上线

支付渠道没配好之前，前台会员页和付费内容都买不了。顺序：先沙箱，再小额真钱。

1. 在支付宝开放平台/微信支付商户平台申请**当面付**或**Native 支付**，拿到 AppID、商户号、私钥和
   平台公钥；
2. 在后台「支付设置」里填这些参数，私钥用 `secret_key_file` 加密存库；回调地址填
   `https://api.example.com/api/v1/pay/notify/alipay`（微信同理），**这个地址必须能从公网访问**，
   否则支付成功了订单也不会变成已支付；
3. 先按渠道的沙箱环境走一遍：下单 → 扫码 → 回调 → 订单变已支付 → 权益到账；
4. 沙箱通过后，把金额调成 0.01 元，用真钱走一遍，确认到账后再把商品价格改回来；
5. 支付补单任务每分钟跑一次，渠道回调丢了也会在一分钟内补上，验证时可以故意把回调地址改错试试。

## 7. 上线检查清单

- [ ] `pay.mock` 已关闭（`config.yaml` 里是 `false`）
- [ ] HTTPS 已生效，四个域名都能打开且证书有效
- [ ] `auth.cookie_domain` 与 `auth.cookie_secure` 正确，www 与 api 之间登录状态能带过去
- [ ] `cors.allow_origins` 是 https 且与前台域名一致
- [ ] SMTP 发信测试通过（注册页收得到验证码）
- [ ] 后台改一篇内容，前台 2 秒内更新（清除通知已打通）
- [ ] `https://www.example.com/sitemap.xml`、`/robots.txt` 能访问且地址是正式域名
- [ ] 页脚显示备案号，链接能打开
- [ ] 用户协议、隐私政策已填写
- [ ] 图片域名能打开上传的图片
- [ ] 备份脚本跑过一次，并且演练过恢复
- [ ] `secret.key` 已另外保存，和数据库备份不在同一台机器上
- [ ] 后台域名已限制来源 IP
- [ ] 装了监控的话，`GET /api/v1/health` 能用作探针

## 8. 日常运维

### 8.1 备份与恢复

包里的 `scripts/backup.sh` 会一次性备份数据库、上传目录、配置和密钥：

```bash
XZJI_DB_PASSWORD='数据库密码' \
  DB_HOST=127.0.0.1 DB_PORT=3306 DB_USER=xzji DB_NAME=xzji \
  UPLOADS_DIR=/opt/xzji/server/data/uploads \
  ./scripts/backup.sh --dir /opt/xzji/backup --keep 7
```

备份文件按 `db-<时间戳>.sql.gz`、`uploads-<时间戳>.tar.gz` 这样命名，`--keep 7` 表示只保留 7 天。
把这条命令加到面板的计划任务里，每天跑一次。

**1Panel 的 MySQL 在容器里**，`mysqldump` 命令要指定成容器里的那一个：

```bash
MYSQLDUMP="docker exec -i 1Panel-mysql-xxxx mysqldump"
```

并且在容器里的 `~/.my.cnf` 写好账号密码（`chmod 600`），脚本就不会把密码放进命令行：

```bash
docker exec -i 1Panel-mysql-xxxx sh -c 'printf "[client]\nuser=xzji\npassword=密码\n" > /root/.my.cnf && chmod 600 /root/.my.cnf'
```

恢复：

```bash
XZJI_DB_PASSWORD='数据库密码' ./scripts/restore.sh \
  --db xzji \
  --db-file /opt/xzji/backup/db-20261005-030000.sql.gz \
  --uploads /opt/xzji/backup/uploads-20261005-030000.tar.gz \
  --yes
```

恢复到**已经存在**的库，目标库要显式写出来，还要加 `--yes` 二次确认；脚本拒绝恢复到 `_test` 结尾的库。
恢复前先备份当前数据——脚本不会替你回滚。

**迁移只进不退**：升级后想回到旧版本，只能靠恢复备份，装新版本前务必先备份。

### 8.2 升级

```bash
# 1. 备份（见上）
# 2. 停服务
systemctl stop xzji-web xzji-server
# 3. 换后端：解压新包覆盖 /opt/xzji/server（config.yaml、data/ 不要覆盖）
# 4. 启动后端，自动执行数据库迁移
systemctl start xzji-server && ./bin/xzji migrate status
# 5. 换前台：解压新包覆盖 /opt/xzji/web（.output 整个替换）
systemctl start xzji-web
# 6. 确认缓存已清除：打开首页，看内容是不是新的
```

### 8.3 日志与排查

- 后端日志：`journalctl -u xzji-server -f`；面板部署时看面板的日志页面；
- 前台日志：`journalctl -u xzji-web -f`；
- 建议给 journald 设个上限（`/etc/systemd/journald.conf` 里 `SystemMaxUse=500M`），否则日志会一直涨；
- 常见问题：
  - **前台改了内容不更新**：先看后端日志里有没有清除通知失败；再看 `NUXT_PURGE_SECRET` 和
    `frontend.purge_secret` 是否一致；Nginx 的 `/_internal/` 是否只允许后端访问、有没有把后端挡住；
  - **图片 404**：`media.dir` 与 Nginx 的图片站点根目录不是同一个目录，或者容器没挂载进去；
  - **登录后刷新就掉线**：`auth.cookie_domain` 不对，或者 www 与 api 不同主域；
  - **上传大图失败**：Nginx 的 `client_max_body_size` 小于 `media.max_size_mb`；
  - **接口 429**：Nginx 的 api 限流挡下了，正常浏览不会触发，确认不是被刷。

### 8.4 证书续期

Let's Encrypt 证书 90 天有效：1Panel/宝塔在证书页面开了自动续期就不用管；用 certbot 的话执行
`certbot renew --dry-run` 确认自动续期任务正常，续期后 `systemctl reload nginx`。

## 9. 本机全新安装演练结果

2026-10-05 在本机（Windows 11 + MariaDB 12.3）用生产构建完整走过一遍，记录如下。

**做法**：新建空库 `xzji_rehearsal`，用 `env: prod` 的配置启动后端自动迁移，按第 5 节的顺序建管理员、
建分类、发布文章和资源、上传图片、配会员等级和套餐、填站点设置，再用前台匿名访问、后台改内容、
注册用户、后台充值、余额购买、解锁取下载地址，最后做备份恢复演练。

**结果**：

| 步骤 | 结果 |
|---|---|
| 自动迁移 + `migrate status` | 5 个迁移全部 applied |
| 创建管理员 | 正常 |
| 上传图片 | 生成 96/192/480/960 四档 WebP 缩略图，取到主色 `#0F0F80` |
| 前台匿名访问 | 首页、分类、标签、会员页、协议页都正常 |
| 页面缓存 | 首页/详情页/分类页重复访问 1.3–1.6ms，命中缓存 |
| 页脚备案号 | 显示文案与链接正确，公安备案查询地址自动取到数字 |
| 协议页未填写时 | 显示「站长尚未填写」并输出 noindex |
| 后台改内容 → 前台更新 | 18:38:05.045 保存，18:38:06.174 前台已重新渲染（**1.13 秒**），新标题立刻可见 |
| 注册用户 | 正常（验证码走 log 驱动，见偏差） |
| 后台调整余额 | 正常，余额流水多出 `admin_adjust` 一条 |
| 余额购买 + 解锁 + 取下载地址 | 正常，余额流水 `purchase` 一条，扣款后余额正确 |
| sitemap / robots | 地址是配置的站点地址，内容是当前已发布内容 |
| 备份恢复 | 25 张表逐表行数一致（共 29 行），5 个上传文件 MD5 全部一致 |

**偏差与原因**（本机条件所限，正式部署时按第 1–8 节做）：

1. **没有 HTTPS**：配置里 `auth.cookie_secure: true` 时，普通浏览器无法在 http 页面保存登录 Cookie，
   所以后台操作是用 Chrome 在 `http://localhost` 下完成的（Chrome 对 localhost 有特例），
   注册、下单这些浏览器端调用改用 curl + Bearer 令牌验证。正式部署必须在 HTTPS 下走一遍。
2. **没有 SMTP**：生产配置要求 `mail.driver: smtp`，本机填的是不通的地址，注册流程改用 `env: dev`
   （log 驱动，验证码打印到日志）完成，用的是同一个库。正式部署要实测收信。
3. **图片域名**：本机没有 img 域名，演练时把 `media.serve` 临时设为 `true`（启动时会打印告警），
   正式部署按 2.6 交给 Nginx。
4. **Nginx 配置没有本机校验**：Windows 上没有 nginx，`deploy/nginx/*.conf` 只做了人工复核，
   正式部署时用 `nginx -t` 验证一次。
5. **systemd、面板步骤没有实测**：本机没有 Linux，第 2、3、4 节里和面板相关的操作按面板文档书写，
   标注了「待实机核对」的地方需要首次部署时确认。

## 10. 演练脚本（备用）

想在一台干净的 Linux 机器上自己复现演练，可以照这个顺序：

```bash
createdb xzji_rehearsal                 # 建空库
vim config.yaml                         # env: prod，dsn 指向演练库
./bin/xzji serve -c config.yaml         # 自动迁移
./bin/xzji migrate -c config.yaml status
./bin/xzji admin create -email admin@example.com
# 然后照着第 9 节的表格逐项走一遍
```

演练完把演练库删掉，别留在生产库旁边。
