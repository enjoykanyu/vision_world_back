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

// VideoHandler 视频处理器
type VideoHandler struct {
	videoClient          *client.VideoServiceClient
	recommendClient      *client.RecommendationServiceClient
	discovery            *discovery.EtcdServiceDiscovery
	etcdEndpoints        []string
	videoServiceAddr     string
	recommendServiceAddr string
	mu                   sync.RWMutex
	lastFailTime         time.Time
	circuitBreaker       *CircuitBreaker
}

// NewVideoHandler 创建视频处理器
func NewVideoHandler(etcdEndpoints []string) (*VideoHandler, error) {
	// 创建服务发现客户端
	serviceDiscovery, err := discovery.NewEtcdServiceDiscovery(etcdEndpoints, "video-service")
	if err != nil {
		return nil, err
	}

	// 创建推荐服务发现客户端
	recommendDiscovery, err := discovery.NewEtcdServiceDiscovery(etcdEndpoints, "recommendation-service")
	if err != nil {
		return nil, err
	}

	handler := &VideoHandler{
		etcdEndpoints:  etcdEndpoints,
		discovery:      serviceDiscovery,
		circuitBreaker: NewCircuitBreaker(),
	}

	// 监听视频服务变化
	serviceDiscovery.WatchService(handler.onVideoServiceChange)

	// 监听推荐服务变化
	recommendDiscovery.WatchService(handler.onRecommendServiceChange)

	return handler, nil
}

// onVideoServiceChange 视频服务变化处理
func (h *VideoHandler) onVideoServiceChange(serviceAddr string, isAdded bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if isAdded {
		if serviceAddr != h.videoServiceAddr {
			log.Printf("Video service address changed from %s to %s", h.videoServiceAddr, serviceAddr)
			h.videoServiceAddr = serviceAddr

			// 关闭旧连接
			if h.videoClient != nil {
				h.videoClient.Close()
				h.videoClient = nil
			}

			// 重置熔断器
			h.circuitBreaker.RecordSuccess()
		}
	} else {
		log.Printf("Video service instance removed: %s", serviceAddr)
		if serviceAddr == h.videoServiceAddr {
			h.videoServiceAddr = ""
			if h.videoClient != nil {
				h.videoClient.Close()
				h.videoClient = nil
			}
		}
	}
}

// onRecommendServiceChange 推荐服务变化处理
func (h *VideoHandler) onRecommendServiceChange(serviceAddr string, isAdded bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if isAdded {
		if serviceAddr != h.recommendServiceAddr {
			log.Printf("Recommendation service address changed from %s to %s", h.recommendServiceAddr, serviceAddr)
			h.recommendServiceAddr = serviceAddr

			// 关闭旧连接
			if h.recommendClient != nil {
				h.recommendClient.Close()
				h.recommendClient = nil
			}

			// 重置熔断器
			h.circuitBreaker.RecordSuccess()
		}
	} else {
		log.Printf("Recommendation service instance removed: %s", serviceAddr)
		if serviceAddr == h.recommendServiceAddr {
			h.recommendServiceAddr = ""
			if h.recommendClient != nil {
				h.recommendClient.Close()
				h.recommendClient = nil
			}
		}
	}
}

// getVideoClient 获取视频服务客户端（懒加载）
func (h *VideoHandler) getVideoClient() (*client.VideoServiceClient, error) {
	h.mu.RLock()
	if h.videoClient != nil && h.videoClient.IsConnected() {
		h.mu.RUnlock()
		return h.videoClient, nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	// 双重检查
	if h.videoClient != nil && h.videoClient.IsConnected() {
		return h.videoClient, nil
	}

	// 检查熔断器
	if !h.circuitBreaker.CanExecute() {
		return nil, fmt.Errorf("circuit breaker is open, please try again later")
	}

	// 检查服务地址
	if h.videoServiceAddr == "" {
		// 尝试发现服务
		serviceAddr, err := h.discovery.DiscoverService()
		if err != nil || serviceAddr == "" {
			h.circuitBreaker.RecordFailure()
			return nil, fmt.Errorf("video service not available: %v", err)
		}
		h.videoServiceAddr = serviceAddr
	}

	// 创建客户端
	videoClient, err := client.NewVideoServiceClient(h.videoServiceAddr)
	if err != nil {
		h.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to create video service client: %v", err)
	}

	h.videoClient = videoClient
	h.circuitBreaker.RecordSuccess()
	log.Printf("Successfully created video service client for %s", h.videoServiceAddr)
	return h.videoClient, nil
}

// getRecommendClient 获取推荐服务客户端（懒加载）
func (h *VideoHandler) getRecommendClient() (*client.RecommendationServiceClient, error) {
	h.mu.RLock()
	if h.recommendClient != nil && h.recommendClient.IsConnected() {
		h.mu.RUnlock()
		return h.recommendClient, nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	// 双重检查
	if h.recommendClient != nil && h.recommendClient.IsConnected() {
		return h.recommendClient, nil
	}

	// 检查熔断器
	if !h.circuitBreaker.CanExecute() {
		return nil, fmt.Errorf("circuit breaker is open, please try again later")
	}

	// 检查服务地址
	if h.recommendServiceAddr == "" {
		// 尝试发现服务
		serviceAddr, err := h.discovery.DiscoverService()
		if err != nil || serviceAddr == "" {
			h.circuitBreaker.RecordFailure()
			return nil, fmt.Errorf("recommendation service not available: %v", err)
		}
		h.recommendServiceAddr = serviceAddr
	}

	// 创建客户端
	recommendClient, err := client.NewRecommendationServiceClient(h.recommendServiceAddr)
	if err != nil {
		h.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to create recommendation service client: %v", err)
	}

	h.recommendClient = recommendClient
	h.circuitBreaker.RecordSuccess()
	log.Printf("Successfully created recommendation service client for %s", h.recommendServiceAddr)
	return h.recommendClient, nil
}

// GetRecommendedVideos 获取推荐视频列表
func (h *VideoHandler) GetRecommendedVideos(c *gin.Context) {
	// 获取查询参数
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "20")
	category := c.Query("category")
	userTags := c.Query("user_tags")
	requestID := c.Query("request_id")

	// 转换参数
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 获取用户ID（如果已登录）
	userID := ""
	if userIDValue, exists := c.Get("user_id"); exists {
		if id, ok := userIDValue.(string); ok {
			userID = id
		}
	}

	// 如果有用户ID，调用推荐服务获取个性化推荐
	if userID != "" {
		recommendClient, err := h.getRecommendClient()
		if err != nil {
			log.Printf("Failed to get recommendation service client: %v", err)
			// 如果推荐服务不可用，降级到通用推荐
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			req := &pb.GetPersonalizedRecommendationsRequest{
				UserId:    userID,
				Page:      uint32(page),
				PageSize:  uint32(pageSize),
				Category:  &category,
				UserTags:  userTags,
				RequestId: requestID,
			}

			resp, err := recommendClient.GetPersonalizedRecommendations(ctx, req)
			if err != nil {
				log.Printf("Failed to get personalized recommendations: %v", err)
				// 如果推荐失败，降级到通用推荐
			} else if resp.StatusCode == 0 {
				c.JSON(http.StatusOK, resp)
				return
			}
		}
	}

	// 获取视频服务客户端，获取通用推荐
	videoClient, err := h.getVideoClient()
	if err != nil {
		log.Printf("Failed to get video service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Video service temporarily unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.GetRecommendVideosRequest{
		Page:     uint32(page),
		PageSize: uint32(pageSize),
	}

	if category != "" {
		req.Category = &category
	}

	resp, err := videoClient.GetRecommendVideos(ctx, req)
	if err != nil {
		log.Printf("Failed to get recommended videos: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get recommended videos"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetPersonalizedVideos 获取个性化推荐视频
func (h *VideoHandler) GetPersonalizedVideos(c *gin.Context) {
	// 获取查询参数
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "20")
	category := c.Query("category")
	userTags := c.Query("user_tags")
	requestID := c.Query("request_id")

	// 转换参数
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 获取用户ID（必须已登录）
	userIDValue, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userID, ok := userIDValue.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// 获取推荐服务客户端
	recommendClient, err := h.getRecommendClient()
	if err != nil {
		log.Printf("Failed to get recommendation service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Recommendation service temporarily unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.GetPersonalizedRecommendationsRequest{
		UserId:    userID,
		Page:      uint32(page),
		PageSize:  uint32(pageSize),
		Category:  &category,
		UserTags:  userTags,
		RequestId: requestID,
	}

	resp, err := recommendClient.GetPersonalizedRecommendations(ctx, req)
	if err != nil {
		log.Printf("Failed to get personalized recommendations: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get personalized recommendations"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetFollowVideos 获取关注用户的视频
func (h *VideoHandler) GetFollowVideos(c *gin.Context) {
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

	// 获取用户ID（必须已登录）
	userIDValue, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userID, ok := userIDValue.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// 获取视频服务客户端
	videoClient, err := h.getVideoClient()
	if err != nil {
		log.Printf("Failed to get video service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Video service temporarily unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.GetFollowVideosRequest{
		UserId:   userID,
		Page:     uint32(page),
		PageSize: uint32(pageSize),
	}

	resp, err := videoClient.GetFollowVideos(ctx, req)
	if err != nil {
		log.Printf("Failed to get follow videos: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get follow videos"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetHotVideos 获取热门视频
func (h *VideoHandler) GetHotVideos(c *gin.Context) {
	// 获取查询参数
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "20")
	category := c.Query("category")
	requestID := c.Query("request_id")

	// 转换参数
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 获取视频服务客户端
	videoClient, err := h.getVideoClient()
	if err != nil {
		log.Printf("Failed to get video service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Video service temporarily unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.GetHotVideosRequest{
		Page:      uint32(page),
		PageSize:  uint32(pageSize),
		Category:  &category,
		RequestId: requestID,
	}

	resp, err := videoClient.GetHotVideos(ctx, req)
	if err != nil {
		log.Printf("Failed to get hot videos: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get hot videos"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// RegisterVideoRoutes 注册视频相关路由
func RegisterVideoRoutes(router *gin.Engine, etcdEndpoints []string) {
	// 创建视频处理器
	videoHandler, err := NewVideoHandler(etcdEndpoints)
	if err != nil {
		log.Fatalf("Failed to create video handler: %v", err)
	}

	// 视频相关路由组
	videoGroup := router.Group("/api/videos")
	{
		// 公开路由
		videoGroup.GET("/recommended", videoHandler.GetRecommendedVideos)
		videoGroup.GET("/hot", videoHandler.GetHotVideos)

		// 需要认证的路由
		authGroup := videoGroup.Group("/")
		authGroup.Use(AuthMiddleware()) // 假设有认证中间件
		{
			authGroup.GET("/personalized", videoHandler.GetPersonalizedVideos)
			authGroup.GET("/follow", videoHandler.GetFollowVideos)
		}
	}
}
