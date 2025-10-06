package model

import (
	"time"
)

// UserStats 用户统计表（可选，用于详细统计和审计）
type UserStats struct {
	ID     uint64 `gorm:"primaryKey;autoIncrement;comment:统计ID"`
	UserID uint64 `gorm:"uniqueIndex;not null;comment:用户ID"`

	// 关注相关统计
	FollowingCount    uint32 `gorm:"default:0;comment:关注数量"`
	FollowersCount    uint32 `gorm:"default:0;comment:粉丝数量"`
	NewFollowersToday uint32 `gorm:"default:0;comment:今日新增粉丝"`
	NewFollowingToday uint32 `gorm:"default:0;comment:今日新增关注"`

	// 内容相关统计
	TotalFavorited uint64 `gorm:"default:0;comment:获赞总数"`
	WorkCount      uint32 `gorm:"default:0;comment:作品数量"`
	FavoriteCount  uint32 `gorm:"default:0;comment:点赞数量"`
	ShareCount     uint32 `gorm:"default:0;comment:分享数量"`
	CommentCount   uint32 `gorm:"default:0;comment:评论数量"`

	// 互动相关统计
	ViewCount uint64 `gorm:"default:0;comment:被观看总数"`
	LikeCount uint32 `gorm:"default:0;comment:被点赞总数"`

	// 每日统计（可按需扩展）
	DailyViews    uint32 `gorm:"default:0;comment:今日观看数"`
	DailyLikes    uint32 `gorm:"default:0;comment:今日点赞数"`
	DailyShares   uint32 `gorm:"default:0;comment:今日分享数"`
	DailyComments uint32 `gorm:"default:0;comment:今日评论数"`

	// 时间戳
	LastStatsReset *time.Time `gorm:"comment:上次统计重置时间"`
	CreatedAt      time.Time  `gorm:"comment:创建时间"`
	UpdatedAt      time.Time  `gorm:"comment:更新时间"`
}

// TableName 设置表名
func (UserStats) TableName() string {
	return "user_stats"
}

// UserStatsDaily 用户每日统计（用于趋势分析）
type UserStatsDaily struct {
	ID     uint64    `gorm:"primaryKey;autoIncrement;comment:统计ID"`
	UserID uint64    `gorm:"index:idx_user_date;not null;comment:用户ID"`
	Date   time.Time `gorm:"index:idx_user_date;type:date;not null;comment:统计日期"`

	// 关注相关
	NewFollowers  uint32 `gorm:"default:0;comment:新增粉丝"`
	NewFollowing  uint32 `gorm:"default:0;comment:新增关注"`
	LostFollowers uint32 `gorm:"default:0;comment:流失粉丝"`
	LostFollowing uint32 `gorm:"default:0;comment:流失关注"`

	// 内容相关
	NewWorks      uint32 `gorm:"default:0;comment:新增作品"`
	DeletedWorks  uint32 `gorm:"default:0;comment:删除作品"`
	NewFavorites  uint32 `gorm:"default:0;comment:新增获赞"`
	LostFavorites uint32 `gorm:"default:0;comment:取消获赞"`

	// 互动相关
	Views    uint32 `gorm:"default:0;comment:观看数"`
	Likes    uint32 `gorm:"default:0;comment:点赞数"`
	Shares   uint32 `gorm:"default:0;comment:分享数"`
	Comments uint32 `gorm:"default:0;comment:评论数"`

	// 时间戳
	CreatedAt time.Time `gorm:"comment:创建时间"`
	UpdatedAt time.Time `gorm:"comment:更新时间"`
}

// TableName 设置表名
func (UserStatsDaily) TableName() string {
	return "user_stats_daily"
}

// StatsSummary 统计汇总响应
type StatsSummary struct {
	FollowingCount uint32 `json:"following_count"`
	FollowersCount uint32 `json:"followers_count"`
	TotalFavorited uint64 `json:"total_favorited"`
	WorkCount      uint32 `json:"work_count"`
	FavoriteCount  uint32 `json:"favorite_count"`
	ViewCount      uint64 `json:"view_count"`
	LikeCount      uint32 `json:"like_count"`
	ShareCount     uint32 `json:"share_count"`
	CommentCount   uint32 `json:"comment_count"`
}

// GrowthTrend 增长趋势
type GrowthTrend struct {
	Date         string `json:"date"`
	NewFollowers uint32 `json:"new_followers"`
	NewFollowing uint32 `json:"new_following"`
	NewWorks     uint32 `json:"new_works"`
	NewFavorites uint32 `json:"new_favorites"`
	Views        uint32 `json:"views"`
	Likes        uint32 `json:"likes"`
}

// StatsComparison 统计对比
type StatsComparison struct {
	Current    StatsSummary       `json:"current"`
	LastWeek   StatsSummary       `json:"last_week"`
	LastMonth  StatsSummary       `json:"last_month"`
	GrowthRate map[string]float64 `json:"growth_rate"` // 增长率
}

// ResetDailyStats 重置每日统计
func (stats *UserStats) ResetDailyStats() {
	stats.DailyViews = 0
	stats.DailyLikes = 0
	stats.DailyShares = 0
	stats.DailyComments = 0
	stats.NewFollowersToday = 0
	stats.NewFollowingToday = 0

	now := time.Now()
	stats.LastStatsReset = &now
}

// UpdateDailyStats 更新每日统计
func (daily *UserStatsDaily) UpdateDailyStats(field string, delta int32) {
	switch field {
	case "new_followers":
		if delta > 0 {
			daily.NewFollowers += uint32(delta)
		}
	case "new_following":
		if delta > 0 {
			daily.NewFollowing += uint32(delta)
		}
	case "lost_followers":
		if delta > 0 {
			daily.LostFollowers += uint32(delta)
		}
	case "lost_following":
		if delta > 0 {
			daily.LostFollowing += uint32(delta)
		}
	case "new_works":
		if delta > 0 {
			daily.NewWorks += uint32(delta)
		}
	case "deleted_works":
		if delta > 0 {
			daily.DeletedWorks += uint32(delta)
		}
	case "new_favorites":
		if delta > 0 {
			daily.NewFavorites += uint32(delta)
		}
	case "lost_favorites":
		if delta > 0 {
			daily.LostFavorites += uint32(delta)
		}
	case "views":
		if delta > 0 {
			daily.Views += uint32(delta)
		}
	case "likes":
		if delta > 0 {
			daily.Likes += uint32(delta)
		}
	case "shares":
		if delta > 0 {
			daily.Shares += uint32(delta)
		}
	case "comments":
		if delta > 0 {
			daily.Comments += uint32(delta)
		}
	}
}
