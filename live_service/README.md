# 直播服务 - Live Service

基于Go语言的微服务架构直播服务系统，提供完整的直播功能支持。

## 🚀 功能特性

### 核心功能
- **直播流管理**: 支持RTMP推流、HLS播放、多清晰度转码
- **直播间管理**: 直播间创建、配置、状态管理
- **实时聊天**: 弹幕消息、用户互动、敏感词过滤
- **礼物系统**: 虚拟礼物、收益统计、特效展示
- **用户管理**: 观看记录、关注关系、用户统计
- **数据统计**: 观看人数、收益统计、热门排行

### 高级特性
- **分布式架构**: 支持水平扩展、负载均衡
- **高并发处理**: Redis缓存、消息队列、数据库优化
- **实时监控**: 直播质量监控、异常告警
- **内容安全**: 敏感词过滤、内容审核
- **多端支持**: Web、移动端、小程序

## 📋 技术架构

### 技术栈
- **语言**: Go 1.21+
- **框架**: gRPC + Protobuf
- **数据库**: MySQL 8.0 + Redis 7.0
- **消息队列**: Apache Kafka
- **容器化**: Docker + Docker Compose
- **监控**: Prometheus + Grafana
- **日志**: Zap + ELK Stack

### 架构设计
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Load Balancer │    │   API Gateway   │    │   CDN & Cache   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  Live Service   │    │  User Service   │   │  Gift Service   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│     MySQL       │    │     Redis       │    │     Kafka       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 🛠️ 环境要求

- Go 1.21 或更高版本
- MySQL 8.0 或更高版本
- Redis 7.0 或更高版本
- Docker 和 Docker Compose (可选)
- Protocol Buffers 编译器

## 📦 安装依赖

```bash
# 克隆项目
git clone <repository-url>
cd vision_world_back/service/live_service

# 安装Go依赖
go mod download

# 安装Protocol Buffers工具
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 生成代码
make proto
```

## 🔧 配置说明

### 数据库配置
```yaml
database:
  mysql:
    host: localhost
    port: 3306
    user: root
    password: password
    database: live_service
    max_open_conns: 100
    max_idle_conns: 10
  
  redis:
    host: localhost
    port: 6379
    password: ""
    db: 0
    pool_size: 100
```

### 服务配置
```yaml
server:
  port: 8080
  grpc_port: 8081
  log_level: info
  environment: development
```

## 🚀 快速开始

### 1. 启动依赖服务
```bash
# 使用Docker Compose启动MySQL和Redis
docker-compose up -d mysql redis

# 或者手动启动本地服务
# MySQL: mysql.server start
# Redis: redis-server
```

### 2. 初始化数据库
```bash
# 连接MySQL并执行初始化脚本
mysql -u root -p < sql/init.sql
```

### 3. 运行服务
```bash
# 开发模式
go run cmd/server/main.go

# 或者使用Makefile
make run
```

### 4. 验证服务
```bash
# 检查服务状态
curl http://localhost:8080/health

# gRPC服务测试
go run cmd/client/main.go
```

## 📊 API 文档

### gRPC 服务接口
- `StartLive`: 开始直播
- `StopLive`: 停止直播
- `GetLiveStream`: 获取直播流信息
- `UpdateLiveStream`: 更新直播流
- `GetLiveRoom`: 获取直播间信息
- `JoinLiveRoom`: 加入直播间
- `LeaveLiveRoom`: 离开直播间
- `SendChatMessage`: 发送聊天消息
- `GetChatMessages`: 获取聊天消息
- `SendGift`: 发送礼物
- `GetGiftList`: 获取礼物列表
- `GetLiveStats`: 获取直播统计

### RESTful API
服务同时提供RESTful API接口，详细文档请访问: `http://localhost:8080/docs`

## 🐳 Docker 部署

### 构建镜像
```bash
docker build -t live-service:latest .
```

### 运行容器
```bash
docker-compose up -d
```

### 查看日志
```bash
docker-compose logs -f live-service
```

## 📈 性能优化

### 数据库优化
- 索引优化：为高频查询字段添加索引
- 分表分库：按用户ID或时间分片
- 读写分离：主从复制，读写分离

### 缓存策略
- Redis缓存：热点数据缓存
- 本地缓存：应用层缓存
- CDN缓存：静态资源缓存

### 并发处理
- 连接池：数据库连接池
- 协程池：限制并发数
- 消息队列：异步处理

## 🔍 监控与日志

### 健康检查
- HTTP健康检查接口: `/health`
- gRPC健康检查服务
- 数据库连接检查

### 指标监控
- Prometheus指标收集
- Grafana可视化展示
- 自定义业务指标

### 日志管理
- 结构化日志输出
- 日志级别控制
- ELK日志分析

## 🔐 安全特性

### 认证授权
- JWT Token认证
- RBAC权限控制
- API访问限流

### 数据安全
- 敏感数据加密
- SQL注入防护
- XSS攻击防护

### 内容安全
- 敏感词过滤
- 内容审核
- 用户举报处理

## 🧪 测试

### 单元测试
```bash
go test ./... -v
```

### 集成测试
```bash
go test ./test/... -v
```

### 性能测试
```bash
go test ./benchmark/... -v
```

## 📚 项目结构

```
live_service/
├── cmd/                    # 应用入口
│   └── server/            # 服务端入口
├── internal/              # 内部代码
│   ├── handler/           # 请求处理器
│   ├── service/           # 业务逻辑
│   ├── repository/        # 数据访问层
│   ├── model/             # 数据模型
│   └── converter/         # 数据转换器
├── pkg/                   # 公共包
│   ├── logger/            # 日志包
│   └── database/          # 数据库连接
├── proto/                 # Protobuf定义
├── sql/                   # SQL脚本
├── configs/               # 配置文件
├── scripts/               # 脚本文件
├── test/                  # 测试文件
├── Dockerfile             # Docker镜像
├── docker-compose.yml     # Docker编排
└── Makefile              # 构建脚本
```

## 🤝 贡献指南

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

## 📝 许可证

此项目基于 MIT 许可证开源 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🆘 支持与帮助

- 文档: [Wiki](https://github.com/your-repo/wiki)
- 问题反馈: [Issues](https://github.com/your-repo/issues)
- 邮件: support@example.com

## 📞 联系方式

- 项目维护者: Your Name
- 邮箱: your.email@example.com
- 微信: your_wechat_id

---

**⭐ 如果这个项目对你有帮助，请给个星标支持一下！**