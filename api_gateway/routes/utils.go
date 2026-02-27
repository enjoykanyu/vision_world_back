package routes

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/status"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 返回成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// Error 返回错误响应
func Error(c *gin.Context, httpCode int, message string) {
	c.JSON(httpCode, Response{
		Code:    httpCode,
		Message: message,
	})
}

// ErrorWithData 返回带数据的错误响应
func ErrorWithData(c *gin.Context, httpCode int, message string, data interface{}) {
	c.JSON(httpCode, Response{
		Code:    httpCode,
		Message: message,
		Data:    data,
	})
}

// HandleGRPCError 处理 gRPC 错误
func HandleGRPCError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	st, ok := status.FromError(err)
	if ok {
		switch st.Code() {
		case 401:
			Error(c, http.StatusUnauthorized, st.Message())
		case 403:
			Error(c, http.StatusForbidden, st.Message())
		case 404:
			Error(c, http.StatusNotFound, st.Message())
		case 400:
			Error(c, http.StatusBadRequest, st.Message())
		default:
			Error(c, http.StatusInternalServerError, st.Message())
		}
	} else {
		log.Printf("gRPC call error: %v", err)
		Error(c, http.StatusInternalServerError, "Internal server error")
	}
}

// WithTimeout 创建带超时的上下文
func WithTimeout(seconds int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
}

// GetUserID 从 gin 上下文中获取用户ID
func GetUserID(c *gin.Context) uint32 {
	if userIDValue, exists := c.Get("user_id"); exists {
		if userID, ok := userIDValue.(uint32); ok {
			return userID
		}
	}
	return 0
}

// GetToken 从请求头中获取 token
func GetToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}
