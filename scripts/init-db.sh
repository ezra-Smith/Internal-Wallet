#!/bin/bash

# 初始化数据库脚本

echo "Initializing databases..."

# 等待MariaDB启动
echo "Waiting for MariaDB to be ready..."
sleep 5

# 执行数据库初始化
docker exec crypto-mariadb mysql -uroot -proot123456 < deploy/docker/init-db/01-init.sql

echo "Database initialization completed!"
