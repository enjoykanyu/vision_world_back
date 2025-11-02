#!/bin/bash

# 视频服务启动脚本

# 设置工作目录
SERVICE_DIR="/Users/kanyu/Desktop/project/kanyu_server/new_project/project/vision_world_back/service/video_service"
cd $SERVICE_DIR

# 创建日志目录
mkdir -p logs

# 检查配置文件
if [ ! -f "config/video-service.yaml" ]; then
    echo "配置文件不存在，使用默认配置"
fi

# 编译服务
echo "正在编译视频服务..."
go build -o video_service cmd/server/main.go

if [ $? -ne 0 ]; then
    echo "编译失败"
    exit 1
fi

echo "视频服务编译成功"

# 启动服务
echo "正在启动视频服务..."
./video_service

# 清理
rm -f video_service