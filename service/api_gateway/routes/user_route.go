package routes

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"api_gateway/client"
	"api_gateway/discovery"
	"api_gateway/proto/proto_gen/proto"
)

// UserHandler 用户服务处理器
type UserHandler struct {
	userClient *client.UserServiceClient
	discovery  *discovery.EtcdServiceDiscovery
}

// NewUserHandler 创建用户处理器
func NewUserHandler(etcdEndpoints []string) (*UserHandler, error) {
	// 创建服务发现客户端
	serviceDiscovery, err := discovery.NewEtcdServiceDiscovery(etcdEndpoints, "user-service")
	if err != nil {
		return nil, err
	}

	// 发现用户服务地址
	serviceAddr, err := serviceDiscovery.DiscoverService()
	if err != nil {
		return nil, err
	}

	// 创建gRPC客户端
	userClient, err := client.NewUserServiceClient(serviceAddr)
	if err != nil {
		return nil, err
	}

	// 创建gRPC服务客户端
	// grpcClient := proto_gen.NewUserServiceClient(userClient.GetConnection())

	handler := &UserHandler{
		userClient: userClient,
		discovery:  serviceDiscovery,
	}

	// 监听服务变化
	serviceDiscovery.WatchService(handler.onServiceChange)

	return handler, nil
}

// onServiceChange 处理服务变化
func (h *UserHandler) onServiceChange(serviceAddr string, isAdded bool) {
	if isAdded {
		log.Printf("Service instance added: %s", serviceAddr)
		// 这里可以实现重连逻辑
	} else {
		log.Printf("Service instance removed: %s", serviceAddr)
	}
}

// Close 关闭资源
func (h *UserHandler) Close() {
	if h.userClient != nil {
		h.userClient.Close()
	}
	if h.discovery != nil {
		h.discovery.Close()
	}
}

// PhoneLoginRequest 手机号登录请求结构
type PhoneLoginRequest struct {
	Phone      string `json:"phone" binding:"required"`
	Password   string `json:"password" binding:"required"`
	DeviceID   string `json:"device_id"`
	OSType     string `json:"os_type"`
	AppVersion string `json:"app_version"`
}

// CodeLoginRequest 验证码登录请求结构
type CodeLoginRequest struct {
	Phone      string `json:"phone" binding:"required"`
	Code       string `json:"code" binding:"required"`
	DeviceID   string `json:"device_id"`
	OSType     string `json:"os_type"`
	AppVersion string `json:"app_version"`
}

// SendSmsRequest 发送短信请求结构
type SendSmsRequest struct {
	Phone   string `json:"phone" binding:"required"`
	SmsType string `json:"sms_type"`
}

// PhoneLogin 手机号登录
func (h *UserHandler) PhoneLogin(c *gin.Context) {
	var req PhoneLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status_code": 400,
			"status_msg":  "参数错误: " + err.Error(),
		})
		return
	}

	// 创建gRPC请求
	grpcReq := &proto_gen.PhoneLoginRequest{
		Phone:      req.Phone,
		Password:   req.Password,
		DeviceId:   req.DeviceID,
		OsType:     req.OSType,
		AppVersion: req.AppVersion,
	}

	// 设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 调用gRPC服务
	resp, err := h.userClient.PhoneLogin(ctx, grpcReq)
	if err != nil {
		log.Printf("PhoneLogin gRPC call failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status_code": 500,
			"status_msg":  "服务调用失败",
		})
		return
	}

	// 返回响应
	c.JSON(http.StatusOK, gin.H{
		"status_code": resp.StatusCode,
		"status_msg":  resp.StatusMsg,
		"token":       resp.Token,
		"expire_time": resp.ExpireTime,
		"user":        resp.User,
		"is_new_user": resp.IsNewUser,
	})
}

// CodeLogin 验证码登录
func (h *UserHandler) CodeLogin(c *gin.Context) {
	var req CodeLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status_code": 400,
			"status_msg":  "参数错误: " + err.Error(),
		})
		return
	}

	// 创建gRPC请求
	grpcReq := &proto_gen.CodeLoginRequest{
		Phone:      req.Phone,
		Code:       req.Code,
		DeviceId:   req.DeviceID,
		OsType:     req.OSType,
		AppVersion: req.AppVersion,
	}

	// 设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 调用gRPC服务
	resp, err := h.userClient.CodeLogin(ctx, grpcReq)
	if err != nil {
		log.Printf("CodeLogin gRPC call failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status_code": 500,
			"status_msg":  "服务调用失败",
		})
		return
	}

	// 返回响应
	c.JSON(http.StatusOK, gin.H{
		"status_code": resp.StatusCode,
		"status_msg":  resp.StatusMsg,
		"token":       resp.Token,
		"expire_time": resp.ExpireTime,
		"user":        resp.User,
		"is_new_user": resp.IsNewUser,
	})
}

// SendSmsCode 发送短信验证码
func (h *UserHandler) SendSmsCode(c *gin.Context) {
	var req SendSmsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status_code": 400,
			"status_msg":  "参数错误: " + err.Error(),
		})
		return
	}

	// 设置默认短信类型
	if req.SmsType == "" {
		req.SmsType = "login"
	}

	// 创建gRPC请求
	grpcReq := &proto_gen.SendSmsRequest{
		Phone:   req.Phone,
		SmsType: req.SmsType,
	}

	// 设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 调用gRPC服务
	resp, err := h.userClient.SendSmsCode(ctx, grpcReq)
	if err != nil {
		log.Printf("SendSmsCode gRPC call failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status_code": 500,
			"status_msg":  "服务调用失败",
		})
		return
	}

	// 返回响应
	c.JSON(http.StatusOK, gin.H{
		"status_code":    resp.StatusCode,
		"status_msg":     resp.StatusMsg,
		"expire_seconds": resp.ExpireSeconds,
	})
}

// GetUserInfo 获取用户信息
func (h *UserHandler) GetUserInfo(c *gin.Context) {
	// 获取用户ID参数
	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status_code": 400,
			"status_msg":  "用户ID格式错误",
		})
		return
	}

	// 获取token（可选）
	token := c.GetHeader("Authorization")
	if token != "" && len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	// 创建gRPC请求
	grpcReq := &proto_gen.GetUserInfoRequest{
		UserId: uint32(userID),
		Token:  token,
	}

	// 设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 调用gRPC服务
	resp, err := h.userClient.GetUserInfo(ctx, grpcReq)
	if err != nil {
		log.Printf("GetUserInfo gRPC call failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status_code": 500,
			"status_msg":  "服务调用失败",
		})
		return
	}

	// 返回响应
	c.JSON(http.StatusOK, gin.H{
		"status_code": resp.StatusCode,
		"status_msg":  resp.StatusMsg,
		"user":        resp.User,
	})
}
