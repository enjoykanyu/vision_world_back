package handler

import (
	"context"
	"fmt"
	pb "live_service/proto/proto_gen/audit"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"

	"live_service/internal/config"
	"live_service/internal/service"
	"live_service/pkg/logger"
	proto_gen "live_service/proto/proto_gen"
	auditv1 "live_service/proto/proto_gen/audit"
)

// LiveServiceHandler 直播服务处理器
type LiveServiceHandler struct {
	config       *config.Config
	logger       logger.Logger
	liveService  service.LiveService
	auditManager interface { // 使用接口定义，降低耦合
		SubmitContent(ctx context.Context, req interface{}) (interface{}, error)
		GetAuditResult(ctx context.Context, req *pb.GetAuditResultRequest) (*pb.GetAuditResultResponse, error)
		Close() error
	}
	proto_gen.UnimplementedLiveServiceServer
}

// NewLiveServiceHandler 创建直播服务处理器
func NewLiveServiceHandler(cfg *config.Config, log logger.Logger, db *gorm.DB, redis *redis.Client) *LiveServiceHandler {
	// 创建直播服务
	liveService := service.NewLiveService(cfg, log, db, redis)

	return &LiveServiceHandler{
		config:      cfg,
		logger:      log,
		liveService: liveService,
	}
}

// SetAuditManager 设置审计管理器
func (h *LiveServiceHandler) SetAuditManager(manager interface {
	SubmitContent(ctx context.Context, req interface{}) (interface{}, error)
	GetAuditResult(ctx context.Context, req *pb.GetAuditResultRequest) (*pb.GetAuditResultResponse, error)
	Close() error
}) {
	h.auditManager = manager
	h.logger.Info("Audit manager set successfully")
}

// StartLive 开始直播
func (h *LiveServiceHandler) StartLive(ctx context.Context, req *proto_gen.StartLiveRequest) (*proto_gen.StartLiveResponse, error) {
	h.logger.Info("StartLive called", "user_id", req.UserId, "title", req.Title)

	// 生成直播流ID (这里简化处理，实际应该从数据库获取)
	streamID := fmt.Sprintf("stream_%d", time.Now().Unix())

	// 如果有审核服务客户端管理器，调用审核服务进行直播间审核
	if h.auditManager != nil {
		// 创建审核请求 - 使用pb_gen生成的类型
		auditReq := &auditv1.SubmitContentRequest{
			ContentId:   fmt.Sprintf("live_%s", streamID),
			ContentType: auditv1.ContentType_CONTENT_TYPE_LIVE, // 使用pb_gen定义的常量
			UploaderId:  req.UserId,
			Content:     req.Description,
			Metadata: map[string]string{
				"title":       req.Title,
				"create_time": time.Now().Format(time.RFC3339),
			},
		}

		// 调用审核服务
		resp, err := h.auditManager.SubmitContent(ctx, auditReq)
		if err != nil {
			h.logger.Error("Failed to submit live content for audit", "error", err, "content_id", auditReq.ContentId)
			// 审核服务调用失败，仍然允许直播开始，但记录日志
			// 这里可以根据业务需求决定是否阻止直播开始
		} else {
			// 类型断言转换为auditv1.SubmitContentResponse
			auditResp, ok := resp.(*auditv1.SubmitContentResponse)
			if !ok {
				h.logger.Error("Failed to cast audit response to auditv1.SubmitContentResponse", "content_id", auditReq.ContentId)
				// 类型转换失败仍允许直播继续
			} else {
				h.logger.Info("Audit response received",
					"content_id", auditReq.ContentId,
					"audit_id", auditResp.AuditId,
					"status", auditResp.Status,
					"level", auditResp.Level)

				if auditResp.Status == auditv1.AuditStatus_AUDIT_STATUS_REJECTED { // 使用pb_gen定义的常量
					h.logger.Warn("Live content rejected by audit",
						"content_id", auditReq.ContentId,
						"status", auditResp.Status,
						"reason", auditResp.Reason,
						"level", auditResp.Level)
					return &proto_gen.StartLiveResponse{
						Code:      403,
						Message:   fmt.Sprintf("直播内容违规，无法开始直播: %s", auditResp.Reason),
						RequestId: req.RequestId,
						Stream:    nil,
						StreamUrl: "",
						StreamKey: "",
					}, nil
				}

				// 如果审核通过或者是待审核状态，允许直播开始
				if auditResp.Status == auditv1.AuditStatus_AUDIT_STATUS_PASSED {
					h.logger.Info("Live content passed audit", "content_id", auditReq.ContentId)
				} else if auditResp.Status == auditv1.AuditStatus_AUDIT_STATUS_PENDING {
					h.logger.Info("Live content pending audit", "content_id", auditReq.ContentId)
				}
			}
		}
	} else {
		h.logger.Warn("Audit manager not available, skipping content audit", "content_id", streamID)
	}

	// TODO: 实现开始直播逻辑
	h.logger.Info("Starting live stream",
		"stream_id", streamID,
		"user_id", req.UserId,
		"title", req.Title)

	return &proto_gen.StartLiveResponse{
		Code:      200,
		Message:   "直播开始成功",
		RequestId: req.RequestId,
		Stream: &proto_gen.LiveStream{
			Id:          3,
			UserId:      req.UserId,
			Title:       req.Title,
			Status:      "live",
			ViewerCount: 0,
		},
		StreamUrl: fmt.Sprintf("rtmp://localhost:1935/live/%s", streamID),
		StreamKey: streamID,
	}, nil
}

// StopLive 结束直播
func (h *LiveServiceHandler) StopLive(ctx context.Context, req *proto_gen.StopLiveRequest) (*proto_gen.StopLiveResponse, error) {
	h.logger.Info("StopLive called")

	// TODO: 实现结束直播逻辑
	return &proto_gen.StopLiveResponse{
		Code:      200,
		Message:   "直播结束成功",
		RequestId: req.RequestId,
	}, nil
}

// GetLiveStream 获取直播流信息
func (h *LiveServiceHandler) GetLiveStream(ctx context.Context, req *proto_gen.GetLiveStreamRequest) (*proto_gen.GetLiveStreamResponse, error) {
	h.logger.Info("GetLiveStream called")

	// TODO: 实现获取直播流信息逻辑
	return &proto_gen.GetLiveStreamResponse{
		Code:      200,
		Message:   "获取直播流信息成功",
		RequestId: req.RequestId,
		Stream:    &proto_gen.LiveStream{},
	}, nil
}

// GetLiveList 获取直播列表
func (h *LiveServiceHandler) GetLiveList(ctx context.Context, req *proto_gen.GetLiveListRequest) (*proto_gen.GetLiveListResponse, error) {
	h.logger.Info("GetLiveList called")

	// TODO: 实现获取直播列表逻辑
	return &proto_gen.GetLiveListResponse{
		Code:      200,
		Message:   "获取直播列表成功",
		RequestId: req.RequestId,
		Streams:   []*proto_gen.LiveStream{},
		Total:     0,
	}, nil
}

// GetHotLiveList 获取热门直播列表
func (h *LiveServiceHandler) GetHotLiveList(ctx context.Context, req *proto_gen.GetHotLiveListRequest) (*proto_gen.GetHotLiveListResponse, error) {
	h.logger.Info("GetHotLiveList called")

	// TODO: 实现获取热门直播列表逻辑
	return &proto_gen.GetHotLiveListResponse{
		Code:      0,
		Message:   "success",
		RequestId: req.RequestId,
		Streams:   []*proto_gen.LiveStream{},
		Total:     0,
	}, nil
}

// JoinLiveRoom 加入直播间
func (h *LiveServiceHandler) JoinLiveRoom(ctx context.Context, req *proto_gen.JoinLiveRoomRequest) (*proto_gen.JoinLiveRoomResponse, error) {
	h.logger.Info("JoinLiveRoom called")

	// TODO: 实现加入直播间逻辑
	return &proto_gen.JoinLiveRoomResponse{
		Code:      200,
		Message:   "加入直播间成功",
		RequestId: req.RequestId,
		Viewer:    &proto_gen.LiveViewer{},
	}, nil
}

// LeaveLiveRoom 离开直播间
func (h *LiveServiceHandler) LeaveLiveRoom(ctx context.Context, req *proto_gen.LeaveLiveRoomRequest) (*proto_gen.LeaveLiveRoomResponse, error) {
	h.logger.Info("LeaveLiveRoom called")

	// TODO: 实现离开直播间逻辑
	return &proto_gen.LeaveLiveRoomResponse{
		Code:      200,
		Message:   "离开直播间成功",
		RequestId: req.RequestId,
	}, nil
}

// SendLiveChat 发送直播聊天消息
func (h *LiveServiceHandler) SendLiveChat(ctx context.Context, req *proto_gen.SendLiveChatRequest) (*proto_gen.SendLiveChatResponse, error) {
	h.logger.Info("SendLiveChat called")

	// TODO: 实现发送直播聊天消息逻辑
	return &proto_gen.SendLiveChatResponse{
		Code:      200,
		Message:   "消息发送成功",
		RequestId: req.RequestId,
		Chat:      &proto_gen.LiveChat{},
	}, nil
}

// GetLiveChatList 获取直播聊天列表
func (h *LiveServiceHandler) GetLiveChatList(ctx context.Context, req *proto_gen.GetLiveChatListRequest) (*proto_gen.GetLiveChatListResponse, error) {
	h.logger.Info("GetLiveChatList called")

	// TODO: 实现获取直播聊天列表逻辑
	return &proto_gen.GetLiveChatListResponse{
		Code:      200,
		Message:   "获取直播聊天列表成功",
		RequestId: req.RequestId,
		Chats:     []*proto_gen.LiveChat{},
	}, nil
}

// SendLiveGift 发送直播礼物
func (h *LiveServiceHandler) SendLiveGift(ctx context.Context, req *proto_gen.SendLiveGiftRequest) (*proto_gen.SendLiveGiftResponse, error) {
	h.logger.Info("SendLiveGift called")

	// TODO: 实现发送直播礼物逻辑
	return &proto_gen.SendLiveGiftResponse{
		Code:      200,
		Message:   "礼物发送成功",
		RequestId: req.RequestId,
		Gift:      &proto_gen.LiveGift{},
	}, nil
}

// GetLiveGiftList 获取直播礼物列表
func (h *LiveServiceHandler) GetLiveGiftList(ctx context.Context, req *proto_gen.GetLiveGiftListRequest) (*proto_gen.GetLiveGiftListResponse, error) {
	h.logger.Info("GetLiveGiftList called")

	// TODO: 实现获取礼物列表逻辑
	return &proto_gen.GetLiveGiftListResponse{
		Code:      200,
		Message:   "获取礼物列表成功",
		RequestId: req.RequestId,
		Gifts:     []*proto_gen.LiveGift{},
		Total:     0,
	}, nil
}

// LikeLive 点赞直播
func (h *LiveServiceHandler) LikeLive(ctx context.Context, req *proto_gen.LikeLiveRequest) (*proto_gen.LikeLiveResponse, error) {
	h.logger.Info("LikeLive called")

	// TODO: 实现点赞直播逻辑
	return &proto_gen.LikeLiveResponse{
		Code:      200,
		Message:   "点赞成功",
		RequestId: req.RequestId,
		LikeCount: 0,
	}, nil
}

// GetLiveViewerList 获取直播观看者列表
func (h *LiveServiceHandler) GetLiveViewerList(ctx context.Context, req *proto_gen.GetLiveViewerListRequest) (*proto_gen.GetLiveViewerListResponse, error) {
	h.logger.Info("GetLiveViewerList called")

	// TODO: 实现获取直播观看者列表逻辑
	return &proto_gen.GetLiveViewerListResponse{
		Code:      200,
		Message:   "获取观看者列表成功",
		RequestId: req.RequestId,
		Viewers:   []*proto_gen.LiveViewer{},
		Total:     0,
	}, nil
}

// GetLiveStats 获取直播统计
func (h *LiveServiceHandler) GetLiveStats(ctx context.Context, req *proto_gen.GetLiveStatsRequest) (*proto_gen.GetLiveStatsResponse, error) {
	h.logger.Info("GetLiveStats called")

	// TODO: 实现获取直播统计逻辑
	return &proto_gen.GetLiveStatsResponse{
		Code:      200,
		Message:   "获取直播统计成功",
		RequestId: req.RequestId,
		Stats:     &proto_gen.LiveStats{},
	}, nil
}

// SearchLive 搜索直播
func (h *LiveServiceHandler) SearchLive(ctx context.Context, req *proto_gen.SearchLiveRequest) (*proto_gen.SearchLiveResponse, error) {
	h.logger.Info("SearchLive called")

	// TODO: 实现搜索直播逻辑
	return &proto_gen.SearchLiveResponse{
		Code:      200,
		Message:   "搜索直播成功",
		RequestId: req.RequestId,
		Streams:   []*proto_gen.LiveStream{},
		Total:     0,
	}, nil
}

// GetLiveCategories 获取直播分类
func (h *LiveServiceHandler) GetLiveCategories(ctx context.Context, req *proto_gen.GetLiveCategoriesRequest) (*proto_gen.GetLiveCategoriesResponse, error) {
	h.logger.Info("GetLiveCategories called")

	// TODO: 实现获取直播分类逻辑
	return &proto_gen.GetLiveCategoriesResponse{
		Code:       0,
		Message:    "success",
		RequestId:  req.RequestId,
		Categories: []*proto_gen.LiveCategory{},
	}, nil
}

// GetLivePlayback 获取直播回放
func (h *LiveServiceHandler) GetLivePlayback(ctx context.Context, req *proto_gen.GetLivePlaybackRequest) (*proto_gen.GetLivePlaybackResponse, error) {
	h.logger.Info("GetLivePlayback called")

	// TODO: 实现获取直播回放逻辑
	return &proto_gen.GetLivePlaybackResponse{
		Code:      200,
		Message:   "获取直播回放成功",
		RequestId: req.RequestId,
		Playback:  &proto_gen.LivePlayback{},
	}, nil
}

// Close 关闭处理器，释放资源
func (h *LiveServiceHandler) Close() error {
	if h.auditManager != nil {
		if err := h.auditManager.Close(); err != nil {
			h.logger.Error("关闭audit服务客户端管理器失败", "error", err)
			return err
		}
		h.logger.Info("audit服务客户端管理器已关闭")
	}
	return nil
}
