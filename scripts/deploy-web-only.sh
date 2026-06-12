#!/bin/bash

# ============================================================================
# Zink Wallet - 前端 Web 单独部署脚本
#
# 此脚本仅部署前端 admin-web 服务，不会影响后端服务、数据库等
#
# 功能：
# 1. 拉取 web/admin 子模块最新代码
# 2. 更新 Swagger 文档（前端构建依赖）
# 3. 重新构建 admin-web 镜像（无缓存）
# 4. 仅重启 admin-web 服务
#
# 可选环境变量：
# - SKIP_SWAGGER_GEN=1：跳过 Swagger 文档生成（不推荐，前端构建需要它）
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/deploy/admin/docker-compose.yml"
ENV_FILE="$PROJECT_ROOT/deploy/admin/.env"
COMPOSE_PROJECT="${COMPOSE_PROJECT:-admin}"

echo "=========================================="
echo "  Zink Wallet - 前端 Web 部署"
echo "=========================================="
echo ""
echo "⚠️  注意：此脚本仅更新前端服务，不会影响后端"
echo ""
echo "项目根目录: $PROJECT_ROOT"
echo ""

# 验证关键路径是否存在
if [[ ! -d "$PROJECT_ROOT/web/admin" ]]; then
  echo "[ERROR] 前端目录不存在: $PROJECT_ROOT/web/admin"
  echo "        请确认项目结构是否正确"
  exit 1
fi

if [[ ! -f "$COMPOSE_FILE" ]]; then
  echo "[ERROR] Docker Compose 文件不存在: $COMPOSE_FILE"
  echo "        请确认部署配置是否正确"
  exit 1
fi

cd "$PROJECT_ROOT"

# 1. 拉取 web/admin 子模块最新代码
echo ""
echo "[Step 1/4] 拉取前端代码（web/admin 子模块）..."
cd "$PROJECT_ROOT/web/admin"
git pull || {
  echo "[WARN] web/admin git pull 失败，继续使用当前代码..."
}
cd "$PROJECT_ROOT"

# 2. 更新 Swagger 文档（前端构建依赖）
echo ""
echo "[Step 2/4] 更新 Swagger 文档（前端构建需要）..."
if [[ "${SKIP_SWAGGER_GEN:-0}" == "1" ]]; then
  echo "[SKIP] 已设置 SKIP_SWAGGER_GEN=1，跳过 Swagger 生成"
  echo "       警告：如果 admin.swagger.json 不存在或过期，前端构建可能失败"
else
  if [[ ! -x "$PROJECT_ROOT/scripts/sync-swagger-http-yaml.sh" ]]; then
    echo "[ERROR] 未找到可执行脚本：$PROJECT_ROOT/scripts/sync-swagger-http-yaml.sh"
    exit 1
  fi
  "$PROJECT_ROOT/scripts/sync-swagger-http-yaml.sh"
  echo "[OK] Swagger 文档已更新"
fi

# 3. 重新构建前端镜像（无缓存）
echo ""
echo "[Step 3/4] 重新构建前端镜像..."
echo "       这可能需要几分钟，请耐心等待..."
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --env-file "$ENV_FILE" build --no-cache admin-web

# 4. 仅重启前端服务
echo ""
echo "[Step 4/4] 重启前端服务..."
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d --force-recreate admin-web

echo ""
echo "=========================================="
echo "  ✅ 前端部署完成！"
echo "=========================================="
echo ""
echo "查看前端服务状态:"
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --env-file "$ENV_FILE" ps admin-web
echo ""
echo "查看前端实时日志:"
echo "  docker compose -p $COMPOSE_PROJECT -f $COMPOSE_FILE --env-file $ENV_FILE logs -f admin-web"
echo ""
