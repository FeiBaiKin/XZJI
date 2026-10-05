#!/usr/bin/env bash
# 备份数据库、上传目录、后端配置和密钥文件。
# 任何一步失败都返回非零；数据库密码写进 600 权限的临时文件传给客户端，
# 不放在命令行里（否则会出现在进程列表），脚本退出时删掉。
set -euo pipefail

BACKUP_DIR=${BACKUP_DIR:-/opt/xzji/backup}
KEEP_DAYS=${KEEP_DAYS:-7}
DB_HOST=${XZJI_DB_HOST:-127.0.0.1}
DB_PORT=${XZJI_DB_PORT:-3306}
DB_USER=${XZJI_DB_USER:-xzji}
DB_NAME=${XZJI_DB_NAME:-xzji}
DB_PASSWORD=${XZJI_DB_PASSWORD:-}
UPLOADS_DIR=${UPLOADS_DIR:-/opt/xzji/server/data/uploads}
CONFIG_FILE=${CONFIG_FILE:-/opt/xzji/server/config.yaml}
SECRET_KEY=${SECRET_KEY:-/opt/xzji/server/data/secret.key}
# 容器里的 MySQL（如 1Panel）用：MYSQLDUMP="docker exec -i 1Panel-mysql-xxx mysqldump"
MYSQLDUMP=${MYSQLDUMP:-mysqldump}

usage() {
  cat <<'EOF'
用法：backup.sh [选项]

  -d, --dir <目录>       备份输出目录（默认 /opt/xzji/backup）
  -k, --keep <天数>      保留多少天，超出的自动删除（默认 7，0 表示不删）
  -D, --db <库名>        数据库名（默认 xzji）
  -H, --host <地址>      MySQL 地址（默认 127.0.0.1）
  -P, --port <端口>      MySQL 端口（默认 3306）
  -u, --user <账号>      MySQL 账号（默认 xzji）
  -U, --uploads <目录>   上传目录（默认 /opt/xzji/server/data/uploads）
  -c, --config <文件>    后端配置文件（默认 /opt/xzji/server/config.yaml）
  -s, --secret-key <文件> 密钥文件（默认 /opt/xzji/server/data/secret.key）
  -h, --help             显示帮助

密码用环境变量 XZJI_DB_PASSWORD 提供，不要写在命令行里（脚本会写进一个
600 权限的临时文件，退出时删除）。数据库在 Docker 容器里时，用
MYSQLDUMP="docker exec -i <容器名> mysqldump"，并把账号密码写进容器里的
~/.my.cnf（chmod 600），这时不要设置 XZJI_DB_PASSWORD。
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    -d|--dir) BACKUP_DIR=$2; shift 2 ;;
    -k|--keep) KEEP_DAYS=$2; shift 2 ;;
    -D|--db) DB_NAME=$2; shift 2 ;;
    -H|--host) DB_HOST=$2; shift 2 ;;
    -P|--port) DB_PORT=$2; shift 2 ;;
    -u|--user) DB_USER=$2; shift 2 ;;
    -U|--uploads) UPLOADS_DIR=$2; shift 2 ;;
    -c|--config) CONFIG_FILE=$2; shift 2 ;;
    -s|--secret-key) SECRET_KEY=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "未知参数：$1" >&2; usage >&2; exit 2 ;;
  esac
done

die() { echo "备份失败：$*" >&2; exit 1; }

case "$DB_NAME" in
  *_test) die "拒绝备份测试库 $DB_NAME" ;;
esac

# 数据库凭据。MariaDB 10 之后的客户端不再读 MYSQL_PWD，所以写一个 600 的临时
# defaults 文件交给客户端；没提供密码时假定客户端自己能认证（如容器里的 ~/.my.cnf）。
DEFAULTS_FILE=""
cleanup() { [ -n "$DEFAULTS_FILE" ] && rm -f "$DEFAULTS_FILE"; }
trap cleanup EXIT

DB_ARGS=(-h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER")
if [ -n "$DB_PASSWORD" ]; then
  DEFAULTS_FILE=$(mktemp) || die "无法创建临时文件"
  chmod 600 "$DEFAULTS_FILE"
  printf '[client]\nuser=%s\npassword=%s\nhost=%s\nport=%s\n' \
    "$DB_USER" "$DB_PASSWORD" "$DB_HOST" "$DB_PORT" > "$DEFAULTS_FILE" || die "写入临时凭据文件失败"
  DB_ARGS=(--defaults-extra-file="$DEFAULTS_FILE")
fi

mkdir -p "$BACKUP_DIR" || die "无法创建备份目录 $BACKUP_DIR"
STAMP=$(date +%Y%m%d-%H%M%S)

echo "开始备份：库=$DB_NAME 目录=$BACKUP_DIR 时间戳=$STAMP"

# 1. 数据库。--single-transaction 保证 InnoDB 表在备份期间不锁表。
# --defaults-extra-file 必须是客户端的第一个参数，所以放在其他选项前面。
# shellcheck disable=SC2086  # MYSQLDUMP 里可能带 docker exec 等前缀，需要按空格拆开
if ! $MYSQLDUMP "${DB_ARGS[@]}" --single-transaction --quick --default-character-set=utf8mb4 \
      "$DB_NAME" | gzip > "$BACKUP_DIR/db-$STAMP.sql.gz"; then
  rm -f "$BACKUP_DIR/db-$STAMP.sql.gz"
  die "导出数据库失败，检查地址、账号和各库权限（MYSQLDUMP=$MYSQLDUMP）"
fi
echo "  数据库 -> db-$STAMP.sql.gz"

# 2. 上传目录
if [ -d "$UPLOADS_DIR" ]; then
  if ! tar czf "$BACKUP_DIR/uploads-$STAMP.tar.gz" -C "$(dirname "$UPLOADS_DIR")" "$(basename "$UPLOADS_DIR")"; then
    die "打包上传目录失败：$UPLOADS_DIR"
  fi
  echo "  上传目录 -> uploads-$STAMP.tar.gz"
else
  echo "  上传目录 $UPLOADS_DIR 不存在，跳过（第一次部署时正常）"
fi

# 3. 配置和密钥。密钥丢了要在后台重新填写所有支付参数，所以单独留一份
if [ -f "$CONFIG_FILE" ]; then
  cp "$CONFIG_FILE" "$BACKUP_DIR/config-$STAMP.yaml" || die "复制配置文件失败"
  chmod 600 "$BACKUP_DIR/config-$STAMP.yaml"
  echo "  配置文件 -> config-$STAMP.yaml（内含数据库密码，权限 600）"
else
  die "找不到配置文件 $CONFIG_FILE"
fi
if [ -f "$SECRET_KEY" ]; then
  cp "$SECRET_KEY" "$BACKUP_DIR/secret-$STAMP.key" || die "复制密钥文件失败"
  chmod 600 "$BACKUP_DIR/secret-$STAMP.key"
  echo "  密钥文件 -> secret-$STAMP.key（不要和数据库备份放在同一台机器上）"
else
  echo "  密钥文件 $SECRET_KEY 不存在，跳过（首次启动前正常）"
fi

# 4. 清理过期备份
if [ "$KEEP_DAYS" -gt 0 ]; then
  removed=$(find "$BACKUP_DIR" -maxdepth 1 -type f \
      \( -name 'db-*.sql.gz' -o -name 'uploads-*.tar.gz' -o -name 'config-*.yaml' -o -name 'secret-*.key' \) \
      -mtime "+$KEEP_DAYS" -print -delete | wc -l)
  echo "  清理 $KEEP_DAYS 天前的备份，删除 $removed 个文件"
fi

echo "备份完成：$BACKUP_DIR"
ls -1sh "$BACKUP_DIR" | tail -n 8
