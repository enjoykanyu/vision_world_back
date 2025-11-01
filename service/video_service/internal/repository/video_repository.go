package repository

import (
	"context"
	"fmt"

	"github.com/vision_world/video_service/internal/config"
	"github.com/vision_world/video_service/internal/model"
	"github.com/vision_world/video_service/pkg/database"
	"github.com/vision_world/video_service/pkg/logger"
	"gorm.io/gorm"
)

// VideoRepository 视频数据访问层
type VideoRepository struct {
	config *config.Config
	db     *model.DB
}

// NewVideoRepository 创建视频数据仓库
func NewVideoRepository(cfg *config.Config) (*VideoRepository, error) {
	// 初始化数据库连接
	if err := database.InitDB(&cfg.Database); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	db := database.GetDB()
	videoDB := model.NewDB(db)

	// 初始化数据表
	if err := videoDB.InitTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	logger.Info("Video repository initialized successfully")

	return &VideoRepository{
		config: cfg,
		db:     videoDB,
	}, nil
}

// Close 关闭仓库
func (r *VideoRepository) Close() error {
	return database.CloseDB()
}

// GetDB 获取数据库实例
func (r *VideoRepository) GetDB() *model.DB {
	return r.db
}

// GetVideoByID 根据ID获取视频
func (r *VideoRepository) GetVideoByID(ctx context.Context, videoID string) (*model.RecommendationVideo, error) {
	var video model.Video
	err := r.db.Where("id = ? AND status = ?", videoID, "normal").First(&video).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	// 获取作者信息
	var author string
	// 简化处理，实际应该查询用户表获取作者信息
	author = "Unknown"

	return model.FromVideoModel(&video, author), nil
}

// GetVideosByIDs 根据ID列表获取视频
func (r *VideoRepository) GetVideosByIDs(ctx context.Context, videoIDs []string) ([]*model.RecommendationVideo, error) {
	if len(videoIDs) == 0 {
		return []*model.RecommendationVideo{}, nil
	}

	var videos []model.Video
	err := r.db.Where("id IN ? AND status = ?", videoIDs, "normal").Find(&videos).Error
	if err != nil {
		return nil, err
	}

	result := make([]*model.RecommendationVideo, len(videos))
	for i, video := range videos {
		// 获取作者信息
		var author string
		// 简化处理，实际应该查询用户表获取作者信息
		author = "Unknown"

		result[i] = model.FromVideoModel(&video, author)
	}

	return result, nil
}

// GetHotVideos 获取热门视频
func (r *VideoRepository) GetHotVideos(ctx context.Context, page, pageSize int, category string) ([]*model.RecommendationVideo, bool, error) {
	offset := (page - 1) * pageSize

	query := r.db.Where("is_public = ? AND status = ?", true, "normal")

	if category != "" {
		query = query.Where("category = ?", category)
	}

	var videos []model.Video
	err := query.Order("play_count DESC, like_count DESC").
		Offset(offset).
		Limit(pageSize + 1). // 多获取一条，用于判断是否有更多数据
		Find(&videos).Error

	if err != nil {
		return nil, false, err
	}

	hasMore := len(videos) > pageSize
	if hasMore {
		videos = videos[:pageSize] // 去掉多获取的那一条
	}

	result := make([]*model.RecommendationVideo, len(videos))
	for i, video := range videos {
		// 获取作者信息
		var author string
		// 简化处理，实际应该查询用户表获取作者信息
		author = "Unknown"

		result[i] = model.FromVideoModel(&video, author)
	}

	return result, hasMore, nil
}

// GetCategoryVideos 获取分类视频
func (r *VideoRepository) GetCategoryVideos(ctx context.Context, category string, page, pageSize int) ([]*model.RecommendationVideo, bool, error) {
	offset := (page - 1) * pageSize

	var videos []model.Video
	err := r.db.Where("is_public = ? AND status = ? AND category = ?", true, "normal", category).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize + 1). // 多获取一条，用于判断是否有更多数据
		Find(&videos).Error

	if err != nil {
		return nil, false, err
	}

	hasMore := len(videos) > pageSize
	if hasMore {
		videos = videos[:pageSize] // 去掉多获取的那一条
	}

	result := make([]*model.RecommendationVideo, len(videos))
	for i, video := range videos {
		// 获取作者信息
		var author string
		// 简化处理，实际应该查询用户表获取作者信息
		author = "Unknown"

		result[i] = model.FromVideoModel(&video, author)
	}

	return result, hasMore, nil
}

// SearchVideos 搜索视频
func (r *VideoRepository) SearchVideos(ctx context.Context, keyword string, page, pageSize int, category string) ([]*model.RecommendationVideo, bool, error) {
	offset := (page - 1) * pageSize

	query := r.db.Where("is_public = ? AND status = ? AND (title LIKE ? OR description LIKE ?)",
		true, "normal", "%"+keyword+"%", "%"+keyword+"%")

	if category != "" {
		query = query.Where("category = ?", category)
	}

	var videos []model.Video
	err := query.Order("created_at DESC").
		Offset(offset).
		Limit(pageSize + 1). // 多获取一条，用于判断是否有更多数据
		Find(&videos).Error

	if err != nil {
		return nil, false, err
	}

	hasMore := len(videos) > pageSize
	if hasMore {
		videos = videos[:pageSize] // 去掉多获取的那一条
	}

	result := make([]*model.RecommendationVideo, len(videos))
	for i, video := range videos {
		// 获取作者信息
		var author string
		// 简化处理，实际应该查询用户表获取作者信息
		author = "Unknown"

		result[i] = model.FromVideoModel(&video, author)
	}

	return result, hasMore, nil
}

// GetVideosByAuthor 根据作者ID获取视频
func (r *VideoRepository) GetVideosByAuthor(ctx context.Context, authorID string, page, pageSize int) ([]*model.RecommendationVideo, bool, error) {
	offset := (page - 1) * pageSize

	var videos []model.Video
	err := r.db.Where("author_id = ? AND status = ?", authorID, "normal").
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize + 1). // 多获取一条，用于判断是否有更多数据
		Find(&videos).Error

	if err != nil {
		return nil, false, err
	}

	hasMore := len(videos) > pageSize
	if hasMore {
		videos = videos[:pageSize] // 去掉多获取的那一条
	}

	result := make([]*model.RecommendationVideo, len(videos))
	for i, video := range videos {
		// 获取作者信息
		var author string
		// 简化处理，实际应该查询用户表获取作者信息
		author = "Unknown"

		result[i] = model.FromVideoModel(&video, author)
	}

	return result, hasMore, nil
}

// CreateVideo 创建视频
func (r *VideoRepository) CreateVideo(ctx context.Context, video *model.Video) error {
	return r.db.Create(video).Error
}

// UpdateVideo 更新视频
func (r *VideoRepository) UpdateVideo(ctx context.Context, video *model.Video) error {
	return r.db.Save(video).Error
}

// DeleteVideo 删除视频
func (r *VideoRepository) DeleteVideo(ctx context.Context, videoID string) error {
	return r.db.Where("id = ?", videoID).Update("status", "deleted").Error
}

// IncrementPlayCount 增加播放次数
func (r *VideoRepository) IncrementPlayCount(ctx context.Context, videoID string) error {
	return r.db.Model(&model.Video{}).Where("id = ?", videoID).
		UpdateColumn("play_count", gorm.Expr("play_count + ?", 1)).Error
}

// IncrementLikeCount 增加点赞次数
func (r *VideoRepository) IncrementLikeCount(ctx context.Context, videoID string) error {
	return r.db.Model(&model.Video{}).Where("id = ?", videoID).
		UpdateColumn("like_count", gorm.Expr("like_count + ?", 1)).Error
}

// DecrementLikeCount 减少点赞次数
func (r *VideoRepository) DecrementLikeCount(ctx context.Context, videoID string) error {
	return r.db.Model(&model.Video{}).Where("id = ?", videoID).
		UpdateColumn("like_count", gorm.Expr("like_count - ?", 1)).Error
}

// GetVideoLike 获取视频点赞记录
func (r *VideoRepository) GetVideoLike(ctx context.Context, videoID, userID string) (*model.VideoLike, error) {
	var like model.VideoLike
	err := r.db.Where("video_id = ? AND user_id = ?", videoID, userID).First(&like).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &like, nil
}

// CreateVideoLike 创建视频点赞记录
func (r *VideoRepository) CreateVideoLike(ctx context.Context, like *model.VideoLike) error {
	return r.db.Create(like).Error
}

// DeleteVideoLike 删除视频点赞记录
func (r *VideoRepository) DeleteVideoLike(ctx context.Context, videoID, userID string) error {
	return r.db.Where("video_id = ? AND user_id = ?", videoID, userID).Delete(&model.VideoLike{}).Error
}

// GetVideoComments 获取视频评论
func (r *VideoRepository) GetVideoComments(ctx context.Context, videoID string, page, pageSize int) ([]*model.VideoComment, bool, error) {
	offset := (page - 1) * pageSize

	var comments []model.VideoComment
	err := r.db.Where("video_id = ? AND status = ?", videoID, "normal").
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize + 1). // 多获取一条，用于判断是否有更多数据
		Find(&comments).Error

	if err != nil {
		return nil, false, err
	}

	hasMore := len(comments) > pageSize
	if hasMore {
		comments = comments[:pageSize] // 去掉多获取的那一条
	}

	// 转换为指针切片
	result := make([]*model.VideoComment, len(comments))
	for i := range comments {
		result[i] = &comments[i]
	}

	return result, hasMore, nil
}

// CreateVideoComment 创建视频评论
func (r *VideoRepository) CreateVideoComment(ctx context.Context, comment *model.VideoComment) error {
	return r.db.Create(comment).Error
}

// DeleteVideoComment 删除视频评论
func (r *VideoRepository) DeleteVideoComment(ctx context.Context, commentID string) error {
	return r.db.Where("id = ?", commentID).Update("status", "deleted").Error
}

// GetVideoShares 获取视频分享记录
func (r *VideoRepository) GetVideoShares(ctx context.Context, videoID string, page, pageSize int) ([]*model.VideoShare, bool, error) {
	offset := (page - 1) * pageSize

	var shares []model.VideoShare
	err := r.db.Where("video_id = ?", videoID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize + 1). // 多获取一条，用于判断是否有更多数据
		Find(&shares).Error

	if err != nil {
		return nil, false, err
	}

	hasMore := len(shares) > pageSize
	if hasMore {
		shares = shares[:pageSize] // 去掉多获取的那一条
	}

	// 转换为指针切片
	result := make([]*model.VideoShare, len(shares))
	for i := range shares {
		result[i] = &shares[i]
	}

	return result, hasMore, nil
}

// CreateVideoShare 创建视频分享记录
func (r *VideoRepository) CreateVideoShare(ctx context.Context, share *model.VideoShare) error {
	return r.db.Create(share).Error
}

// GetVideoFavorites 获取视频收藏记录
func (r *VideoRepository) GetVideoFavorites(ctx context.Context, videoID string, page, pageSize int) ([]*model.VideoFavorite, bool, error) {
	offset := (page - 1) * pageSize

	var favorites []model.VideoFavorite
	err := r.db.Where("video_id = ?", videoID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize + 1). // 多获取一条，用于判断是否有更多数据
		Find(&favorites).Error

	if err != nil {
		return nil, false, err
	}

	hasMore := len(favorites) > pageSize
	if hasMore {
		favorites = favorites[:pageSize] // 去掉多获取的那一条
	}

	// 转换为指针切片
	result := make([]*model.VideoFavorite, len(favorites))
	for i := range favorites {
		result[i] = &favorites[i]
	}

	return result, hasMore, nil
}

// CreateVideoFavorite 创建视频收藏记录
func (r *VideoRepository) CreateVideoFavorite(ctx context.Context, favorite *model.VideoFavorite) error {
	return r.db.Create(favorite).Error
}

// DeleteVideoFavorite 删除视频收藏记录
func (r *VideoRepository) DeleteVideoFavorite(ctx context.Context, videoID, userID string) error {
	return r.db.Where("video_id = ? AND user_id = ?", videoID, userID).Delete(&model.VideoFavorite{}).Error
}

// GetVideoViews 获取视频观看记录
func (r *VideoRepository) GetVideoViews(ctx context.Context, videoID string, page, pageSize int) ([]*model.VideoView, bool, error) {
	offset := (page - 1) * pageSize

	var views []model.VideoView
	err := r.db.Where("video_id = ?", videoID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize + 1). // 多获取一条，用于判断是否有更多数据
		Find(&views).Error

	if err != nil {
		return nil, false, err
	}

	hasMore := len(views) > pageSize
	if hasMore {
		views = views[:pageSize] // 去掉多获取的那一条
	}

	// 转换为指针切片
	result := make([]*model.VideoView, len(views))
	for i := range views {
		result[i] = &views[i]
	}

	return result, hasMore, nil
}

// CreateVideoView 创建视频观看记录
func (r *VideoRepository) CreateVideoView(ctx context.Context, view *model.VideoView) error {
	return r.db.Create(view).Error
}

// GetVideoCategories 获取视频分类列表
func (r *VideoRepository) GetVideoCategories(ctx context.Context) ([]*model.VideoCategory, error) {
	var categories []*model.VideoCategory
	err := r.db.Find(&categories).Error
	return categories, err
}

// GetVideoTags 获取视频标签列表
func (r *VideoRepository) GetVideoTags(ctx context.Context) ([]*model.VideoTag, error) {
	var tags []*model.VideoTag
	err := r.db.Find(&tags).Error
	return tags, err
}

// GetVideoTagsByVideoID 获取视频的标签
func (r *VideoRepository) GetVideoTagsByVideoID(ctx context.Context, videoID string) ([]*model.VideoTag, error) {
	var tags []*model.VideoTag
	err := r.db.Joins("JOIN video_tag_relations ON video_tags.id = video_tag_relations.tag_id").
		Where("video_tag_relations.video_id = ?", videoID).
		Find(&tags).Error
	return tags, err
}

// CreateVideoTag 创建视频标签
func (r *VideoRepository) CreateVideoTag(ctx context.Context, tag *model.VideoTag) error {
	return r.db.Create(tag).Error
}

// CreateVideoTagRelation 创建视频标签关联
func (r *VideoRepository) CreateVideoTagRelation(ctx context.Context, relation *model.VideoTagRelation) error {
	return r.db.Create(relation).Error
}

// GetRecommendVideos 获取推荐视频
func (r *VideoRepository) GetRecommendVideos(ctx context.Context, userID string, page, pageSize int) ([]*model.RecommendationVideo, bool, error) {
	offset := (page - 1) * pageSize

	// 简化推荐算法，实际应该基于用户兴趣、历史行为等进行推荐
	var videos []model.Video
	err := r.db.Where("is_public = ? AND status = ?", true, "normal").
		Order("play_count DESC, like_count DESC").
		Offset(offset).
		Limit(pageSize + 1). // 多获取一条，用于判断是否有更多数据
		Find(&videos).Error

	if err != nil {
		return nil, false, err
	}

	hasMore := len(videos) > pageSize
	if hasMore {
		videos = videos[:pageSize] // 去掉多获取的那一条
	}

	result := make([]*model.RecommendationVideo, len(videos))
	for i, video := range videos {
		// 获取作者信息
		var author string
		// 简化处理，实际应该查询用户表获取作者信息
		author = "Unknown"

		result[i] = model.FromVideoModel(&video, author)
	}

	return result, hasMore, nil
}
