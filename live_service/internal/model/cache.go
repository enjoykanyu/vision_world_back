package model

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisKey Redis键前缀定义
const (
	// 直播缓存相关
	LiveStreamCacheKey  = "live:stream:%d"        // 直播流缓存
	LiveRoomCacheKey    = "live:room:%d"          // 直播间缓存
	LiveViewerCacheKey  = "live:viewer:%d:%d"     // 直播观看者缓存
	LiveStreamListKey   = "live:stream:list:%s"   // 直播流列表缓存
	LiveHotListKey      = "live:hot:list"         // 热门直播列表缓存
	LiveCategoryListKey = "live:category:%d:list" // 分类直播列表缓存

	// 统计相关
	LiveStatsCacheKey  = "live:stats:%d"        // 直播统计缓存
	LiveTrendCacheKey  = "live:trend:%d:%s"     // 直播趋势缓存
	LiveViewerStatsKey = "live:viewer:stats:%d" // 观看者统计缓存
	LiveGiftStatsKey   = "live:gift:stats:%d"   // 礼物统计缓存

	// 分布式锁相关
	LiveStreamLockKey = "lock:live:stream:%d"    // 直播流操作锁
	LiveRoomLockKey   = "lock:live:room:%d"      // 直播间操作锁
	LiveViewerLockKey = "lock:live:viewer:%d:%d" // 观看者操作锁
	LiveGiftLockKey   = "lock:live:gift:%d"      // 礼物操作锁

	// 计数器相关
	LiveCounterKey       = "counter:live:%s:%d"     // 直播计数器
	GlobalLiveCounterKey = "counter:live:global:%s" // 全局直播计数器

	// 实时数据相关
	LiveRealTimeKey    = "live:realtime:%d"     // 实时直播数据
	LiveViewerCountKey = "live:viewer:count:%d" // 实时观看人数
	LiveLikeCountKey   = "live:like:count:%d"   // 实时点赞数
	LiveGiftRankKey    = "live:gift:rank:%d"    // 实时礼物排行

	// 推荐相关
	LiveRecommendKey     = "live:recommend:%d"      // 直播推荐缓存
	LiveUserRecommendKey = "live:user:recommend:%d" // 用户直播推荐
)

// CacheTTL 缓存过期时间定义
const (
	LiveStreamTTL   = 5 * time.Minute  // 直播流缓存5分钟
	LiveRoomTTL     = 10 * time.Minute // 直播间缓存10分钟
	LiveViewerTTL   = 2 * time.Minute  // 观看者缓存2分钟
	LiveStatsTTL    = 1 * time.Minute  // 统计缓存1分钟
	LiveListTTL     = 30 * time.Second // 直播列表缓存30秒
	LiveHotListTTL  = 10 * time.Second // 热门列表缓存10秒
	LiveRealTimeTTL = 5 * time.Second  // 实时数据缓存5秒
	LiveTrendTTL    = 5 * time.Minute  // 趋势缓存5分钟
	LockExpiration  = 10 * time.Second // 分布式锁过期时间
)

// LiveStreamCache 直播流缓存数据结构
type LiveStreamCache struct {
	StreamID     uint64    `json:"stream_id"`
	StreamKey    string    `json:"stream_key"`
	Title        string    `json:"title"`
	UserID       uint64    `json:"user_id"`
	RoomID       uint64    `json:"room_id"`
	CategoryID   uint32    `json:"category_id"`
	Status       uint8     `json:"status"`
	StreamURL    string    `json:"stream_url"`
	ThumbnailURL string    `json:"thumbnail_url"`
	ViewerCount  uint32    `json:"viewer_count"`
	LikeCount    uint32    `json:"like_count"`
	GiftCount    uint32    `json:"gift_count"`
	IsPublic     bool      `json:"is_public"`
	VideoQuality string    `json:"video_quality"`
	StartedAt    time.Time `json:"started_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// LiveRoomCache 直播间缓存数据结构
type LiveRoomCache struct {
	RoomID       uint64    `json:"room_id"`
	RoomNumber   string    `json:"room_number"`
	Name         string    `json:"name"`
	UserID       uint64    `json:"user_id"`
	CoverImage   string    `json:"cover_image"`
	Status       uint8     `json:"status"`
	MaxViewers   uint32    `json:"max_viewers"`
	TotalStreams uint32    `json:"total_streams"`
	TotalViewers uint64    `json:"total_viewers"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// LiveViewerCache 观看者缓存数据结构
type LiveViewerCache struct {
	ViewerID      uint64    `json:"viewer_id"`
	UserID        uint64    `json:"user_id"`
	UserNickname  string    `json:"user_nickname"`
	UserAvatar    string    `json:"user_avatar"`
	UserLevel     uint8     `json:"user_level"`
	EnterTime     time.Time `json:"enter_time"`
	WatchDuration uint32    `json:"watch_duration"`
	IsLiked       bool      `json:"is_liked"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// LiveStatsCache 直播统计缓存数据结构
type LiveStatsCache struct {
	StreamID       uint64    `json:"stream_id"`
	TotalViewers   uint64    `json:"total_viewers"`
	CurrentViewers uint32    `json:"current_viewers"`
	MaxViewers     uint32    `json:"max_viewers"`
	LikeCount      uint32    `json:"like_count"`
	GiftCount      uint32    `json:"gift_count"`
	CommentCount   uint32    `json:"comment_count"`
	ShareCount     uint32    `json:"share_count"`
	Duration       uint32    `json:"duration"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// LiveTrendCache 直播趋势缓存数据结构
type LiveTrendCache struct {
	StreamID    uint64       `json:"stream_id"`
	Period      string       `json:"period"` // day/hour/minute
	ViewerTrend []TrendPoint `json:"viewer_trend"`
	LikeTrend   []TrendPoint `json:"like_trend"`
	GiftTrend   []TrendPoint `json:"gift_trend"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// TrendPoint 趋势数据点
type TrendPoint struct {
	Time      string `json:"time"`
	Value     uint32 `json:"value"`
	Timestamp int64  `json:"timestamp"`
}

// LiveListCache 直播列表缓存数据结构
type LiveListCache struct {
	Streams   []LiveStreamCache `json:"streams"`
	Total     int64             `json:"total"`
	Page      int               `json:"page"`
	PageSize  int               `json:"page_size"`
	HasMore   bool              `json:"has_more"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// LiveHotListCache 热门直播列表缓存数据结构
type LiveHotListCache struct {
	Streams   []LiveStreamCache `json:"streams"`
	Total     int64             `json:"total"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// LiveGiftRankCache 礼物排行缓存数据结构
type LiveGiftRankCache struct {
	StreamID  uint64        `json:"stream_id"`
	Rankings  []GiftRanking `json:"rankings"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// GiftRanking 礼物排行项
type GiftRanking struct {
	UserID       uint64 `json:"user_id"`
	UserNickname string `json:"user_nickname"`
	UserAvatar   string `json:"user_avatar"`
	TotalValue   uint64 `json:"total_value"`
	GiftCount    uint32 `json:"gift_count"`
	Rank         int    `json:"rank"`
}

// CacheHelper 缓存辅助函数

// GetLiveStreamCacheKey 获取直播流缓存键
func GetLiveStreamCacheKey(streamID uint64) string {
	return fmt.Sprintf(LiveStreamCacheKey, streamID)
}

// GetLiveRoomCacheKey 获取直播间缓存键
func GetLiveRoomCacheKey(roomID uint64) string {
	return fmt.Sprintf(LiveRoomCacheKey, roomID)
}

// GetLiveViewerCacheKey 获取观看者缓存键
func GetLiveViewerCacheKey(streamID, userID uint64) string {
	return fmt.Sprintf(LiveViewerCacheKey, streamID, userID)
}

// GetLiveStreamListKey 获取直播流列表缓存键
func GetLiveStreamListKey(listType string) string {
	return fmt.Sprintf(LiveStreamListKey, listType)
}

// GetLiveCategoryListKey 获取分类直播列表缓存键
func GetLiveCategoryListKey(categoryID uint32) string {
	return fmt.Sprintf(LiveCategoryListKey, categoryID)
}

// GetLiveStatsCacheKey 获取直播统计缓存键
func GetLiveStatsCacheKey(streamID uint64) string {
	return fmt.Sprintf(LiveStatsCacheKey, streamID)
}

// GetLiveTrendCacheKey 获取直播趋势缓存键
func GetLiveTrendCacheKey(streamID uint64, period string) string {
	return fmt.Sprintf(LiveTrendCacheKey, streamID, period)
}

// GetLiveViewerStatsKey 获取观看者统计缓存键
func GetLiveViewerStatsKey(streamID uint64) string {
	return fmt.Sprintf(LiveViewerStatsKey, streamID)
}

// GetLiveGiftStatsKey 获取礼物统计缓存键
func GetLiveGiftStatsKey(streamID uint64) string {
	return fmt.Sprintf(LiveGiftStatsKey, streamID)
}

// GetLiveStreamLockKey 获取直播流操作锁键
func GetLiveStreamLockKey(streamID uint64) string {
	return fmt.Sprintf(LiveStreamLockKey, streamID)
}

// GetLiveRoomLockKey 获取直播间操作锁键
func GetLiveRoomLockKey(roomID uint64) string {
	return fmt.Sprintf(LiveRoomLockKey, roomID)
}

// GetLiveViewerLockKey 获取观看者操作锁键
func GetLiveViewerLockKey(streamID, userID uint64) string {
	return fmt.Sprintf(LiveViewerLockKey, streamID, userID)
}

// GetLiveCounterKey 获取直播计数器键
func GetLiveCounterKey(counterType string, streamID uint64) string {
	return fmt.Sprintf(LiveCounterKey, counterType, streamID)
}

// GetLiveRealTimeKey 获取实时直播数据键
func GetLiveRealTimeKey(streamID uint64) string {
	return fmt.Sprintf(LiveRealTimeKey, streamID)
}

// GetLiveViewerCountKey 获取实时观看人数键
func GetLiveViewerCountKey(streamID uint64) string {
	return fmt.Sprintf(LiveViewerCountKey, streamID)
}

// GetLiveLikeCountKey 获取实时点赞数键
func GetLiveLikeCountKey(streamID uint64) string {
	return fmt.Sprintf(LiveLikeCountKey, streamID)
}

// GetLiveGiftRankKey 获取实时礼物排行键
func GetLiveGiftRankKey(streamID uint64) string {
	return fmt.Sprintf(LiveGiftRankKey, streamID)
}

// GetLiveRecommendKey 获取直播推荐键
func GetLiveRecommendKey(userID uint64) string {
	return fmt.Sprintf(LiveRecommendKey, userID)
}

// GetLiveUserRecommendKey 获取用户直播推荐键
func GetLiveUserRecommendKey(userID uint64) string {
	return fmt.Sprintf(LiveUserRecommendKey, userID)
}

// ToJSON 转换为JSON字符串
func (c *LiveStreamCache) ToJSON() (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON 从JSON字符串解析
func (c *LiveStreamCache) FromJSON(data string) error {
	return json.Unmarshal([]byte(data), c)
}

// FromJSONBytes 从JSON字节数组解析
func (c *LiveStreamCache) FromJSONBytes(data []byte) error {
	return json.Unmarshal(data, c)
}

// SetCache 设置缓存
func SetCache(ctx context.Context, redisClient *redis.Client, key string, data interface{}, expiration time.Duration) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	return redisClient.Set(ctx, key, jsonData, expiration).Err()
}

// GetCache 获取缓存
func GetCache(ctx context.Context, redisClient *redis.Client, key string, dest interface{}) error {
	data, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("cache miss")
		}
		return fmt.Errorf("failed to get cache: %w", err)
	}

	return json.Unmarshal([]byte(data), dest)
}

// DeleteCache 删除缓存
func DeleteCache(ctx context.Context, redisClient *redis.Client, key string) error {
	return redisClient.Del(ctx, key).Err()
}

// GetLiveViewerCountCacheKey 获取直播观看人数缓存键
func GetLiveViewerCountCacheKey(streamID uint64) string {
	return GetLiveViewerCountKey(streamID)
}
