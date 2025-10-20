package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware 认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取Authorization头
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		// 检查Bearer前缀
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be Bearer {token}"})
			c.Abort()
			return
		}

		// 这里应该验证token的有效性
		// 为了简化，我们假设token是有效的，并从中提取用户ID
		// 在实际项目中，应该调用用户服务验证token
		token := parts[1]

		// 模拟从token中提取用户ID
		// 实际项目中应该解析JWT或调用验证服务
		userID := extractUserIDFromToken(token)
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// 将用户ID存储在上下文中
		c.Set("user_id", userID)
		c.Next()
	}
}

// extractUserIDFromToken 从token中提取用户ID
// 这是一个简化的实现，实际项目中应该解析JWT或调用验证服务
func extractUserIDFromToken(token string) string {
	// 这里只是示例，实际应该解析JWT或调用验证服务
	// 假设token格式为 "user_{userID}"
	if strings.HasPrefix(token, "user_") {
		return strings.TrimPrefix(token, "user_")
	}

	// 如果是有效的token格式，返回一个示例用户ID
	if token == "example_token" {
		return "12345"
	}

	return ""
}
