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
	"api_gateway/middleware"
	danmakupb "api_gateway/proto/proto_gen/danmaku"

	"github.com/gin-gonic/gin"
)

// DanmakuHandler 弹幕处理器
type DanmakuHandler struct {
	danmakuClient      *client.DanmakuServiceClient
	discovery          *discovery.EtcdServiceDiscovery
	etcdEndpoints      []string
	danmakuServiceAddr string
	mu                 sync.RWMutex
	lastFailTime       time.Time
	circuitBreaker     *CircuitBreaker
}

// NewDanmakuHandler 创建弹幕处理器
func NewDanmakuHandler(etcdEndpoints []string) (*DanmakuHandler, error) {
	// 创建服务发现客户端
	serviceDiscovery, err := discovery.NewEtcdServiceDiscovery(etcdEndpoints, "social-service")
	if err != nil {
		return nil, err
	}

	handler := &DanmakuHandler{
		etcdEndpoints:  etcdEndpoints,
		discovery:      serviceDiscovery,
		circuitBreaker: NewCircuitBreaker(),
	}

	// 监听弹幕服务变化
	serviceDiscovery.WatchService(handler.onDanmakuServiceChange)

	return handler, nil
}

// onDanmakuServiceChange 弹幕服务变化处理
func (h *DanmakuHandler) onDanmakuServiceChange(serviceAddr string, isAdded bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if isAdded {
		if serviceAddr != h.danmakuServiceAddr {
			log.Printf("Danmaku service address changed from %s to %s", h.danmakuServiceAddr, serviceAddr)
			h.danmakuServiceAddr = serviceAddr

			// 关闭旧连接
			if h.danmakuClient != nil {
				h.danmakuClient.Close()
				h.danmakuClient = nil
			}

			// 重置熔断器
			h.circuitBreaker.RecordSuccess()
		}
	} else {
		log.Printf("Danmaku service instance removed: %s", serviceAddr)
		if serviceAddr == h.danmakuServiceAddr {
			h.danmakuServiceAddr = ""
			if h.danmakuClient != nil {
				h.danmakuClient.Close()
				h.danmakuClient = nil
			}
		}
	}
}

// getDanmakuClient 获取弹幕服务客户端（懒加载）
func (h *DanmakuHandler) getDanmakuClient() (*client.DanmakuServiceClient, error) {
	h.mu.RLock()
	if h.danmakuClient != nil && h.danmakuClient.IsConnected() {
		h.mu.RUnlock()
		return h.danmakuClient, nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	// 双重检查
	if h.danmakuClient != nil && h.danmakuClient.IsConnected() {
		return h.danmakuClient, nil
	}

	// 检查熔断器
	if !h.circuitBreaker.CanExecute() {
		return nil, fmt.Errorf("circuit breaker is open, please try again later")
	}

	// 检查服务地址
	if h.danmakuServiceAddr == "" {
		// 尝试发现服务
		serviceAddr, err := h.discovery.DiscoverService()
		if err != nil || serviceAddr == "" {
			h.circuitBreaker.RecordFailure()
			return nil, fmt.Errorf("danmaku service not available: %v", err)
		}
		h.danmakuServiceAddr = serviceAddr
	}

	// 创建客户端
	danmakuClient, err := client.NewDanmakuServiceClient(h.danmakuServiceAddr)
	if err != nil {
		h.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to create danmaku service client: %v", err)
	}

	h.danmakuClient = danmakuClient
	h.circuitBreaker.RecordSuccess()
	log.Printf("Successfully created danmaku service client for %s", h.danmakuServiceAddr)
	return h.danmakuClient, nil
}

// SendDanmaku 发送弹幕
func (h *DanmakuHandler) SendDanmaku(c *gin.Context) {
	// 获取请求参数
	var req struct {
		VideoID        uint32  `json:"video_id" binding:"required"`
		Text           string  `json:"text" binding:"required,max=200"`
		Color          string  `json:"color"`
		VideoTimestamp float32 `json:"video_timestamp" binding:"required"`
		Speed          string  `json:"speed"`
	}

	// 绑定请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters: " + err.Error()})
		return
	}

	// 获取弹幕服务客户端
	danmakuClient, err := h.getDanmakuClient()
	if err != nil {
		log.Printf("Failed to get danmaku service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Danmaku service temporarily unavailable"})
		return
	}

	// 调用弹幕服务的SendDanmaku接口
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	grpcReq := &danmakupb.SendDanmakuRequest{
		VideoId:        req.VideoID,
		Text:           req.Text,
		Color:          req.Color,
		VideoTimestamp: req.VideoTimestamp,
		Speed:          req.Speed,
	}

	resp, err := danmakuClient.SendDanmaku(ctx, grpcReq)
	if err != nil {
		log.Printf("Failed to send danmaku: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send danmaku"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetDanmakus 获取视频弹幕列表
func (h *DanmakuHandler) GetDanmakus(c *gin.Context) {
	// 获取路径参数
	videoIDStr := c.Param("video_id")
	videoID, err := strconv.ParseUint(videoIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	// 获取查询参数
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "20")

	// 转换参数
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 获取弹幕服务客户端
	danmakuClient, err := h.getDanmakuClient()
	if err != nil {
		log.Printf("Failed to get danmaku service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Danmaku service temporarily unavailable"})
		return
	}

	// 调用弹幕服务的GetDanmakus接口
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	grpcReq := &danmakupb.GetDanmakusRequest{
		VideoId:  uint32(videoID),
		Page:     int32(page),
		PageSize: int32(pageSize),
	}

	resp, err := danmakuClient.GetDanmakus(ctx, grpcReq)
	if err != nil {
		log.Printf("Failed to get danmakus: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get danmakus"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Close 关闭连接
func (h *DanmakuHandler) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.danmakuClient != nil {
		h.danmakuClient.Close()
		h.danmakuClient = nil
	}
}

// RegisterDanmakuRoutes 注册弹幕相关路由
func RegisterDanmakuRoutes(router *gin.Engine, etcdEndpoints []string) {
	// 创建弹幕处理器
	danmakuHandler, err := NewDanmakuHandler(etcdEndpoints)
	if err != nil {
		log.Fatalf("Failed to create danmaku handler: %v", err)
	}

	// 弹幕相关路由组
	danmakuGroup := router.Group("/api/danmaku")
	{
		// 公开路由
		danmakuGroup.GET("/:video_id", danmakuHandler.GetDanmakus)

		// 需要认证的路由
		authGroup := danmakuGroup.Group("/")
		authGroup.Use(middleware.RequireAuthMiddleware())
		{
			authGroup.POST("/send", danmakuHandler.SendDanmaku)
		}
	}
}
