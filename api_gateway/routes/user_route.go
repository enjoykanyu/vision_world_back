package routes

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"api_gateway/client"
	"api_gateway/discovery"
	pb "api_gateway/proto/proto_gen/user"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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

	// 转换为前端期望的格式
	loginResponse := gin.H{
		"status_msg": resp.StatusMsg,
		"token":      resp.Token,
	}

	// 如果有用户信息，添加到响应中
	if resp.User != nil {
		user := gin.H{
			"id":               resp.User.Id,
			"name":             resp.User.Name,
			"phone":            resp.User.Phone,
			"follow_count":     resp.User.FollowCount,
			"follower_count":   resp.User.FollowerCount,
			"avatar":           resp.User.Avatar,
			"background_image": resp.User.BackgroundImage,
			"signature":        resp.User.Signature,
			"total_favorited":  resp.User.TotalFavorited,
			"work_count":       resp.User.WorkCount,
			"favorite_count":   resp.User.FavoriteCount,
			"create_time":      resp.User.CreateTime,
			"last_login_time":  resp.User.LastLoginTime,
			"user_type":        resp.User.UserType,
		}
		loginResponse["user"] = user
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": loginResponse,
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

	// 转换为前端期望的格式
	loginResponse := gin.H{
		"status_msg": resp.StatusMsg,
		"token":      resp.Token,
	}

	// 如果有用户信息，添加到响应中
	if resp.User != nil {
		user := gin.H{
			"id":               resp.User.Id,
			"name":             resp.User.Name,
			"phone":            resp.User.Phone,
			"follow_count":     resp.User.FollowCount,
			"follower_count":   resp.User.FollowerCount,
			"avatar":           resp.User.Avatar,
			"background_image": resp.User.BackgroundImage,
			"signature":        resp.User.Signature,
			"total_favorited":  resp.User.TotalFavorited,
			"work_count":       resp.User.WorkCount,
			"favorite_count":   resp.User.FavoriteCount,
			"create_time":      resp.User.CreateTime,
			"last_login_time":  resp.User.LastLoginTime,
			"user_type":        resp.User.UserType,
		}
		loginResponse["user"] = user
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": loginResponse,
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

// UpdateUserInfo 更新用户信息
func (h *UserHandler) UpdateUserInfo(c *gin.Context) {
	var req pb.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing authorization token"})
		return
	}

	if strings.HasPrefix(token, "Bearer ") {
		token = token[7:]
	}
	req.Token = token

	userClient, err := h.getUserClient()
	if err != nil {
		log.Printf("Failed to get user service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "User service temporarily unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := userClient.UpdateUserInfo(ctx, &req)
	if err != nil {
		log.Printf("UpdateUserInfo error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update user info failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status_code": resp.StatusCode,
		"status_msg":  resp.StatusMsg,
	})
}

// UploadAvatar 上传用户头像
func (h *UserHandler) UploadAvatar(c *gin.Context) {
	// 从请求头获取token
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing authorization token"})
		return
	}

	if strings.HasPrefix(token, "Bearer ") {
		token = token[7:]
	}

	// 验证token并获取用户ID
	verifyReq := &pb.VerifyTokenRequest{
		Token: token,
	}

	userClient, err := h.getUserClient()
	if err != nil {
		log.Printf("Failed to get user service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "User service temporarily unavailable"})
		return
	}

	verifyResp, err := userClient.VerifyToken(context.Background(), verifyReq)
	log.Println(verifyResp)
	log.Println(err)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	userId := verifyResp.UserId

	// 解析multipart表单
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data"})
		return
	}

	files := form.File["file"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	file := files[0]
	if !strings.HasPrefix(file.Header.Get("Content-Type"), "image/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File must be an image"})
		return
	}

	// 打开文件
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer src.Close()

	// 生成唯一文件名
	filename := fmt.Sprintf("user_%d_%s", userId, file.Filename)

	// 调用minio客户端上传文件
	// 初始化minio客户端
	minioClient, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		log.Printf("Failed to initialize minio client: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize minio client"})
		return
	}

	// 检查存储桶是否存在
	exists, err := minioClient.BucketExists(context.Background(), "vision-world")
	if err != nil {
		log.Printf("Failed to check bucket existence: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check bucket existence"})
		return
	}

	// 如果存储桶不存在则创建
	if !exists {
		err = minioClient.MakeBucket(context.Background(), "vision-world", minio.MakeBucketOptions{Region: "us-east-1"})
		if err != nil {
			log.Printf("Failed to create bucket: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create bucket"})
			return
		}
	}

	// 上传文件到minio
	_, err = minioClient.PutObject(context.Background(), "vision-world", "avatars/"+filename, src, file.Size, minio.PutObjectOptions{
		ContentType: file.Header.Get("Content-Type"),
	})
	if err != nil {
		log.Printf("Failed to upload file to minio: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload file to minio"})
		return
	}

	// 设置文件公共访问权限
	err = minioClient.SetBucketPolicy(context.Background(), "vision-world", fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Principal": "*",
			"Action": ["s3:GetObject"],
			"Resource": ["arn:aws:s3:::vision-world/avatars/*"]
		}]
	}`))
	if err != nil {
		log.Printf("Failed to set bucket policy: %v", err)
		// 权限设置失败不影响上传结果
	}

	// 生成minio访问地址
	minioUrl := fmt.Sprintf("http://localhost:9000/vision-world/avatars/%s", filename)

	// 不再自动更新用户信息，由前端在保存修改时统一更新

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"url":  minioUrl,
	})
}

// GetUserInfo 获取用户信息
func (h *UserHandler) GetUserInfo(c *gin.Context) {
	var userId uint32
	var err error

	// 尝试从路径参数获取用户ID
	userIdStr := c.Param("id")
	if userIdStr != "" {
		// 从路径参数获取ID
		id, err := strconv.ParseUint(userIdStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user id"})
			return
		}
		userId = uint32(id)
	} else {
		// 从认证信息获取用户ID（例如从token中解析）
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization token"})
			return
		}

		// 移除Bearer前缀（如果有）
		if strings.HasPrefix(authHeader, "Bearer ") {
			authHeader = authHeader[7:]
		}

		// 调用认证服务验证token并获取用户ID
		verifyReq := &pb.VerifyTokenRequest{
			Token: authHeader,
		}

		userClient, err := h.getUserClient()
		if err != nil {
			log.Printf("Failed to get user service client: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "User service temporarily unavailable"})
			return
		}

		verifyResp, err := userClient.VerifyToken(context.Background(), verifyReq)
		if err != nil || verifyResp.StatusCode != 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		userId = verifyResp.UserId
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
	log.Println(resp)
	log.Println("用户")
	// 转换为前端期望的格式
	userResponse := gin.H{
		"id":               resp.User.Id,
		"name":             resp.User.Name,
		"phone":            resp.User.Phone,
		"follow_count":     resp.User.FollowCount,
		"follower_count":   resp.User.FollowerCount,
		"avatar":           resp.User.Avatar,
		"background_image": resp.User.BackgroundImage,
		"signature":        resp.User.Signature,
		"total_favorited":  resp.User.TotalFavorited,
		"work_count":       resp.User.WorkCount,
		"favorite_count":   resp.User.FavoriteCount,
		"create_time":      resp.User.CreateTime,
		"last_login_time":  resp.User.LastLoginTime,
		"user_type":        resp.User.UserType,
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": userResponse,
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

// Logout 用户退出登录
func (h *UserHandler) Logout(c *gin.Context) {
	// 从请求头获取token
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing authorization token"})
		return
	}

	// 移除Bearer前缀（如果有）
	if strings.HasPrefix(token, "Bearer ") {
		token = token[7:]
	}

	req := &pb.LogoutRequest{
		Token: token,
	}

	userClient, err := h.getUserClient()
	if err != nil {
		log.Printf("Failed to get user service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "User service temporarily unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := userClient.LogOut(ctx, req)
	if err != nil {
		log.Printf("Logout error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Logout failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": resp,
	})
}
