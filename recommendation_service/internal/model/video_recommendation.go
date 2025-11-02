package model

import (
	"time"
)

// Video 视频模型
type Video struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	VideoID      string    `gorm:"uniqueIndex;not null" json:"video_id"`
	Title        string    `gorm:"not null" json:"title"`
	Description  string    `json:"description"`
	CoverURL     string    `json:"cover_url"`
	VideoURL     string    `json:"video_url"`
	Duration     uint32    `json:"duration"`
	ViewCount    uint32    `json:"view_count"`
	LikeCount    uint32    `json:"like_count"`
	CommentCount uint32    `json:"comment_count"`
	ShareCount   uint32    `json:"share_count"`
	AuthorID     string    `json:"author_id"`
	AuthorName   string    `json:"author_name"`
	AuthorAvatar string    `json:"author_avatar"`
	Tags         string    `json:"tags"` // 逗号分隔的标签
	Category     string    `json:"category"`
	Score        float64   `json:"score"` // 推荐分数
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 指定表名
func (Video) TableName() string {
	return "videos"
}

// UserPreference 用户偏好模型
type UserPreference struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID          string    `gorm:"index;not null" json:"user_id"`
	Categories      string    `json:"categories"`       // 逗号分隔的偏好分类
	Tags            string    `json:"tags"`             // 逗号分隔的偏好标签
	CategoryWeights string    `json:"category_weights"` // JSON格式的分类权重
	TagWeights      string    `json:"tag_weights"`      // JSON格式的标签权重
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TableName 指定表名
func (UserPreference) TableName() string {
	return "user_preferences"
}

// UserAction 用户行为模型
type UserAction struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID        string    `gorm:"index;not null" json:"user_id"`
	VideoID       string    `gorm:"index;not null" json:"video_id"`
	ActionType    string    `gorm:"not null" json:"action_type"` // view, like, share, comment, complete
	Duration      float64   `json:"duration"`                    // 观看时长（秒）
	TotalDuration float64   `json:"total_duration"`              // 视频总时长（秒）
	Timestamp     int64     `json:"timestamp"`                   // 时间戳
	CreatedAt     time.Time `json:"created_at"`
}

// TableName 指定表名
func (UserAction) TableName() string {
	return "user_actions"
}
