#!/usr/bin/env bash

# Zink Wallet - 本地开发启动脚本
#
# 启动服务：
# - Docker 基础设施 (MariaDB, Redis, ETCD, Jaeger)
# - market-rpc (端口 9015, Binance 行情流)
# - accounting-rpc (端口 9016, 会计账本服务)
# - admin-rpc (端口 9013)
# - business-rpc (端口 8081)
# - swap-rpc (端口 9014)
# - chainrpc-rpc (端口 9005)
# - chainsync-rpc (端口 9001)
# - api-gateway (端口 8080)
#
# 用法：
#   ./scripts/dev-local.sh start       # 启动所有服务
#   ./scripts/dev-local.sh stop        # 停止应用服务
#   ./scripts/dev-local.sh restart     # 重启应用服务
#   ./scripts/dev-local.sh status      # 查看服务状态
#   ./scripts/dev-local.sh logs        # 查看日志 (默认 api-gateway)
#   ./scripts/dev-local.sh logs admin  # 查看 admin-rpc 日志
#   ./scripts/dev-local.sh logs business # 查看 business-rpc 日志

set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 路径设置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

COMPOSE_FILE="$PROJECT_ROOT/deploy/docker/docker-compose.yml"
PID_DIR="$PROJECT_ROOT/.pids/dev-local"
LOG_DIR="$PROJECT_ROOT/logs/dev-local"
APP_PORTS=(8080 8081 9001 9005 9013 9014 9015 9016)

mkdir -p "$PID_DIR" "$LOG_DIR"

ENV_FILE_ADMIN="$PROJECT_ROOT/deploy/admin/.env"
GATEWAY_LOCAL_YAML="$PROJECT_ROOT/api-gateway/etc/gateway.local.yaml"

load_admin_env_if_present() {
  if [ -f "$ENV_FILE_ADMIN" ]; then
    set -a
    # shellcheck disable=SC1090
    . "$ENV_FILE_ADMIN"
    set +a
  fi

  : "${S3_ENABLED:=false}"
  : "${S3_USE_PATH_STYLE:=false}"
  : "${S3_PREFIX:=admin}"
  export S3_ENABLED S3_USE_PATH_STYLE S3_PREFIX
}

render_gateway_local_yaml() {
  mkdir -p "$PROJECT_ROOT/api-gateway/etc"
  cat >"$GATEWAY_LOCAL_YAML" <<'EOF'
Name: api-gateway
Mode: dev

Host: 0.0.0.0
Port: 8080

Log:
  ServiceName: api-gateway
  Mode: console
  Path: logs/gateway
  Level: info
  Compress: true
  KeepDays: 7
  StackCooldownMillis: 100

Auth:
  AccessSecret: "F0Fu3arY13GQFG1JinnlkHLxaJzHpZPavVKZjCJySx+d15GVCKmKGdJv4C1gnzqizhjgU9gkU6VeykpfYn5UUQ=="
  AccessExpire: 86400
  RefreshSecret: "Tcs10IiTu9GFUExWeqBvv+Z61k4GcM/5UKrV3rJHnVeitEI9eQc8dBV/V81CMONBGnrtPPPRU2saIJtfiM2mWA=="
  RefreshExpire: 604800

AdminAuth:
  AccessSecret: "4kKh3PGT4ByQyDZFny+T9ZXO0SHsWJVpddZCXzfOKo2t9XHn2nDp7xEiKviDTn8eJbz/k5kn8a//GRVUTHsGGg=="
  AccessExpire: 28800
  RefreshSecret: "GwOJj9HIRijQk34JYz7nZhMH8zsp/gIrtp3XsRdjyU8qjGIn57Rj/jaAapzoZ/tLUYB4ikxQc59Qujcpm+D5IA=="
  RefreshExpire: 604800

BusinessRpc:
  Etcd:
    Hosts:
      - 127.0.0.1:15279
    Key: business.rpc
  Timeout: 30000

ChainRpcRpc:
  Etcd:
    Hosts:
      - 127.0.0.1:15279
    Key: chainrpc.rpc
  Timeout: 30000

ChainSyncRpc:
  Etcd:
    Hosts:
      - 127.0.0.1:15279
    Key: chainsync.rpc
  Timeout: 30000

SignerRpc:
  Etcd:
    Hosts:
      - 127.0.0.1:15279
    Key: signer.rpc
  Timeout: 30000

AdminRpc:
  Etcd:
    Hosts:
      - 127.0.0.1:15279
    Key: admin.rpc
  Timeout: 30000

Redis:
  Host: 127.0.0.1
  Port: 16380
  Password: "redis123456"
  DB: 0

RateLimit:
  Enable: true
  UseRedis: true
  GlobalRate: 1000
  GlobalBurst: 2000

RequestTimeout: 30000

S3:
  Enabled: ${S3_ENABLED}
  Region: "${S3_REGION}"
  Bucket: "${S3_BUCKET}"
  Endpoint: "${S3_ENDPOINT}"
  AccessKeyID: "${S3_ACCESS_KEY_ID}"
  SecretAccessKey: "${S3_SECRET_ACCESS_KEY}"
  SessionToken: "${S3_SESSION_TOKEN}"
  PublicBaseURL: "${S3_PUBLIC_BASE_URL}"
  Prefix: "${S3_PREFIX}"
  UsePathStyle: ${S3_USE_PATH_STYLE}
  ACL: "${S3_ACL}"

Cors:
  AllowOrigins:
    - "*"
  AllowMethods:
    - "GET"
    - "POST"
    - "PUT"
    - "DELETE"
    - "PATCH"
    - "OPTIONS"
  AllowHeaders:
    - "Content-Type"
    - "Authorization"
    - "Idempotency-Key"
    - "X-Request-ID"
    - "X-Trace-ID"
  ExposeHeaders:
    - "X-Request-ID"
    - "X-Trace-ID"
  AllowCredentials: false
  MaxAge: 86400

TrustedProxies: []
EOF
}

# Docker Compose 兼容
docker_compose() {
  if docker compose version >/dev/null 2>&1; then
    docker compose "$@"
  elif command -v docker-compose >/dev/null 2>&1; then
    docker-compose "$@"
  else
    echo -e "${RED}[错误] docker compose 未找到${NC}" >&2
    exit 1
  fi
}

# 等待端口可用
wait_for_port() {
  local host="$1"
  local port="$2"
  local name="${3:-$host:$port}"
  local retries="${4:-60}"

  for _ in $(seq 1 "$retries"); do
    if nc -z "$host" "$port" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done
  echo -e "${RED}[错误] 等待 $name ($host:$port) 超时${NC}" >&2
  return 1
}

# 获取端口监听进程
get_port_pids() {
  local port="$1"
  lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true
}

# 清理端口占用
cleanup_ports() {
  for port in "${APP_PORTS[@]}"; do
    local pids
    pids="$(get_port_pids "$port")"
    if [ -n "$pids" ]; then
      echo -e "${YELLOW}[清理] 端口 $port 被占用，正在清理: $pids${NC}"
      echo "$pids" | xargs kill >/dev/null 2>&1 || true
      sleep 1
      echo "$pids" | xargs kill -9 >/dev/null 2>&1 || true
    fi
  done
}

# 启动基础设施
infra_start() {
  echo -e "${BLUE}[基础设施] 正在启动 Docker 服务...${NC}"
  
  if docker_compose -f "$COMPOSE_FILE" up --help 2>/dev/null | grep -q -- "--wait"; then
    docker_compose -f "$COMPOSE_FILE" up -d --wait
  else
    docker_compose -f "$COMPOSE_FILE" up -d
  fi

  echo -e "${YELLOW}[等待] MariaDB (15306)...${NC}"
  wait_for_port 127.0.0.1 15306 "MariaDB"
  
  echo -e "${YELLOW}[等待] Redis (16380)...${NC}"
  wait_for_port 127.0.0.1 16380 "Redis"
  
  echo -e "${YELLOW}[等待] ETCD (15279)...${NC}"
  wait_for_port 127.0.0.1 15279 "ETCD"
  
  echo -e "${GREEN}[基础设施] 就绪${NC}"
}

# 启动单个服务
start_service() {
  local name="$1"
  local dir="$2"
  local main_file="$3"
  local port="$4"
  shift 4
  local extra_args=("$@")

  echo -e "${YELLOW}[启动] $name (端口: $port)...${NC}"

  : >"$LOG_DIR/${name}.log"

  (
    cd "$PROJECT_ROOT/$dir"
    nohup go run "$main_file" "${extra_args[@]}" >"$LOG_DIR/${name}.log" 2>&1 &
    echo $! >"$PID_DIR/${name}.pid"
  )

  if wait_for_port 127.0.0.1 "$port" "$name" 30; then
    local pid
    pid="$(cat "$PID_DIR/${name}.pid")"
    echo -e "${GREEN}  ✓ $name 已启动 (PID: $pid)${NC}"
  else
    echo -e "${RED}  ✗ $name 启动失败，查看日志: $LOG_DIR/${name}.log${NC}"
    tail -n 50 "$LOG_DIR/${name}.log" || true
    return 1
  fi
}

# 停止单个服务
stop_service() {
  local name="$1"
  local pid_file="$PID_DIR/${name}.pid"

  if [ -f "$pid_file" ]; then
    local pid
    pid="$(cat "$pid_file")"
    if kill -0 "$pid" >/dev/null 2>&1; then
      echo -e "${YELLOW}[停止] $name (PID: $pid)${NC}"
      kill "$pid" >/dev/null 2>&1 || true
      sleep 1
      kill -9 "$pid" >/dev/null 2>&1 || true
    fi
    rm -f "$pid_file"
  fi
}

# 启动所有服务
cmd_start() {
  echo -e "${BLUE}========================================"
  echo "  Zink Wallet - 本地开发环境启动"
  echo -e "========================================${NC}"
  echo ""

  # 检查 Go 环境
  if ! command -v go &>/dev/null; then
    echo -e "${RED}[错误] 未检测到 Go 环境${NC}"
    exit 1
  fi
  echo -e "${GREEN}[信息] Go 版本: $(go version)${NC}"
  echo ""

  # 启动基础设施
  infra_start
  echo ""

  # 加载 deploy/admin/.env（可选）并生成 api-gateway 本地配置（包含可选 S3 设置）
  load_admin_env_if_present
  render_gateway_local_yaml

  # 清理端口
  cleanup_ports

  # 1. 启动 market-rpc (行情数据服务，其他服务可能依赖)
  start_service "market-rpc" "services/market/rpc" "market.go" 9015 -f "etc/market.local.yaml"
  sleep 2

  # 2. 启动 accounting-rpc (会计账本服务，business 依赖)
  start_service "accounting-rpc" "services/accounting/rpc" "accounting.go" 9016 -f "etc/accounting.local.yaml"
  sleep 2

  # 3. 启动 business-rpc (其他服务可能依赖)
  start_service "business-rpc" "services/business/rpc" "business.go" 8081 -f "etc/business.local.yaml"
  sleep 2

  # 4. 启动 swap-rpc (business 可能依赖)
  start_service "swap-rpc" "services/swap/rpc" "swap.go" 9014 -f "etc/swap.local.yaml"
  sleep 2

  # 5. 启动 admin-rpc
  start_service "admin-rpc" "services/admin/rpc" "admin.go" 9013 -f "etc/admin.local.yaml"
  sleep 2

  # 6. 启动 chainrpc-rpc
  start_service "chainrpc-rpc" "services/chainrpc/rpc" "chainrpc.go" 9005 -f "etc/chainrpc.local.yaml"
  sleep 2

  # 7. 启动 chainsync-rpc
  start_service "chainsync-rpc" "services/chainsync/rpc" "chainsync.go" 9001 -f "etc/chainsync.local.yaml"
  sleep 2

  # 8. 启动 api-gateway (最后启动，依赖其他 RPC 服务)
  # 设置 Swagger UI 地址（指向 Docker 中的 swagger-ui 服务）
  export GATEWAY_SWAGGER_UI_URL="http://localhost:18080"
  start_service "api-gateway" "api-gateway" "gateway.go" 8080 -f "etc/gateway.local.yaml" -r "routes.yaml"

  echo ""
  echo -e "${GREEN}========================================"
  echo "  所有服务已启动！"
  echo -e "========================================${NC}"
  echo ""
  echo "服务列表："
  echo "  - API Gateway:    http://localhost:8080"
  echo "  - Swagger UI:     http://localhost:8080/docs/"
  echo "  - Market RPC:     127.0.0.1:9015 (Health, Binance Ticker Streamer)"
  echo "  - Accounting RPC: 127.0.0.1:9016 (gRPC, 会计账本)"
  echo "  - Business RPC:   127.0.0.1:8081 (gRPC)"
  echo "  - Swap RPC:       127.0.0.1:9014 (gRPC)"
  echo "  - Admin RPC:      127.0.0.1:9013 (gRPC)"
  echo "  - ChainRPC RPC:   127.0.0.1:9005 (gRPC)"
  echo "  - ChainSync RPC:  127.0.0.1:9001 (gRPC)"
  echo ""
  echo "基础设施："
  echo "  - MariaDB:        127.0.0.1:15306"
  echo "  - Redis:          127.0.0.1:16380"
  echo "  - ETCD:           127.0.0.1:15279"
  echo "  - Jaeger UI:      http://localhost:16686"
  echo "  - Swagger UI:     http://localhost:18080 (直接访问)"
  echo ""
  echo "日志目录: $LOG_DIR/"
  echo ""
  echo "管理命令："
  echo "  - 查看日志:       ./scripts/dev-local.sh logs [admin|business|market|gateway]"
  echo "  - 停止服务:       ./scripts/dev-local.sh stop"
  echo "  - 重启服务:       ./scripts/dev-local.sh restart"
  echo "  - 查看状态:       ./scripts/dev-local.sh status"
}

# 停止所有服务
cmd_stop() {
  echo -e "${BLUE}[停止] 正在停止应用服务...${NC}"

  stop_service "api-gateway"
  stop_service "chainsync-rpc"
  stop_service "chainrpc-rpc"
  stop_service "admin-rpc"
  stop_service "swap-rpc"
  stop_service "business-rpc"
  stop_service "accounting-rpc"
  stop_service "market-rpc"
  
  cleanup_ports

  echo -e "${GREEN}[完成] 应用服务已停止${NC}"
  echo ""
  echo -e "${YELLOW}[提示] Docker 基础设施仍在运行，如需停止请执行:${NC}"
  echo "  docker compose -f $COMPOSE_FILE down"
}

# 重启服务
cmd_restart() {
  cmd_stop
  echo ""
  cmd_start
}

# 查看状态
cmd_status() {
  echo -e "${BLUE}========================================"
  echo "  服务状态"
  echo -e "========================================${NC}"
  echo ""

  check_service() {
    local name="$1"
    local port="$2"
    local pid_file="$PID_DIR/${name}.pid"

    if nc -z 127.0.0.1 "$port" >/dev/null 2>&1; then
      local pid=""
      if [ -f "$pid_file" ]; then
        pid="$(cat "$pid_file")"
      fi
      echo -e "  ${GREEN}✓${NC} $name (端口: $port${pid:+, PID: $pid})"
    else
      echo -e "  ${RED}✗${NC} $name (端口: $port) - 未运行"
    fi
  }

  echo "应用服务："
  check_service "api-gateway" 8080
  check_service "admin-rpc" 9013
  check_service "swap-rpc" 9014
  check_service "business-rpc" 8081
  check_service "chainrpc-rpc" 9005
  check_service "chainsync-rpc" 9001
  check_service "accounting-rpc" 9016
  check_service "market-rpc" 9015
  echo ""

  echo "基础设施："
  for svc_port in "MariaDB:15306" "Redis:16380" "ETCD:15279" "Jaeger:16686"; do
    local svc="${svc_port%%:*}"
    local port="${svc_port##*:}"
    if nc -z 127.0.0.1 "$port" >/dev/null 2>&1; then
      echo -e "  ${GREEN}✓${NC} $svc (端口: $port)"
    else
      echo -e "  ${RED}✗${NC} $svc (端口: $port) - 未运行"
    fi
  done
}

# 查看日志
cmd_logs() {
  local service="${1:-gateway}"
  local log_file=""

  case "$service" in
    admin)
      log_file="$LOG_DIR/admin-rpc.log"
      ;;
    business)
      log_file="$LOG_DIR/business-rpc.log"
      ;;
    swap)
      log_file="$LOG_DIR/swap-rpc.log"
      ;;
    market)
      log_file="$LOG_DIR/market-rpc.log"
      ;;
    accounting)
      log_file="$LOG_DIR/accounting-rpc.log"
      ;;
    chainrpc)
      log_file="$LOG_DIR/chainrpc-rpc.log"
      ;;
    chainsync)
      log_file="$LOG_DIR/chainsync-rpc.log"
      ;;
    gateway|api-gateway|*)
      log_file="$LOG_DIR/api-gateway.log"
      ;;
  esac

  if [ ! -f "$log_file" ]; then
    echo -e "${RED}[错误] 日志文件不存在: $log_file${NC}"
    exit 1
  fi

  echo -e "${BLUE}[日志] $log_file${NC}"
  tail -n 100 -f "$log_file"
}

# 帮助信息
cmd_help() {
  cat <<EOF
Zink Wallet - 本地开发启动脚本

用法: ./scripts/dev-local.sh <命令> [参数]

命令:
  start       启动所有服务 (基础设施 + market + admin + business + swap + gateway)
  stop        停止应用服务 (保留 Docker 基础设施)
  restart     重启应用服务
  status      查看服务状态
  logs        查看日志 (默认 gateway)
              logs admin      - 查看 admin-rpc 日志
              logs business   - 查看 business-rpc 日志
              logs swap       - 查看 swap-rpc 日志
              logs market     - 查看 market-rpc 日志
              logs accounting - 查看 accounting-rpc 日志
              logs chainrpc   - 查看 chainrpc-rpc 日志
              logs chainsync  - 查看 chainsync-rpc 日志
              logs gateway    - 查看 api-gateway 日志
  help        显示此帮助信息

示例:
  ./scripts/dev-local.sh start
  ./scripts/dev-local.sh logs admin
  ./scripts/dev-local.sh stop
EOF
}

# 主入口
case "${1:-help}" in
  start)
    cmd_start
    ;;
  stop)
    cmd_stop
    ;;
  restart)
    cmd_restart
    ;;
  status)
    cmd_status
    ;;
  logs)
    cmd_logs "${2:-}"
    ;;
  help|--help|-h)
    cmd_help
    ;;
  *)
    echo -e "${RED}[错误] 未知命令: $1${NC}"
    cmd_help
    exit 1
    ;;
esac
