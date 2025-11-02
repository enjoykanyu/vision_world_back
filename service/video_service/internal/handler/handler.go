package handler

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/vision_world/video_service/internal/config"
	"github.com/vision_world/video_service/internal/discovery"
	"github.com/vision_world/video_service/internal/model"
	"github.com/vision_world/video_service/internal/queue"
	"github.com/vision_world/video_service/internal/service"
	"github.com/vision_world/video_service/pkg/logger"
	"github.com/vision_world/video_service/pkg/minio"
	auditpb "github.com/vision_world/video_service/proto/proto_gen/audit"
	userpb "github.com/vision_world/video_service/proto/proto_gen/user"
	pb "github.com/vision_world/video_service/proto/proto_gen/video"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// VideoHandler 视频服务处理器
type VideoHandler struct {
	pb.UnimplementedVideoServiceServer
	config          *config.Config
	videoService    *service.VideoService
	auditClient     auditpb.AuditServiceClient
	auditConn       *grpc.ClientConn
	queueClient     *queue.RabbitMQClient
	minioClient     *minio.Client
	discoveryClient discovery.ServiceDiscovery
	serviceID       string
	logger          logger.Logger
}

// NewVideoHandler 创建视频处理器
func NewVideoHandler(cfg *config.Config, log logger.Logger, db *gorm.DB, redis *redis.Client) (*VideoHandler, error) {

	videoService, err := service.NewVideoService(cfg, db, redis)
	if err != nil {
		return nil, fmt.Errorf("failed to create video service: %w", err)
	}

	// 创建MinIO客户端
	minioClient, err := minio.NewClient(minio.Config{
		Endpoint:        cfg.MinIO.Endpoint,
		AccessKeyID:     cfg.MinIO.AccessKeyID,
		SecretAccessKey: cfg.MinIO.SecretAccessKey,
		UseSSL:          cfg.MinIO.UseSSL,
		BucketName:      cfg.MinIO.BucketName,
		Location:        cfg.MinIO.Location,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	// 创建RabbitMQ客户端
	queueClient, err := queue.NewRabbitMQClient(cfg, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create RabbitMQ client: %w", err)
	}

	// 创建audit_service客户端连接
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Services.AuditService.Timeout)*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, cfg.Services.AuditService.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		queueClient.Close() // 清理RabbitMQ连接
		return nil, fmt.Errorf("failed to connect to audit service: %w", err)
	}

	auditClient := auditpb.NewAuditServiceClient(conn)

	log.Info("Connected to audit service",
		"address", cfg.Services.AuditService.Address)

	// 创建服务发现客户端
	var discoveryClient discovery.ServiceDiscovery
	if cfg.Discovery.Type != "" {
		discoveryClient, err = discovery.NewServiceDiscovery(&cfg.Discovery, cfg.Server.Name)
		if err != nil {
			log.Error("Failed to create service discovery client", "error", err)
			// 服务发现创建失败不影响服务启动，只记录错误
		} else {
			log.Info("Service discovery client created",
				"type", cfg.Discovery.Type)
		}
	}

	// 生成服务ID
	host, port, err := getHostAndPort(cfg.Server.Address)
	if err != nil {
		log.Error("Failed to parse server address", "error", err)
		host = "localhost"
		port = 50052 // 默认端口
	}

	serviceID := fmt.Sprintf("%s-%s-%d", cfg.Server.Name, host, port)

	return &VideoHandler{
		config:          cfg,
		videoService:    videoService,
		auditClient:     auditClient,
		auditConn:       conn,
		queueClient:     queueClient,
		minioClient:     minioClient,
		discoveryClient: discoveryClient,
		serviceID:       serviceID,
		logger:          log,
	}, nil
}

// getHostAndPort 从地址中解析主机和端口
func getHostAndPort(address string) (string, int, error) {
	// 移除地址前缀
	address = strings.TrimPrefix(address, ":")

	// 如果地址只包含端口，使用localhost作为主机
	if !strings.Contains(address, ":") {
		port, err := strconv.Atoi(address)
		if err != nil {
			return "", 0, fmt.Errorf("invalid port: %s", address)
		}
		return "localhost", port, nil
	}

	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, fmt.Errorf("failed to split host and port: %w", err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port: %s", portStr)
	}

	return host, port, nil
}

// RegisterService 注册服务到服务发现
func (h *VideoHandler) RegisterService() error {
	if h.discoveryClient == nil {
		h.logger.Info("Service discovery not configured, skipping registration")
		return nil
	}

	// 解析服务器地址
	host, port, err := getHostAndPort(h.config.Server.Address)
	if err != nil {
		return fmt.Errorf("failed to parse server address: %w", err)
	}

	// 构建服务信息
	serviceInfo := &discovery.ServiceInfo{
		ID:   h.serviceID,
		Name: h.config.Server.Name,
		Host: host,
		Port: port,
		Tags: []string{"video", "grpc", h.config.Server.Environment},
		Meta: map[string]string{
			"version":     h.config.Server.Version,
			"environment": h.config.Server.Environment,
		},
		Check: &discovery.HealthCheck{
			GRPC:                           fmt.Sprintf("%s:%d", host, port),
			Interval:                       fmt.Sprintf("%ds", h.config.Discovery.Interval),
			Timeout:                        "5s",
			DeregisterCriticalServiceAfter: "30s",
		},
	}

	// 注册服务
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.discoveryClient.Register(ctx, serviceInfo); err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	h.logger.Info("Service registered successfully",
		"service_id", h.serviceID,
		"service_name", h.config.Server.Name,
		"address", fmt.Sprintf("%s:%d", host, port))

	return nil
}

// Close 关闭处理器
func (h *VideoHandler) Close() error {
	var errs []error

	// 注销服务
	if h.discoveryClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := h.discoveryClient.Deregister(ctx, h.serviceID); err != nil {
			h.logger.Error("Failed to deregister service", "error", err)
			errs = append(errs, err)
		} else {
			h.logger.Info("Service deregistered successfully", "service_id", h.serviceID)
		}

		if err := h.discoveryClient.Close(); err != nil {
			h.logger.Error("Failed to close discovery client", "error", err)
			errs = append(errs, err)
		}
	}

	// 关闭RabbitMQ连接
	if h.queueClient != nil {
		if err := h.queueClient.Close(); err != nil {
			h.logger.Error("Failed to close RabbitMQ connection", "error", err)
			errs = append(errs, err)
		}
	}

	// 关闭audit_service连接
	if h.auditConn != nil {
		if err := h.auditConn.Close(); err != nil {
			h.logger.Error("Failed to close audit service connection", "error", err)
			errs = append(errs, err)
		}
	}

	if h.videoService != nil {
		if err := h.videoService.Close(); err != nil {
			h.logger.Error("Failed to close video service", "error", err)
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiple errors occurred during close: %v", errs)
	}

	return nil
}

// ==================== 视频发布相关接口 ====================

// UploadVideo 上传视频
func (h *VideoHandler) UploadVideo(ctx context.Context, req *pb.UploadVideoRequest) (*pb.UploadVideoResponse, error) {
	h.logger.Info("UploadVideo called", "token", req.Token, "file_name", req.FileName)

	// TODO: 验证用户token
	// TODO: 从token中解析用户ID
	userID := "user_123" // 临时用户ID

	// 生成视频ID (这里简化处理，实际应该从数据库获取)
	videoID := uint32(time.Now().Unix())

	// 创建对象名称
	objectName := fmt.Sprintf("videos/%d/%s", videoID, req.FileName)

	// 将视频数据上传到MinIO
	videoURL, err := h.minioClient.UploadFileFromReader(ctx, objectName,
		bytes.NewReader(req.VideoData),
		int64(len(req.VideoData)),
		"video/mp4")
	if err != nil {
		h.logger.Error("Failed to upload video to MinIO", "error", err)
		return &pb.UploadVideoResponse{
			StatusCode: 500,
			StatusMsg:  "视频上传失败",
			VideoId:    0,
		}, nil
	}

	h.logger.Info("Video uploaded to MinIO successfully",
		"video_url", videoURL,
		"object_name", objectName)

	// 保存视频信息到数据库 (简化处理)
	// TODO: 实际应该调用videoService保存视频信息

	// 发送审核消息到RabbitMQ队列
	auditMessage := &queue.AuditMessage{
		ContentID:    fmt.Sprintf("video_%d", videoID),
		ContentType:  "video",
		Title:        req.Title,
		URL:          videoURL,
		Metadata:     req.Description,
		UploaderID:   userID,
		UploaderName: userID, // TODO: 从用户信息获取用户名
	}

	if err := h.queueClient.PublishAuditMessage(ctx, auditMessage); err != nil {
		h.logger.Error("Failed to publish audit message", "error", err)
		// 审核消息发送失败，但视频已上传成功，可以继续返回成功状态
		h.logger.Warn("Audit message failed, but video uploaded successfully")
	} else {
		h.logger.Info("Audit message published successfully",
			"content_id", auditMessage.ContentID,
			"content_type", auditMessage.ContentType)
	}

	// 视频进入审核中状态
	statusCode := int32(202)
	statusMsg := "视频上传成功，正在审核中"

	return &pb.UploadVideoResponse{
		StatusCode: statusCode,
		StatusMsg:  statusMsg,
		VideoId:    videoID,
		VideoUrl:   videoURL,
	}, nil
}

// PublishVideo 发布视频
func (h *VideoHandler) PublishVideo(ctx context.Context, req *pb.PublishVideoRequest) (*pb.PublishVideoResponse, error) {
	h.logger.Info("PublishVideo called", zap.String("title", req.Title), zap.String("token", req.Token))

	// TODO: 验证用户token
	// TODO: 实现视频发布逻辑

	// 生成视频ID (这里简化处理，实际应该从数据库获取)
	videoID := uint32(time.Now().Unix())

	// 发送审核消息到RabbitMQ队列
	auditMessage := &queue.AuditMessage{
		ContentID:    fmt.Sprintf("video_%d", videoID),
		ContentType:  "video",
		Title:        req.Title,
		URL:          "", // PublishVideo方法没有视频URL，可以留空或从其他来源获取
		Metadata:     req.Description,
		UploaderID:   req.Token,
		UploaderName: req.Token, // TODO: 从用户信息获取用户名
	}

	if err := h.queueClient.PublishAuditMessage(ctx, auditMessage); err != nil {
		h.logger.Error("Failed to publish audit message", zap.Error(err))
		return &pb.PublishVideoResponse{
			StatusCode: 500,
			StatusMsg:  "审核消息发送失败",
			VideoId:    0,
		}, nil
	}

	h.logger.Info("Audit message published successfully",
		zap.String("content_id", auditMessage.ContentID),
		zap.String("content_type", auditMessage.ContentType))

	// 视频进入审核中状态
	statusCode := int32(202)
	statusMsg := "视频发布成功，正在审核中"

	return &pb.PublishVideoResponse{
		StatusCode: statusCode,
		StatusMsg:  statusMsg,
		VideoId:    videoID,
	}, nil
}

// DeleteVideo 删除视频
func (h *VideoHandler) DeleteVideo(ctx context.Context, req *pb.DeleteVideoRequest) (*pb.DeleteVideoResponse, error) {
	h.logger.Info("DeleteVideo called", zap.Uint32("video_id", req.VideoId))

	// TODO: 验证用户token和权限
	// TODO: 实现视频删除逻辑

	return &pb.DeleteVideoResponse{
		StatusCode: 0,
		StatusMsg:  "success",
	}, nil
}

// ==================== 视频信息获取接口 ====================

// GetVideoInfo 获取单个视频信息
func (h *VideoHandler) GetVideoInfo(ctx context.Context, req *pb.GetVideoInfoRequest) (*pb.VideoResponse, error) {
	h.logger.Info("GetVideoInfo called", zap.Uint32("video_id", req.VideoId))

	// 调用service层获取视频信息
	videoID := strconv.FormatUint(uint64(req.VideoId), 10)
	video, err := h.videoService.GetVideoByID(ctx, videoID)
	if err != nil {
		h.logger.Error("Failed to get video info", zap.Error(err))
		return &pb.VideoResponse{
			StatusCode: 500,
			StatusMsg:  "获取视频信息失败",
		}, nil
	}

	if video == nil {
		return &pb.VideoResponse{
			StatusCode: 404,
			StatusMsg:  "视频不存在",
		}, nil
	}

	// 增加播放量
	_, err = h.videoService.UpdateVideoViewCount(ctx, videoID, 1)
	if err != nil {
		h.logger.Error("Failed to update view count", zap.Error(err))
		// 不影响主流程，只记录错误
	}

	// 转换为protobuf格式
	pbVideo := h.convertToProtoVideo(video)

	return &pb.VideoResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Video:      pbVideo,
	}, nil
}

// GetVideoInfos 批量获取视频信息
func (h *VideoHandler) GetVideoInfos(ctx context.Context, req *pb.GetVideoInfosRequest) (*pb.GetVideoInfosResponse, error) {
	h.logger.Info("GetVideoInfos called", zap.Int("video_count", len(req.VideoIds)))

	// 转换视频ID列表
	videoIDs := make([]string, 0, len(req.VideoIds))
	for _, videoId := range req.VideoIds {
		videoIDs = append(videoIDs, strconv.FormatUint(uint64(videoId), 10))
	}

	// 调用service层获取视频信息
	videos, err := h.videoService.GetVideosByIDs(ctx, videoIDs)
	if err != nil {
		h.logger.Error("Failed to get videos info", zap.Error(err))
		return &pb.GetVideoInfosResponse{
			StatusCode: 500,
			StatusMsg:  "获取视频信息失败",
		}, nil
	}

	// 转换为protobuf格式
	pbVideos := make([]*pb.Video, 0, len(videos))
	for _, video := range videos {
		pbVideos = append(pbVideos, h.convertToProtoVideo(video))
	}

	return &pb.GetVideoInfosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     pbVideos,
	}, nil
}

// ==================== 视频列表相关接口 ====================

// GetUserVideos 获取用户发布的视频列表
func (h *VideoHandler) GetUserVideos(ctx context.Context, req *pb.GetUserVideosRequest) (*pb.GetUserVideosResponse, error) {
	h.logger.Info("GetUserVideos called", zap.Uint32("user_id", req.UserId), zap.Uint32("page", req.Page))

	// TODO: 实现获取用户视频列表逻辑

	videos := make([]*pb.Video, 0)
	for i := uint32(0); i < req.PageSize; i++ {
		videos = append(videos, &pb.Video{
			Id:         uint32(i + 1),
			Title:      "TODO: User Video Title",
			CoverUrl:   "TODO: Cover URL",
			VideoUrl:   "TODO: Video URL",
			PlayCount:  100,
			LikeCount:  50,
			CreateTime: time.Now().Unix(),
		})
	}

	return &pb.GetUserVideosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     videos,
		Total:      100, // TODO: 真实的总数
		HasMore:    true,
	}, nil
}

// GetRecommendVideos 获取推荐视频列表
func (h *VideoHandler) GetRecommendVideos(ctx context.Context, req *pb.GetRecommendVideosRequest) (*pb.GetRecommendVideosResponse, error) {
	category := ""
	if req.Category != nil {
		category = *req.Category
	}
	h.logger.Info("GetRecommendVideos called", zap.Uint32("page", req.Page), zap.String("category", category))

	// 调用service层获取热门视频作为推荐视频
	page := int(req.Page)
	pageSize := int(req.PageSize)

	videos, hasMore, err := h.videoService.GetHotVideos(ctx, page, pageSize, category)
	if err != nil {
		h.logger.Error("Failed to get recommend videos", zap.Error(err))
		return &pb.GetRecommendVideosResponse{
			StatusCode: 500,
			StatusMsg:  "获取推荐视频失败",
		}, nil
	}

	// 转换为protobuf格式
	pbVideos := make([]*pb.Video, 0, len(videos))
	for _, video := range videos {
		pbVideos = append(pbVideos, h.convertToProtoVideo(video))
	}

	return &pb.GetRecommendVideosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     pbVideos,
		HasMore:    hasMore,
	}, nil
}

// GetFollowVideos 获取关注用户的视频列表
func (h *VideoHandler) GetFollowVideos(ctx context.Context, req *pb.GetFollowVideosRequest) (*pb.GetFollowVideosResponse, error) {
	h.logger.Info("GetFollowVideos called", zap.Uint32("page", req.Page))

	// TODO: 验证用户token
	// TODO: 实现获取关注用户视频逻辑

	videos := make([]*pb.Video, 0)
	for i := uint32(0); i < req.PageSize; i++ {
		videos = append(videos, &pb.Video{
			Id:         uint32(i + 1),
			Title:      "TODO: Followed User Video Title",
			CoverUrl:   "TODO: Cover URL",
			VideoUrl:   "TODO: Video URL",
			PlayCount:  200,
			LikeCount:  100,
			CreateTime: time.Now().Unix(),
		})
	}

	return &pb.GetFollowVideosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     videos,
		HasMore:    true,
	}, nil
}

// GetHotVideos 获取热门视频列表
func (h *VideoHandler) GetHotVideos(ctx context.Context, req *pb.GetHotVideosRequest) (*pb.GetHotVideosResponse, error) {
	category := ""
	if req.Category != nil {
		category = *req.Category
	}
	h.logger.Info("GetHotVideos called", zap.Uint32("page", req.Page), zap.String("category", category))

	// 调用service层获取热门视频
	page := int(req.Page)
	pageSize := int(req.PageSize)

	videos, hasMore, err := h.videoService.GetHotVideos(ctx, page, pageSize, category)
	if err != nil {
		h.logger.Error("Failed to get hot videos", zap.Error(err))
		return &pb.GetHotVideosResponse{
			StatusCode: 500,
			StatusMsg:  "获取热门视频失败",
		}, nil
	}

	// 转换为protobuf格式
	pbVideos := make([]*pb.Video, 0, len(videos))
	for _, video := range videos {
		pbVideos = append(pbVideos, h.convertToProtoVideo(video))
	}

	return &pb.GetHotVideosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     pbVideos,
		HasMore:    hasMore,
	}, nil
}

// GetCategoryVideos 获取分类视频列表
func (h *VideoHandler) GetCategoryVideos(ctx context.Context, req *pb.GetCategoryVideosRequest) (*pb.GetCategoryVideosResponse, error) {
	h.logger.Info("GetCategoryVideos called", zap.String("category", req.Category), zap.Uint32("page", req.Page))

	// 调用service层获取分类视频
	page := int(req.Page)
	pageSize := int(req.PageSize)

	videos, hasMore, err := h.videoService.GetCategoryVideos(ctx, req.Category, page, pageSize)
	if err != nil {
		h.logger.Error("Failed to get category videos", zap.Error(err))
		return &pb.GetCategoryVideosResponse{
			StatusCode: 500,
			StatusMsg:  "获取分类视频失败",
		}, nil
	}

	// 转换为protobuf格式
	pbVideos := make([]*pb.Video, 0, len(videos))
	for _, video := range videos {
		pbVideos = append(pbVideos, h.convertToProtoVideo(video))
	}

	return &pb.GetCategoryVideosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     pbVideos,
		HasMore:    hasMore,
	}, nil
}

// SearchVideos 搜索视频
func (h *VideoHandler) SearchVideos(ctx context.Context, req *pb.SearchVideosRequest) (*pb.SearchVideosResponse, error) {
	category := ""
	if req.Category != nil {
		category = *req.Category
	}
	h.logger.Info("SearchVideos called", zap.String("keyword", req.Keyword), zap.String("category", category), zap.Uint32("page", req.Page))

	// 调用service层搜索视频
	page := int(req.Page)
	pageSize := int(req.PageSize)

	videos, hasMore, err := h.videoService.SearchVideos(ctx, req.Keyword, page, pageSize, category)
	if err != nil {
		h.logger.Error("Failed to search videos", zap.Error(err))
		return &pb.SearchVideosResponse{
			StatusCode: 500,
			StatusMsg:  "搜索视频失败",
		}, nil
	}

	// 转换为protobuf格式
	pbVideos := make([]*pb.Video, 0, len(videos))
	for _, video := range videos {
		pbVideos = append(pbVideos, h.convertToProtoVideo(video))
	}

	return &pb.SearchVideosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     pbVideos,
		HasMore:    hasMore,
	}, nil
}

// ==================== 视频互动相关接口 ====================

// LikeVideo 点赞/取消点赞视频
func (h *VideoHandler) LikeVideo(ctx context.Context, req *pb.LikeVideoRequest) (*pb.LikeVideoResponse, error) {
	actionType := "like"
	if !req.ActionType {
		actionType = "unlike"
	}
	h.logger.Info("LikeVideo called", zap.Uint32("video_id", req.VideoId), zap.String("action_type", actionType))

	// TODO: 验证用户token

	// 调用service层更新点赞数
	videoID := strconv.FormatUint(uint64(req.VideoId), 10)
	var increment int32
	if req.ActionType {
		increment = 1
	} else {
		increment = -1
	}

	likeCount, err := h.videoService.UpdateVideoLikeCount(ctx, videoID, increment)
	if err != nil {
		h.logger.Error("Failed to update like count", zap.Error(err))
		return &pb.LikeVideoResponse{
			StatusCode: 500,
			StatusMsg:  "更新点赞数失败",
			LikeCount:  0,
		}, nil
	}

	return &pb.LikeVideoResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		LikeCount:  uint32(likeCount),
	}, nil
}

// GetUserLikedVideos 获取用户点赞的视频列表
func (h *VideoHandler) GetUserLikedVideos(ctx context.Context, req *pb.GetUserLikedVideosRequest) (*pb.GetUserLikedVideosResponse, error) {
	h.logger.Info("GetUserLikedVideos called", zap.Uint32("user_id", req.UserId), zap.Uint32("page", req.Page))

	// TODO: 实现获取用户点赞视频逻辑

	videos := make([]*pb.Video, 0)
	for i := uint32(0); i < req.PageSize; i++ {
		videos = append(videos, &pb.Video{
			Id:         uint32(i + 1),
			Title:      "TODO: Liked Video Title",
			CoverUrl:   "TODO: Cover URL",
			VideoUrl:   "TODO: Video URL",
			PlayCount:  300,
			LikeCount:  200,
			CreateTime: time.Now().Unix(),
		})
	}

	return &pb.GetUserLikedVideosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     videos,
		Total:      50, // TODO: 真实的总数
		HasMore:    true,
	}, nil
}

// ShareVideo 分享视频
func (h *VideoHandler) ShareVideo(ctx context.Context, req *pb.ShareVideoRequest) (*pb.ShareVideoResponse, error) {
	h.logger.Info("ShareVideo called", zap.Uint32("video_id", req.VideoId), zap.String("share_type", req.ShareType))

	// TODO: 验证用户token
	// TODO: 实现分享逻辑

	return &pb.ShareVideoResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		ShareUrl:   "TODO: Generated share URL",
	}, nil
}

// ==================== 视频评论相关接口 ====================

// CommentVideo 发表评论
func (h *VideoHandler) CommentVideo(ctx context.Context, req *pb.CommentRequest) (*pb.CommentResponse, error) {
	h.logger.Info("CommentVideo called", zap.Uint32("video_id", req.VideoId), zap.String("content", req.Content))

	// TODO: 验证用户token
	// TODO: 实现评论逻辑

	return &pb.CommentResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Comment: &pb.Comment{
			Id:         1, // TODO: 真实的评论ID
			Content:    req.Content,
			VideoId:    req.VideoId,
			ParentId:   req.ParentId,
			LikeCount:  0,
			CreateTime: time.Now().Unix(),
		},
	}, nil
}

// DeleteComment 删除评论
func (h *VideoHandler) DeleteComment(ctx context.Context, req *pb.DeleteCommentRequest) (*pb.DeleteCommentResponse, error) {
	h.logger.Info("DeleteComment called", zap.Uint32("comment_id", req.CommentId))

	// TODO: 验证用户token和权限
	// TODO: 实现删除评论逻辑

	return &pb.DeleteCommentResponse{
		StatusCode: 0,
		StatusMsg:  "success",
	}, nil
}

// GetVideoComments 获取视频评论列表
func (h *VideoHandler) GetVideoComments(ctx context.Context, req *pb.GetVideoCommentsRequest) (*pb.GetVideoCommentsResponse, error) {
	h.logger.Info("GetVideoComments called", zap.Uint32("video_id", req.VideoId), zap.Uint32("page", req.Page), zap.String("sort_order", req.SortOrder))

	// TODO: 实现获取评论列表逻辑

	comments := make([]*pb.Comment, 0)
	for i := uint32(0); i < req.PageSize; i++ {
		comments = append(comments, &pb.Comment{
			Id:         uint32(i + 1),
			Content:    "TODO: Comment content",
			VideoId:    req.VideoId,
			LikeCount:  10,
			CreateTime: time.Now().Unix(),
		})
	}

	return &pb.GetVideoCommentsResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Comments:   comments,
		Total:      100, // TODO: 真实的总数
		HasMore:    true,
	}, nil
}

// ==================== 辅助方法 ====================

// convertToProtoVideo 将模型转换为protobuf格式
func (h *VideoHandler) convertToProtoVideo(video *model.RecommendationVideo) *pb.Video {
	if video == nil {
		return nil
	}

	// 解析视频ID
	videoID, err := strconv.ParseUint(video.VideoID, 10, 32)
	if err != nil {
		h.logger.Error("Failed to parse video ID", zap.String("video_id", video.VideoID), zap.Error(err))
		videoID = 0
	}

	// 解析标签
	tags := make([]string, 0)
	if video.Tags != "" {
		tags = strings.Split(video.Tags, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}

	// 创建作者信息
	author := &userpb.User{
		Id:   0, // TODO: 从其他地方获取作者ID
		Name: video.Author,
	}

	return &pb.Video{
		Id:           uint32(videoID),
		Title:        video.Title,
		Description:  video.Description,
		CoverUrl:     video.CoverURL,
		VideoUrl:     video.PlayURL,
		PlayCount:    uint32(video.ViewCount),
		LikeCount:    uint32(video.LikeCount),
		CommentCount: 0, // RecommendationVideo 中没有这个字段
		ShareCount:   0, // RecommendationVideo 中没有这个字段
		CreateTime:   video.CreatedAt.Unix(),
		UpdateTime:   video.UpdatedAt.Unix(),
		Duration:     uint32(video.Duration),
		Resolution:   "",       // RecommendationVideo 中没有这个字段
		Status:       "normal", // 默认状态
		IsPublic:     true,     // 默认公开
		Category:     video.Category,
		Author:       author,
		Tags:         tags,
	}
}
