package model

import (
	"encoding/json"
	"fmt"
	"time"
)

// RedisKey Redis键前缀定义
const (
	// 用户缓存相关
	UserInfoCacheKey    = "user:info:%d"             // 用户信息缓存
	UserStatsCacheKey   = "user:stats:%d"            // 用户统计缓存
	UserFollowCacheKey  = "user:follow:%d:%d"        // 用户关注列表缓存
	UserFanCacheKey     = "user:fan:%d:%d"           // 用户粉丝列表缓存
	UserFollowStatusKey = "user:follow:status:%d:%d" // 关注状态缓存

	// 统计相关
	UserTrendCacheKey = "user:trend:%d:%s" // 用户趋势缓存
	HotUsersCacheKey  = "users:hot:%s"     // 热门用户缓存
	NewUsersCacheKey  = "users:new:%s"     // 新用户缓存

	// 分布式锁相关
	UserFollowLockKey = "lock:user:follow:%d:%d" // 关注操作锁
	UserStatsLockKey  = "lock:user:stats:%d"     // 统计更新锁

	// 计数器相关
	UserCounterKey   = "counter:user:%s:%d" // 用户计数器
	GlobalCounterKey = "counter:global:%s"  // 全局计数器
)

// CacheTTL 缓存过期时间定义
const (
	UserInfoTTL     = 30 * time.Minute // 用户信息缓存30分钟
	UserStatsTTL    = 10 * time.Minute // 用户统计缓存10分钟
	UserFollowTTL   = 15 * time.Minute // 关注列表缓存15分钟
	UserTrendTTL    = 1 * time.Hour    // 趋势缓存1小时
	HotUsersTTL     = 5 * time.Minute  // 热门用户缓存5分钟
	FollowStatusTTL = 5 * time.Minute  // 关注状态缓存5分钟
)

// UserCache 用户缓存数据结构
type UserCache struct {
	UserID          uint64    `json:"user_id"`
	Username        string    `json:"username"`
	Nickname        string    `json:"nickname"`
	AvatarURL       string    `json:"avatar_url"`
	BackgroundImage string    `json:"background_image"`
	Signature       string    `json:"signature"`
	IsVerified      bool      `json:"is_verified"`
	UserType        string    `json:"user_type"`
	Status          uint8     `json:"status"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// UserStatsCache 用户统计缓存
type UserStatsCache struct {
	UserID         uint64    `json:"user_id"`
	FollowingCount uint32    `json:"following_count"`
	FollowersCount uint32    `json:"followers_count"`
	TotalFavorited uint64    `json:"total_favorited"`
	WorkCount      uint32    `json:"work_count"`
	FavoriteCount  uint32    `json:"favorite_count"`
	ViewCount      uint64    `json:"view_count"`
	LikeCount      uint32    `json:"like_count"`
	ShareCount     uint32    `json:"share_count"`
	CommentCount   uint32    `json:"comment_count"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// FollowListCache 关注列表缓存
type FollowListCache struct {
	UserID    uint64      `json:"user_id"`
	ListType  string      `json:"list_type"` // following/followers
	Users     []UserCache `json:"users"`
	Total     int64       `json:"total"`
	Page      int         `json:"page"`
	PageSize  int         `json:"page_size"`
	HasMore   bool        `json:"has_more"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// FollowStatusCache 关注状态缓存
type FollowStatusCache struct {
	ActorID    uint64    `json:"actor_id"`
	TargetID   uint64    `json:"target_id"`
	IsFollow   bool      `json:"is_follow"`
	FollowTime time.Time `json:"follow_time,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// UserTrendCache 用户趋势缓存
type UserTrendCache struct {
	UserID      uint64        `json:"user_id"`
	Period      string        `json:"period"` // day/week/month
	GrowthTrend []GrowthPoint `json:"growth_trend"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// GrowthPoint 增长数据点
type GrowthPoint struct {
	Date      string `json:"date"`
	Followers uint32 `json:"followers"`
	Following uint32 `json:"following"`
	Favorites uint32 `json:"favorites"`
	Works     uint32 `json:"works"`
}

// HotUserCache 热门用户缓存
type HotUserCache struct {
	Users     []UserCache `json:"users"`
	Total     int64       `json:"total"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// CacheHelper 缓存辅助函数

// GetUserInfoCacheKey 获取用户信息缓存键
func GetUserInfoCacheKey(userID uint64) string {
	return fmt.Sprintf(UserInfoCacheKey, userID)
}

// GetUserStatsCacheKey 获取用户统计缓存键
func GetUserStatsCacheKey(userID uint64) string {
	return fmt.Sprintf(UserStatsCacheKey, userID)
}

// GetUserFollowCacheKey 获取用户关注列表缓存键
func GetUserFollowCacheKey(userID uint64, page int) string {
	return fmt.Sprintf(UserFollowCacheKey, userID, page)
}

// GetUserFanCacheKey 获取用户粉丝列表缓存键
func GetUserFanCacheKey(userID uint64, page int) string {
	return fmt.Sprintf(UserFanCacheKey, userID, page)
}

// GetFollowStatusCacheKey 获取关注状态缓存键
func GetFollowStatusCacheKey(actorID, targetID uint64) string {
	return fmt.Sprintf(UserFollowStatusKey, actorID, targetID)
}

// GetUserTrendCacheKey 获取用户趋势缓存键
func GetUserTrendCacheKey(userID uint64, period string) string {
	return fmt.Sprintf(UserTrendCacheKey, userID, period)
}

// GetHotUsersCacheKey 获取热门用户缓存键
func GetHotUsersCacheKey(category string) string {
	return fmt.Sprintf(HotUsersCacheKey, category)
}

// GetUserFollowLockKey 获取用户关注操作锁键
func GetUserFollowLockKey(actorID, targetID uint64) string {
	return fmt.Sprintf(UserFollowLockKey, actorID, targetID)
}

// GetUserStatsLockKey 获取用户统计更新锁键
func GetUserStatsLockKey(userID uint64) string {
	return fmt.Sprintf(UserStatsLockKey, userID)
}

// GetUserCounterKey 获取用户计数器键
func GetUserCounterKey(counterType string, userID uint64) string {
	return fmt.Sprintf(UserCounterKey, counterType, userID)
}

// GetGlobalCounterKey 获取全局计数器键
func GetGlobalCounterKey(counterType string) string {
	return fmt.Sprintf(GlobalCounterKey, counterType)
}

// ToJSON 转换为JSON字符串
func (c *UserCache) ToJSON() (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON 从JSON字符串解析
func (c *UserCache) FromJSON(data string) error {
	return json.Unmarshal([]byte(data), c)
}

// ToJSON 转换为JSON字符串
func (s *UserStatsCache) ToJSON() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON 从JSON字符串解析
func (s *UserStatsCache) FromJSON(data string) error {
	return json.Unmarshal([]byte(data), s)
}

// IsExpired 检查缓存是否过期
func (c *UserCache) IsExpired(ttl time.Duration) bool {
	return time.Since(c.UpdatedAt) > ttl
}

// IsExpired 检查统计缓存是否过期
func (s *UserStatsCache) IsExpired(ttl time.Duration) bool {
	return time.Since(s.UpdatedAt) > ttl
}
