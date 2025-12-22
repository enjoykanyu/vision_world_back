package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"api_gateway/client"
	"api_gateway/discovery"
	"api_gateway/middleware"
	recpb "api_gateway/proto_gen/recommendation"
	pb "api_gateway/proto_gen/video"

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
		Token:    userID,
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
		Token:    userID,
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

	req := &pb.GetHotVideosRequest{}

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

	RegisterVideoRoutesWithHandler(router, videoHandler)
}

// RegisterVideoRoutesWithHandler 使用已有的视频处理器注册路由
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

// HandleVideoUpload 处理视频上传请求
func (h *VideoHandler) HandleVideoUpload(c *gin.Context) {
	// 获取表单数据
	title := c.PostForm("title")
	description := c.PostForm("description")
	category := c.PostForm("category")
	tagsStr := c.PostForm("tags")

	// 验证必填字段
	if title == "" {
		log.Printf("Title is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Title is required"})
		return
	}

	// 获取上传的文件
	file, header, err := c.Request.FormFile("video")
	if err != nil {
		log.Printf("Video file is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video file is required"})
		return
	}
	defer file.Close()

	// 验证文件大小（最大500MB）
	const maxFileSize = 500 * 1024 * 1024 // 500MB
	if header.Size > maxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File size exceeds 500MB limit"})
		log.Printf("File size exceeds 500MB limit")
		return
	}

	// 验证文件类型
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowedExts := []string{".mp4", ".avi", ".mov", ".wmv", ".flv", ".mkv", ".webm"}
	isValid := false
	for _, allowed := range allowedExts {
		if ext == allowed {
			isValid = true
			break
		}
	}
	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video file format. Allowed formats: mp4, avi, mov, wmv, flv, mkv, webm"})
		log.Printf("Invalid video file format. Allowed formats: mp4, avi, mov, wmv, flv, mkv, webm")
		return
	}

	// 读取文件内容到字节数组
	fileBytes := make([]byte, header.Size)
	_, err = file.Read(fileBytes)
	if err != nil {
		log.Printf("Failed to read video file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read video file"})
		return
	}

	// 获取视频服务客户端
	videoClient, err := h.getVideoClient()
	if err != nil {
		log.Printf("Failed to get video service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Video service temporarily unavailable"})
		return
	}

	//token := fmt.Sprintf("%v", tokenValue)

	// 准备标签
	var tags []string
	if tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}
	}

	// 调用视频服务的UploadVideo接口
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // 上传可能需要更长时间
	defer cancel()

	req := &pb.UploadVideoRequest{
		//Token:       token,
		VideoData:   fileBytes,
		FileName:    header.Filename,
		Title:       title,
		Description: description,
		Category:    category,
		Tags:        tags,
	}

	resp, err := videoClient.UploadVideo(ctx, req)
	if err != nil {
		log.Printf("Failed to upload video: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload video"})
		return
	}

	if resp.StatusCode != 0 {
		log.Printf("Video upload failed with status code %d: %s", resp.StatusCode, resp.StatusMsg)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": resp.StatusMsg,
			"code":  resp.StatusCode,
		})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"message":   "Video uploaded successfully",
		"video_id":  resp.VideoId,
		"video_url": resp.VideoUrl,
	})
}
func (h *VideoHandler) HandleVideoPublish(c *gin.Context) {
	// 获取表单数据
	videoId := c.PostForm("video_id")
	title := c.PostForm("title")
	description := c.PostForm("description")
	tagsStr := c.PostForm("tags")
	videoType := c.PostForm("type")
	source := c.PostForm("source")
	privacy := c.DefaultPostForm("privacy", "public")

	// 验证必填字段
	if videoId == "" {
		log.Printf("Video ID is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID is required"})
		return
	}

	if title == "" {
		log.Printf("Title is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Title is required"})
		return
	}

	// 获取视频服务客户端
	videoClient, err := h.getVideoClient()
	if err != nil {
		log.Printf("Failed to get video service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Video service temporarily unavailable"})
		return
	}

	// 获取用户ID（从认证中间件）
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

	// 解析标签（JSON格式）
	var tags []string
	if tagsStr != "" {
		// 尝试解析JSON格式的标签
		if err := json.Unmarshal([]byte(tagsStr), &tags); err != nil {
			// 如果JSON解析失败，尝试用逗号分隔
			log.Printf("Failed to parse tags as JSON, trying comma split: %v", err)
			tags = strings.Split(tagsStr, ",")
			for i, tag := range tags {
				tags[i] = strings.TrimSpace(tag)
				// 移除可能的引号
				tags[i] = strings.Trim(tag, `\"'`)
			}
		}
	}

	// 处理封面上传
	var coverURL string
	if coverFile, coverHeader, err := c.Request.FormFile("cover"); err == nil {
		defer coverFile.Close()

		// 验证封面大小（最大2MB）
		const maxCoverSize = 2 * 1024 * 1024 // 2MB
		if coverHeader.Size > maxCoverSize {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cover size exceeds 2MB limit"})
			log.Printf("Cover size exceeds 2MB limit")
			return
		}

		// 验证封面类型
		coverExt := strings.ToLower(filepath.Ext(coverHeader.Filename))
		allowedCoverExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
		isValidCover := false
		for _, allowed := range allowedCoverExts {
			if coverExt == allowed {
				isValidCover = true
				break
			}
		}
		if !isValidCover {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cover file format. Allowed formats: jpg, jpeg, png, gif, webp"})
			log.Printf("Invalid cover file format")
			return
		}

		// 这里应该调用存储服务上传封面，获取coverURL
		// 暂时使用模拟URL
		coverURL = fmt.Sprintf("http://localhost:9000/covers/%s%s", videoId, coverExt)
		log.Printf("Cover uploaded to %s", coverURL)
	}

	// 调用视频服务的PublishVideo接口
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 设置可选字段
	isPublic := privacy == "public"
	var typePtr, sourcePtr *string
	if videoType != "" {
		typePtr = &videoType
	}
	if source != "" {
		sourcePtr = &source
	}

	req := &pb.PublishVideoRequest{
		Token:       userID,
		Title:       title,
		Description: description,
		CoverUrl:    coverURL,
		VideoUrl:    fmt.Sprintf("http://localhost:9000/videos/%s.mp4", videoId), // 这里应该从视频服务获取真实的视频URL
		Tags:        tags,
		IsPublic:    &isPublic,
		Type:        typePtr,
		Source:      sourcePtr,
		VideoId:     videoId, // 添加视频ID，来自表单中的video_id
	}

	resp, err := videoClient.PublishVideo(ctx, req)
	if err != nil {
		log.Printf("Failed to publish video: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to publish video"})
		return
	}

	if resp.StatusCode != 0 {
		log.Printf("Video publish failed with status code %d: %s", resp.StatusCode, resp.StatusMsg)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": resp.StatusMsg,
			"code":  resp.StatusCode,
		})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"message":  "Video published successfully",
		"video_id": resp.VideoId,
	})
}

// GetUserPublishedVideos 获取用户发布的视频列表
func (h *VideoHandler) GetUserPublishedVideos(c *gin.Context) {
	// 获取查询参数
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "10")

	// 转换参数
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}

	// 获取用户ID（从认证中间件）
	userIDValue, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDStr, ok := userIDValue.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// 将用户ID转换为uint32
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user ID"})
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

	// 调用视频服务的GetUserVideos接口
	req := &pb.GetUserVideosRequest{
		UserId:   uint32(userID),
		Token:    userIDStr,
		Page:     uint32(page),
		PageSize: uint32(pageSize),
	}

	resp, err := videoClient.GetUserVideos(ctx, req)
	if err != nil {
		log.Printf("Failed to get user videos: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user videos"})
		return
	}

	if resp.StatusCode != 0 {
		log.Printf("GetUserVideos failed with status code %d: %s", resp.StatusCode, resp.StatusMsg)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": resp.StatusMsg,
			"code":  resp.StatusCode,
		})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"status_code": 0,
		"status_msg":  "Success",
		"videos":      resp.Videos,
		"total":       resp.Total,
		"has_more":    resp.HasMore,
	})
}

// GetVideoDetail 获取视频详情
func (h *VideoHandler) GetVideoDetail(c *gin.Context) {
	// 获取视频ID
	videoID := c.Param("id")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID is required"})
		return
	}

	// 获取视频服务客户端
	videoClient, err := h.getVideoClient()
	if err != nil {
		log.Printf("Failed to get video service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Video service temporarily unavailable"})
		return
	}

	// 调用视频服务的GetVideoInfo接口
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	videoIDUint, err := strconv.ParseUint(videoID, 10, 32)
	if err != nil {
		log.Printf("Failed to parse video ID: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	req := &pb.GetVideoInfoRequest{
		VideoId: uint32(videoIDUint),
	}

	resp, err := videoClient.GetVideoInfo(ctx, req)
	if err != nil {
		log.Printf("Failed to get video info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get video info"})
		return
	}

	if resp.StatusCode != 0 {
		log.Printf("GetVideoInfo failed with status code %d: %s", resp.StatusCode, resp.StatusMsg)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": resp.StatusMsg,
			"code":  resp.StatusCode,
		})
		return
	}

	// 字段映射：将protobuf响应转换为前端期望的JSON格式
	video := resp.Video
	mappedVideo := gin.H{
		"video_id":       video.Id,
		"title":          video.Title,
		"description":    video.Description,
		"cover_url":      video.CoverUrl,
		"video_url":      video.VideoUrl,
		"view_count":     video.PlayCount,
		"like_count":     video.LikeCount,
		"comment_count":  video.CommentCount,
		"share_count":    video.ShareCount,
		"favorite_count": video.FavoriteCount,
		"is_liked":       video.IsLiked,
		"is_favorite":    video.IsFavorite,
		"tags":           video.Tags,
		"location":       video.Location,
		"category":       video.Category,
		"create_time":    video.CreateTime,
		"update_time":    video.UpdateTime,
		"duration":       video.Duration,
		"resolution":     video.Resolution,
		"is_public":      video.IsPublic,
		"status":         video.Status,
		"author": gin.H{
			"id":             video.Author.Id,
			"username":       video.Author.Name,
			"avatar":         video.Author.Avatar,
			"follower_count": video.Author.FollowerCount,
		},
	}

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"video": mappedVideo,
		},
		"status": "success",
	})
}

// RegisterVideoRoutesWithHandler 使用已有的视频处理器注册路由
func RegisterVideoRoutesWithHandler(router *gin.Engine, videoHandler *VideoHandler) {
	// 视频相关路由组
	videoGroup := router.Group("/api/video")
	{
		// 公开路由
		videoGroup.GET("/recommended", videoHandler.GetRecommendedVideos)
		videoGroup.GET("/hot", videoHandler.GetHotVideos)
		// 公开视频详情路由，使用/api/videos/:id格式
		videoGroup.GET("/:id", videoHandler.GetVideoDetail)
		// 需要认证的路由
		authGroup := videoGroup.Group("/")
		authGroup.Use(middleware.RequireAuthMiddleware())
		{
			authGroup.GET("/personalized", videoHandler.GetPersonalizedVideos)
			authGroup.GET("/follow", videoHandler.GetFollowVideos)
			authGroup.GET("/user/published", videoHandler.GetUserPublishedVideos)
			authGroup.POST("/publish", videoHandler.HandleVideoPublish)
			authGroup.POST("/upload", videoHandler.HandleVideoUpload)

		}
	}
}
