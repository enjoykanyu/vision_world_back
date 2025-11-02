package model

import (
	"time"
)

// RecommendationVideo 推荐服务使用的视频模型
type RecommendationVideo struct {
	VideoID     string    `json:"video_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Author      string    `json:"author"`
	Category    string    `json:"category"`
	Tags        string    `json:"tags"`
	Duration    int32     `json:"duration"`
	CoverURL    string    `json:"cover_url"`
	PlayURL     string    `json:"play_url"`
	ViewCount   int64     `json:"view_count"`
	LikeCount   int64     `json:"like_count"`
	Score       float64   `json:"score"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName 返回表名
func (RecommendationVideo) TableName() string {
	return "recommendation_videos"
}

// ToVideoModel 转换为Video模型
func (r *RecommendationVideo) ToVideoModel() *Video {
	return &Video{
		ID:          0, // 需要从数据库获取
		UserID:      0, // 需要从数据库获取
		Title:       r.Title,
		Description: r.Description,
		CoverURL:    r.CoverURL,
		VideoURL:    r.PlayURL,
		Duration:    uint32(r.Duration),
		Tags:        r.Tags,
		Category:    r.Category,
		PlayCount:   uint32(r.ViewCount),
		LikeCount:   uint32(r.LikeCount),
		IsPublic:    true,
		Status:      "normal",
	}
}

// FromVideoModel 从Video模型转换
func FromVideoModel(video *Video, author string) *RecommendationVideo {
	if video == nil {
		return nil
	}

	return &RecommendationVideo{
		VideoID:     string(rune(video.ID)), // 简化处理，实际应该使用更合适的ID
		Title:       video.Title,
		Description: video.Description,
		Author:      author,
		Category:    video.Category,
		Tags:        video.Tags,
		Duration:    int32(video.Duration),
		CoverURL:    video.CoverURL,
		PlayURL:     video.VideoURL,
		ViewCount:   int64(video.PlayCount),
		LikeCount:   int64(video.LikeCount),
		Score:       0, // 需要计算
		CreatedAt:   video.CreatedAt,
		UpdatedAt:   video.UpdatedAt,
	}
}
