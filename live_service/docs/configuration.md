# Live Service 配置文档

## 概述

Live Service 支持多种配置方式，包括环境变量、配置文件和命令行参数。配置优先级为：命令行参数 > 环境变量 > 配置文件 > 默认值。

## 环境变量配置

### 基础配置

| 环境变量 | 说明 | 默认值 | 示例 |
|---------|------|--------|------|
| `ENVIRONMENT` | 运行环境 | `development` | `production`, `staging`, `development` |
| `LOG_LEVEL` | 日志级别 | `info` | `debug`, `info`, `warn`, `error` |
| `GIN_MODE` | Gin框架模式 | `debug` | `debug`, `release`, `test` |

### 服务器配置

| 环境变量 | 说明 | 默认值 | 示例 |
|---------|------|--------|------|
| `SERVER_PORT` | HTTP服务器端口 | `8080` | `8080`, `80`, `443` |
| `GRPC_PORT` | gRPC服务器端口 | `8081` | `8081`, `9090` |

### 数据库配置

| 环境变量 | 说明 | 默认值 | 示例 |
|---------|------|--------|------|
| `MYSQL_HOST` | MySQL主机地址 | `localhost` | `localhost`, `127.0.0.1`, `mysql-service` |
| `MYSQL_PORT` | MySQL端口 | `3306` | `3306` |
| `MYSQL_USER` | MySQL用户名 | `root` | `root`, `live_user` |
| `MYSQL_PASSWORD` | MySQL密码 | `password` | `your_password` |
| `MYSQL_DATABASE` | 数据库名称 | `live_service` | `live_service` |
| `MYSQL_MAX_OPEN_CONNS` | 最大打开连接数 | `100` | `100` |
| `MYSQL_MAX_IDLE_CONNS` | 最大空闲连接数 | `10` | `10` |

### Redis配置

| 环境变量 | 说明 | 默认值 | 示例 |
|---------|------|--------|------|
| `REDIS_HOST` | Redis主机地址 | `localhost` | `localhost`, `127.0.0.1`, `redis-service` |
| `REDIS_PORT` | Redis端口 | `6379` | `6379` |
| `REDIS_PASSWORD` | Redis密码 | `` | `your_password` |
| `REDIS_DB` | Redis数据库编号 | `0` | `0`, `1`, `2` |
| `REDIS_POOL_SIZE` | 连接池大小 | `100` | `100` |

## 配置文件

### YAML配置示例

创建 `configs/live-service.yaml` 文件：

```yaml
# 服务配置
server:
  port: 8080
  grpc_port: 8081
  log_level: info
  environment: development

# 数据库配置
database:
  mysql:
    host: localhost
    port: 3306
    user: root
    password: password
    database: live_service
    max_open_conns: 100
    max_idle_conns: 10
    conn_max_lifetime: 3600  # 秒
    conn_max_idle_time: 300  # 秒
  
  redis:
    host: localhost
    port: 6379
    password: ""
    db: 0
    pool_size: 100
    min_idle_conns: 10
    max_retries: 3
    dial_timeout: 5s
    read_timeout: 3s
    write_timeout: 3s

# 直播配置
live:
  # 推流配置
  stream:
    protocol: rtmp        # 推流协议: rtmp, webrtc
    rtmp_port: 1935
    webrtc_port: 8088
    enable_recording: true
    recording_path: /var/recordings
    
  # 转码配置
  transcode:
    enabled: true
    resolutions:
      - 1080p
      - 720p
      - 480p
    bitrates:
      1080p: 5000
      720p: 2500
      480p: 1000
    
  # CDN配置
  cdn:
    enabled: false
    provider: aliyun      # aliyun, tencent, aws
    domain: live.example.com
    
  # 限制配置
  limits:
    max_viewers_per_stream: 100000
    max_chat_per_second: 100
    max_gift_per_second: 50
    max_stream_duration: 86400  # 24小时

# 礼物配置
gift:
  # 礼物分类
  categories:
    - name: normal
      display_name: 普通礼物
      sort_order: 1
    - name: rare
      display_name: 稀有礼物
      sort_order: 2
    - name: legendary
      display_name: 传说礼物
      sort_order: 3
  
  # 默认礼物
  default_gifts:
    - name: rose
      display_name: 玫瑰
      icon: 🌹
      price: 10
      coin_price: 10
      category: normal
      level: 1
      effect_type: emoji
      effect_value: 🌹
    - name: heart
      display_name: 爱心
      icon: ❤️
      price: 20
      coin_price: 20
      category: normal
      level: 1
      effect_type: emoji
      effect_value: ❤️
    - name: diamond
      display_name: 钻石
      icon: 💎
      price: 100
      coin_price: 100
      category: rare
      level: 3
      effect_type: sparkle
      effect_value: 💎✨

# 缓存配置
cache:
  # Redis缓存
  redis:
    # 直播流缓存
    live_stream_expire: 300      # 5分钟
    live_room_expire: 300      # 5分钟
    viewer_count_expire: 30    # 30秒
    chat_message_expire: 300   # 5分钟
    stats_expire: 3600         # 1小时
    ranking_expire: 300        # 5分钟
    
  # 本地缓存
  local:
    enabled: true
    max_size: 1000
    expire_time: 60            # 1分钟

# 安全配置
security:
  # 认证配置
  auth:
    enabled: true
    jwt_secret: your_jwt_secret_key
    jwt_expire: 86400          # 24小时
    
  # 限流配置
  rate_limit:
    enabled: true
    requests_per_minute: 1000
    burst_size: 100
    
  # 内容审核
  content_moderation:
    enabled: true
    sensitive_words_file: configs/sensitive_words.txt
    ai_moderation_enabled: false
    
  # 用户限制
  user_limits:
    max_streams_per_user: 10
    max_chat_length: 500
    max_gift_message_length: 200

# 监控配置
monitoring:
  # Prometheus指标
  prometheus:
    enabled: true
    path: /metrics
    
  # 健康检查
  health_check:
    enabled: true
    path: /health
    interval: 30s
    timeout: 5s
    
  # 应用指标
  metrics:
    enabled: true
    collect_interval: 10s
    
  # 链路追踪
  tracing:
    enabled: false
    provider: jaeger      # jaeger, zipkin, otel
    endpoint: http://localhost:14268/api/traces

# 日志配置
logging:
  # 日志级别
  level: info           # debug, info, warn, error
  
  # 日志格式
  format: json          # json, console
  
  # 输出配置
  output:
    type: file          # file, console, both
    path: logs/live-service.log
    max_size: 100       # MB
    max_backups: 10
    max_age: 30         # days
    compress: true
    
  # 日志字段
  fields:
    service: live-service
    environment: development
    version: 1.0.0

# 消息队列配置
message_queue:
  # Kafka配置
  kafka:
    enabled: false
    brokers:
      - localhost:9092
    topics:
      live_events: live-events
      chat_messages: chat-messages
      gift_events: gift-events
      user_actions: user-actions
    
  # Redis队列
  redis_queue:
    enabled: true
    max_retry: 3
    retry_interval: 5s

# 外部服务配置
external_services:
  # 用户服务
  user_service:
    enabled: true
    endpoint: http://user-service:8080
    timeout: 5s
    retry_count: 3
    
  # 支付服务
  payment_service:
    enabled: true
    endpoint: http://payment-service:8080
    timeout: 10s
    retry_count: 3
    
  # 通知服务
  notification_service:
    enabled: true
    endpoint: http://notification-service:8080
    timeout: 5s
    retry_count: 2
    
  # 文件存储服务
  storage_service:
    enabled: true
    provider: minio      # minio, aws-s3, aliyun-oss
    endpoint: http://minio:9000
    access_key: minioadmin
    secret_key: minioadmin
    bucket: live-service
```

## 环境配置示例

### 开发环境

```bash
# 基础配置
export ENVIRONMENT=development
export LOG_LEVEL=debug
export GIN_MODE=debug

# 服务器配置
export SERVER_PORT=8080
export GRPC_PORT=8081

# 数据库配置
export MYSQL_HOST=localhost
export MYSQL_PORT=3306
export MYSQL_USER=root
export MYSQL_PASSWORD=password
export MYSQL_DATABASE=live_service_dev

# Redis配置
export REDIS_HOST=localhost
export REDIS_PORT=6379
export REDIS_PASSWORD=
export REDIS_DB=0
```

### 测试环境

```bash
# 基础配置
export ENVIRONMENT=staging
export LOG_LEVEL=info
export GIN_MODE=release

# 服务器配置
export SERVER_PORT=8080
export GRPC_PORT=8081

# 数据库配置
export MYSQL_HOST=test-mysql
export MYSQL_PORT=3306
export MYSQL_USER=live_user
export MYSQL_PASSWORD=test_password
export MYSQL_DATABASE=live_service_test

# Redis配置
export REDIS_HOST=test-redis
export REDIS_PORT=6379
export REDIS_PASSWORD=test_redis_password
export REDIS_DB=1
```

### 生产环境

```bash
# 基础配置
export ENVIRONMENT=production
export LOG_LEVEL=warn
export GIN_MODE=release

# 服务器配置
export SERVER_PORT=8080
export GRPC_PORT=8081

# 数据库配置
export MYSQL_HOST=prod-mysql-cluster
export MYSQL_PORT=3306
export MYSQL_USER=live_service_user
export MYSQL_PASSWORD=your_secure_password
export MYSQL_DATABASE=live_service_prod
export MYSQL_MAX_OPEN_CONNS=200
export MYSQL_MAX_IDLE_CONNS=50

# Redis配置
export REDIS_HOST=prod-redis-cluster
export REDIS_PORT=6379
export REDIS_PASSWORD=your_secure_redis_password
export REDIS_DB=0
export REDIS_POOL_SIZE=200
```

## Docker 配置

### Docker Compose 环境变量

在 `docker-compose.yml` 中配置环境变量：

```yaml
version: '3.8'

services:
  live-service:
    image: live-service:latest
    environment:
      # 基础配置
      - ENVIRONMENT=production
      - LOG_LEVEL=info
      - GIN_MODE=release
      
      # 服务器配置
      - SERVER_PORT=8080
      - GRPC_PORT=8081
      
      # 数据库配置
      - MYSQL_HOST=mysql
      - MYSQL_PORT=3306
      - MYSQL_USER=live_user
      - MYSQL_PASSWORD_FILE=/run/secrets/mysql_password
      - MYSQL_DATABASE=live_service
      
      # Redis配置
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - REDIS_PASSWORD_FILE=/run/secrets/redis_password
      - REDIS_DB=0
    secrets:
      - mysql_password
      - redis_password

secrets:
  mysql_password:
    file: ./secrets/mysql_password.txt
  redis_password:
    file: ./secrets/redis_password.txt
```

### Kubernetes ConfigMap

在 Kubernetes 中使用 ConfigMap 管理配置：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: live-service-config
data:
  ENVIRONMENT: "production"
  LOG_LEVEL: "info"
  GIN_MODE: "release"
  SERVER_PORT: "8080"
  GRPC_PORT: "8081"
  MYSQL_HOST: "mysql-service"
  MYSQL_PORT: "3306"
  MYSQL_DATABASE: "live_service"
  REDIS_HOST: "redis-service"
  REDIS_PORT: "6379"
  REDIS_DB: "0"
---
apiVersion: v1
kind: Secret
metadata:
  name: live-service-secrets
type: Opaque
data:
  mysql-user: bGl2ZV91c2Vy  # base64 encoded 'live_user'
  mysql-password: eW91cl9wYXNzd29yZA==  # base64 encoded 'your_password'
  redis-password: eW91cl9yZWRpc19wYXNzd29yZA==  # base64 encoded 'your_redis_password'
```

## 配置验证

### 健康检查

配置完成后，可以通过以下方式验证配置是否正确：

```bash
# 检查服务状态
curl http://localhost:8080/health

# 检查版本信息
curl http://localhost:8080/version

# 检查指标
curl http://localhost:8080/metrics
```

### 日志验证

查看服务启动日志，确认配置加载是否正确：

```bash
# 查看Docker日志
docker-compose logs live-service

# 查看应用日志
tail -f logs/live-service.log
```

## 配置最佳实践

### 1. 安全配置
- 不要在代码中硬编码敏感信息
- 使用环境变量或配置文件管理敏感数据
- 生产环境使用强密码和加密连接
- 定期轮换密码和密钥

### 2. 性能配置
- 根据服务器资源调整连接池大小
- 合理设置缓存过期时间
- 监控数据库和Redis连接数
- 使用CDN加速静态资源

### 3. 监控配置
- 启用健康检查和指标收集
- 配置合适的日志级别
- 设置告警阈值
- 定期查看监控数据

### 4. 扩展配置
- 使用配置中心管理配置
- 支持配置热更新
- 配置版本管理
- 环境配置分离

## 故障排查

### 常见问题

1. **数据库连接失败**
   - 检查数据库服务是否运行
   - 验证连接参数是否正确
   - 检查网络连通性
   - 查看数据库日志

2. **Redis连接失败**
   - 检查Redis服务状态
   - 验证连接参数
   - 检查密码是否正确
   - 查看Redis日志

3. **端口冲突**
   - 检查端口是否被占用
   - 修改服务端口配置
   - 使用不同的端口

4. **配置不生效**
   - 检查环境变量名称是否正确
   - 验证配置文件路径
   - 重启服务使配置生效

### 调试技巧

```bash
# 查看环境变量
env | grep LIVE

# 检查配置文件
cat configs/live-service.yaml

# 测试数据库连接
mysql -h localhost -u root -p -e "SELECT 1"

# 测试Redis连接
redis-cli ping

# 查看服务日志
tail -f logs/live-service.log
```