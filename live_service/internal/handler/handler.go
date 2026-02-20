package handler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"

	"live_service/internal/config"
	"live_service/internal/service"
	"live_service/pkg/logger"
	livepb "live_service/proto/proto_gen/live"
)

// LiveServiceHandler 直播服务处理器
type LiveServiceHandler struct {
	config      *config.Config
	logger      logger.Logger
	liveService service.LiveService
	livepb.UnimplementedLiveServiceServer
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

// CreateRoom 创建直播间
func (h *LiveServiceHandler) CreateRoom(ctx context.Context, req *livepb.CreateRoomRequest) (*livepb.CreateRoomResponse, error) {
	h.logger.Info("CreateRoom called", "user_id", req.UserId, "title", req.Title)

	// TODO: 实现创建直播间逻辑
	roomID := uint64(time.Now().Unix())
	streamKey := fmt.Sprintf("stream_%d_%d", req.UserId, roomID)

	return &livepb.CreateRoomResponse{
		Code:      0,
		Message:   "创建直播间成功",
		RoomId:    roomID,
		StreamKey: streamKey,
		PushUrl:   fmt.Sprintf("rtmp://localhost:1935/live/%s", streamKey),
	}, nil
}

// StartLive 开始直播
func (h *LiveServiceHandler) StartLive(ctx context.Context, req *livepb.StartLiveRequest) (*livepb.StartLiveResponse, error) {
	log.Println("StartLive called", "user_id", req.UserId, "title", req.Title, "category", req.Category)

	// 调用service开始直播
	stream, err := h.liveService.StartLive(ctx, req.UserId, req.Title, req.Category)
	if err != nil {
		log.Println("StartLive failed", "error", err)
		return &livepb.StartLiveResponse{
			Code:    1,
			Message: "开始直播失败: " + err.Error(),
		}, nil
	}

	// 生成各种播放地址
	log.Println("Live stream started",
		"stream_id", stream.ID,
		"stream_key", stream.StreamKey,
		"stream_url", stream.StreamURL,
		"playback_url", stream.PlaybackURL)

	return &livepb.StartLiveResponse{
		Code:      0,
		Message:   "直播开始成功",
		RoomId:    stream.RoomID,
		StreamKey: stream.StreamKey,
		PushUrl:   stream.StreamURL,
		PlayUrl:   stream.PlaybackURL,
		FlvUrl:    fmt.Sprintf("http://localhost:8085/live/%s.flv", stream.StreamKey),
		WebrtcUrl: fmt.Sprintf("webrtc://localhost:1985/live/%s", stream.StreamKey),
		IsNewRoom: true,
	}, nil
}

// StopLive 结束直播
func (h *LiveServiceHandler) StopLive(ctx context.Context, req *livepb.StopLiveRequest) (*livepb.StopLiveResponse, error) {
	h.logger.Info("StopLive called", "user_id", req.UserId, "room_id", req.RoomId)

	// TODO: 实现结束直播逻辑
	return &livepb.StopLiveResponse{
		Code:    0,
		Message: "直播结束成功",
	}, nil
}

// GetRoomInfo 获取房间信息
func (h *LiveServiceHandler) GetRoomInfo(ctx context.Context, req *livepb.GetRoomInfoRequest) (*livepb.GetRoomInfoResponse, error) {
	h.logger.Info("GetRoomInfo called", "room_id", req.RoomId)

	// 获取直播间信息
	room, err := h.liveService.GetLiveRoom(ctx, req.RoomId)
	if err != nil {
		h.logger.Error("Failed to get live room", "error", err)
		return &livepb.GetRoomInfoResponse{
			Code:    1,
			Message: "获取房间信息失败: " + err.Error(),
		}, nil
	}

	if room == nil {
		return &livepb.GetRoomInfoResponse{
			Code:    1,
			Message: "直播间不存在",
		}, nil
	}

	// 获取当前正在进行的直播流
	stream, _ := h.liveService.GetLiveStreamByRoomID(ctx, req.RoomId)

	// 转换状态并设置播放地址
	status := "offline"
	var playUrl string
	var streamKey string
	var startedAt int64

	if room.Status == 1 && stream != nil {
		status = "streaming"
		playUrl = stream.PlaybackURL
		streamKey = stream.StreamKey
		if stream.StartedAt != nil {
			startedAt = stream.StartedAt.Unix()
		}
	} else if room.Status == 2 {
		status = "banned"
	}

	return &livepb.GetRoomInfoResponse{
		Code:    0,
		Message: "获取房间信息成功",
		Room: &livepb.LiveRoom{
			Id:          room.ID,
			UserId:      room.UserID,
			Title:       room.Name,
			Category:    "娱乐", // TODO: 从分类表获取
			Status:      status,
			CoverUrl:    room.CoverImage,
			StreamKey:   streamKey,
			PlayUrl:     playUrl,
			OnlineCount: 0, // TODO: 从在线人数统计获取
			StartedAt:   startedAt,
			CreatedAt:   room.CreatedAt.Unix(),
			UpdatedAt:   room.UpdatedAt.Unix(),
		},
	}, nil
}

// EnterRoom 进入直播间
func (h *LiveServiceHandler) EnterRoom(ctx context.Context, req *livepb.EnterRoomRequest) (*livepb.EnterRoomResponse, error) {
	h.logger.Info("EnterRoom called", "user_id", req.UserId, "room_id", req.RoomId)

	// TODO: 实现进入直播间逻辑
	streamKey := fmt.Sprintf("stream_%d", req.RoomId)

	return &livepb.EnterRoomResponse{
		Code:        0,
		Message:     "进入直播间成功",
		PlayUrl:     fmt.Sprintf("http://localhost:8085/live/%s.m3u8", streamKey),
		OnlineCount: 100,
	}, nil
}

// LeaveRoom 离开直播间
func (h *LiveServiceHandler) LeaveRoom(ctx context.Context, req *livepb.LeaveRoomRequest) (*livepb.LeaveRoomResponse, error) {
	h.logger.Info("LeaveRoom called", "user_id", req.UserId, "room_id", req.RoomId)

	// TODO: 实现离开直播间逻辑
	return &livepb.LeaveRoomResponse{
		Code:    0,
		Message: "离开直播间成功",
	}, nil
}

// GetLiveList 获取直播列表
func (h *LiveServiceHandler) GetLiveList(ctx context.Context, req *livepb.GetLiveListRequest) (*livepb.GetLiveListResponse, error) {
	h.logger.Info("GetLiveList called", "category", req.Category, "page", req.Page)

	// TODO: 实现获取直播列表逻辑
	rooms := []*livepb.LiveRoom{
		{
			Id:          1,
			UserId:      1,
			Title:       "测试直播间1",
			Category:    "娱乐",
			Status:      "streaming",
			OnlineCount: 100,
		},
		{
			Id:          2,
			UserId:      2,
			Title:       "测试直播间2",
			Category:    "游戏",
			Status:      "streaming",
			OnlineCount: 200,
		},
	}

	return &livepb.GetLiveListResponse{
		Code:    0,
		Message: "获取直播列表成功",
		Rooms:   rooms,
		Total:   int32(len(rooms)),
	}, nil
}

// Close 关闭处理器，释放资源
func (h *LiveServiceHandler) Close() error {
	return nil
}
