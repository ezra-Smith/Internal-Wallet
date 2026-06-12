#!/bin/bash

# 加密货币交易所 - 服务启动脚本 (Linux/Mac)

set -e  # 遇到错误立即退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 获取项目根目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

echo -e "${BLUE}========================================"
echo "  加密货币交易所 - 服务启动脚本"
echo -e "========================================${NC}"
echo ""
echo "项目目录: $PROJECT_ROOT"
echo ""

# 检查 Go 环境
if ! command -v go &> /dev/null; then
    echo -e "${RED}[错误] 未检测到 Go 环境，请先安装 Go${NC}"
    exit 1
fi

echo -e "${GREEN}[信息] Go 版本:${NC}"
go version
echo ""

# 日志目录
LOGS_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOGS_DIR"

echo -e "${BLUE}========================================"
echo "  步骤 1/2: 启动后端 RPC 服务"
echo -e "========================================${NC}"
echo ""

# PID 文件目录
PID_DIR="$PROJECT_ROOT/.pids"
mkdir -p "$PID_DIR"

# 函数：启动服务
start_service() {
    local name=$1
    local port=$2
    local dir=$3
    local main_file=$4
    local config=$5

    echo -e "${YELLOW}[$6/7] 正在启动 $name (端口: $port)...${NC}"

    cd "$PROJECT_ROOT/$dir"
    nohup go run "$main_file" -f "$config" > "$LOGS_DIR/${name}.log" 2>&1 &
    local pid=$!
    echo $pid > "$PID_DIR/${name}.pid"

    echo -e "${GREEN}  ✓ $name 已启动 (PID: $pid)${NC}"
    sleep 2
}

# 1. Account Service
start_service "account-rpc" "9001" "services/account/rpc" "account.go" "etc/account.yaml" 1

# 2. Wallet Service
start_service "wallet-rpc" "9002" "services/wallet/rpc" "wallet.go" "etc/wallet.yaml" 2

# 3. Trade Service
start_service "trade-rpc" "9003" "services/trade/rpc" "trade.go" "etc/trade.yaml" 3

# 4. Matching Service
start_service "matching-rpc" "9004" "services/matching/rpc" "matching.go" "etc/matching.yaml" 4

# 5. Market Service
start_service "market-rpc" "9005" "services/market/rpc" "market.go" "etc/market.yaml" 5

# 6. Settlement Service
start_service "settlement-rpc" "9006" "services/settlement/rpc" "settlement.go" "etc/settlement.yaml" 6

# 7. Risk Service
start_service "risk-rpc" "9007" "services/risk/rpc" "risk.go" "etc/risk.yaml" 7

echo ""
echo -e "${GREEN}[信息] 等待后端服务完全启动...${NC}"
sleep 5

echo ""
echo -e "${BLUE}========================================"
echo "  步骤 2/2: 启动 API Gateway"
echo -e "========================================${NC}"
echo ""

# 8. API Gateway (最后启动)
echo -e "${YELLOW}[8/8] 正在启动 API Gateway (端口: 8080)...${NC}"
cd "$PROJECT_ROOT/api-gateway"
nohup go run gateway.go -f etc/gateway.yaml > "$LOGS_DIR/api-gateway.log" 2>&1 &
gateway_pid=$!
echo $gateway_pid > "$PID_DIR/api-gateway.pid"
echo -e "${GREEN}  ✓ API Gateway 已启动 (PID: $gateway_pid)${NC}"

echo ""
echo -e "${GREEN}========================================"
echo "  所有服务已启动！"
echo -e "========================================${NC}"
echo ""
echo "服务列表:"
echo "  1. Account RPC       - localhost:9001  (账号+用户账户服务)"
echo "  2. Wallet RPC        - localhost:9002  (钱包服务)"
echo "  3. Trade RPC         - localhost:9003  (交易服务)"
echo "  4. Matching RPC      - localhost:9004  (撮合服务)"
echo "  5. Market RPC        - localhost:9005  (行情服务)"
echo "  6. Settlement RPC    - localhost:9006  (结算服务)"
echo "  7. Risk RPC          - localhost:9007  (风控服务)"
echo "  8. API Gateway       - http://localhost:8080  (HTTP网关)"
echo ""
echo "日志位置:"
echo "  - 所有服务日志:    $LOGS_DIR/"
echo "  - RPC 服务日志:    services/*/rpc/logs/"
echo ""
echo "管理命令:"
echo "  - 查看日志:        tail -f $LOGS_DIR/api-gateway.log"
echo "  - 停止所有服务:    ./scripts/stop-all.sh"
echo "  - 查看服务状态:    ./scripts/status.sh"
echo ""
echo -e "${YELLOW}[提示] 服务已在后台运行${NC}"
