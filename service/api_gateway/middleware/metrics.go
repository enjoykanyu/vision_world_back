package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// 自定义监控指标
var (
	// HTTP请求计数器
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vision_world_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	// HTTP请求持续时间
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "vision_world_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// 业务指标：用户注册数量
	userRegistrationsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "vision_world_user_registrations_total",
			Help: "Total number of user registrations",
		},
	)

	// 业务指标：用户登录数量
	userLoginsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vision_world_user_logins_total",
			Help: "Total number of user logins",
		},
		[]string{"status"},
	)
)

func init() {
	// 注册自定义指标
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(userRegistrationsTotal)
	prometheus.MustRegister(userLoginsTotal)
}

// MetricsMiddleware 监控中间件
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 处理请求
		c.Next()

		// 记录指标
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		httpRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(duration)
	}
}

// RecordUserRegistration 记录用户注册
func RecordUserRegistration() {
	userRegistrationsTotal.Inc()
}

// RecordUserLogin 记录用户登录
func RecordUserLogin(status string) {
	userLoginsTotal.WithLabelValues(status).Inc()
}

// GetHTTPRequestsTotal 获取HTTP请求总数指标（用于测试）
func GetHTTPRequestsTotal() *prometheus.CounterVec {
	return httpRequestsTotal
}

// GetHTTPRequestDuration 获取HTTP请求持续时间指标（用于测试）
func GetHTTPRequestDuration() *prometheus.HistogramVec {
	return httpRequestDuration
}

// GetUserRegistrationsTotal 获取用户注册总数指标（用于测试）
func GetUserRegistrationsTotal() prometheus.Counter {
	return userRegistrationsTotal
}

// GetUserLoginsTotal 获取用户登录总数指标（用于测试）
func GetUserLoginsTotal() *prometheus.CounterVec {
	return userLoginsTotal
}
