package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	err := r.db.GetDB().Where("id = ?", videoID).First(&video).Error
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
	err := r.db.GetDB().Where("id IN ?", videoIDs).Find(&videos).Error
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

	query := r.db.GetDB().Where("is_public = ? AND status = ?", true, "normal")

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
	err := r.db.GetDB().Where("is_public = ? AND status = ? AND category = ?", true, "normal", category).
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

	query := r.db.GetDB().Where("is_public = ? AND status = ? AND (title LIKE ? OR description LIKE ?)",
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

// UpdateVideoViewCount 更新视频播放量
func (r *VideoRepository) UpdateVideoViewCount(ctx context.Context, videoID string, increment int64) (int64, error) {
	result := r.db.GetDB().Model(&model.Video{}).
		Where("id = ?", videoID).
		UpdateColumn("play_count", gorm.Expr("play_count + ?", increment))

	if result.Error != nil {
		return 0, result.Error
	}

	// 获取更新后的播放量
	var video model.Video
	err := r.db.GetDB().Select("play_count").Where("id = ?", videoID).First(&video).Error
	if err != nil {
		return 0, err
	}

	return int64(video.PlayCount), nil
}

// UpdateVideoLikeCount 更新视频点赞数
func (r *VideoRepository) UpdateVideoLikeCount(ctx context.Context, videoID string, increment int32) (int64, error) {
	result := r.db.GetDB().Model(&model.Video{}).
		Where("id = ?", videoID).
		UpdateColumn("like_count", gorm.Expr("like_count + ?", increment))

	if result.Error != nil {
		return 0, result.Error
	}

	// 获取更新后的点赞数
	var video model.Video
	err := r.db.GetDB().Select("like_count").Where("id = ?", videoID).First(&video).Error
	if err != nil {
		return 0, err
	}

	return int64(video.LikeCount), nil
}

// GetVideosByAuthor 根据作者获取视频
func (r *VideoRepository) GetVideosByAuthor(ctx context.Context, author string, page, pageSize int) ([]*model.RecommendationVideo, bool, error) {
	offset := (page - 1) * pageSize

	// 简化处理，实际应该通过用户ID查询
	var videos []model.Video
	err := r.db.GetDB().Where("is_public = ? AND status = ?", true, "normal").
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
		result[i] = model.FromVideoModel(&video, author)
	}

	return result, hasMore, nil
}

// GetVideosByTags 根据标签获取视频
func (r *VideoRepository) GetVideosByTags(ctx context.Context, tags []string, page, pageSize int) ([]*model.RecommendationVideo, bool, error) {
	if len(tags) == 0 {
		return []*model.RecommendationVideo{}, false, nil
	}

	offset := (page - 1) * pageSize

	// 构建查询条件，匹配任一标签
	var conditions []string
	var args []interface{}

	for _, tag := range tags {
		conditions = append(conditions, "tags LIKE ?")
		args = append(args, "%"+tag+"%")
	}

	query := "is_public = ? AND status = ? AND (" + strings.Join(conditions, " OR ") + ")"
	args = append([]interface{}{true, "normal"}, args...)

	var videos []model.Video
	err := r.db.GetDB().Where(query, args...).
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
