package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthCheck 健康检查处理器
func HealthCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"message":   "Vision World Gateway is running",
			"timestamp": time.Now().Unix(),
			"metrics":   "Prometheus metrics available at /metrics",
		})
	}
}

// GrafanaHealthCheck Grafana健康检查处理器
func GrafanaHealthCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"service":   "vision_world_gateway",
			"version":   "1.0.0",
			"uptime":    time.Since(startTime).Seconds(),
		})
	}
}

var startTime = time.Now()

// GetStartTime 获取服务启动时间
func GetStartTime() time.Time {
	return startTime
}

// GetUptime 获取服务运行时间（秒）
func GetUptime() float64 {
	return time.Since(startTime).Seconds()
}
