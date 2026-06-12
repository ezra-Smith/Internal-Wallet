#!/bin/bash

# ============================================================================
# Zink Wallet - 数据库迁移脚本
#
# 执行 deploy/docker/init-db/ 目录下的 SQL 迁移文件（幂等）
#
# 用法：
#   ./scripts/migrate-db.sh              # 使用默认配置
#   ./scripts/migrate-db.sh --dry-run    # 仅列出将执行的文件，不实际执行
#
# 环境变量：
#   COMPOSE_PROJECT  - Docker Compose 项目名（默认：admin）
#   DB_NAME          - 数据库名（默认：crypto_wallet）
#   DB_USER          - 数据库用户（默认：root）
#   DB_PASS          - 数据库密码（默认：root123456）
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/deploy/admin/docker-compose.yml"
ENV_FILE="$PROJECT_ROOT/deploy/admin/.env"
COMPOSE_PROJECT="${COMPOSE_PROJECT:-admin}"
INIT_DB_DIR="$PROJECT_ROOT/deploy/docker/init-db"

# 数据库连接参数
DB_NAME="${DB_NAME:-crypto_wallet}"
DB_USER="${DB_USER:-root}"
DB_PASS="${DB_PASS:-root123456}"

# 解析命令行参数
DRY_RUN=0
while [ $# -gt 0 ]; do
  case "$1" in
    --dry-run)
      DRY_RUN=1
      ;;
    --help|-h)
      echo "用法: $0 [选项]"
      echo ""
      echo "选项:"
      echo "  --dry-run    仅列出将执行的迁移文件，不实际执行"
      echo "  --help, -h   显示此帮助信息"
      exit 0
      ;;
    *)
      echo "[ERROR] 未知参数: $1" >&2
      echo "使用 --help 查看帮助" >&2
      exit 1
      ;;
  esac
  shift
done

# 检查迁移目录是否存在
if [ ! -d "$INIT_DB_DIR" ]; then
  echo "[ERROR] 迁移目录不存在: $INIT_DB_DIR" >&2
  exit 1
fi

# Dry-run 模式：仅列出文件
if [ "$DRY_RUN" -eq 1 ]; then
  echo "[db] Dry-run 模式：以下迁移文件将被执行"
  echo ""
  # 文件命名约定：NN-*.sql（00-99 范围，支持 01- 开头）
  for file in "$INIT_DB_DIR"/[0-9][0-9]-*.sql; do
    [ -e "$file" ] || continue
    echo "  - $(basename "$file")"
  done
  echo ""
  echo "[db] 共 $(ls -1 "$INIT_DB_DIR"/[0-9][0-9]-*.sql 2>/dev/null | wc -l | tr -d ' ') 个文件"
  exit 0
fi

# 获取 MariaDB 容器 ID
MARIADB_CID="$(docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --env-file "$ENV_FILE" ps -q mariadb 2>/dev/null || true)"
if [ -z "$MARIADB_CID" ]; then
  echo "[ERROR] MariaDB 容器未运行" >&2
  echo "[ERROR] 请先启动基础设施: docker compose -p $COMPOSE_PROJECT -f $COMPOSE_FILE up -d mariadb" >&2
  exit 1
fi

# 等待 MariaDB 就绪
echo "[db] 检查 MariaDB 连接..."
MAX_RETRIES=30
RETRY_COUNT=0
while ! docker exec "$MARIADB_CID" mysql -u"$DB_USER" -p"$DB_PASS" -e "SELECT 1" >/dev/null 2>&1; do
  RETRY_COUNT=$((RETRY_COUNT + 1))
  if [ "$RETRY_COUNT" -ge "$MAX_RETRIES" ]; then
    echo "[ERROR] MariaDB 连接超时" >&2
    exit 1
  fi
  echo "[db] 等待 MariaDB 就绪... ($RETRY_COUNT/$MAX_RETRIES)"
  sleep 1
done

# 确保数据库存在
echo "[db] 确保数据库 $DB_NAME 存在..."
docker exec "$MARIADB_CID" mysql -u"$DB_USER" -p"$DB_PASS" \
  -e "CREATE DATABASE IF NOT EXISTS ${DB_NAME} DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" \
  >/dev/null 2>&1 || true

# 定义 SQL 执行函数
apply_sql() {
  local file="$1"
  if [ ! -f "$file" ]; then
    echo "[ERROR] SQL 文件未找到: $file" >&2
    return 1
  fi
  echo "[db] 执行迁移: $(basename "$file")"
  if ! docker exec -i "$MARIADB_CID" mysql -u"$DB_USER" -p"$DB_PASS" "$DB_NAME" <"$file" 2>&1; then
    echo "[ERROR] 迁移失败: $(basename "$file")" >&2
    return 1
  fi
}

echo "[db] 开始执行 init-db 迁移（幂等文件）..."
echo ""

# 按数字顺序执行所有 NN-*.sql 迁移文件
# 文件命名约定：NN-*.sql（00-99 范围，支持 01- 开头）
migration_count=0
migration_failed=0
for file in "$INIT_DB_DIR"/[0-9][0-9]-*.sql; do
  [ -e "$file" ] || continue  # 处理无匹配文件的情况
  if apply_sql "$file"; then
    ((migration_count++)) || true
  else
    ((migration_failed++)) || true
    # 继续执行其他迁移，但记录失败
  fi
done

echo ""
if [ "$migration_failed" -gt 0 ]; then
  echo "=========================================="
  echo "  ⚠️  数据库迁移完成（有错误）"
  echo "=========================================="
  echo "成功: $migration_count 个文件"
  echo "失败: $migration_failed 个文件"
  echo ""
  echo "请检查上述错误并手动处理"
  exit 1
else
  echo "=========================================="
  echo "  ✅ 数据库迁移完成"
  echo "=========================================="
  echo "共执行 $migration_count 个迁移文件"
fi
