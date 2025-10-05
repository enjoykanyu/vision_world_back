# Middleware 中间件

本目录包含 Vision World Gateway 的所有中间件组件。

## 中间件列表

### 1. Metrics Middleware (`metrics.go`)
负责收集和暴露 Prometheus 监控指标。

**功能：**
- HTTP 请求计数 (`vision_world_http_requests_total`)
- HTTP 请求持续时间 (`vision_world_http_request_duration_seconds`)
- 用户注册计数 (`vision_world_user_registrations_total`)
- 用户登录计数 (`vision_world_user_logins_total`)

**使用：**
```go
router.Use(middleware.MetricsMiddleware())
```

**业务指标记录：**
```go
// 记录用户注册
middleware.RecordUserRegistration()

// 记录用户登录成功
middleware.RecordUserLogin("success")

// 记录用户登录失败
middleware.RecordUserLogin("failure")
```

### 2. CORS Middleware (`cors.go`)
处理跨域请求配置。

**功能：**
- 支持所有来源
- 支持常见 HTTP 方法
- 支持认证头
- 12 小时缓存时间

**使用：**
```go
router.Use(middleware.CORSMiddleware())
```

### 3. Logger Middleware (`logger.go`)
自定义日志记录中间件。

**功能：**
- 结构化日志格式
- 包含时间戳、方法、路径、状态码等信息
- 错误恢复处理

**使用：**
```go
router.Use(middleware.LoggerMiddleware())
router.Use(middleware.RecoveryMiddleware())
```

### 4. Health Check Middleware (`health.go`)
健康检查和系统状态中间件。

**功能：**
- 基础健康检查 (`/health`)
- Grafana 健康检查 (`/grafana/health`)
- 系统运行时间统计

**使用：**
```go
router.GET("/health", middleware.HealthCheck())
router.GET("/grafana/health", middleware.GrafanaHealthCheck())
```

## 使用示例

```go
package main

import (
    "github.com/gin-gonic/gin"
    "web/middleware"
)

func main() {
    router := gin.New()
    
    // 基础中间件
    router.Use(middleware.LoggerMiddleware())
    router.Use(middleware.RecoveryMiddleware())
    router.Use(middleware.CORSMiddleware())
    router.Use(middleware.MetricsMiddleware())
    
    // 健康检查
    router.GET("/health", middleware.HealthCheck())
    router.GET("/grafana/health", middleware.GrafanaHealthCheck())
    
    // 其他路由...
}
```

## 监控指标说明

### HTTP 指标
- `vision_world_http_requests_total{method, endpoint, status}` - HTTP 请求总数
- `vision_world_http_request_duration_seconds{method, endpoint}` - HTTP 请求持续时间

### 业务指标
- `vision_world_user_registrations_total` - 用户注册总数
- `vision_world_user_logins_total{status}` - 用户登录总数（按状态分类）

### 系统指标（自动收集）
- `go_goroutines` - Goroutine 数量
- `go_memstats_alloc_bytes` - 内存分配
- `process_cpu_seconds_total` - CPU 使用时间

## 测试指标

```bash
# 查看所有指标
curl http://localhost:8080/metrics

# 查看特定指标
curl http://localhost:8080/metrics | grep vision_world

# 健康检查
curl http://localhost:8080/health
curl http://localhost:8080/grafana/health
```

## 扩展开发

如需添加新的监控指标：

1. 在 `metrics.go` 中定义新的指标
2. 在相应的业务逻辑中调用记录函数
3. 更新文档说明

示例：
```go
// 定义新指标
orderTotal := prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "vision_world_orders_total",
        Help: "Total number of orders",
    },
    []string{"status"},
)

// 注册指标
prometheus.MustRegister(orderTotal)

// 记录指标
orderTotal.WithLabelValues("completed").Inc()
```