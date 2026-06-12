#!/bin/bash

# 加密货币交易所 - 服务停止脚本 (Linux/Mac)

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 获取项目根目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PID_DIR="$PROJECT_ROOT/.pids"

echo -e "${BLUE}========================================"
echo "  停止所有微服务"
echo -e "========================================${NC}"
echo ""

# 服务列表
services=(
    "account-rpc"
    "wallet-rpc"
    "trade-rpc"
    "matching-rpc"
    "market-rpc"
    "settlement-rpc"
    "risk-rpc"
    "api-gateway"
)

stopped_count=0
not_running_count=0

# 停止所有服务
for service in "${services[@]}"; do
    pid_file="$PID_DIR/${service}.pid"

    if [ -f "$pid_file" ]; then
        pid=$(cat "$pid_file")

        if ps -p "$pid" > /dev/null 2>&1; then
            echo -e "${YELLOW}正在停止 $service (PID: $pid)...${NC}"
            kill "$pid" 2>/dev/null || kill -9 "$pid" 2>/dev/null

            # 等待进程结束
            timeout=5
            while ps -p "$pid" > /dev/null 2>&1 && [ $timeout -gt 0 ]; do
                sleep 1
                ((timeout--))
            done

            if ps -p "$pid" > /dev/null 2>&1; then
                kill -9 "$pid" 2>/dev/null
            fi

            echo -e "${GREEN}  ✓ $service 已停止${NC}"
            ((stopped_count++))
        else
            echo -e "${YELLOW}  [警告] $service 未运行 (PID: $pid 不存在)${NC}"
            ((not_running_count++))
        fi

        rm -f "$pid_file"
    else
        echo -e "${YELLOW}  [警告] $service 未运行 (PID 文件不存在)${NC}"
        ((not_running_count++))
    fi
done

# 清理 PID 目录
if [ -d "$PID_DIR" ]; then
    rm -rf "$PID_DIR"
fi

echo ""
echo -e "${GREEN}========================================"
echo "  服务停止完成！"
echo -e "========================================${NC}"
echo ""
echo "统计:"
echo "  - 已停止: $stopped_count 个服务"
echo "  - 未运行: $not_running_count 个服务"
echo ""

# 检查是否还有残留的 go run 进程
go_processes=$(pgrep -f "go run" || true)
if [ -n "$go_processes" ]; then
    echo -e "${YELLOW}检测到残留的 go run 进程:${NC}"
    ps aux | grep "go run" | grep -v grep
    echo ""
    echo -e "${YELLOW}是否要停止所有 go run 进程？ (y/N)${NC}"
    read -r confirm
    if [[ "$confirm" =~ ^[Yy]$ ]]; then
        pkill -f "go run"
        echo -e "${GREEN}  ✓ 已停止所有 Go 进程${NC}"
    fi
fi

echo ""
echo -e "${BLUE}[提示] 可以使用 ./scripts/start-all.sh 重新启动服务${NC}"
