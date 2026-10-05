#!/usr/bin/env bash
# 用备份文件恢复数据库和上传目录。
# 目标库必须显式指定，并且加 --yes 确认；测试库（_test 结尾）一律拒绝，避免误覆盖。
set -euo pipefail

DB_HOST=${XZJI_DB_HOST:-127.0.0.1}
DB_PORT=${XZJI_DB_PORT:-3306}
DB_USER=${XZJI_DB_USER:-xzji}
DB_PASSWORD=${XZJI_DB_PASSWORD:-}
UPLOADS_DIR=${UPLOADS_DIR:-/opt/xzji/server/data/uploads}
# 容器里的 MySQL 用：MYSQL="docker exec -i 1Panel-mysql-xxx mysql"
MYSQL=${MYSQL:-mysql}

TARGET_DB=""
DB_FILE=""
UPLOADS_FILE=""
YES=0

usage() {
  cat <<'EOF'
用法：restore.sh --db <库名> --db-file <备份文件> [选项] --yes

  -D, --db <库名>        要恢复到哪个库（必填）
  -f, --db-file <文件>   db-*.sql.gz 备份文件（必填）
  -U, --uploads <文件>   uploads-*.tar.gz 备份文件（可选）
      --uploads-dir <目录> 上传目录的解压位置（默认 /opt/xzji/server/data/uploads）
  -H, --host <地址>      MySQL 地址（默认 127.0.0.1）
  -P, --port <端口>      MySQL 端口（默认 3306）
  -u, --user <账号>      MySQL 账号（默认 xzji）
      --yes              确认执行，会覆盖目标库里现有的数据
  -h, --help             显示帮助

密码用环境变量 XZJI_DB_PASSWORD 提供，不要在命令行里写（脚本写进 600 权限的
临时文件，退出时删除）。数据库在 Docker 容器里时，用 MYSQL="docker exec -i
<容器名> mysql"，并把账号密码写进容器里的 ~/.my.cnf（chmod 600），这时不要设置
XZJI_DB_PASSWORD。恢复前先备份当前数据库：脚本不会替你回滚。
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    -D|--db) TARGET_DB=$2; shift 2 ;;
    -f|--db-file) DB_FILE=$2; shift 2 ;;
    -U|--uploads) UPLOADS_FILE=$2; shift 2 ;;
    --uploads-dir) UPLOADS_DIR=$2; shift 2 ;;
    -H|--host) DB_HOST=$2; shift 2 ;;
    -P|--port) DB_PORT=$2; shift 2 ;;
    -u|--user) DB_USER=$2; shift 2 ;;
    --yes) YES=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "未知参数：$1" >&2; usage >&2; exit 2 ;;
  esac
done

die() { echo "恢复失败：$*" >&2; exit 1; }

[ -n "$TARGET_DB" ] || { usage >&2; die "必须用 --db 指定目标库"; }
[ -n "$DB_FILE" ] || { usage >&2; die "必须用 --db-file 指定备份文件"; }
[ -f "$DB_FILE" ] || die "找不到备份文件 $DB_FILE"

case "$TARGET_DB" in
  *_test) die "拒绝恢复到测试库 $TARGET_DB" ;;
esac

if [ "$YES" -ne 1 ]; then
  die "目标库 $TARGET_DB 里的现有数据会被覆盖，确认后加 --yes 重新执行"
fi

echo "检查备份文件完整性…"
gzip -t "$DB_FILE" || die "备份文件损坏或不完整：$DB_FILE"

# 凭据处理与备份脚本相同：写 600 权限的临时 defaults 文件，退出时删除
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

# 目标库必须已经存在：建库属于部署步骤，不在这里替用户决定字符集和权限
$MYSQL "${DB_ARGS[@]}" -N -B \
  -e "select schema_name from information_schema.schemata where schema_name='$TARGET_DB'" | grep -q . \
  || die "数据库 $TARGET_DB 不存在，先执行 deploy/sql/create-database.sql 建库并授权"

echo "恢复数据库 $TARGET_DB…"
gunzip -c "$DB_FILE" | $MYSQL "${DB_ARGS[@]}" "$TARGET_DB" || die "导入数据库失败"
echo "  数据库已恢复"

if [ -n "$UPLOADS_FILE" ]; then
  [ -f "$UPLOADS_FILE" ] || die "找不到上传目录备份 $UPLOADS_FILE"
  mkdir -p "$UPLOADS_DIR" || die "无法创建 $UPLOADS_DIR"
  echo "恢复上传目录到 $UPLOADS_DIR…"
  # 备份包里是第一层目录名，解压到它的上一级，解出来的目录名与备份时一致
  tar xzf "$UPLOADS_FILE" -C "$(dirname "$UPLOADS_DIR")" || die "解压上传目录失败"
  echo "  上传目录已恢复"
fi

echo "恢复完成。启动后端前确认 config.yaml 里的 db.dsn 指向 $TARGET_DB。"
