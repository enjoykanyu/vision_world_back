package model

import (
	"time"

	"gorm.io/gorm"
)

// Video 视频信息表
type Video struct {
	ID            uint32         `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID        uint32         `gorm:"index;not null;comment:用户ID" json:"user_id"`
	Title         string         `gorm:"size:200;not null;comment:视频标题" json:"title"`
	Description   string         `gorm:"size:1000;comment:视频描述" json:"description"`
	CoverURL      string         `gorm:"size:500;not null;comment:封面URL" json:"cover_url"`
	VideoURL      string         `gorm:"size:500;not null;comment:视频URL" json:"video_url"`
	Duration      uint32         `gorm:"not null;comment:视频时长(秒)" json:"duration"`
	Resolution    string         `gorm:"size:20;comment:分辨率" json:"resolution"`
	Size          uint64         `gorm:"comment:文件大小(字节)" json:"size"`
	Tags          string         `gorm:"size:500;comment:标签，逗号分隔" json:"tags"`
	Location      string         `gorm:"size:100;comment:拍摄地点" json:"location"`
	MusicID       *uint32        `gorm:"index;comment:背景音乐ID" json:"music_id"`
	MusicTitle    string         `gorm:"size:200;comment:音乐标题" json:"music_title"`
	MusicURL      string         `gorm:"size:500;comment:音乐URL" json:"music_url"`
	Category      string         `gorm:"size:50;index;comment:视频分类" json:"category"`
	PlayCount     uint32         `gorm:"default:0;comment:播放次数" json:"play_count"`
	LikeCount     uint32         `gorm:"default:0;comment:点赞数" json:"like_count"`
	CommentCount  uint32         `gorm:"default:0;comment:评论数" json:"comment_count"`
	ShareCount    uint32         `gorm:"default:0;comment:分享数" json:"share_count"`
	FavoriteCount uint32         `gorm:"default:0;comment:收藏数" json:"favorite_count"`
	IsPublic      bool           `gorm:"default:true;comment:是否公开" json:"is_public"`
	Status        string         `gorm:"size:20;default:normal;comment:状态" json:"status"` // normal, deleted, banned, reviewing
	ExtraData     string         `gorm:"type:text;comment:扩展数据" json:"extra_data"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

func (Video) TableName() string {
	return "videos"
}

// VideoLike 视频点赞表
type VideoLike struct {
	ID        uint32    `gorm:"primaryKey;autoIncrement" json:"id"`
	VideoID   uint32    `gorm:"index:idx_video_user;not null;comment:视频ID" json:"video_id"`
	UserID    uint32    `gorm:"index:idx_video_user;index;not null;comment:用户ID" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (VideoLike) TableName() string {
	return "video_likes"
}

// VideoComment 视频评论表
type VideoComment struct {
	ID            uint32         `gorm:"primaryKey;autoIncrement" json:"id"`
	VideoID       uint32         `gorm:"index;not null;comment:视频ID" json:"video_id"`
	UserID        uint32         `gorm:"index;not null;comment:用户ID" json:"user_id"`
	ParentID      *uint32        `gorm:"index;comment:回复的评论ID" json:"parent_id"`
	ReplyToUserID *uint32        `gorm:"comment:回复的用户ID" json:"reply_to_user_id"`
	Content       string         `gorm:"size:1000;not null;comment:评论内容" json:"content"`
	LikeCount     uint32         `gorm:"default:0;comment:点赞数" json:"like_count"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

func (VideoComment) TableName() string {
	return "video_comments"
}

// VideoShare 视频分享表
type VideoShare struct {
	ID        uint32    `gorm:"primaryKey;autoIncrement" json:"id"`
	VideoID   uint32    `gorm:"index;not null;comment:视频ID" json:"video_id"`
	UserID    uint32    `gorm:"index;not null;comment:用户ID" json:"user_id"`
	ShareType string    `gorm:"size:20;comment:分享类型" json:"share_type"`
	ShareURL  string    `gorm:"size:500;comment:分享链接" json:"share_url"`
	CreatedAt time.Time `json:"created_at"`
}

func (VideoShare) TableName() string {
	return "video_shares"
}

// VideoFavorite 视频收藏表
type VideoFavorite struct {
	ID        uint32    `gorm:"primaryKey;autoIncrement" json:"id"`
	VideoID   uint32    `gorm:"index:idx_video_user_fav;not null;comment:视频ID" json:"video_id"`
	UserID    uint32    `gorm:"index:idx_video_user_fav;index;not null;comment:用户ID" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (VideoFavorite) TableName() string {
	return "video_favorites"
}

// VideoView 视频观看记录表
type VideoView struct {
	ID        uint32    `gorm:"primaryKey;autoIncrement" json:"id"`
	VideoID   uint32    `gorm:"index;not null;comment:视频ID" json:"video_id"`
	UserID    *uint32   `gorm:"index;comment:用户ID(未登录为空)" json:"user_id"`
	IP        string    `gorm:"size:45;comment:IP地址" json:"ip"`
	UserAgent string    `gorm:"size:500;comment:用户代理" json:"user_agent"`
	WatchTime uint32    `gorm:"comment:观看时长(秒)" json:"watch_time"`
	Progress  float32   `gorm:"comment:观看进度(0-1)" json:"progress"`
	CreatedAt time.Time `json:"created_at"`
}

func (VideoView) TableName() string {
	return "video_views"
}

// VideoCategory 视频分类表
type VideoCategory struct {
	ID          uint32    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:50;not null;unique;comment:分类名称" json:"name"`
	Description string    `gorm:"size:200;comment:分类描述" json:"description"`
	IconURL     string    `gorm:"size:500;comment:分类图标URL" json:"icon_url"`
	SortOrder   uint32    `gorm:"default:0;comment:排序顺序" json:"sort_order"`
	IsActive    bool      `gorm:"default:true;comment:是否启用" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (VideoCategory) TableName() string {
	return "video_categories"
}

// VideoTag 视频标签表
type VideoTag struct {
	ID        uint32    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"size:50;not null;unique;comment:标签名称" json:"name"`
	UseCount  uint32    `gorm:"default:0;comment:使用次数" json:"use_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (VideoTag) TableName() string {
	return "video_tags"
}

// VideoTagRelation 视频标签关联表
type VideoTagRelation struct {
	ID        uint32    `gorm:"primaryKey;autoIncrement" json:"id"`
	VideoID   uint32    `gorm:"index:idx_video_tag;not null;comment:视频ID" json:"video_id"`
	TagID     uint32    `gorm:"index:idx_video_tag;not null;comment:标签ID" json:"tag_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (VideoTagRelation) TableName() string {
	return "video_tag_relations"
}
