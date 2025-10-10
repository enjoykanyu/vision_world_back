# Search Service

搜索服务是VisionWorld项目的核心服务之一，负责提供视频、用户和内容的搜索功能。

## 目录结构

```
search_service/
├── cmd/
│   └── server/           # 服务启动入口
│       └── main.go
├── config/               # 配置文件
│   └── search-service.yaml
├── internal/             # 内部模块
│   ├── config/           # 配置解析
│   ├── discovery/        # 服务发现
│   ├── handler/          # 请求处理器
│   ├── model/            # 数据模型
│   ├── repository/       # 数据访问层
│   └── service/          # 业务逻辑层
└── pkg/                  # 公共包
    ├── database/         # 数据库连接
    └── logger/           # 日志记录
```

## 功能特性

1. **多类型搜索**：支持视频、用户、内容等多种类型的搜索
2. **全文检索**：基于Elasticsearch实现高效的全文检索
3. **模糊搜索**：支持模糊匹配提高搜索准确性
4. **搜索建议**：提供搜索关键词建议功能
5. **搜索过滤**：支持多种字段的过滤条件
6. **搜索排序**：支持按相关性、时间等字段排序
7. **搜索日志**：记录搜索行为用于分析优化
8. **缓存机制**：使用Redis缓存热门搜索结果

## 配置说明

配置文件位于 `config/search-service.yaml`，主要配置项包括：

- 服务器配置
- 数据库连接配置
- Redis连接配置
- Elasticsearch配置
- 搜索相关配置（分页、超时、模糊搜索等）
- 索引配置
- 分词配置
- 搜索类型配置
- 搜索建议配置
- 日志配置
- 缓存配置

## 启动服务

```bash
cd cmd/server
go run main.go
```

## API接口

TODO: 待定义gRPC接口

## 开发指南

1. 添加新的搜索类型：
   - 在配置文件中添加类型配置
   - 在[model](file:///Users/kanyu/Desktop/project/kanyu_server/new_project/project/vision_world_back/service/search_service/internal/model/search.go)目录下创建对应的数据模型
   - 在[repository](file:///Users/kanyu/Desktop/project/kanyu_server/new_project/project/vision_world_back/service/search_service/internal/repository/search_repository.go)中实现数据访问逻辑
   - 在[service](file:///Users/kanyu/Desktop/project/kanyu_server/new_project/project/vision_world_back/service/search_service/internal/service/search_service.go)中实现业务逻辑

2. 调整搜索算法：
   - 修改[repository](file:///Users/kanyu/Desktop/project/kanyu_server/new_project/project/vision_world_back/service/search_service/internal/repository/search_repository.go)中的搜索实现
   - 调整配置文件中的相关参数

3. 优化搜索性能：
   - 调整Elasticsearch配置
   - 优化索引策略
   - 调整缓存策略