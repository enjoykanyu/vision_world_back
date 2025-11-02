package model

import (
	"time"
)

// UserFollow 用户关注关系表
type UserFollow struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement;comment:关注关系ID"`
	FollowerID  uint64     `gorm:"not null;index:idx_follower;comment:关注者ID"`
	FollowingID uint64     `gorm:"not null;index:idx_following;comment:被关注者ID"`
	CreatedAt   time.Time  `gorm:"comment:创建时间"`
	DeletedAt   *time.Time `gorm:"index;comment:删除时间"`
}

// TableName 设置表名
func (UserFollow) TableName() string {
	return "user_follows"
}

// FollowStats 关注统计信息
type FollowStats struct {
	FollowingCount uint32 `json:"following_count"`
	FollowersCount uint32 `json:"followers_count"`
}

// UserWithFollowStatus 用户信息和关注状态
type UserWithFollowStatus struct {
	User
	IsFollow bool `json:"is_follow"`
}

// IsValidFollow 检查关注关系是否有效
func (uf *UserFollow) IsValidFollow() bool {
	return uf.FollowerID != 0 && uf.FollowingID != 0 && uf.FollowerID != uf.FollowingID
}

// FollowListRequest 关注列表请求参数
type FollowListRequest struct {
	UserID     uint32
	ActorID    uint32
	Page       int
	PageSize   int
	TimeCursor int64 // 时间游标，用于分页
}

// FollowListResponse 关注列表响应
type FollowListResponse struct {
	Users      []UserWithFollowStatus
	Total      int64
	NextCursor int64
	HasMore    bool
}

// FollowActionRequest 关注操作请求
type FollowActionRequest struct {
	ActorID  uint32 `json:"actor_id" binding:"required"`
	TargetID uint32 `json:"target_id" binding:"required"`
	Action   string `json:"action" binding:"required,oneof=follow unfollow"`
}

// FollowActionResponse 关注操作响应
type FollowActionResponse struct {
	StatusCode int32  `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
	IsFollow   bool   `json:"is_follow"`
}
