#!/bin/bash

# ============================================================================
# Internal Wallet - 快速更新脚本（默认跳过 Swagger 校验）
#
# 此脚本用于日常部署更新，不会影响数据库和 Redis 数据
# 默认跳过 Swagger 生成产物校验（SKIP_SWAGGER_CHECK=1）
# 更多选项请使用: ./deploy.sh help
#
# 可选环境变量：
# - SKIP_GEN_CHECK=1：跳过 Step 2/5（proto）和 Step 3/5（Swagger）生成产物校验（不推荐）
# - SKIP_PROTO_GEN_CHECK=1：仅跳过 Step 2/5（proto）生成产物校验（不推荐）
# - SKIP_SWAGGER_CHECK=0：强制启用 Swagger 校验（覆盖默认行为）
# ============================================================================

# 默认跳过 Swagger 校验（可通过设置 SKIP_SWAGGER_CHECK=0 覆盖）
export SKIP_SWAGGER_CHECK="${SKIP_SWAGGER_CHECK:-1}"

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/deploy/admin/docker-compose.yml"
ENV_FILE="$PROJECT_ROOT/deploy/admin/.env"
COMPOSE_PROJECT="${COMPOSE_PROJECT:-admin}"

fail_if_git_dirty() {
  local title="${1:-}"
  local check_pattern="${2:-}"  # 可选：只检查匹配特定模式的文件
  local changes
  
  if [[ -n "$check_pattern" ]]; then
    # 只检查匹配模式的文件
    changes=$(git status --porcelain | grep -E "$check_pattern" || true)
  else
    # 检查所有文件
    changes=$(git status --porcelain)
  fi
  
  if [[ -n "$changes" ]]; then
    echo ""
    echo "=========================================="
    echo "  ❌ 检测到未提交的生成产物变更"
    echo "=========================================="
    if [[ -n "$title" ]]; then
      echo "[原因] $title"
    fi
    echo ""
    echo "[变更列表]"
    if [[ -n "$check_pattern" ]]; then
      git status --porcelain | grep -E "$check_pattern"
    else
      git status --porcelain
    fi
    echo ""
    echo "请开发先在本地执行以下命令并提交生成产物："
    echo "  - make proto"
    echo "  - ./scripts/sync-swagger-http-yaml.sh"
    echo ""
    echo "然后再重新执行：./docs/swagger/update-skip-sg.sh"
    echo ""
    exit 1
  fi
}

echo "=========================================="
echo "  Internal Wallet - 更新部署"
echo "=========================================="

cd "$PROJECT_ROOT"

# 1. 拉取最新代码
echo ""
echo "[Step 1/5] 拉取最新代码..."
git pull origin "$(git rev-parse --abbrev-ref HEAD)" || {
  echo "[WARN] Git pull 失败，继续使用当前代码..."
}

# 1.1 更新 web/admin 子模块
echo ""
echo "[Step 1.1/5] 更新 web/admin 代码..."
cd "$PROJECT_ROOT/web/admin"
git pull || {
  echo "[WARN] web/admin git pull 失败，继续使用当前代码..."
}
cd "$PROJECT_ROOT"

# 2. 校验：生成 proto 相关代码（需要 goctl）
echo ""
echo "[Step 2/5] 校验 proto 生成产物是否已提交..."
if [[ "${SKIP_GEN_CHECK:-0}" == "1" || "${SKIP_PROTO_GEN_CHECK:-0}" == "1" ]]; then
  if [[ "${SKIP_GEN_CHECK:-0}" == "1" ]]; then
    echo "[SKIP] 已设置 SKIP_GEN_CHECK=1，跳过 proto 生成产物校验（不推荐）"
  else
    echo "[SKIP] 已设置 SKIP_PROTO_GEN_CHECK=1，跳过 proto 生成产物校验（不推荐）"
  fi
else
  if ! command -v make >/dev/null 2>&1; then
    echo "[ERROR] 未检测到 make，无法校验 proto 生成产物。请先安装构建工具或在开发机生成并提交产物。"
    exit 1
  fi
  if ! command -v goctl >/dev/null 2>&1; then
    echo "[ERROR] 未检测到 goctl，无法校验 proto 生成产物。请先安装 goctl，或在开发机生成并提交产物。"
    exit 1
  fi
  # 保存执行前的 git 状态（只检查 proto 相关文件）
  git_before=$(git status --porcelain | grep -E "(proto/pb/.*\.pb\.go|services/.*/rpc/.*/server/.*server\.go)" || true)
  make proto
  # 只检查 proto 相关的文件变更（.pb.go 文件和生成的 server 文件）
  fail_if_git_dirty "运行 make proto 后出现未提交变更（说明生成产物未同步提交）" "(proto/pb/.*\.pb\.go|services/.*/rpc/.*/server/.*server\.go)"
fi

# 3. 校验：刷新 Swagger/OpenAPI 产物（docs/swagger/*）
#
# 说明：
# - routes 事实源：api-gateway/routes.yaml
# - 默认输出目录：docs/swagger
# - 脚本用法：
#   ./scripts/sync-swagger-http-yaml.sh
#   ./scripts/sync-swagger-http-yaml.sh path/to/routes.yaml path/to/output_dir
echo ""
echo "[Step 3/5] 校验 Swagger/OpenAPI 产物是否已提交..."
if [[ "${SKIP_GEN_CHECK:-0}" == "1" || "${SKIP_SWAGGER_CHECK:-0}" == "1" ]]; then
  if [[ "${SKIP_GEN_CHECK:-0}" == "1" ]]; then
    echo "[SKIP] 已设置 SKIP_GEN_CHECK=1，跳过 Swagger 生成产物校验（不推荐）"
  else
    echo "[SKIP] 已设置 SKIP_SWAGGER_CHECK=1，跳过 Swagger 生成产物校验（不推荐）"
  fi
else
  if [[ ! -x "$PROJECT_ROOT/scripts/sync-swagger-http-yaml.sh" ]]; then
    echo "[ERROR] 未找到可执行脚本：$PROJECT_ROOT/scripts/sync-swagger-http-yaml.sh"
    exit 1
  fi
  # 保存执行前的 git 状态（只检查 swagger 相关文件）
  git_before=$(git status --porcelain | grep -E "docs/swagger/" || true)
  "$PROJECT_ROOT/scripts/sync-swagger-http-yaml.sh"
  # 只检查 swagger 相关的文件变更
  fail_if_git_dirty "运行 ./scripts/sync-swagger-http-yaml.sh 后出现未提交变更（说明 Swagger/OpenAPI 产物未同步提交）" "docs/swagger/"
fi

# 4. 重新构建前端（无缓存）
echo ""
echo "[Step 4/5] 重新构建前端..."
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --env-file "$ENV_FILE" build --no-cache admin-web

# 5. 重启应用服务（不影响数据库和 Redis）
echo ""
echo "[Step 5/5] 重启服务..."
# 注意：仅 restart 不会让已运行容器重新读取 .env 变更，所以这里统一强制重建全部服务容器
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d --force-recreate --remove-orphans

echo ""
echo "=========================================="
echo "  ✅ 更新完成！"
echo "=========================================="
echo ""
docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" --env-file "$ENV_FILE" ps
echo ""
echo "提示: 使用 ./deploy.sh logs [service] 查看日志"


