package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/vision_world/video_service/internal/config"
	"github.com/vision_world/video_service/internal/service"
	"github.com/vision_world/video_service/pkg/logger"
	pb "github.com/vision_world/video_service/proto/proto_gen/video"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	auditpb "audit_service/proto_gen/audit/v1"
)

// VideoHandler 视频服务处理器
type VideoHandler struct {
	pb.UnimplementedVideoServiceServer
	config       *config.Config
	videoService *service.VideoService
	auditClient  auditpb.AuditServiceClient
	auditConn    *grpc.ClientConn
}

// NewVideoHandler 创建视频处理器
func NewVideoHandler(cfg *config.Config) (*VideoHandler, error) {
	videoService, err := service.NewVideoService(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create video service: %w", err)
	}

	// 创建audit_service客户端连接
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Services.AuditService.Timeout)*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, cfg.Services.AuditService.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to audit service: %w", err)
	}

	auditClient := auditpb.NewAuditServiceClient(conn)

	logger.Info("Connected to audit service",
		zap.String("address", cfg.Services.AuditService.Address))

	return &VideoHandler{
		config:       cfg,
		videoService: videoService,
		auditClient:  auditClient,
		auditConn:    conn,
	}, nil
}

// RegisterService 注册服务到服务发现
func (h *VideoHandler) RegisterService() error {
	// TODO: 实现服务发现注册逻辑
	logger.Info("Registering video service to discovery",
		zap.String("service", h.config.Server.Name),
		zap.String("address", h.config.Server.Address))
	return nil
}

// Close 关闭处理器
func (h *VideoHandler) Close() error {
	// 关闭audit_service连接
	if h.auditConn != nil {
		if err := h.auditConn.Close(); err != nil {
			logger.Error("Failed to close audit service connection", zap.Error(err))
		}
	}

	if h.videoService != nil {
		return h.videoService.Close()
	}
	return nil
}

// ==================== 视频发布相关接口 ====================

// PublishVideo 发布视频
func (h *VideoHandler) PublishVideo(ctx context.Context, req *pb.PublishVideoRequest) (*pb.PublishVideoResponse, error) {
	logger.Info("PublishVideo called", zap.String("title", req.Title), zap.String("user_id", req.UserId))

	// TODO: 验证用户token
	// TODO: 实现视频发布逻辑

	// 生成视频ID (这里简化处理，实际应该从数据库获取)
	videoID := uint32(time.Now().Unix())

	// 调用审核服务进行内容审核
	auditReq := &auditpb.SubmitContentRequest{
		ContentId:   fmt.Sprintf("video_%d", videoID),
		ContentType: auditpb.ContentType_CONTENT_TYPE_VIDEO,
		UploaderId:  req.UserId,
		Title:       req.Title,
		Content:     req.Description,
		CreateTime:  time.Now().Format(time.RFC3339),
	}

	auditResp, err := h.auditClient.SubmitContent(ctx, auditReq)
	if err != nil {
		logger.Error("Failed to submit content for audit", zap.Error(err))
		return &pb.PublishVideoResponse{
			StatusCode: 500,
			StatusMsg:  "审核服务调用失败",
			VideoId:    0,
		}, nil
	}

	logger.Info("Content submitted for audit",
		zap.String("content_id", auditReq.ContentId),
		zap.String("audit_id", auditResp.AuditId),
		zap.String("status", auditResp.Status.String()))

	// 根据审核结果决定视频状态
	var statusMsg string
	var statusCode int32

	switch auditResp.Status {
	case auditpb.AuditStatus_AUDIT_STATUS_PASSED:
		statusCode = 0
		statusMsg = "视频发布成功"
	case auditpb.AuditStatus_AUDIT_STATUS_PENDING, auditpb.AuditStatus_AUDIT_STATUS_UNDER_REVIEW:
		statusCode = 202
		statusMsg = "视频发布成功，正在审核中"
	case auditpb.AuditStatus_AUDIT_STATUS_REJECTED:
		statusCode = 403
		statusMsg = "视频内容违规，发布失败"
	default:
		statusCode = 202
		statusMsg = "视频发布成功，等待审核"
	}

	return &pb.PublishVideoResponse{
		StatusCode: statusCode,
		StatusMsg:  statusMsg,
		VideoId:    videoID,
	}, nil
}

// DeleteVideo 删除视频
func (h *VideoHandler) DeleteVideo(ctx context.Context, req *pb.DeleteVideoRequest) (*pb.DeleteVideoResponse, error) {
	logger.Info("DeleteVideo called", zap.Uint32("video_id", req.VideoId))

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
	logger.Info("GetVideoInfo called", zap.Uint32("video_id", req.VideoId))

	// TODO: 实现获取视频信息逻辑

	return &pb.VideoResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Video: &pb.Video{
			Id:           req.VideoId,
			Title:        "TODO: Video Title",
			Description:  "TODO: Video Description",
			CoverUrl:     "TODO: Cover URL",
			VideoUrl:     "TODO: Video URL",
			PlayCount:    100,
			LikeCount:    50,
			CommentCount: 20,
			ShareCount:   10,
			CreateTime:   time.Now().Unix(),
			Duration:     60,
			Resolution:   "1080p",
			Status:       "normal",
			IsPublic:     true,
		},
	}, nil
}

// GetVideoInfos 批量获取视频信息
func (h *VideoHandler) GetVideoInfos(ctx context.Context, req *pb.GetVideoInfosRequest) (*pb.GetVideoInfosResponse, error) {
	logger.Info("GetVideoInfos called", zap.Int("video_count", len(req.VideoIds)))

	// TODO: 实现批量获取视频信息逻辑

	videos := make([]*pb.Video, 0, len(req.VideoIds))
	for _, videoId := range req.VideoIds {
		videos = append(videos, &pb.Video{
			Id:         videoId,
			Title:      "TODO: Video Title",
			CoverUrl:   "TODO: Cover URL",
			VideoUrl:   "TODO: Video URL",
			PlayCount:  100,
			LikeCount:  50,
			CreateTime: time.Now().Unix(),
		})
	}

	return &pb.GetVideoInfosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     videos,
	}, nil
}

// ==================== 视频列表相关接口 ====================

// GetUserVideos 获取用户发布的视频列表
func (h *VideoHandler) GetUserVideos(ctx context.Context, req *pb.GetUserVideosRequest) (*pb.GetUserVideosResponse, error) {
	logger.Info("GetUserVideos called", zap.Uint32("user_id", req.UserId), zap.Uint32("page", req.Page))

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
	logger.Info("GetRecommendVideos called", zap.Uint32("page", req.Page), zap.String("category", category))

	// TODO: 实现推荐算法逻辑

	videos := make([]*pb.Video, 0)
	for i := uint32(0); i < req.PageSize; i++ {
		videos = append(videos, &pb.Video{
			Id:         uint32(i + 1),
			Title:      "TODO: Recommended Video Title",
			CoverUrl:   "TODO: Cover URL",
			VideoUrl:   "TODO: Video URL",
			PlayCount:  1000,
			LikeCount:  500,
			CreateTime: time.Now().Unix(),
		})
	}

	return &pb.GetRecommendVideosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     videos,
		HasMore:    true,
	}, nil
}

// GetFollowVideos 获取关注用户的视频列表
func (h *VideoHandler) GetFollowVideos(ctx context.Context, req *pb.GetFollowVideosRequest) (*pb.GetFollowVideosResponse, error) {
	logger.Info("GetFollowVideos called", zap.Uint32("page", req.Page))

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

// ==================== 视频互动相关接口 ====================

// LikeVideo 点赞/取消点赞视频
func (h *VideoHandler) LikeVideo(ctx context.Context, req *pb.LikeVideoRequest) (*pb.LikeVideoResponse, error) {
	actionType := "like"
	if !req.ActionType {
		actionType = "unlike"
	}
	logger.Info("LikeVideo called", zap.Uint32("video_id", req.VideoId), zap.String("action_type", actionType))

	// TODO: 验证用户token
	// TODO: 实现点赞/取消点赞逻辑

	return &pb.LikeVideoResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		LikeCount:  150, // TODO: 真实的点赞数
	}, nil
}

// GetUserLikedVideos 获取用户点赞的视频列表
func (h *VideoHandler) GetUserLikedVideos(ctx context.Context, req *pb.GetUserLikedVideosRequest) (*pb.GetUserLikedVideosResponse, error) {
	logger.Info("GetUserLikedVideos called", zap.Uint32("user_id", req.UserId), zap.Uint32("page", req.Page))

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
	logger.Info("ShareVideo called", zap.Uint32("video_id", req.VideoId), zap.String("share_type", req.ShareType))

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
	logger.Info("CommentVideo called", zap.Uint32("video_id", req.VideoId), zap.String("content", req.Content))

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
	logger.Info("DeleteComment called", zap.Uint32("comment_id", req.CommentId))

	// TODO: 验证用户token和权限
	// TODO: 实现删除评论逻辑

	return &pb.DeleteCommentResponse{
		StatusCode: 0,
		StatusMsg:  "success",
	}, nil
}

// GetVideoComments 获取视频评论列表
func (h *VideoHandler) GetVideoComments(ctx context.Context, req *pb.GetVideoCommentsRequest) (*pb.GetVideoCommentsResponse, error) {
	logger.Info("GetVideoComments called", zap.Uint32("video_id", req.VideoId), zap.Uint32("page", req.Page), zap.String("sort_order", req.SortOrder))

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
