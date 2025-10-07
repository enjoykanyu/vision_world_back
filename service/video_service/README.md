# Video Service 视频服务

视频服务负责处理所有与视频相关的业务逻辑，包括视频发布、获取、互动等功能。

## 功能特性

- 🎥 视频发布与管理
- 📺 视频信息获取
- 📊 视频统计与计数
- 👍 视频点赞与互动
- 💬 视频评论系统
- 🔖 视频收藏功能
- 📤 视频分享功能
- 🎯 个性化推荐
- 📱 多平台支持

## 项目结构

```
video_service/
├── cmd/server/          # 服务入口
├── config/              # 配置文件
├── internal/            # 内部代码
│   ├── config/          # 配置管理
│   ├── handler/         # gRPC处理器
│   ├── model/           # 数据模型
│   ├── repository/      # 数据访问层
│   └── service/         # 业务逻辑层
├── pkg/                 # 公共包
│   ├── database/        # 数据库连接
│   └── logger/          # 日志管理
├── proto/proto_gen/     # 生成的protobuf代码
└── logs/                # 日志文件
```

## API接口

### 视频发布相关
- `PublishVideo` - 发布视频
- `DeleteVideo` - 删除视频

### 视频信息获取
- `GetVideoInfo` - 获取单个视频信息
- `GetVideoInfos` - 批量获取视频信息

### 视频列表相关
- `GetUserVideos` - 获取用户发布的视频列表
- `GetRecommendVideos` - 获取推荐视频列表
- `GetFollowVideos` - 获取关注用户的视频列表

### 视频互动相关
- `LikeVideo` - 点赞/取消点赞视频
- `GetUserLikedVideos` - 获取用户点赞的视频列表
- `ShareVideo` - 分享视频

### 视频评论相关
- `CommentVideo` - 发表评论
- `DeleteComment` - 删除评论
- `GetVideoComments` - 获取视频评论列表

## 快速开始

1. 安装依赖
```bash
go mod download
```

2. 配置数据库
修改 `config/video-service.yaml` 中的数据库配置

3. 运行服务
```bash
go run cmd/server/main.go
```

## 配置说明

服务配置文件位于 `config/video-service.yaml`，包含以下配置项：

- **server**: 服务端口和基本信息
- **database**: MySQL数据库连接配置
- **redis**: Redis缓存配置
- **kafka**: Kafka消息队列配置
- **discovery**: 服务发现配置
- **log**: 日志配置

## 数据库设计

主要数据表：
- `videos` - 视频信息表
- `video_likes` - 视频点赞表
- `video_comments` - 视频评论表
- `video_shares` - 视频分享表
- `video_favorites` - 视频收藏表
- `video_views` - 视频观看记录表
- `video_categories` - 视频分类表
- `video_tags` - 视频标签表
- `video_tag_relations` - 视频标签关联表

## TODO

- [ ] 实现具体的业务逻辑
- [ ] 添加单元测试
- [ ] 集成用户认证
- [ ] 实现推荐算法
- [ ] 添加监控和告警
- [ ] 优化数据库查询
- [ ] 添加缓存策略
- [ ] 实现文件上传功能