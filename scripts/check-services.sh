#!/bin/bash

# 检查所有服务状态

echo "Checking infrastructure services status..."
echo ""

services=(
    "MariaDB:3306"
    "Redis:6379"
    "Kafka:9092"
    "ETCD:2379"
    "Prometheus:9090"
    "Grafana:3000"
    "Jaeger:16686"
)

for service in "${services[@]}"; do
    name="${service%:*}"
    port="${service#*:}"

    if nc -z localhost "$port" 2>/dev/null; then
        echo "✓ $name is running on port $port"
    else
        echo "✗ $name is NOT running on port $port"
    fi
done

echo ""
echo "Docker containers:"
docker ps --filter "name=crypto-" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
