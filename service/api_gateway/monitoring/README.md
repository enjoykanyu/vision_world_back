# Vision World Gateway 监控配置

## 概述
本项目集成了 Prometheus 和 Grafana 监控，提供完整的应用性能监控解决方案。

## 快速开始

### 1. Prometheus 配置

创建 `prometheus.yml` 文件：

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'vision-world-gateway'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 5s
    scrape_timeout: 3s

  - job_name: 'grafana-health'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/grafana/health'
    scrape_interval: 10s
```

### 2. 启动 Prometheus

```bash
docker run -d \
  --name=prometheus \
  -p 9090:9090 \
  -v $(pwd)/prometheus.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus:latest
```

### 3. 启动 Grafana

```bash
docker run -d \
  --name=grafana \
  -p 3000:3000 \
  -e GF_SECURITY_ADMIN_PASSWORD=admin \
  grafana/grafana:latest
```

### 4. 配置 Grafana

1. 访问 http://localhost:3000
2. 登录：admin/admin
3. 添加数据源：
   - 类型：Prometheus
   - URL：http://prometheus:9090
   - 保存并测试

## 可用指标

### HTTP 指标
- `vision_world_http_requests_total` - HTTP请求总数
- `vision_world_http_request_duration_seconds` - HTTP请求持续时间

### 业务指标
- `vision_world_user_registrations_total` - 用户注册总数
- `vision_world_user_logins_total` - 用户登录总数

### 系统指标（自动收集）
- `go_goroutines` - Goroutine数量
- `go_memstats_alloc_bytes` - 内存分配
- `process_cpu_seconds_total` - CPU使用时间

## 监控端点

- **应用指标**: http://localhost:8080/metrics
- **健康检查**: http://localhost:8080/health
- **Grafana健康检查**: http://localhost:8080/grafana/health

## Grafana 仪表板

### 导入仪表板

1. 在Grafana中点击 "+" -> "Import"
2. 上传 `grafana-dashboard.json` 文件
3. 选择Prometheus数据源
4. 点击 "Import"

### 关键面板

1. **HTTP请求概览**
   - 请求速率
   - 错误率
   - 响应时间分布

2. **业务指标**
   - 用户注册趋势
   - 登录成功率
   - 活跃用户

3. **系统性能**
   - Goroutine数量
   - 内存使用情况
   - CPU使用率

## 告警配置

在Prometheus中配置告警规则：

```yaml
groups:
- name: vision-world-alerts
  rules:
  - alert: HighErrorRate
    expr: rate(vision_world_http_requests_total{status=~"5.."}[5m]) > 0.1
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High error rate detected"
      description: "Error rate is above 10%"

  - alert: HighLatency
    expr: histogram_quantile(0.95, vision_world_http_request_duration_seconds_bucket) > 1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High latency detected"
      description: "95th percentile latency is above 1 second"
```

## 本地开发

### 查看指标

```bash
# 查看所有指标
curl http://localhost:8080/metrics

# 查看特定指标
curl http://localhost:8080/metrics | grep vision_world
```

### 测试监控

```bash
# 生成一些测试请求
for i in {1..100}; do
  curl -s http://localhost:8080/health > /dev/null
  curl -s http://localhost:8080/grafana/health > /dev/null
done
```

## 故障排除

### 常见问题

1. **指标未显示**
   - 检查应用是否正常运行
   - 确认Prometheus能够访问 `/metrics` 端点
   - 检查Prometheus配置中的目标地址

2. **Grafana无法连接Prometheus**
   - 确认Prometheus容器正在运行
   - 检查网络连接
   - 验证Prometheus的URL配置

3. **自定义指标不显示**
   - 确认指标名称拼写正确
   - 检查指标的label匹配
   - 查看Prometheus的target页面

### 调试命令

```bash
# 检查Prometheus目标
curl http://localhost:9090/api/v1/targets

# 检查指标
curl http://localhost:9090/api/v1/query?query=vision_world_http_requests_total

# 查看Prometheus日志
docker logs prometheus
```