# Vision World Gateway 监控设置指南

本指南将帮助您设置完整的 Prometheus 和 Grafana 监控环境。

## 快速开始

### 1. 启动监控服务

```bash
# 启动 Prometheus 和 Grafana
docker-compose -f docker-compose.monitoring.yml up -d

# 查看服务状态
docker-compose -f docker-compose.monitoring.yml ps
```

### 2. 访问监控界面

- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (默认用户名/密码: admin/admin)

### 3. 启动 Vision World Gateway

```bash
# 确保您的 Go 应用运行在端口 8080
go run main.go
```

## 详细配置

### Prometheus 配置

Prometheus 配置文件位于 `monitoring/prometheus.yml`，包含以下配置：

- 每 15 秒抓取一次指标
- 监控 Vision World Gateway (端口 8080)
- 监控 Prometheus 自身
- 可选的 Node Exporter 监控

### Grafana 配置

Grafana 已预配置：

- 数据源：Prometheus (http://prometheus:9090)
- 仪表板：Vision World Gateway Dashboard
- 自动导入的仪表板文件位于 `monitoring/grafana/provisioning/dashboards/`

## 监控指标说明

### HTTP 指标

| 指标名称 | 类型 | 描述 |
|---------|------|------|
| `vision_world_http_requests_total` | Counter | HTTP 请求总数 |
| `vision_world_http_request_duration_seconds` | Histogram | HTTP 请求持续时间 |

### 业务指标

| 指标名称 | 类型 | 描述 |
|---------|------|------|
| `vision_world_user_registrations_total` | Counter | 用户注册总数 |
| `vision_world_user_logins_total` | Counter | 用户登录总数 |

### 系统指标（自动收集）

| 指标名称 | 类型 | 描述 |
|---------|------|------|
| `go_goroutines` | Gauge | Goroutine 数量 |
| `go_memstats_alloc_bytes` | Gauge | 内存分配 |
| `process_cpu_seconds_total` | Counter | CPU 使用时间 |

## 使用示例

### 测试监控端点

```bash
# 健康检查
curl http://localhost:8080/health

# Grafana 健康检查
curl http://localhost:8080/grafana/health

# 查看 Prometheus 指标
curl http://localhost:8080/metrics
```

### 在业务代码中记录指标

```go
// 记录用户注册
middleware.RecordUserRegistration()

// 记录用户登录成功
middleware.RecordUserLogin("success")

// 记录用户登录失败
middleware.RecordUserLogin("failure")
```

## 仪表板功能

Vision World Gateway Dashboard 包含以下面板：

1. **HTTP Requests Total** - 总请求数统计
2. **HTTP Request Duration** - 请求延迟分布（50th 和 95th 百分位）
3. **User Registrations** - 用户注册总数
4. **User Login Success Rate** - 用户登录成功率饼图
5. **HTTP Requests by Method** - 按 HTTP 方法的请求分布
6. **HTTP Status Code Distribution** - HTTP 状态码分布
7. **Goroutines** - Goroutine 数量趋势

## 告警配置（可选）

您可以在 Prometheus 中添加告警规则：

```yaml
# monitoring/alerts.yml
groups:
  - name: vision-world-alerts
    rules:
      - alert: HighErrorRate
        expr: rate(vision_world_http_requests_total{status=~"5.."}[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"
          description: "Error rate is above 10%"

      - alert: HighLatency
        expr: histogram_quantile(0.95, rate(vision_world_http_request_duration_seconds_bucket[5m])) > 0.5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High latency detected"
          description: "95th percentile latency is above 500ms"
```

然后在 `prometheus.yml` 中添加：

```yaml
rule_files:
  - "alerts.yml"
```

## 故障排除

### 常见问题

1. **Prometheus 无法连接到应用**
   - 确保应用在端口 8080 运行
   - 检查防火墙设置
   - 验证 `/metrics` 端点是否可访问

2. **Grafana 无法显示数据**
   - 检查 Prometheus 数据源配置
   - 验证 Prometheus 是否正在抓取指标
   - 查看 Prometheus 日志：`docker logs vision-world-prometheus`

3. **指标不显示**
   - 确保应用已启动并处理请求
   - 检查 `/metrics` 端点输出
   - 验证 Prometheus 抓取配置

### 调试命令

```bash
# 查看 Prometheus 目标状态
curl http://localhost:9090/api/v1/targets

# 查询特定指标
curl 'http://localhost:9090/api/v1/query?query=vision_world_http_requests_total'

# 查看服务日志
docker-compose -f docker-compose.monitoring.yml logs -f prometheus
docker-compose -f docker-compose.monitoring.yml logs -f grafana
```

## 扩展阅读

- [Prometheus 官方文档](https://prometheus.io/docs/)
- [Grafana 官方文档](https://grafana.com/docs/)
- [Go Prometheus 客户端库](https://github.com/prometheus/client_golang)

## 支持

如有问题，请查看：
- `monitoring/README.md` - 监控配置文档
- `middleware/README.md` - 中间件文档
- 应用日志和 Docker 日志