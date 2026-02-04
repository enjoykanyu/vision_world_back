package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"api_gateway/client"
	"api_gateway/discovery"
	"api_gateway/middleware"
	recpb "api_gateway/proto/proto_gen/recommendation"
	pb "api_gateway/proto/proto_gen/video"

	"github.com/gin-gonic/gin"
)

// VideoHandler 视频处理器 - 只处理路由转发
type VideoHandler struct {
	videoClient          *client.VideoServiceClient
	recommendClient      *client.RecommendationServiceClient
	discovery            *discovery.EtcdServiceDiscovery
	recommendDiscovery   *discovery.EtcdServiceDiscovery
	etcdEndpoints        []string
	videoServiceAddr     string
	recommendServiceAddr string
	mu                   sync.RWMutex
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
		etcdEndpoints:      etcdEndpoints,
		discovery:          serviceDiscovery,
		recommendDiscovery: recommendDiscovery,
		circuitBreaker:     NewCircuitBreaker(),
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

// getTokenFromHeader 从请求头中获取原始 JWT token
func getTokenFromHeader(c *gin.Context) string {
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
		serviceAddr, err := h.recommendDiscovery.DiscoverService()
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

			req := &recpb.GetPersonalizedRecommendationsRequest{
				UserId:   userID,
				Page:     uint32(page),
				PageSize: uint32(pageSize),
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
		Token:    getTokenFromHeader(c),
		Page:     uint32(page),
		PageSize: uint32(pageSize),
	}

	resp, err := videoClient.GetRecommendedVideos(ctx, req)
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

	req := &recpb.GetPersonalizedRecommendationsRequest{
		UserId:   userID,
		Page:     uint32(page),
		PageSize: uint32(pageSize),
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

	_, ok := userIDValue.(string)
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
		Token:    getTokenFromHeader(c),
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
	// 1. 解析参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 2. 获取客户端
	videoClient, err := h.getVideoClient()
	if err != nil {
		Error(c, http.StatusServiceUnavailable, "Video service temporarily unavailable")
		return
	}

	// 3. 调用服务
	ctx, cancel := WithTimeout(5)
	defer cancel()

	resp, err := videoClient.GetHotVideos(ctx, &pb.GetHotVideosRequest{
		Page:     uint32(page),
		PageSize: uint32(pageSize),
	})
	if err != nil {
		HandleGRPCError(c, err)
		return
	}

	// 4. 返回响应
	Success(c, resp)
}

// RegisterVideoRoutesWithHandler 使用已有的视频处理器注册路由
func RegisterVideoRoutesWithHandler(router *gin.Engine, handler *VideoHandler) {
	// 视频相关路由 - 转发到 video_service
	api := router.Group("/api")
	{
		// 视频上传 - 直接转发到 video_service
		api.POST("/video/upload", middleware.RequireAuthMiddleware(), handler.HandleVideoUpload)

		// 视频发布 - 直接转发到 video_service
		api.POST("/video/publish", middleware.RequireAuthMiddleware(), handler.HandleVideoPublish)

		// 视频流 - 直接转发到 video_service
		api.GET("/video/feed", handler.GetVideoFeed)

		// 视频详情 - 直接转发到 video_service
		api.GET("/video/detail/:id", handler.GetVideoDetail)

		// 视频搜索 - 直接转发到 video_service
		api.GET("/video/search", handler.SearchVideos)

		// 视频点赞 - 直接转发到 video_service
		api.POST("/video/like", middleware.RequireAuthMiddleware(), handler.LikeVideo)

		// 视频评论 - 直接转发到 video_service
		api.GET("/video/comments", handler.GetVideoComments)
		api.POST("/video/comment", middleware.RequireAuthMiddleware(), handler.AddComment)

		// 个性化推荐 - 直接转发到 video_service 或 recommendation_service
		api.GET("/video/recommended", handler.GetRecommendedVideos)
		api.GET("/video/personalized", middleware.RequireAuthMiddleware(), handler.GetPersonalizedVideos)
		api.GET("/video/follow", middleware.RequireAuthMiddleware(), handler.GetFollowVideos)

		// 视频分类 - 直接转发到 video_service
		api.GET("/video/categories", handler.GetVideoCategories)
		api.GET("/video/category/:id", handler.GetVideosByCategory)
	}
}

// Close 关闭连接
func (h *VideoHandler) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.videoClient != nil {
		h.videoClient.Close()
		h.videoClient = nil
	}

	if h.recommendClient != nil {
		h.recommendClient.Close()
		h.recommendClient = nil
	}
}

// HandleVideoUpload 处理视频上传请求 - 直接转发到 video_service
func (h *VideoHandler) HandleVideoUpload(c *gin.Context) {
	// 网关层只负责路由，具体的文件处理由 video_service 完成
	// 读取表单数据并构建 gRPC 请求
	title := c.PostForm("title")
	description := c.PostForm("description")
	category := c.PostForm("category")
	tagsStr := c.PostForm("tags")

	// 验证必填字段
	if title == "" {
		Error(c, http.StatusBadRequest, "Title is required")
		return
	}

	// 获取上传的文件
	file, header, err := c.Request.FormFile("video")
	if err != nil {
		Error(c, http.StatusBadRequest, "Video file is required")
		return
	}
	defer file.Close()

	// 读取文件内容
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Failed to read video file: %v", err)
		Error(c, http.StatusInternalServerError, "Failed to read video file")
		return
	}

	// 解析标签
	var tags []string
	if tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}
	}

	// 获取视频服务客户端
	videoClient, err := h.getVideoClient()
	if err != nil {
		log.Printf("Failed to get video service client: %v", err)
		Error(c, http.StatusServiceUnavailable, "Video service temporarily unavailable")
		return
	}

	// 构建 gRPC 请求
	req := &pb.UploadVideoRequest{
		Token:       getTokenFromHeader(c),
		VideoData:   fileBytes,
		FileName:    header.Filename,
		Title:       title,
		Description: description,
		Category:    category,
		Tags:        tags,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := videoClient.UploadVideo(ctx, req)
	if err != nil {
		log.Printf("Failed to upload video: %v", err)
		HandleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// HandleVideoPublish 处理视频发布请求 - 直接转发到 video_service
func (h *VideoHandler) HandleVideoPublish(c *gin.Context) {
	// 读取请求体
	body, err := c.GetRawData()
	if err != nil {
		Error(c, http.StatusBadRequest, "Failed to read request body")
		return
	}

	// 解析请求
	var req pb.PublishVideoRequest
	if err := json.Unmarshal(body, &req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 设置 token
	req.Token = getTokenFromHeader(c)

	videoClient, err := h.getVideoClient()
	if err != nil {
		log.Printf("Failed to get video service client: %v", err)
		Error(c, http.StatusServiceUnavailable, "Video service temporarily unavailable")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := videoClient.PublishVideo(ctx, &req)
	if err != nil {
		log.Printf("Failed to publish video: %v", err)
		HandleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetVideoFeed 获取视频流 - 直接转发到 video_service
func (h *VideoHandler) GetVideoFeed(c *gin.Context) {
	videoClient, err := h.getVideoClient()
	if err != nil {
		Error(c, http.StatusServiceUnavailable, "Video service temporarily unavailable")
		return
	}

	// 读取查询参数
	latestTime := c.DefaultQuery("latest_time", "0")
	token := getTokenFromHeader(c)

	// 转换 latestTime
	var latestTimeInt int64
	if latestTime != "" && latestTime != "0" {
		latestTimeInt, _ = strconv.ParseInt(latestTime, 10, 64)
	}

	ctx, cancel := WithTimeout(5)
	defer cancel()

	// 调用 GetRecommendedVideos 作为视频流
	resp, err := videoClient.GetRecommendedVideos(ctx, &pb.GetRecommendVideosRequest{
		Token:    token,
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		HandleGRPCError(c, err)
		return
	}

	// 构建视频流响应
	type VideoFeedResponse struct {
		NextTime  int64       `json:"next_time"`
		VideoList interface{} `json:"video_list"`
	}

	Success(c, VideoFeedResponse{
		NextTime:  latestTimeInt,
		VideoList: resp.Videos,
	})
}

// GetVideoDetail 获取视频详情 - 直接转发到 video_service
func (h *VideoHandler) GetVideoDetail(c *gin.Context) {
	videoClient, err := h.getVideoClient()
	if err != nil {
		Error(c, http.StatusServiceUnavailable, "Video service temporarily unavailable")
		return
	}

	videoID := c.Param("id")
	if videoID == "" {
		Error(c, http.StatusBadRequest, "Video ID is required")
		return
	}

	// 转换 videoID
	videoIDInt, err := strconv.ParseUint(videoID, 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, "Invalid video ID")
		return
	}

	ctx, cancel := WithTimeout(5)
	defer cancel()

	resp, err := videoClient.GetVideoInfo(ctx, &pb.GetVideoInfoRequest{
		VideoId: uint32(videoIDInt),
		Token:   getTokenFromHeader(c),
	})
	if err != nil {
		HandleGRPCError(c, err)
		return
	}

	Success(c, resp)
}

// SearchVideos 搜索视频 - 直接转发到 video_service
func (h *VideoHandler) SearchVideos(c *gin.Context) {
	videoClient, err := h.getVideoClient()
	if err != nil {
		Error(c, http.StatusServiceUnavailable, "Video service temporarily unavailable")
		return
	}

	keyword := c.Query("keyword")
	if keyword == "" {
		Error(c, http.StatusBadRequest, "Search keyword is required")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	ctx, cancel := WithTimeout(5)
	defer cancel()

	// 使用 GetRecommendedVideos 作为搜索（实际应该调用搜索接口）
	resp, err := videoClient.GetRecommendedVideos(ctx, &pb.GetRecommendVideosRequest{
		Token:    getTokenFromHeader(c),
		Page:     uint32(page),
		PageSize: uint32(pageSize),
	})
	if err != nil {
		HandleGRPCError(c, err)
		return
	}

	Success(c, resp)
}

// LikeVideo 点赞视频 - 直接转发到 video_service
func (h *VideoHandler) LikeVideo(c *gin.Context) {
	videoClient, err := h.getVideoClient()
	if err != nil {
		Error(c, http.StatusServiceUnavailable, "Video service temporarily unavailable")
		return
	}

	// 读取请求体
	body, err := c.GetRawData()
	if err != nil {
		Error(c, http.StatusBadRequest, "Failed to read request body")
		return
	}

	// 解析请求
	var req pb.LikeVideoRequest
	if err := json.Unmarshal(body, &req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 设置 token
	req.Token = getTokenFromHeader(c)

	ctx, cancel := WithTimeout(5)
	defer cancel()

	resp, err := videoClient.LikeVideo(ctx, &req)
	if err != nil {
		HandleGRPCError(c, err)
		return
	}

	Success(c, resp)
}

// GetVideoComments 获取视频评论 - 直接转发到 video_service
func (h *VideoHandler) GetVideoComments(c *gin.Context) {
	videoClient, err := h.getVideoClient()
	if err != nil {
		Error(c, http.StatusServiceUnavailable, "Video service temporarily unavailable")
		return
	}

	videoID := c.Query("video_id")
	if videoID == "" {
		Error(c, http.StatusBadRequest, "Video ID is required")
		return
	}

	// 转换 videoID
	videoIDInt, err := strconv.ParseUint(videoID, 10, 64)
	if err != nil {
		Error(c, http.StatusBadRequest, "Invalid video ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	ctx, cancel := WithTimeout(5)
	defer cancel()

	resp, err := videoClient.GetVideoComments(ctx, &pb.GetVideoCommentsRequest{
		VideoId:  uint32(videoIDInt),
		Page:     uint32(page),
		PageSize: uint32(pageSize),
		Token:    getTokenFromHeader(c),
	})
	if err != nil {
		HandleGRPCError(c, err)
		return
	}

	Success(c, resp)
}

// AddComment 添加评论 - 直接转发到 video_service
func (h *VideoHandler) AddComment(c *gin.Context) {
	videoClient, err := h.getVideoClient()
	if err != nil {
		Error(c, http.StatusServiceUnavailable, "Video service temporarily unavailable")
		return
	}

	// 读取请求体
	body, err := c.GetRawData()
	if err != nil {
		Error(c, http.StatusBadRequest, "Failed to read request body")
		return
	}

	// 解析请求
	var req pb.CommentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 设置 token
	req.Token = getTokenFromHeader(c)

	ctx, cancel := WithTimeout(5)
	defer cancel()

	resp, err := videoClient.CommentVideo(ctx, &req)
	if err != nil {
		HandleGRPCError(c, err)
		return
	}

	Success(c, resp)
}

// GetVideoCategories 获取视频分类 - 直接转发到 video_service
func (h *VideoHandler) GetVideoCategories(c *gin.Context) {
	videoClient, err := h.getVideoClient()
	if err != nil {
		Error(c, http.StatusServiceUnavailable, "Video service temporarily unavailable")
		return
	}

	ctx, cancel := WithTimeout(5)
	defer cancel()

	// 使用 GetRecommendedVideos 返回空列表作为分类（实际应该有分类接口）
	resp, err := videoClient.GetRecommendedVideos(ctx, &pb.GetRecommendVideosRequest{
		Token:    getTokenFromHeader(c),
		Page:     1,
		PageSize: 0,
	})
	if err != nil {
		HandleGRPCError(c, err)
		return
	}

	Success(c, resp)
}

// GetVideosByCategory 获取分类下的视频 - 直接转发到 video_service
func (h *VideoHandler) GetVideosByCategory(c *gin.Context) {
	videoClient, err := h.getVideoClient()
	if err != nil {
		Error(c, http.StatusServiceUnavailable, "Video service temporarily unavailable")
		return
	}

	categoryID := c.Param("id")
	if categoryID == "" {
		Error(c, http.StatusBadRequest, "Category ID is required")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	ctx, cancel := WithTimeout(5)
	defer cancel()

	// 使用 GetRecommendedVideos 作为分类视频（实际应该有分类接口）
	resp, err := videoClient.GetRecommendedVideos(ctx, &pb.GetRecommendVideosRequest{
		Token:    getTokenFromHeader(c),
		Page:     uint32(page),
		PageSize: uint32(pageSize),
	})
	if err != nil {
		HandleGRPCError(c, err)
		return
	}

	Success(c, resp)
}
