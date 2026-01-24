package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"api_gateway/client"
	"api_gateway/discovery"
	"api_gateway/middleware"
	"api_gateway/pkg/minio"
	recpb "api_gateway/proto/proto_gen/recommendation"
	userpb "api_gateway/proto/proto_gen/user"
	pb "api_gateway/proto/proto_gen/video"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/status"
)

// VideoHandler 视频处理器
type VideoHandler struct {
	videoClient          *client.VideoServiceClient
	userClient           *client.UserServiceClient
	recommendClient      *client.RecommendationServiceClient
	discovery            *discovery.EtcdServiceDiscovery
	recommendDiscovery   *discovery.EtcdServiceDiscovery
	etcdEndpoints        []string
	videoServiceAddr     string
	recommendServiceAddr string
	minioClient          *minio.Client
	bucketName           string
	mu                   sync.RWMutex
	lastFailTime         time.Time
	circuitBreaker       *CircuitBreaker
}

// NewVideoHandler 创建视频处理器
func NewVideoHandler(etcdEndpoints []string, minioClient *minio.Client, bucketName string) (*VideoHandler, error) {
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
		minioClient:        minioClient,
		bucketName:         bucketName,
		circuitBreaker:     NewCircuitBreaker(),
	}

	// 创建用户服务客户端
	userClient, err := client.NewUserServiceClient(etcdEndpoints[0])
	if err != nil {
		log.Printf("Failed to create user service client: %v", err)
		// 用户服务连接失败不影响视频服务启动，只记录错误
	} else {
		handler.userClient = userClient
		log.Printf("Successfully connected to user service")
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
func RegisterVideoRoutes(router *gin.Engine, etcdEndpoints []string, minioClient *minio.Client, bucketName string) {
	// 创建视频处理器
	videoHandler, err := NewVideoHandler(etcdEndpoints, minioClient, bucketName)
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

// HandleVideoUpload 处理视频上传请求这里后端会进行转码
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

	// 获取用户ID（中间件已验证token）
	userIDValue, exists := c.Get("user_id")
	if !exists {
		log.Printf("User not authenticated")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户未认证"})
		return
	}
	_, ok := userIDValue.(string)
	if !ok {
		log.Printf("Invalid user ID")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无效的用户ID"})
		return
	}

	// 获取原始token（传递给video_service进行二次验证）
	authHeader := c.GetHeader("Authorization")
	token := ""
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			token = strings.TrimSpace(parts[1])
		}
	}

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
		Token:       token,
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
	_, ok := userIDValue.(string)
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
		Token:       getTokenFromHeader(c),
		Title:       title,
		Description: description,
		CoverUrl:    coverURL,
		VideoUrl:    fmt.Sprintf("http://localhost:9000/videos/%s.mp4", videoId),
		Tags:        tags,
		IsPublic:    &isPublic,
		Type:        typePtr,
		Source:      sourcePtr,
		VideoId:     videoId,
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

// RetryTranscode 重试视频转码
func (h *VideoHandler) RetryTranscode(c *gin.Context) {
	// 获取请求参数
	videoIDStr := c.PostForm("video_id")
	if videoIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID is required"})
		return
	}

	// 转换视频ID
	videoID, err := strconv.ParseUint(videoIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	// 获取用户ID（从认证中间件）
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

	// 调用视频服务的RetryTranscode接口
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.RetryTranscodeRequest{
		Token:   getTokenFromHeader(c),
		VideoId: uint32(videoID),
	}

	resp, err := videoClient.RetryTranscode(ctx, req)
	if err != nil {
		log.Printf("Failed to retry transcode: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retry transcode"})
		return
	}

	if resp.StatusCode != 0 {
		log.Printf("Retry transcode failed with status code %d: %s", resp.StatusCode, resp.StatusMsg)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": resp.StatusMsg,
			"code":  resp.StatusCode,
		})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"message": resp.StatusMsg,
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
		Token:    getTokenFromHeader(c),
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
	// 处理视频列表，移除video_url字段
	for i := range resp.Videos {
		resp.Videos[i].VideoUrl = ""
	}

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
		"playlist_url":   video.PlaylistUrl,
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

// ProxyHLSStream 代理HLS视频流请求
func (h *VideoHandler) ProxyHLSStream(c *gin.Context) {
	videoID := c.Param("id")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID is required"})
		return
	}

	// 获取请求的文件路径（例如：hls/index.m3u8 或 hls/segment_0.ts）
	filePath := c.Param("filepath")
	if filePath == "" {
		filePath = "hls/index.m3u8"
	}

	// 清理路径，移除前导斜杠
	filePath = strings.TrimPrefix(filePath, "/")

	// 确保文件路径以hls/开头
	if !strings.HasPrefix(filePath, "hls/") {
		filePath = "hls/" + filePath
	}

	// 构建MinIO对象路径
	objectName := fmt.Sprintf("%s/%s", videoID, filePath)

	log.Printf("Proxying HLS stream for video %s, file: %s, object: %s", videoID, filePath, objectName)

	presignedURL, err := h.minioClient.GeneratePresignedURL(context.Background(), objectName, 24*time.Hour)
	if err != nil {
		log.Printf("Failed to generate presigned URL: %v", err)

		videoClient, err := h.getVideoClient()
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			videoIDUint, parseErr := strconv.ParseUint(videoID, 10, 32)
			if parseErr == nil {
				req := &pb.GetVideoInfoRequest{
					VideoId: uint32(videoIDUint),
				}

				resp, err := videoClient.GetVideoInfo(ctx, req)
				if err == nil && resp.StatusCode == 0 && resp.Video.VideoUrl != "" {
					log.Printf("MinIO unavailable for video %s, providing fallback URL", videoID)
					c.JSON(http.StatusServiceUnavailable, gin.H{
						"error":        "Video stream service temporarily unavailable",
						"fallback_url": resp.Video.VideoUrl,
						"message":      "Please try again later or use the provided fallback URL",
					})
					return
				}
			}
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch video stream"})
		return
	}

	log.Printf("Fetching from MinIO: %s", presignedURL)

	respMinio, err := http.Get(presignedURL)
	if err != nil {
		log.Printf("Failed to fetch from MinIO: %v", err)

		videoClient, err := h.getVideoClient()
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			videoIDUint, parseErr := strconv.ParseUint(videoID, 10, 32)
			if parseErr == nil {
				req := &pb.GetVideoInfoRequest{
					VideoId: uint32(videoIDUint),
				}

				resp, err := videoClient.GetVideoInfo(ctx, req)
				if err == nil && resp.StatusCode == 0 && resp.Video.VideoUrl != "" {
					log.Printf("MinIO unavailable for video %s, providing fallback URL", videoID)
					c.JSON(http.StatusServiceUnavailable, gin.H{
						"error":        "Video stream service temporarily unavailable",
						"fallback_url": resp.Video.VideoUrl,
						"message":      "Please try again later or use the provided fallback URL",
					})
					return
				}
			}
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch video stream"})
		return
	}
	defer respMinio.Body.Close()

	// 检查MinIO响应状态
	if respMinio.StatusCode != http.StatusOK {
		log.Printf("MinIO returned status %d for video %s, file: %s", respMinio.StatusCode, videoID, filePath)
		if respMinio.StatusCode == http.StatusNotFound {
			// HLS文件不存在，检查是否转码未完成
			videoClient, err := h.getVideoClient()
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				videoIDUint, parseErr := strconv.ParseUint(videoID, 10, 32)
				if parseErr == nil {
					req := &pb.GetVideoInfoRequest{
						VideoId: uint32(videoIDUint),
					}

					resp, err := videoClient.GetVideoInfo(ctx, req)
					if err == nil && resp.StatusCode == 0 {
						// 检查是否有HLS播放列表URL
						if resp.Video.PlaylistUrl == "" {
							// 没有HLS播放列表，可能转码未完成
							if resp.Video.VideoUrl != "" {
								// 提供原始视频URL作为降级方案
								c.JSON(http.StatusOK, gin.H{
									"error":        "HLS stream not available",
									"fallback_url": resp.Video.VideoUrl,
									"message":      "Video transcoding may be in progress, using original video URL",
								})
								return
							}
						}
					}
				}
			}

			c.JSON(http.StatusNotFound, gin.H{"error": "Video stream file not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch video stream"})
		}
		return
	}

	// 检查Content-Type，确保返回的是M3U8或TS文件
	contentType := respMinio.Header.Get("Content-Type")
	log.Printf("MinIO returned Content-Type: %s for video %s, file: %s", contentType, videoID, filePath)

	// 如果请求的是m3u8文件，但返回的不是M3U8类型，说明HLS文件不存在
	if strings.HasSuffix(filePath, ".m3u8") {
		if !strings.Contains(contentType, "mpegurl") && !strings.Contains(contentType, "m3u8") && !strings.Contains(contentType, "text/plain") && !strings.Contains(contentType, "x-mpegurl") {
			log.Printf("Invalid Content-Type %s for M3U8 file, HLS file may not exist", contentType)

			// 获取转码状态
			videoClient, err := h.getVideoClient()
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				videoIDUint, parseErr := strconv.ParseUint(videoID, 10, 32)
				if parseErr == nil {
					req := &pb.GetVideoInfoRequest{
						VideoId: uint32(videoIDUint),
					}

					resp, err := videoClient.GetVideoInfo(ctx, req)
					if err == nil && resp.StatusCode == 0 && resp.Video != nil {
						transcodeStatus := "unknown"
						playlistURL := resp.Video.PlaylistUrl

						if playlistURL != "" {
							transcodeStatus = "completed"
						} else {
							transcodeStatus = "pending"
						}

						log.Printf("HLS file not found (invalid content type) for video %s, transcode status: %s", videoID, transcodeStatus)

						// 如果转码已完成，但Content-Type不正确，仍返回文件内容（可能是Content-Type设置问题）
						if transcodeStatus == "completed" {
							log.Printf("Transcode completed but invalid content type %s, serving file anyway", contentType)
							io.Copy(c.Writer, respMinio.Body)
							return
						}

						// 转码未完成，返回错误提示
						errorPlaylist := fmt.Sprintf("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-ERROR: %s\n#EXT-X-FALLBACK-URL: /api/video/%s/original\n", transcodeStatus, videoID)
						c.Header("Content-Type", "application/vnd.apple.mpegurl")
						c.Header("Access-Control-Allow-Origin", "*")
						c.Header("Cache-Control", "no-cache")
						c.Header("X-Transcode-Status", transcodeStatus)
						c.Header("X-Fallback-URL", fmt.Sprintf("/api/video/%s/original", videoID))
						c.String(http.StatusNotFound, errorPlaylist)
						return
					}
				}
			}

			c.JSON(http.StatusNotFound, gin.H{"error": "HLS file not found"})
			return
		}
	}

	// 如果请求的是ts文件，但返回的不是TS类型，说明分片文件不存在
	if strings.HasSuffix(filePath, ".ts") {
		if !strings.Contains(contentType, "mp2t") && !strings.Contains(contentType, "video/MP2T") {
			log.Printf("Invalid Content-Type %s for TS file", contentType)
			c.JSON(http.StatusNotFound, gin.H{"error": "Video segment file not found"})
			return
		}
	}

	// 如果请求的是m3u8播放列表，需要修改其中的分片URL为API网关地址
	if strings.HasSuffix(filePath, ".m3u8") {
		body, err := io.ReadAll(respMinio.Body)
		if err != nil {
			log.Printf("Failed to read m3u8 content: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read playlist"})
			return
		}

		// 将相对路径的分片URL替换为API网关的绝对路径
		playlistContent := string(body)
		gatewayURL := fmt.Sprintf("/api/video/%s/stream/", videoID)
		playlistContent = strings.ReplaceAll(playlistContent, "segment_", gatewayURL+"segment_")
		playlistContent = strings.ReplaceAll(playlistContent, "../", gatewayURL)

		log.Printf("Modified playlist content length: %d", len(playlistContent))

		// 设置响应头
		c.Header("Content-Type", "application/vnd.apple.mpegurl")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Cache-Control", "max-age=3600")

		c.String(http.StatusOK, playlistContent)
		return
	}

	// 对于ts分片文件，支持Range请求和缓存
	for key, values := range respMinio.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// 设置统一的跨域和缓存策略（B站风格）
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Range, Content-Type")
	c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges, ETag")

	// 智能缓存策略
	if strings.HasSuffix(filePath, ".m3u8") {
		// M3U8播放列表：较短缓存时间（5分钟）
		c.Header("Cache-Control", "public, max-age=300, s-maxage=300")
		c.Header("ETag", fmt.Sprintf(`"%s"`, generateETag(videoID, filePath)))
	} else if strings.HasSuffix(filePath, ".ts") {
		// TS分片文件：较长缓存时间（24小时）
		c.Header("Cache-Control", "public, max-age=86400, s-maxage=86400, immutable")
		c.Header("ETag", fmt.Sprintf(`"%s"`, generateETag(videoID, filePath)))
	} else {
		// 其他文件：中等缓存时间（1小时）
		c.Header("Cache-Control", "public, max-age=3600, s-maxage=3600")
	}

	c.Header("Accept-Ranges", "bytes") // 支持Range请求

	// 处理Range请求 - 直接通过MinIO预签名URL支持
	if rangeHeader := c.Request.Header.Get("Range"); rangeHeader != "" {
		// 重新生成带Range的预签名URL
		req, err := http.NewRequest("GET", presignedURL, nil)
		if err != nil {
			log.Printf("Failed to create request: %v", err)
			c.Status(respMinio.StatusCode)
			io.Copy(c.Writer, respMinio.Body)
			return
		}

		// 添加Range头
		req.Header.Set("Range", rangeHeader)

		// 发送请求
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("Failed to fetch with range: %v", err)
			c.Status(respMinio.StatusCode)
			io.Copy(c.Writer, respMinio.Body)
			return
		}
		defer resp.Body.Close()

		// 复制响应头
		for key, values := range resp.Header {
			for _, value := range values {
				c.Header(key, value)
			}
		}

		// 发送部分内容响应
		c.Status(resp.StatusCode)
		io.Copy(c.Writer, resp.Body)
		return
	}

	// 正常流式传输
	c.Status(respMinio.StatusCode)
	io.Copy(c.Writer, respMinio.Body)

	log.Printf("Successfully streamed %s for video %s", filePath, videoID)
}

// GetVideoSegments 获取视频分片信息
func (h *VideoHandler) GetVideoSegments(c *gin.Context) {
	videoID := c.Param("id")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID is required"})
		return
	}

	// 尝试从MinIO获取分片元数据文件
	segmentsPath := fmt.Sprintf("%s/segments.json", videoID)
	presignedURL, err := h.minioClient.GeneratePresignedURL(context.Background(), segmentsPath, 5*time.Minute)
	if err != nil {
		log.Printf("Failed to generate presigned URL for segments.json: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Segment metadata not found"})
		return
	}

	// 获取分片元数据
	resp, err := http.Get(presignedURL)
	if err != nil {
		log.Printf("Failed to fetch segment metadata: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch segment metadata"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Segment metadata not found, status: %d", resp.StatusCode)
		c.JSON(http.StatusNotFound, gin.H{"error": "Segment metadata not found"})
		return
	}

	// 读取并解析JSON
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read segment metadata: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read segment metadata"})
		return
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(body, &metadata); err != nil {
		log.Printf("Failed to parse segment metadata: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse segment metadata"})
		return
	}

	// 返回分片信息
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   metadata,
	})
}

// generateETag 生成ETag用于缓存验证
func generateETag(videoID, filePath string) string {
	// 使用视频ID和文件路径生成唯一的ETag
	return fmt.Sprintf("%s-%s", videoID, filePath)
}

// CheckUploadStatus 检查上传进度
func (h *VideoHandler) CheckUploadStatus(c *gin.Context) {
	_ = c.Query("fileId") // 忽略未使用的fileId
	// 实际场景中使用Redis存储已上传分片列表
	c.JSON(http.StatusOK, gin.H{"uploadedChunks": []int{}})
}

// UploadChunk 处理分片上传
func (h *VideoHandler) UploadChunk(c *gin.Context) {
	fileId := c.PostForm("fileId")
	chunkIndexStr := c.PostForm("chunkIndex")
	totalChunksStr := c.PostForm("totalChunks")

	log.Printf("UploadChunk: fileId=%s, chunkIndex=%s, totalChunks=%s", fileId, chunkIndexStr, totalChunksStr)

	if fileId == "" {
		log.Printf("UploadChunk: fileId is empty")
		c.JSON(http.StatusBadRequest, gin.H{"error": "fileId不能为空"})
		return
	}

	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil {
		log.Printf("UploadChunk: invalid chunkIndex: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的分片索引"})
		return
	}

	totalChunks, err := strconv.Atoi(totalChunksStr)
	if err != nil {
		log.Printf("UploadChunk: invalid totalChunks: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的总分片数"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		log.Printf("UploadChunk: no file uploaded: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "未上传分片文件"})
		return
	}

	log.Printf("UploadChunk: file=%s, size=%d", file.Filename, file.Size)

	src, err := file.Open()
	if err != nil {
		log.Printf("UploadChunk: failed to open file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("打开分片失败: %v", err)})
		return
	}
	defer src.Close()

	// 保存分片到临时目录
	tmpDir := fmt.Sprintf("/tmp/upload/%s", fileId)
	log.Printf("UploadChunk: creating tmpDir: %s", tmpDir)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		log.Printf("UploadChunk: failed to create tmpDir: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建临时目录失败: %v", err)})
		return
	}

	tmpPath := fmt.Sprintf("%s/chunk_%d", tmpDir, chunkIndex)
	log.Printf("UploadChunk: creating tmpPath: %s", tmpPath)

	out, err := os.Create(tmpPath)
	if err != nil {
		log.Printf("UploadChunk: failed to create file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建临时文件失败: %v", err)})
		return
	}
	defer out.Close()

	written, err := io.Copy(out, src)
	if err != nil {
		log.Printf("UploadChunk: failed to copy file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存分片失败: %v", err)})
		return
	}
	log.Printf("UploadChunk: successfully saved chunk %d, written=%d", chunkIndex, written)

	// 检查是否所有分片都已上传
	allChunksUploaded := true
	for i := 0; i < totalChunks; i++ {
		chunkPath := fmt.Sprintf("%s/chunk_%d", tmpDir, i)
		if _, err := os.Stat(chunkPath); os.IsNotExist(err) {
			allChunksUploaded = false
			break
		}
	}

	if allChunksUploaded {
		// 合并所有分片
		outputPath := fmt.Sprintf("/tmp/upload/%s/%s", fileId, fileId)
		outputFile, err := os.Create(outputPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建输出文件失败: %v", err)})
			return
		}
		defer outputFile.Close()

		for i := 0; i < totalChunks; i++ {
			chunkPath := fmt.Sprintf("%s/chunk_%d", tmpDir, i)
			chunkFile, err := os.Open(chunkPath)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("打开分片文件失败: %v", err)})
				return
			}
			defer chunkFile.Close()

			_, err = io.Copy(outputFile, chunkFile)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("合并分片失败: %v", err)})
				return
			}
		}

		// 上传合并后的文件到MinIO
		// 注意：文件上传到MinIO在CompleteUpload中处理，这里只合并不分发

		// 不清理临时文件，由CompleteUpload处理
	}

	c.JSON(http.StatusOK, gin.H{"message": "分片上传成功"})
}

// CompleteUpload 完成分片上传并转码为HLS
func (h *VideoHandler) CompleteUpload(c *gin.Context) {
	var req struct {
		FileId string `json:"fileId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("CompleteUpload: 参数错误: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	log.Printf("CompleteUpload: fileId=%s", req.FileId)

	// 检查合并后的文件是否存在
	outputPath := fmt.Sprintf("/tmp/upload/%s/%s", req.FileId, req.FileId)
	var videoURL string
	var uploadToMinIO bool = false

	if _, err := os.Stat(outputPath); err == nil {
		// 文件存在，上传到MinIO
		log.Printf("CompleteUpload: 本地文件存在: %s", outputPath)
		uploadToMinIO = true
	} else {
		log.Printf("CompleteUpload: 本地文件不存在: %s", outputPath)
	}

	// 获取视频服务客户端
	videoClient, err := h.getVideoClient()
	if err != nil {
		log.Printf("CompleteUpload: Failed to get video service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "视频服务暂时不可用"})
		return
	}

	// 获取原始token（传递给video_service进行二次验证）
	authHeader := c.GetHeader("Authorization")
	token := ""
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			token = strings.TrimSpace(parts[1])
		}
	}

	// 如果本地文件存在，先上传到MinIO
	var videoData []byte
	if uploadToMinIO && h.minioClient != nil {
		log.Printf("CompleteUpload: 上传文件到MinIO...")
		objectName := fmt.Sprintf("videos/%s.mp4", req.FileId)
		minioURL, err := h.minioClient.UploadFile(context.Background(), objectName, outputPath, "video/mp4")
		if err != nil {
			log.Printf("CompleteUpload: 上传到MinIO失败: %v", err)
			// 上传失败仍继续，让视频服务使用默认URL
		} else {
			videoURL = minioURL
			log.Printf("CompleteUpload: 上传MinIO成功: %s", videoURL)

			// 从MinIO下载文件内容，传递给video_service进行转码
			log.Printf("CompleteUpload: 从MinIO下载文件用于转码...")
			reader, err := h.minioClient.GetFile(context.Background(), objectName)
			if err != nil {
				log.Printf("CompleteUpload: 从MinIO下载文件失败: %v", err)
				err = nil // Clear error since we have a fallback URL
			} else {
				defer reader.Close()
				videoData, err = io.ReadAll(reader)
				if err != nil {
					log.Printf("CompleteUpload: 读取文件内容失败: %v", err)
					videoData = []byte{}
				} else {
					log.Printf("CompleteUpload: 文件下载成功, size=%d", len(videoData))
				}
			}

			// 清理临时文件
			os.RemoveAll(fmt.Sprintf("/tmp/upload/%s", req.FileId))
		}
	}

	// 调用视频服务的UploadVideo方法
	resp, err := videoClient.UploadVideo(context.Background(), &pb.UploadVideoRequest{
		Token:     token,
		FileName:  fmt.Sprintf("%s.mp4", req.FileId),
		Title:     "待编辑的视频",
		VideoData: videoData,
	})

	if err != nil {
		log.Printf("CompleteUpload: Failed to call UploadVideo: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      fmt.Sprintf("创建视频记录失败: %v", err),
			"file_id":    req.FileId,
			"need_retry": true,
		})
		return
	}

	if resp.StatusCode != 0 {
		log.Printf("CompleteUpload: UploadVideo failed with status code %d: %s", resp.StatusCode, resp.StatusMsg)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": resp.StatusMsg,
			"code":  resp.StatusCode,
		})
		return
	}

	log.Printf("CompleteUpload: Video created successfully, video_id=%d, video_url=%s", resp.VideoId, resp.VideoUrl)

	c.JSON(http.StatusOK, gin.H{
		"message":   "上传完成",
		"video_id":  resp.VideoId,
		"video_url": resp.VideoUrl,
		"file_id":   req.FileId,
		"playURL":   fmt.Sprintf("/api/play/%s", resp.VideoId),
	})
}

// LikeVideo 点赞/取消点赞视频
func (h *VideoHandler) LikeVideo(c *gin.Context) {
	// 获取视频ID
	videoIDStr := c.Param("id")
	videoID, err := strconv.ParseUint(videoIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	// 获取请求体
	var req struct {
		ActionType bool `json:"action_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
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

	// 调用视频服务点赞接口
	resp, err := videoClient.LikeVideo(ctx, &pb.LikeVideoRequest{
		Token:      getTokenFromHeader(c),
		VideoId:    uint32(videoID),
		ActionType: req.ActionType,
	})
	if err != nil {
		log.Printf("Failed to like video: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to like video"})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, resp)
}

// FavoriteVideo 收藏/取消收藏视频
func (h *VideoHandler) FavoriteVideo(c *gin.Context) {
	// 获取视频ID
	videoIDStr := c.Param("id")
	videoID, err := strconv.ParseUint(videoIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	// 获取请求体
	var req struct {
		ActionType bool `json:"action_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
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

	// 调用视频服务收藏接口
	resp, err := videoClient.FavoriteVideo(ctx, &pb.FavoriteVideoRequest{
		Token:      getTokenFromHeader(c),
		VideoId:    uint32(videoID),
		ActionType: req.ActionType,
	})
	if err != nil {
		log.Printf("Failed to favorite video: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to favorite video"})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, resp)
}

// ShareVideo 分享视频
func (h *VideoHandler) ShareVideo(c *gin.Context) {
	// 获取视频ID
	videoIDStr := c.Param("id")
	videoID, err := strconv.ParseUint(videoIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	// 获取请求体
	var req struct {
		ShareType string `json:"share_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
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

	// 调用视频服务分享接口
	resp, err := videoClient.ShareVideo(ctx, &pb.ShareVideoRequest{
		Token:     getTokenFromHeader(c),
		VideoId:   uint32(videoID),
		ShareType: req.ShareType,
	})
	if err != nil {
		log.Printf("Failed to share video: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to share video"})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, resp)
}

// GetVideoStats 获取视频互动数据（点赞、收藏、转发数量）
func (h *VideoHandler) GetVideoStats(c *gin.Context) {
	// 获取视频ID
	videoIDStr := c.Param("id")
	videoID, err := strconv.ParseUint(videoIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
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

	// 调用视频服务获取视频信息
	resp, err := videoClient.GetVideoInfo(ctx, &pb.GetVideoInfoRequest{
		VideoId: uint32(videoID),
		Token:   getTokenFromHeader(c),
	})
	if err != nil {
		log.Printf("Failed to get video info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get video stats"})
		return
	}

	// 提取互动数据
	if resp.StatusCode != 0 || resp.Video == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get video stats"})
		return
	}

	// 返回互动数据
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"like_count":     resp.Video.LikeCount,
			"favorite_count": resp.Video.FavoriteCount,
			"share_count":    resp.Video.ShareCount,
			"is_liked":       resp.Video.IsLiked,
			"is_favorite":    resp.Video.IsFavorite,
		},
	})
}

// GetVideoComments 获取视频评论列表
func (h *VideoHandler) GetVideoComments(c *gin.Context) {
	// 获取查询参数
	videoIDStr := c.Query("video_id")
	if videoIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "video_id is required"})
		return
	}

	videoID, err := strconv.ParseUint(videoIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video_id"})
		return
	}

	pageStr := c.DefaultQuery("page", "1")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSizeStr := c.DefaultQuery("page_size", "10")
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}

	sortOrder := c.DefaultQuery("sort_order", "hot")

	// 获取视频服务客户端
	videoClient, err := h.getVideoClient()
	if err != nil {
		log.Printf("Failed to get video service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Video service temporarily unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 调用视频服务获取评论列表
	resp, err := videoClient.GetVideoComments(ctx, &pb.GetVideoCommentsRequest{
		VideoId:   uint32(videoID),
		Token:     getTokenFromHeader(c),
		Page:      uint32(page),
		PageSize:  uint32(pageSize),
		SortOrder: sortOrder,
	})

	if err != nil {
		log.Printf("[GetVideoComments] gRPC调用失败: %v", err)
		// 尝试打印更详细的错误信息
		if st, ok := status.FromError(err); ok {
			log.Printf("[GetVideoComments] gRPC状态码: %d, 错误描述: %s", st.Code(), st.Message())
			for _, detail := range st.Details() {
				log.Printf("[GetVideoComments] 错误详情: %v", detail)
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get video comments"})
		return
	}

	// 为评论添加用户信息
	// 创建完整的评论结构体，包含用户信息
	type CommentWithUser struct {
		Id          uint32            `json:"id"`
		User        *userpb.User      `json:"user"`
		Content     string            `json:"content"`
		VideoId     uint32            `json:"video_id"`
		ParentId    *uint32           `json:"parent_id,omitempty"`
		ReplyToUser *userpb.User      `json:"reply_to_user,omitempty"`
		LikeCount   uint32            `json:"like_count"`
		IsLiked     bool              `json:"is_liked"`
		CreateTime  int64             `json:"create_time"`
		Replies     []CommentWithUser `json:"replies,omitempty"`
	}

	commentsWithUser := make([]CommentWithUser, 0, len(resp.Comments))
	for _, comment := range resp.Comments {
		repliesWithUser := make([]CommentWithUser, 0, len(comment.Replies))
		for _, reply := range comment.Replies {
			repliesWithUser = append(repliesWithUser, CommentWithUser{
				Id:          reply.Id,
				User:        reply.User,
				Content:     reply.Content,
				VideoId:     reply.VideoId,
				ParentId:    reply.ParentId,
				ReplyToUser: reply.ReplyToUser,
				LikeCount:   reply.LikeCount,
				IsLiked:     reply.IsLiked,
				CreateTime:  reply.CreateTime,
			})
		}

		commentWithUser := CommentWithUser{
			Id:          comment.Id,
			User:        comment.User,
			Content:     comment.Content,
			VideoId:     comment.VideoId,
			ParentId:    comment.ParentId,
			ReplyToUser: comment.ReplyToUser,
			LikeCount:   comment.LikeCount,
			IsLiked:     comment.IsLiked,
			CreateTime:  comment.CreateTime,
			Replies:     repliesWithUser,
		}

		commentsWithUser = append(commentsWithUser, commentWithUser)
	}

	// 返回评论列表
	c.JSON(http.StatusOK, gin.H{
		"status_code": resp.StatusCode,
		"status_msg":  resp.StatusMsg,
		"comments":    commentsWithUser,
		"total":       resp.Total,
		"has_more":    resp.HasMore,
	})
}

// CommentVideo 发布评论
func (h *VideoHandler) CommentVideo(c *gin.Context) {
	// 解析请求体
	var req struct {
		VideoID  interface{} `json:"video_id" binding:"required"`
		Content  string      `json:"content" binding:"required"`
		ParentID *uint32     `json:"parent_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证必填字段
	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}

	// 解析视频ID（支持字符串和数字类型）
	var videoID uint32
	switch v := req.VideoID.(type) {
	case string:
		parsedID, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video_id"})
			return
		}
		videoID = uint32(parsedID)
	case float64:
		videoID = uint32(v)
	case int:
		videoID = uint32(v)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video_id type"})
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

	// 调用视频服务发表评论
	resp, err := videoClient.CommentVideo(ctx, &pb.CommentRequest{
		Token:    getTokenFromHeader(c),
		VideoId:  videoID,
		Content:  req.Content,
		ParentId: req.ParentID,
	})
	if err != nil {
		log.Printf("Failed to comment video: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to comment video"})
		return
	}

	// 返回评论结果
	c.JSON(http.StatusOK, gin.H{
		"status_code": resp.StatusCode,
		"status_msg":  resp.StatusMsg,
		"comment":     resp.Comment,
	})
}

// LikeComment 点赞/取消点赞评论
func (h *VideoHandler) LikeComment(c *gin.Context) {
	// 解析请求体
	var req struct {
		CommentID  uint32 `json:"comment_id" binding:"required"`
		ActionType bool   `json:"action_type" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	// 调用视频服务点赞评论
	resp, err := videoClient.LikeComment(ctx, &pb.LikeCommentRequest{
		Token:      getTokenFromHeader(c),
		CommentId:  req.CommentID,
		ActionType: req.ActionType,
	})
	if err != nil {
		log.Printf("Failed to like comment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to like comment"})
		return
	}

	// 返回点赞结果
	c.JSON(http.StatusOK, gin.H{
		"status_code": resp.StatusCode,
		"status_msg":  resp.StatusMsg,
		"like_count":  resp.LikeCount,
		"is_liked":    resp.IsLiked,
	})
}

// ReplyComment 回复评论
func (h *VideoHandler) ReplyComment(c *gin.Context) {
	// 解析请求体
	var req struct {
		CommentID uint32 `json:"comment_id" binding:"required"`
		Content   string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证必填字段
	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
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

	// 调用视频服务回复评论
	resp, err := videoClient.CommentVideo(ctx, &pb.CommentRequest{
		Token:    getTokenFromHeader(c),
		VideoId:  0,
		Content:  req.Content,
		ParentId: &req.CommentID,
	})
	if err != nil {
		log.Printf("Failed to reply comment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reply comment"})
		return
	}

	// 返回回复结果
	c.JSON(http.StatusOK, gin.H{
		"status_code": resp.StatusCode,
		"status_msg":  resp.StatusMsg,
		"comment":     resp.Comment,
	})
}

// GetCommentReplies 获取评论回复列表
func (h *VideoHandler) GetCommentReplies(c *gin.Context) {
	// 获取评论ID
	commentIDStr := c.Param("comment_id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid comment_id"})
		return
	}

	// 获取分页参数
	pageStr := c.DefaultQuery("page", "1")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSizeStr := c.DefaultQuery("page_size", "10")
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 50 {
		pageSize = 10
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

	// 调用视频服务获取评论回复
	// 注意：由于当前proto没有专门的GetCommentReplies接口，我们使用GetVideoComments
	// 并过滤出指定评论的回复
	resp, err := videoClient.GetVideoComments(ctx, &pb.GetVideoCommentsRequest{
		VideoId:   0,
		Token:     getTokenFromHeader(c),
		Page:      uint32(page),
		PageSize:  uint32(pageSize),
		SortOrder: "time_desc",
	})
	if err != nil {
		log.Printf("Failed to get comment replies: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get comment replies"})
		return
	}

	// 过滤出指定评论的回复
	var replies []*pb.Comment
	for _, comment := range resp.Comments {
		if comment.ParentId != nil && *comment.ParentId == uint32(commentID) {
			replies = append(replies, comment)
		}
	}

	// 返回回复列表
	c.JSON(http.StatusOK, gin.H{
		"status_code": resp.StatusCode,
		"status_msg":  resp.StatusMsg,
		"replies":     replies,
		"total":       uint32(len(replies)),
		"has_more":    false,
	})
}

// RegisterVideoRoutesWithHandler 使用已有的视频处理器注册路由
func RegisterVideoRoutesWithHandler(router *gin.Engine, videoHandler *VideoHandler) {
	// 分片上传路由组 - 需要认证
	uploadGroup := router.Group("/api/upload")
	uploadGroup.Use(middleware.RequireAuthMiddleware())
	{
		uploadGroup.GET("/check", videoHandler.CheckUploadStatus)
		uploadGroup.POST("/chunk", videoHandler.UploadChunk)
		uploadGroup.POST("/complete", videoHandler.CompleteUpload)
	}

	// 视频相关路由组
	videoGroup := router.Group("/api/video")
	{
		// 公开路由
		videoGroup.GET("/recommended", videoHandler.GetRecommendedVideos)
		videoGroup.GET("/hot", videoHandler.GetHotVideos)
		// 公开视频详情路由
		videoGroup.GET("/:id", videoHandler.GetVideoDetail)
		// 分片信息接口
		videoGroup.GET("/:id/segments", videoHandler.GetVideoSegments)
		// HLS视频流代理路由（支持分片传输）
		videoGroup.GET("/:id/stream/*filepath", videoHandler.ProxyHLSStream)
		// 视频互动数据接口
		videoGroup.GET("/:id/stats", videoHandler.GetVideoStats)
		// 视频评论相关接口
		videoGroup.GET("/comments", videoHandler.GetVideoComments)
		videoGroup.GET("/comment/:comment_id/replies", videoHandler.GetCommentReplies)
		// 需要认证的路由
		authGroup := videoGroup.Group("/")
		authGroup.Use(middleware.RequireAuthMiddleware())
		{
			authGroup.GET("/personalized", videoHandler.GetPersonalizedVideos)
			authGroup.GET("/follow", videoHandler.GetFollowVideos)
			authGroup.GET("/user/published", videoHandler.GetUserPublishedVideos)
			authGroup.POST("/publish", videoHandler.HandleVideoPublish)
			authGroup.POST("/upload", videoHandler.HandleVideoUpload)
			authGroup.POST("/retry-transcode", videoHandler.RetryTranscode)
			authGroup.POST("/:id/like", videoHandler.LikeVideo)
			authGroup.POST("/:id/favorite", videoHandler.FavoriteVideo)
			authGroup.POST("/:id/share", videoHandler.ShareVideo)
			// 评论相关接口（需要认证）
			authGroup.POST("/comment", videoHandler.CommentVideo)
			authGroup.POST("/comment/like", videoHandler.LikeComment)
			authGroup.POST("/comment/reply", videoHandler.ReplyComment)
		}
	}
}
