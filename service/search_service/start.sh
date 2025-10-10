#!/bin/bash

# 启动搜索服务

# 设置工作目录
WORK_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$WORK_DIR"

# 设置环境变量
export SEARCH_SERVICE_CONFIG="./config/search-service.yaml"

# 启动服务
echo "Starting search service..."
go run cmd/server/main.go

echo "Search service started successfully."