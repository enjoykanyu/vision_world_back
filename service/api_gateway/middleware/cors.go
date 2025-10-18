package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"time"
)

// CORSConfig CORS配置
func CORSConfig() cors.Config {
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization", "X-Requested-With", "Accept", "Cache-Control", "X-Platform", "X-Client-Version", "X-Request-ID"}
	config.ExposeHeaders = []string{"Content-Length", "Authorization"}
	config.AllowCredentials = true
	config.MaxAge = 12 * time.Hour
	return config
}

// CORSMiddleware CORS中间件
func CORSMiddleware() gin.HandlerFunc {
	return cors.New(CORSConfig())
}
