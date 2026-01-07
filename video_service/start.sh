#!/bin/bash

# 视频服务启动脚本

# 设置工作目录
SERVICE_DIR="/Users/kanyu/Desktop/project/kanyu_server/new_project/project/vision_world_back/video_service"
cd $SERVICE_DIR

# 创建日志目录
mkdir -p logs

# 检查配置文件
if [ ! -f "config/video-service.yaml" ]; then
    echo "配置文件不存在，正在创建默认配置..."
    mkdir -p config
    cat > config/video-service.yaml << EOF
# 视频服务配置文件

server:
  port: 8081
  host: "0.0.0.0"

# 数据库配置
database:
  host: "localhost"
  port: 3306
  username: "root"
  password: "password"
  dbname: "video_service"
  charset: "utf8mb4"
  parseTime: true
  loc: "Local"

# Redis配置
redis:
  host: "localhost"
  port: 6379
  password: ""
  db: 0
  pool_size: 10
  min_idle_conns: 5

# MinIO配置
minio:
  endpoint: "localhost:9000"
  access_key: "minioadmin"
  secret_key: "minioadmin"
  bucket_name: "vision-world"
  secure: false

# RabbitMQ配置
rabbitmq:
  host: "localhost"
  port: 5672
  username: "guest"
  password: "guest"
  exchange_name: "video_exchange"
  queue_name: "video_queue"
  routing_key: "video_routing_key"

# 日志配置
log:
  level: "info"
  file_path: "logs/video_service.log"
  max_size: 100
  max_backups: 10
  max_age: 7
  compress: false

# 服务注册配置
service:
  name: "video-service"
  version: "1.0.0"
  etcd_endpoints: ["localhost:2379"]
  ttl: 30
  interval: 10

# 审核服务配置
audit:
  exchange_name: "audit_exchange"
  queue_name: "audit_queue"
  routing_key: "audit_routing_key"
EOF
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