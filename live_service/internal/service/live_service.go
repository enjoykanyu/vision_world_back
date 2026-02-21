package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"

	"live_service/internal/config"
	"live_service/internal/model"
	"live_service/internal/repository"
	"live_service/pkg/logger"
)

// LiveService 直播服务接口
type LiveService interface {
	// 直播流管理
	StartLive(ctx context.Context, userID uint64, title string, category string) (*model.LiveStream, error)
	StopLive(ctx context.Context, userID uint64) error
	GetLiveStream(ctx context.Context, streamID uint64) (*model.LiveStream, error)
	GetLiveStreamByRoomID(ctx context.Context, roomID uint64) (*model.LiveStream, error)
	GetLiveList(ctx context.Context, page, pageSize int, categoryID uint32) ([]*model.LiveStream, int64, error)
	GetHotLiveList(ctx context.Context, page, pageSize int) ([]*model.LiveStream, int64, error)

	// 直播间管理
	GetLiveRoom(ctx context.Context, roomID uint64) (*model.LiveRoom, error)
	JoinLiveRoom(ctx context.Context, streamID, userID uint64) (*model.LiveViewer, error)
	LeaveLiveRoom(ctx context.Context, streamID, userID uint64) error
	GetLiveViewerList(ctx context.Context, streamID uint64, page, pageSize int) ([]*model.LiveViewer, int64, error)

	// 聊天消息
	SendLiveChat(ctx context.Context, streamID, userID uint64, content, contentType string) (*model.LiveChat, error)
	GetLiveChatList(ctx context.Context, streamID uint64, page, pageSize int) ([]*model.LiveChat, int64, error)

	// 礼物系统
	SendLiveGift(ctx context.Context, streamID, userID uint64, giftID uint32, giftCount uint32) (*model.LiveGift, error)
	GetLiveGiftList(ctx context.Context, streamID uint64, page, pageSize int) ([]*model.LiveGift, int64, error)

	// 互动功能
	LikeLive(ctx context.Context, streamID, userID uint64) error

	// 搜索和推荐
	SearchLive(ctx context.Context, keyword string, page, pageSize int) ([]*model.LiveStream, int64, error)
	GetLiveCategories(ctx context.Context) ([]*LiveCategory, error)

	// 统计和分析
	GetLiveStats(ctx context.Context, streamID uint64) (*LiveStats, error)
	GetLivePlayback(ctx context.Context, streamID uint64) (*LivePlayback, error)
}

// LiveCategory 直播分类
type LiveCategory struct {
	ID        uint32 `json:"id"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sort_order"`
	IsActive  bool   `json:"is_active"`
}

// LiveStats 直播统计
type LiveStats struct {
	StreamID       uint64 `json:"stream_id"`
	TotalViewers   uint64 `json:"total_viewers"`
	CurrentViewers uint32 `json:"current_viewers"`
	MaxViewers     uint32 `json:"max_viewers"`
	LikeCount      uint32 `json:"like_count"`
	GiftCount      uint32 `json:"gift_count"`
	CommentCount   uint32 `json:"comment_count"`
	ShareCount     uint32 `json:"share_count"`
	Duration       uint32 `json:"duration"`
	GiftValue      uint64 `json:"gift_value"`
}

// LivePlayback 直播回放
type LivePlayback struct {
	StreamID    uint64 `json:"stream_id"`
	PlaybackURL string `json:"playback_url"`
	Duration    uint32 `json:"duration"`
	FileSize    uint64 `json:"file_size"`
	Format      string `json:"format"`
	Quality     string `json:"quality"`
	CreatedAt   int64  `json:"created_at"`
}

// liveService 直播服务实现
type liveService struct {
	config        *config.Config
	logger        logger.Logger
	liveRepo      repository.LiveRepository
	streamManager StreamManager
	chatManager   ChatManager
	giftManager   GiftManager
}

// NewLiveService 创建直播服务
func NewLiveService(cfg *config.Config, log logger.Logger, db *gorm.DB, redis *redis.Client) LiveService {
	liveRepo := repository.NewLiveRepository(db, redis, log)
	streamManager := NewStreamManager(cfg, log, liveRepo)
	chatManager := NewChatManager(cfg, log, liveRepo)
	giftManager := NewGiftManager(cfg, log, liveRepo)

	return &liveService{
		config:        cfg,
		logger:        log,
		liveRepo:      liveRepo,
		streamManager: streamManager,
		chatManager:   chatManager,
		giftManager:   giftManager,
	}
}

// StartLive 开始直播
func (s *liveService) StartLive(ctx context.Context, userID uint64, title string, category string) (*model.LiveStream, error) {
	log.Println("Starting live stream", "userID", userID, "title", title)

	// 1. 查询或创建直播间
	room, err := s.liveRepo.GetLiveRoomByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	var roomID uint64
	if room == nil {
		// 1.1 新建直播间
		newRoom := &model.LiveRoom{
			UserID: userID,
			Name:   title,
			Status: 0, // 离线状态
		}
		if err := s.liveRepo.CreateLiveRoom(ctx, newRoom); err != nil {
			return nil, fmt.Errorf("failed to create live room: %w", err)
		}
		roomID = newRoom.ID
		log.Println("Created new live room", "roomID", roomID)
	} else {
		roomID = room.ID
	}
	//if room == nil {
	//	// 1.1 新建直播间
	//	roomNumber := generateRoomNumber()
	//	newRoom := &model.LiveRoom{
	//		UserID:     userID,
	//		Name:       title,
	//		Status:     0, // 离线状态
	//		RoomNumber: roomNumber,
	//	}
	//	if err := s.liveRepo.CreateLiveRoom(ctx, newRoom); err != nil {
	//		return nil, fmt.Errorf("failed to create live room: %w", err)
	//	}
	//	roomID = newRoom.ID
	//	log.Println("Created new live room", "roomID", roomID, "roomNumber", roomNumber)
	//} else {
	//	roomID = room.ID
	//}
	// 2. 检查用户是否已有进行中的直播
	existingStream, err := s.liveRepo.GetLiveStreamByUserID(ctx, userID)

	if err == nil && existingStream != nil {
		return nil, errors.New("user already has an active live stream")
	}

	// 3. 创建直播流记录
	stream, err := s.streamManager.CreateLiveStream(ctx, roomID, userID, title, category)
	if err != nil {
		return nil, err
	}

	// 4. 生成各种播放地址
	playURLs := s.streamManager.GeneratePlayURL(stream.StreamKey)

	// 5. 更新直播间状态为在线
	if err := s.liveRepo.UpdateLiveRoomStatus(ctx, roomID, 1); err != nil {
		log.Println("Failed to update room status", "error", err)
	}

	// 6. 更新直播流状态为直播中
	if err := s.liveRepo.UpdateLiveStreamStatus(ctx, stream.ID, model.LiveStatusStreaming); err != nil {
		log.Println("Failed to update stream status", "error", err)
	}

	// 7. 返回直播流信息（包含推流地址和播放地址）
	stream.PlaybackURL = playURLs["hls"] // 设置默认播放地址
	stream.Status = model.LiveStatusStreaming

	log.Println("Live stream started successfully",
		"streamID", stream.ID,
		"streamKey", stream.StreamKey,
		"pushURL", stream.StreamURL)

	return stream, nil
}

// StopLive 结束直播
func (s *liveService) StopLive(ctx context.Context, userID uint64) error {
	existingStream, err := s.liveRepo.GetLiveStreamByUserID(ctx, userID)
	streamID := existingStream.ID
	log.Println("Stopping live stream", "streamID", streamID, "userID", userID)

	// 1. 获取直播流信息
	stream, err := s.liveRepo.GetLiveStream(ctx, streamID)
	if err != nil {
		log.Println("Failed to get live stream", "error", err)
		return fmt.Errorf("failed to get live stream: %w", err)
	}
	if stream == nil {
		log.Println("Live stream not found", "streamID", streamID)
		return errors.New("live stream not found")
	}

	// 2. 验证用户权限
	if stream.UserID != userID {
		log.Println("Permission denied", "streamID", streamID, "userID", userID)
		return errors.New("permission denied: not the stream owner")
	}

	// 3. 更新直播流状态为已结束
	if err := s.liveRepo.UpdateLiveStreamStatus(ctx, streamID, model.LiveStatusEnded); err != nil {
		log.Println("Failed to update stream status", "error", err)
		return fmt.Errorf("failed to update stream status: %w", err)
	}

	// 4. 更新直播间状态为离线
	if err := s.liveRepo.UpdateLiveRoomStatus(ctx, stream.RoomID, model.RoomStatusOffline); err != nil {
		log.Println("Failed to update room status", "error", err)
		return fmt.Errorf("failed to update room status: %w", err)
	}

	// TODO: 计算直播时长、生成回放文件、清理相关资源

	return nil
}

// GetLiveStream 获取直播流信息
func (s *liveService) GetLiveStream(ctx context.Context, streamID uint64) (*model.LiveStream, error) {
	log.Println("Getting live stream info", "streamID", streamID)

	// TODO: 实现获取直播流信息逻辑
	// 这里应该包含：
	// 1. 从缓存或数据库获取直播流信息
	// 2. 更新观看统计
	// 3. 返回格式化数据

	return &model.LiveStream{
		ID:     streamID,
		Status: model.LiveStatusStreaming,
	}, nil
}

// GetLiveRoom 获取直播间信息
func (s *liveService) GetLiveRoom(ctx context.Context, roomID uint64) (*model.LiveRoom, error) {
	log.Println("Getting live room info", "roomID", roomID)
	return s.liveRepo.GetLiveRoom(ctx, roomID)
}

// GetLiveStreamByRoomID 根据房间ID获取正在进行的直播流
func (s *liveService) GetLiveStreamByRoomID(ctx context.Context, roomID uint64) (*model.LiveStream, error) {
	log.Println("Getting live stream by room ID", "roomID", roomID)
	return s.liveRepo.GetLiveStreamByRoomID(ctx, roomID)
}

// GetLiveList 获取直播列表
func (s *liveService) GetLiveList(ctx context.Context, page, pageSize int, categoryID uint32) ([]*model.LiveStream, int64, error) {
	log.Println("Getting live list", "page", page, "pageSize", pageSize, "categoryID", categoryID)

	// TODO: 实现获取直播列表逻辑
	// 这里应该包含：
	// 1. 根据分类筛选直播
	// 2. 按热度或时间排序
	// 3. 分页查询
	// 4. 返回格式化的直播列表

	return []*model.LiveStream{}, 0, nil
}

// GetHotLiveList 获取热门直播列表
func (s *liveService) GetHotLiveList(ctx context.Context, page, pageSize int) ([]*model.LiveStream, int64, error) {
	log.Println("Getting hot live list", "page", page, "pageSize", pageSize)

	// TODO: 实现获取热门直播列表逻辑
	// 这里应该包含：
	// 1. 根据热度算法排序
	// 2. 考虑观看人数、点赞数、礼物数等因素
	// 3. 分页查询
	// 4. 返回热门直播列表

	return []*model.LiveStream{}, 0, nil
}

// JoinLiveRoom 加入直播间
func (s *liveService) JoinLiveRoom(ctx context.Context, streamID, userID uint64) (*model.LiveViewer, error) {
	log.Println("Joining live room", "streamID", streamID, "userID", userID)

	// TODO: 实现加入直播间逻辑
	// 这里应该包含：
	// 1. 验证直播间状态
	// 2. 创建观看者记录
	// 3. 更新观看人数
	// 4. 发送系统消息
	// 5. 返回观看者信息

	return &model.LiveViewer{
		ID:       1,
		StreamID: streamID,
		UserID:   userID,
	}, nil
}

// LeaveLiveRoom 离开直播间
func (s *liveService) LeaveLiveRoom(ctx context.Context, streamID, userID uint64) error {
	log.Println("Leaving live room", "streamID", streamID, "userID", userID)

	// TODO: 实现离开直播间逻辑
	// 这里应该包含：
	// 1. 更新观看者记录
	// 2. 减少观看人数
	// 3. 计算观看时长

	return nil
}

// GetLiveViewerList 获取直播观看者列表
func (s *liveService) GetLiveViewerList(ctx context.Context, streamID uint64, page, pageSize int) ([]*model.LiveViewer, int64, error) {
	log.Println("Getting live viewer list", "streamID", streamID, "page", page, "pageSize", pageSize)

	// TODO: 实现获取观看者列表逻辑
	// 这里应该包含：
	// 1. 查询当前观看者
	// 2. 按进入时间排序
	// 3. 分页查询
	// 4. 返回观看者列表

	return []*model.LiveViewer{}, 0, nil
}

// SendLiveChat 发送直播聊天消息
func (s *liveService) SendLiveChat(ctx context.Context, streamID, userID uint64, content, contentType string) (*model.LiveChat, error) {
	log.Println("Sending live chat", "streamID", streamID, "userID", userID)

	// TODO: 实现发送聊天消息逻辑
	// 这里应该包含：
	// 1. 验证用户权限
	// 2. 内容过滤和审核
	// 3. 创建聊天消息
	// 4. 推送给其他观看者
	// 5. 更新聊天统计

	return &model.LiveChat{
		ID:       1,
		StreamID: streamID,
		UserID:   userID,
		Content:  content,
	}, nil
}

// GetLiveChatList 获取直播聊天列表
func (s *liveService) GetLiveChatList(ctx context.Context, streamID uint64, page, pageSize int) ([]*model.LiveChat, int64, error) {
	s.logger.Info("Getting live chat list", "streamID", streamID, "page", page, "pageSize", pageSize)

	// TODO: 实现获取聊天列表逻辑
	// 这里应该包含：
	// 1. 查询聊天记录
	// 2. 按时间排序
	// 3. 分页查询
	// 4. 返回聊天列表

	return []*model.LiveChat{}, 0, nil
}

// SendLiveGift 发送直播礼物
func (s *liveService) SendLiveGift(ctx context.Context, streamID, userID uint64, giftID uint32, giftCount uint32) (*model.LiveGift, error) {
	log.Println("Sending live gift", "streamID", streamID, "userID", userID, "giftID", giftID)

	// TODO: 实现发送礼物逻辑
	// 这里应该包含：
	// 1. 验证用户余额
	// 2. 扣除用户金币
	// 3. 创建礼物记录
	// 4. 增加主播收益
	// 5. 发送礼物特效
	// 6. 更新礼物统计

	return &model.LiveGift{
		ID:       1,
		StreamID: streamID,
		UserID:   userID,
		GiftID:   giftID,
	}, nil
}

// GetLiveGiftList 获取直播礼物列表
func (s *liveService) GetLiveGiftList(ctx context.Context, streamID uint64, page, pageSize int) ([]*model.LiveGift, int64, error) {
	log.Println("Getting live gift list", "streamID", streamID, "page", page, "pageSize", pageSize)

	// TODO: 实现获取礼物列表逻辑
	// 这里应该包含：
	// 1. 查询礼物记录
	// 2. 按价值排序
	// 3. 分页查询
	// 4. 返回礼物列表

	return []*model.LiveGift{}, 0, nil
}

// LikeLive 点赞直播
func (s *liveService) LikeLive(ctx context.Context, streamID, userID uint64) error {
	log.Println("Liking live stream", "streamID", streamID, "userID", userID)

	// TODO: 实现点赞逻辑
	// 这里应该包含：
	// 1. 检查是否已点赞
	// 2. 创建点赞记录
	// 3. 更新点赞统计
	// 4. 发送点赞特效

	return nil
}

// SearchLive 搜索直播
func (s *liveService) SearchLive(ctx context.Context, keyword string, page, pageSize int) ([]*model.LiveStream, int64, error) {
	log.Println("Searching live streams", "keyword", keyword, "page", page, "pageSize", pageSize)

	// TODO: 实现搜索直播逻辑
	// 这里应该包含：
	// 1. 关键词分词
	// 2. 全文搜索
	// 3. 相关性排序
	// 4. 分页查询
	// 5. 返回搜索结果

	return []*model.LiveStream{}, 0, nil
}

// GetLiveCategories 获取直播分类
func (s *liveService) GetLiveCategories(ctx context.Context) ([]*LiveCategory, error) {
	log.Println("Getting live categories")

	// TODO: 实现获取直播分类逻辑
	// 这里应该返回预设的直播分类列表

	return []*LiveCategory{}, nil
}

// GetLiveStats 获取直播统计
func (s *liveService) GetLiveStats(ctx context.Context, streamID uint64) (*LiveStats, error) {
	log.Println("Getting live stats", "streamID", streamID)

	// TODO: 实现获取直播统计逻辑
	// 这里应该包含：
	// 1. 查询观看人数
	// 2. 查询互动数据
	// 3. 计算直播时长
	// 4. 返回统计信息

	return &LiveStats{
		StreamID: streamID,
	}, nil
}

// GetLivePlayback 获取直播回放
func (s *liveService) GetLivePlayback(ctx context.Context, streamID uint64) (*LivePlayback, error) {
	log.Println("Getting live playback", "streamID", streamID)

	// TODO: 实现获取直播回放逻辑
	// 这里应该包含：
	// 1. 检查回放文件是否存在
	// 2. 获取回放文件信息
	// 3. 生成播放地址
	// 4. 返回回放信息

	return &LivePlayback{
		StreamID: streamID,
	}, nil
}

// generateRoomNumber 生成唯一的房间号
// 格式: 时间戳(36进制) + 4位随机字符
func generateRoomNumber() string {
	// 使用时间戳的36进制表示（缩短长度）
	timestamp := time.Now().Unix()
	timestampPart := strconv.FormatInt(timestamp, 36)

	// 生成4位随机字符
	b := make([]byte, 2)
	rand.Read(b)
	randomPart := hex.EncodeToString(b)

	// 组合成房间号（大写）
	roomNumber := timestampPart + randomPart
	if len(roomNumber) > 20 {
		roomNumber = roomNumber[:20]
	}
	return roomNumber
}
