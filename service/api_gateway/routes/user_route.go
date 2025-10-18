package routes

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"api_gateway/client"
	"api_gateway/discovery"
	pb "api_gateway/proto/proto_gen/proto"

	"github.com/gin-gonic/gin"
)

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	failCount    int
	lastFailTime time.Time
	isOpen       bool
	mutex        sync.Mutex
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		lastFailTime: time.Now(),
	}
}

// CanExecute 检查是否可以执行请求
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if cb.isOpen {
		// 熔断器开启，检查是否过了冷却时间（30秒）
		if time.Since(cb.lastFailTime) > 30*time.Second {
			cb.isOpen = false
			cb.failCount = 0
			return true
		}
		return false
	}
	return true
}

// RecordSuccess 记录成功
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.failCount = 0
	cb.isOpen = false
}

// RecordFailure 记录失败
func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.failCount++
	cb.lastFailTime = time.Now()

	// 连续失败3次开启熔断器
	if cb.failCount >= 3 {
		cb.isOpen = true
		log.Printf("Circuit breaker opened due to %d consecutive failures", cb.failCount)
	}
}

// UserHandler 用户处理器
type UserHandler struct {
	userClient     *client.UserServiceClient
	discovery      *discovery.EtcdServiceDiscovery
	etcdEndpoints  []string
	serviceAddr    string
	mu             sync.RWMutex
	lastFailTime   time.Time
	circuitBreaker *CircuitBreaker
}

// NewUserHandler 创建用户处理器
func NewUserHandler(etcdEndpoints []string) (*UserHandler, error) {
	// 创建服务发现客户端
	serviceDiscovery, err := discovery.NewEtcdServiceDiscovery(etcdEndpoints, "user-service")
	if err != nil {
		return nil, err
	}

	handler := &UserHandler{
		etcdEndpoints:  etcdEndpoints,
		discovery:      serviceDiscovery,
		circuitBreaker: NewCircuitBreaker(),
	}

	// 监听服务变化
	serviceDiscovery.WatchService(handler.onServiceChange)

	return handler, nil
}

// onServiceChange 服务变化处理
func (h *UserHandler) onServiceChange(serviceAddr string, isAdded bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if isAdded {
		if serviceAddr != h.serviceAddr {
			log.Printf("User service address changed from %s to %s", h.serviceAddr, serviceAddr)
			h.serviceAddr = serviceAddr

			// 关闭旧连接
			if h.userClient != nil {
				h.userClient.Close()
				h.userClient = nil
			}

			// 重置熔断器
			h.circuitBreaker.RecordSuccess()
		}
	} else {
		log.Printf("User service instance removed: %s", serviceAddr)
		if serviceAddr == h.serviceAddr {
			h.serviceAddr = ""
			if h.userClient != nil {
				h.userClient.Close()
				h.userClient = nil
			}
		}
	}
}

// getUserClient 获取用户服务客户端（懒加载）
func (h *UserHandler) getUserClient() (*client.UserServiceClient, error) {
	h.mu.RLock()
	if h.userClient != nil && h.userClient.IsConnected() {
		h.mu.RUnlock()
		return h.userClient, nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	// 双重检查
	if h.userClient != nil && h.userClient.IsConnected() {
		return h.userClient, nil
	}

	// 检查熔断器
	if !h.circuitBreaker.CanExecute() {
		return nil, fmt.Errorf("circuit breaker is open, please try again later")
	}

	// 检查服务地址
	if h.serviceAddr == "" {
		// 尝试发现服务
		serviceAddr, err := h.discovery.DiscoverService()
		if err != nil || serviceAddr == "" {
			h.circuitBreaker.RecordFailure()
			return nil, fmt.Errorf("user service not available: %v", err)
		}
		h.serviceAddr = serviceAddr
	}

	// 创建客户端
	userClient, err := client.NewUserServiceClient(h.serviceAddr)
	if err != nil {
		h.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to create user service client: %v", err)
	}

	h.userClient = userClient
	h.circuitBreaker.RecordSuccess()
	log.Printf("Successfully created user service client for %s", h.serviceAddr)
	return h.userClient, nil
}

// PhoneLogin 手机号登录
func (h *UserHandler) PhoneLogin(c *gin.Context) {
	var req pb.PhoneLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	userClient, err := h.getUserClient()
	if err != nil {
		log.Printf("Failed to get user service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "User service temporarily unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := userClient.PhoneLogin(ctx, &req)
	if err != nil {
		log.Printf("PhoneLogin error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Login failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": resp,
	})
}

// CodeLogin 验证码登录
func (h *UserHandler) CodeLogin(c *gin.Context) {
	var req pb.CodeLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	userClient, err := h.getUserClient()
	if err != nil {
		log.Printf("Failed to get user service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "User service temporarily unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := userClient.CodeLogin(ctx, &req)
	if err != nil {
		log.Printf("CodeLogin error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Login failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": resp,
	})
}

// SendSmsCode 发送短信验证码
func (h *UserHandler) SendSmsCode(c *gin.Context) {
	var req pb.SendSmsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// 设置默认短信类型
	if req.SmsType == "" {
		req.SmsType = "login"
	}

	userClient, err := h.getUserClient()
	if err != nil {
		log.Printf("Failed to get user service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "User service temporarily unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := userClient.SendSmsCode(ctx, &req)
	if err != nil {
		log.Printf("SendSmsCode error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send SMS"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": resp,
	})
}

// GetUserInfo 获取用户信息
func (h *UserHandler) GetUserInfo(c *gin.Context) {
	// 用户请求只需要id参数
	userIdStr := c.Param("id")
	if userIdStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user id parameter"})
		return
	}

	userId, err := strconv.ParseUint(userIdStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user id"})
		return
	}

	req := &pb.GetUserInfoRequest{
		UserId: uint32(userId),
		Token:  c.GetHeader("Authorization"), // 可选的token
	}

	userClient, err := h.getUserClient()
	if err != nil {
		log.Printf("Failed to get user service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "User service temporarily unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := userClient.GetUserInfo(ctx, req)
	if err != nil {
		log.Printf("GetUserInfo error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": resp,
	})
}

// Close 关闭处理器
func (h *UserHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.userClient != nil {
		return h.userClient.Close()
	}
	return nil
}

// VerifyToken 验证Token
func (h *UserHandler) VerifyToken(c *gin.Context) {
	var req pb.VerifyTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	userClient, err := h.getUserClient()
	if err != nil {
		log.Printf("Failed to get user service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "User service temporarily unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := userClient.VerifyToken(ctx, &req)
	if err != nil {
		log.Printf("VerifyToken error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token verification failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": resp,
	})
}

// RefreshToken 刷新Token
func (h *UserHandler) RefreshToken(c *gin.Context) {
	var req pb.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	userClient, err := h.getUserClient()
	if err != nil {
		log.Printf("Failed to get user service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "User service temporarily unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := userClient.RefreshToken(ctx, &req)
	if err != nil {
		log.Printf("RefreshToken error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token refresh failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": resp,
	})
}
