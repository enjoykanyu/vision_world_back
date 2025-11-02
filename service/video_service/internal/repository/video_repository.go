package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/go-redis/redis/v8"
	"github.com/vision_world/video_service/internal/model"
	"gorm.io/gorm"
)

// VideoRepository 视频数据访问接口
type VideoRepository interface {
	// 基础CRUD操作
	CreateVideo(ctx context.Context, video *model.Video) error
	GetVideoByID(ctx context.Context, videoID string) (*model.RecommendationVideo, error)
	GetVideosByIDs(ctx context.Context, videoIDs []string) ([]*model.RecommendationVideo, error)

	// 视频列表查询
	GetHotVideos(ctx context.Context, page, pageSize int, category string) ([]*model.RecommendationVideo, bool, error)
	GetCategoryVideos(ctx context.Context, category string, page, pageSize int) ([]*model.RecommendationVideo, bool, error)
	SearchVideos(ctx context.Context, keyword string, page, pageSize int, category string) ([]*model.RecommendationVideo, bool, error)
	GetVideosByAuthor(ctx context.Context, author string, page, pageSize int) ([]*model.RecommendationVideo, bool, error)

	// 统计数据更新
	IncrementPlayCount(ctx context.Context, videoID string) error
	IncrementLikeCount(ctx context.Context, videoID string) error
	DecrementLikeCount(ctx context.Context, videoID string) error

	// 资源清理
	Close() error
}

// videoRepository 视频数据访问实现
type videoRepository struct {
	db    *gorm.DB
	redis *redis.Client
}

// NewVideoRepository 创建视频数据访问对象
func NewVideoRepository(db *gorm.DB, redis *redis.Client) VideoRepository {
	return &videoRepository{
		db:    db,
		redis: redis,
	}
}

// CreateVideo 创建视频
func (r *videoRepository) CreateVideo(ctx context.Context, video *model.Video) error {
	// 保存到数据库
	if err := r.db.WithContext(ctx).Create(video).Error; err != nil {
		return fmt.Errorf("failed to create video: %w", err)
	}

	// 不需要缓存，因为新创建的视频会通过其他方法获取时再缓存
	return nil
}

// GetVideoByID 根据ID获取视频详情
func (r *videoRepository) GetVideoByID(ctx context.Context, videoID string) (*model.RecommendationVideo, error) {
	// 先从缓存获取
	if r.redis != nil {
		cacheKey := fmt.Sprintf("%s%s", model.CacheKeyVideo, videoID)
		cached, err := r.redis.Get(ctx, cacheKey).Result()
		if err == nil {
			var video model.RecommendationVideo
			if json.Unmarshal([]byte(cached), &video) == nil {
				return &video, nil
			}
		}
	}

	// 缓存未命中，从数据库获取
	var video model.Video
	id, err := strconv.ParseUint(videoID, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid video ID: %s", videoID)
	}

	if err := r.db.WithContext(ctx).Where("id = ? AND status = ?", id, "normal").First(&video).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("video not found: %s", videoID)
		}
		return nil, fmt.Errorf("failed to get video: %w", err)
	}

	// 转换为RecommendationVideo模型
	recVideo := r.convertToRecommendationVideo(&video)

	// 存入缓存
	if r.redis != nil {
		cacheKey := fmt.Sprintf("%s%s", model.CacheKeyVideo, videoID)
		if data, err := json.Marshal(recVideo); err == nil {
			r.redis.Set(ctx, cacheKey, data, model.CacheExpireShort)
		}
	}

	return recVideo, nil
}

// GetVideosByIDs 根据ID列表获取视频详情
func (r *videoRepository) GetVideosByIDs(ctx context.Context, videoIDs []string) ([]*model.RecommendationVideo, error) {
	var videos []*model.RecommendationVideo
	var uncachedIDs []string

	// 先从缓存获取
	if r.redis != nil {
		for _, videoID := range videoIDs {
			cacheKey := fmt.Sprintf("%s%s", model.CacheKeyVideo, videoID)
			cached, err := r.redis.Get(ctx, cacheKey).Result()
			if err == nil {
				var video model.RecommendationVideo
				if json.Unmarshal([]byte(cached), &video) == nil {
					videos = append(videos, &video)
				}
			} else {
				uncachedIDs = append(uncachedIDs, videoID)
			}
		}
	} else {
		uncachedIDs = videoIDs
	}

	// 从数据库获取未缓存的视频
	if len(uncachedIDs) > 0 {
		var dbVideos []model.Video
		var ids []uint32

		for _, videoID := range uncachedIDs {
			id, err := strconv.ParseUint(videoID, 10, 32)
			if err == nil {
				ids = append(ids, uint32(id))
			}
		}

		if len(ids) > 0 {
			if err := r.db.WithContext(ctx).Where("id IN ? AND status = ?", ids, "normal").Find(&dbVideos).Error; err != nil {
				return nil, fmt.Errorf("failed to get videos: %w", err)
			}

			for _, video := range dbVideos {
				recVideo := r.convertToRecommendationVideo(&video)
				videos = append(videos, recVideo)

				// 存入缓存
				if r.redis != nil {
					cacheKey := fmt.Sprintf("%s%d", model.CacheKeyVideo, video.ID)
					if data, err := json.Marshal(recVideo); err == nil {
						r.redis.Set(ctx, cacheKey, data, model.CacheExpireShort)
					}
				}
			}
		}
	}

	return videos, nil
}

// GetHotVideos 获取热门视频
func (r *videoRepository) GetHotVideos(ctx context.Context, page, pageSize int, category string) ([]*model.RecommendationVideo, bool, error) {
	var videos []model.Video
	var total int64

	query := r.db.WithContext(ctx).Where("status = ? AND is_public = ?", "normal", true)

	if category != "" {
		query = query.Where("category = ?", category)
	}

	// 获取总数
	if err := query.Model(&model.Video{}).Count(&total).Error; err != nil {
		return nil, false, fmt.Errorf("failed to count videos: %w", err)
	}

	// 获取分页数据
	offset := (page - 1) * pageSize
	if err := query.Order("play_count DESC, like_count DESC").
		Offset(offset).Limit(pageSize).
		Find(&videos).Error; err != nil {
		return nil, false, fmt.Errorf("failed to get hot videos: %w", err)
	}

	// 转换为RecommendationVideo模型
	recVideos := make([]*model.RecommendationVideo, 0, len(videos))
	for _, video := range videos {
		recVideo := r.convertToRecommendationVideo(&video)
		recVideos = append(recVideos, recVideo)
	}

	hasMore := int64(offset+pageSize) < total
	return recVideos, hasMore, nil
}

// GetCategoryVideos 获取分类视频
func (r *videoRepository) GetCategoryVideos(ctx context.Context, category string, page, pageSize int) ([]*model.RecommendationVideo, bool, error) {
	var videos []model.Video
	var total int64

	query := r.db.WithContext(ctx).Where("status = ? AND is_public = ? AND category = ?", "normal", true, category)

	// 获取总数
	if err := query.Model(&model.Video{}).Count(&total).Error; err != nil {
		return nil, false, fmt.Errorf("failed to count videos: %w", err)
	}

	// 获取分页数据
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&videos).Error; err != nil {
		return nil, false, fmt.Errorf("failed to get category videos: %w", err)
	}

	// 转换为RecommendationVideo模型
	recVideos := make([]*model.RecommendationVideo, 0, len(videos))
	for _, video := range videos {
		recVideo := r.convertToRecommendationVideo(&video)
		recVideos = append(recVideos, recVideo)
	}

	hasMore := int64(offset+pageSize) < total
	return recVideos, hasMore, nil
}

// SearchVideos 搜索视频
func (r *videoRepository) SearchVideos(ctx context.Context, keyword string, page, pageSize int, category string) ([]*model.RecommendationVideo, bool, error) {
	var videos []model.Video
	var total int64

	query := r.db.WithContext(ctx).Where("status = ? AND is_public = ?", "normal", true)

	if keyword != "" {
		query = query.Where("title LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	if category != "" {
		query = query.Where("category = ?", category)
	}

	// 获取总数
	if err := query.Model(&model.Video{}).Count(&total).Error; err != nil {
		return nil, false, fmt.Errorf("failed to count videos: %w", err)
	}

	// 获取分页数据
	offset := (page - 1) * pageSize
	if err := query.Order("play_count DESC").
		Offset(offset).Limit(pageSize).
		Find(&videos).Error; err != nil {
		return nil, false, fmt.Errorf("failed to search videos: %w", err)
	}

	// 转换为RecommendationVideo模型
	recVideos := make([]*model.RecommendationVideo, 0, len(videos))
	for _, video := range videos {
		recVideo := r.convertToRecommendationVideo(&video)
		recVideos = append(recVideos, recVideo)
	}

	hasMore := int64(offset+pageSize) < total
	return recVideos, hasMore, nil
}

// GetVideosByAuthor 根据作者获取视频
func (r *videoRepository) GetVideosByAuthor(ctx context.Context, author string, page, pageSize int) ([]*model.RecommendationVideo, bool, error) {
	var videos []model.Video
	var total int64

	// 这里假设author是用户名，实际可能需要先根据用户名获取用户ID
	// 简化处理，直接使用author作为用户名查询
	query := r.db.WithContext(ctx).Where("status = ? AND is_public = ?", "normal", true)

	// TODO: 根据实际业务逻辑调整作者查询条件
	// 这里简化处理，假设author是用户ID
	userID, err := strconv.ParseUint(author, 10, 32)
	if err == nil {
		query = query.Where("user_id = ?", userID)
	}

	// 获取总数
	if err := query.Model(&model.Video{}).Count(&total).Error; err != nil {
		return nil, false, fmt.Errorf("failed to count videos: %w", err)
	}

	// 获取分页数据
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&videos).Error; err != nil {
		return nil, false, fmt.Errorf("failed to get author videos: %w", err)
	}

	// 转换为RecommendationVideo模型
	recVideos := make([]*model.RecommendationVideo, 0, len(videos))
	for _, video := range videos {
		recVideo := r.convertToRecommendationVideo(&video)
		recVideos = append(recVideos, recVideo)
	}

	hasMore := int64(offset+pageSize) < total
	return recVideos, hasMore, nil
}

// IncrementPlayCount 增加视频播放量
func (r *videoRepository) IncrementPlayCount(ctx context.Context, videoID string) error {
	id, err := strconv.ParseUint(videoID, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid video ID: %s", videoID)
	}

	// 更新数据库
	if err := r.db.WithContext(ctx).Model(&model.Video{}).
		Where("id = ?", id).
		UpdateColumn("play_count", gorm.Expr("play_count + ?", 1)).Error; err != nil {
		return fmt.Errorf("failed to increment play count: %w", err)
	}

	// 更新缓存
	if r.redis != nil {
		cacheKey := fmt.Sprintf("%s%s", model.CacheKeyVideo, videoID)
		// 简单处理，直接删除缓存，下次访问时重新加载
		r.redis.Del(ctx, cacheKey)
	}

	return nil
}

// IncrementLikeCount 增加视频点赞数
func (r *videoRepository) IncrementLikeCount(ctx context.Context, videoID string) error {
	id, err := strconv.ParseUint(videoID, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid video ID: %s", videoID)
	}

	// 更新数据库
	if err := r.db.WithContext(ctx).Model(&model.Video{}).
		Where("id = ?", id).
		UpdateColumn("like_count", gorm.Expr("like_count + ?", 1)).Error; err != nil {
		return fmt.Errorf("failed to increment like count: %w", err)
	}

	// 更新缓存
	if r.redis != nil {
		cacheKey := fmt.Sprintf("%s%s", model.CacheKeyVideo, videoID)
		// 简单处理，直接删除缓存，下次访问时重新加载
		r.redis.Del(ctx, cacheKey)
	}

	return nil
}

// DecrementLikeCount 减少视频点赞数
func (r *videoRepository) DecrementLikeCount(ctx context.Context, videoID string) error {
	id, err := strconv.ParseUint(videoID, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid video ID: %s", videoID)
	}

	// 更新数据库
	if err := r.db.WithContext(ctx).Model(&model.Video{}).
		Where("id = ? AND like_count > 0", id).
		UpdateColumn("like_count", gorm.Expr("like_count - ?", 1)).Error; err != nil {
		return fmt.Errorf("failed to decrement like count: %w", err)
	}

	// 更新缓存
	if r.redis != nil {
		cacheKey := fmt.Sprintf("%s%s", model.CacheKeyVideo, videoID)
		// 简单处理，直接删除缓存，下次访问时重新加载
		r.redis.Del(ctx, cacheKey)
	}

	return nil
}

// Close 关闭资源
func (r *videoRepository) Close() error {
	if r.redis != nil {
		return r.redis.Close()
	}
	return nil
}

// convertToRecommendationVideo 将Video模型转换为RecommendationVideo模型
func (r *videoRepository) convertToRecommendationVideo(video *model.Video) *model.RecommendationVideo {
	return &model.RecommendationVideo{
		VideoID:     strconv.Itoa(int(video.ID)),
		Title:       video.Title,
		Description: video.Description,
		Author:      strconv.Itoa(int(video.UserID)), // 简化处理，实际应该查询用户名
		Category:    video.Category,
		Tags:        video.Tags,
		Duration:    int32(video.Duration),
		CoverURL:    video.CoverURL,
		PlayURL:     video.VideoURL,
		ViewCount:   int64(video.PlayCount),
		LikeCount:   int64(video.LikeCount),
		Score:       0, // 需要根据推荐算法计算
		CreatedAt:   video.CreatedAt,
		UpdatedAt:   video.UpdatedAt,
	}
}
