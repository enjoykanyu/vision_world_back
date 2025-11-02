# 企业级微服务RPC调用重构方案

## 概述

本方案基于api_gateway的企业级实践，重构vision_world_back项目中各微服务之间的RPC调用方式，解决当前服务间直接依赖、硬编码地址、缺乏容错机制等问题。

## 核心改进点

### 1. 独立客户端库模式
- **问题**：当前服务直接导入其他服务的proto生成代码，导致强耦合
- **解决方案**：为每个服务创建独立的客户端库，封装所有RPC调用细节
- **实现**：创建`audit_service/client/`目录，包含完整的客户端实现

### 2. 服务发现与注册
- **问题**：服务间使用硬编码地址，缺乏动态发现机制
- **解决方案**：基于etcd的服务发现机制
- **实现**：
  - 服务启动时向etcd注册自身地址
  - 客户端通过etcd发现目标服务地址
  - 支持服务地址变更监听和自动更新

### 3. 熔断器模式
- **问题**：缺乏容错机制，服务故障会导致级联失败
- **解决方案**：实现熔断器模式，防止雪崩效应
- **实现**：
  - 失败计数和状态切换
  - 冷却时间机制
  - 健康状态检查

### 4. 连接池管理
- **问题**：每次RPC调用都创建新连接，资源浪费
- **解决方案**：连接池和懒加载机制
- **实现**：
  - 双重检查锁定模式的客户端获取
  - 连接复用和自动重连
  - 连接状态监控

## 文件结构

### audit_service客户端库
```
service/audit_service/client/
├── circuit_breaker.go    # 熔断器实现
├── client.go            # gRPC客户端封装
├── discovery.go         # 服务发现接口
├── discovery_etcd.go    # etcd服务发现实现
├── manager.go           # 客户端管理器
├── manager_new.go       # 简化版管理器
└── types.go             # 适配器类型定义
```

### 关键特性

1. **类型适配器**：创建独立的类型定义，避免直接依赖proto生成代码
2. **接口抽象**：使用接口定义降低服务间耦合度
3. **错误处理**：完善的错误处理和重试机制
4. **监控日志**：详细的调用日志和性能监控
5. **优雅关闭**：支持资源清理和连接关闭

## 使用方式

### 1. 创建客户端管理器
```go
// 创建audit_service客户端管理器
auditManager, err := auditclient.NewAuditClientManager(
    []string{"localhost:2379"}, // etcd地址
)
if err != nil {
    log.Error("Failed to create audit client manager", "error", err)
}
defer auditManager.Close()
```

### 2. 调用RPC方法
```go
// 提交内容审核
req := &auditclient.SubmitContentRequest{
    ContentId:   streamId,
    ContentType: 2, // 直播内容类型
    UploaderId:  userId,
    Title:       title,
    Content:     description,
    CreateTime:  time.Now().Format(time.RFC3339),
}

resp, err := auditManager.SubmitContent(ctx, req)
if err != nil {
    // 处理错误（包含熔断器状态）
    return err
}
```

### 3. 集成到服务处理器
```go
// 通过依赖注入传入auditManager
handler := handler.NewLiveServiceHandler(
    cfg, log, db, redisClient, auditManager,
)
```

## 优势对比

| 方面 | 原方案 | 新方案 |
|------|--------|--------|
| **耦合度** | 高（直接导入proto代码） | 低（独立客户端库） |
| **服务发现** | 无（硬编码地址） | 有（etcd动态发现） |
| **容错机制** | 无 | 熔断器模式 |
| **连接管理** | 每次新建连接 | 连接池复用 |
| **可维护性** | 差（修改影响多个服务） | 好（独立封装） |
| **可测试性** | 差 | 好（接口抽象） |

## 迁移步骤

1. **第一阶段**：创建audit_service独立客户端库
2. **第二阶段**：修改live_service使用新客户端
3. **第三阶段**：为其他服务创建客户端库
4. **第四阶段**：逐步替换所有服务间调用

## 运行方式

### 使用企业级handler（推荐）
```bash
cd service/live_service/cmd/server
go run main_new.go
```

### 使用简化版main（测试对比）
```bash
cd service/live_service/cmd/server
go run main_simple.go
```

## 后续优化

1. **配置化**：将etcd地址、熔断器参数等改为配置项
2. **监控指标**：集成Prometheus监控RPC调用指标
3. **链路追踪**：添加分布式链路追踪支持
4. **负载均衡**：实现更智能的负载均衡算法
5. **自动重试**：添加指数退避重试机制

## 总结

通过采用api_gateway的企业级实践，我们建立了一套完整的微服务RPC调用解决方案，显著提升了系统的可靠性、可维护性和扩展性。该方案可以作为项目中所有服务间调用的标准模式进行推广。