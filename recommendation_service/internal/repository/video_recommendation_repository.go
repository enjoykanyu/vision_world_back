package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"recommendation_service/internal/model"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// RecommendationRepository 推荐仓储接口
type RecommendationRepository interface {
	// 获取个性化推荐视频
	GetPersonalizedVideos(ctx context.Context, userID string, page, pageSize int, category string, userTags string) ([]*model.Video, bool, error)

	// 获取通用推荐视频
	GetGeneralVideos(ctx context.Context, page, pageSize int, category string) ([]*model.Video, bool, error)

	// 获取用户偏好
	GetUserPreferences(ctx context.Context, userID string) (*model.UserPreference, error)

	// 更新用户偏好
	UpdateUserPreferences(ctx context.Context, userID string, categories, tags string, categoryWeights, tagWeights map[string]float64) error

	// 记录用户行为
	RecordUserAction(ctx context.Context, userID, videoID, actionType string, duration, totalDuration float64, timestamp int64) error

	// 获取用户行为历史
	GetUserActions(ctx context.Context, userID string, limit int) ([]*model.UserAction, error)

	// 获取热门视频
	GetHotVideos(ctx context.Context, page, pageSize int, category string) ([]*model.Video, bool, error)
}

// recommendationRepository 推荐仓储实现
type recommendationRepository struct {
	db    *gorm.DB
	redis *redis.Client
}

// NewRecommendationRepository 创建推荐仓储
func NewRecommendationRepository(db *gorm.DB, redis *redis.Client) RecommendationRepository {
	return &recommendationRepository{
		db:    db,
		redis: redis,
	}
}

// GetPersonalizedVideos 获取个性化推荐视频
func (r *recommendationRepository) GetPersonalizedVideos(ctx context.Context, userID string, page, pageSize int, category string, userTags string) ([]*model.Video, bool, error) {
	// 先尝试从缓存获取
	cacheKey := fmt.Sprintf("personalized_videos:%s:%d:%d:%s", userID, page, pageSize, category)
	cached, err := r.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var videos []*model.Video
		if err := json.Unmarshal([]byte(cached), &videos); err == nil {
			// 检查是否有更多数据
			hasMore := len(videos) == pageSize
			return videos, hasMore, nil
		}
	}

	// 获取用户偏好
	userPref, err := r.GetUserPreferences(ctx, userID)
	if err != nil {
		// 如果没有用户偏好，回退到通用推荐
		return r.GetGeneralVideos(ctx, page, pageSize, category)
	}

	// 计算偏移量
	offset := (page - 1) * pageSize

	// 构建查询
	query := r.db.WithContext(ctx).Model(&model.Video{})

	// 如果指定了分类，添加分类过滤
	if category != "" {
		query = query.Where("category = ?", category)
	}

	// 基于用户偏好调整查询权重
	if userPref.Categories != "" {
		categories := parseStringToArray(userPref.Categories)
		if len(categories) > 0 {
			query = query.Where("category IN ?", categories)
		}
	}

	// 按推荐分数和创建时间排序
	query = query.Order("score DESC, created_at DESC")

	// 执行查询
	var videos []*model.Video
	if err := query.Offset(offset).Limit(pageSize + 1).Find(&videos).Error; err != nil {
		return nil, false, err
	}

	// 检查是否有更多数据
	hasMore := len(videos) > pageSize
	if hasMore {
		videos = videos[:pageSize]
	}

	// 缓存结果
	if len(videos) > 0 {
		videoData, _ := json.Marshal(videos)
		r.redis.Set(ctx, cacheKey, videoData, 5*time.Minute)
	}

	return videos, hasMore, nil
}

// GetGeneralVideos 获取通用推荐视频
func (r *recommendationRepository) GetGeneralVideos(ctx context.Context, page, pageSize int, category string) ([]*model.Video, bool, error) {
	// 先尝试从缓存获取
	cacheKey := fmt.Sprintf("general_videos:%d:%d:%s", page, pageSize, category)
	cached, err := r.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var videos []*model.Video
		if err := json.Unmarshal([]byte(cached), &videos); err == nil {
			// 检查是否有更多数据
			hasMore := len(videos) == pageSize
			return videos, hasMore, nil
		}
	}

	// 计算偏移量
	offset := (page - 1) * pageSize

	// 构建查询
	query := r.db.WithContext(ctx).Model(&model.Video{})

	// 如果指定了分类，添加分类过滤
	if category != "" {
		query = query.Where("category = ?", category)
	}

	// 按推荐分数和创建时间排序
	query = query.Order("score DESC, created_at DESC")

	// 执行查询
	var videos []*model.Video
	if err := query.Offset(offset).Limit(pageSize + 1).Find(&videos).Error; err != nil {
		return nil, false, err
	}

	// 检查是否有更多数据
	hasMore := len(videos) > pageSize
	if hasMore {
		videos = videos[:pageSize]
	}

	// 缓存结果
	if len(videos) > 0 {
		videoData, _ := json.Marshal(videos)
		r.redis.Set(ctx, cacheKey, videoData, 10*time.Minute)
	}

	return videos, hasMore, nil
}

// GetUserPreferences 获取用户偏好
func (r *recommendationRepository) GetUserPreferences(ctx context.Context, userID string) (*model.UserPreference, error) {
	// 先尝试从缓存获取
	cacheKey := fmt.Sprintf("user_preferences:%s", userID)
	cached, err := r.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var pref model.UserPreference
		if err := json.Unmarshal([]byte(cached), &pref); err == nil {
			return &pref, nil
		}
	}

	// 从数据库获取
	var pref model.UserPreference
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&pref).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // 用户偏好不存在
		}
		return nil, err
	}

	// 缓存结果
	prefData, _ := json.Marshal(pref)
	r.redis.Set(ctx, cacheKey, prefData, 30*time.Minute)

	return &pref, nil
}

// UpdateUserPreferences 更新用户偏好
func (r *recommendationRepository) UpdateUserPreferences(ctx context.Context, userID string, categories, tags string, categoryWeights, tagWeights map[string]float64) error {
	// 序列化权重
	categoryWeightsJSON, _ := json.Marshal(categoryWeights)
	tagWeightsJSON, _ := json.Marshal(tagWeights)

	// 查找现有偏好
	var pref model.UserPreference
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&pref).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}

	if err == gorm.ErrRecordNotFound {
		// 创建新偏好
		pref = model.UserPreference{
			UserID:          userID,
			Categories:      categories,
			Tags:            tags,
			CategoryWeights: string(categoryWeightsJSON),
			TagWeights:      string(tagWeightsJSON),
		}
		if err := r.db.WithContext(ctx).Create(&pref).Error; err != nil {
			return err
		}
	} else {
		// 更新现有偏好
		pref.Categories = categories
		pref.Tags = tags
		pref.CategoryWeights = string(categoryWeightsJSON)
		pref.TagWeights = string(tagWeightsJSON)
		if err := r.db.WithContext(ctx).Save(&pref).Error; err != nil {
			return err
		}
	}

	// 更新缓存
	cacheKey := fmt.Sprintf("user_preferences:%s", userID)
	prefData, _ := json.Marshal(pref)
	r.redis.Set(ctx, cacheKey, prefData, 30*time.Minute)

	return nil
}

// RecordUserAction 记录用户行为
func (r *recommendationRepository) RecordUserAction(ctx context.Context, userID, videoID, actionType string, duration, totalDuration float64, timestamp int64) error {
	// 创建用户行为记录
	action := model.UserAction{
		UserID:        userID,
		VideoID:       videoID,
		ActionType:    actionType,
		Duration:      duration,
		TotalDuration: totalDuration,
		Timestamp:     timestamp,
	}

	// 保存到数据库
	if err := r.db.WithContext(ctx).Create(&action).Error; err != nil {
		return err
	}

	// 更新视频统计信息
	if actionType == "like" {
		r.db.WithContext(ctx).Model(&model.Video{}).Where("video_id = ?", videoID).UpdateColumn("like_count", gorm.Expr("like_count + ?", 1))
	} else if actionType == "view" {
		r.db.WithContext(ctx).Model(&model.Video{}).Where("video_id = ?", videoID).UpdateColumn("view_count", gorm.Expr("view_count + ?", 1))
	} else if actionType == "share" {
		r.db.WithContext(ctx).Model(&model.Video{}).Where("video_id = ?", videoID).UpdateColumn("share_count", gorm.Expr("share_count + ?", 1))
	}

	// 异步更新推荐分数（实际项目中可以使用消息队列）
	go r.updateVideoScore(videoID)

	return nil
}

// GetUserActions 获取用户行为历史
func (r *recommendationRepository) GetUserActions(ctx context.Context, userID string, limit int) ([]*model.UserAction, error) {
	var actions []*model.UserAction
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("timestamp DESC").
		Limit(limit).
		Find(&actions).Error; err != nil {
		return nil, err
	}
	return actions, nil
}

// GetHotVideos 获取热门视频
func (r *recommendationRepository) GetHotVideos(ctx context.Context, page, pageSize int, category string) ([]*model.Video, bool, error) {
	// 先尝试从缓存获取
	cacheKey := fmt.Sprintf("hot_videos:%d:%d:%s", page, pageSize, category)
	cached, err := r.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var videos []*model.Video
		if err := json.Unmarshal([]byte(cached), &videos); err == nil {
			// 检查是否有更多数据
			hasMore := len(videos) == pageSize
			return videos, hasMore, nil
		}
	}

	// 计算偏移量
	offset := (page - 1) * pageSize

	// 构建查询
	query := r.db.WithContext(ctx).Model(&model.Video{})

	// 如果指定了分类，添加分类过滤
	if category != "" {
		query = query.Where("category = ?", category)
	}

	// 按观看次数、点赞数和创建时间综合排序
	query = query.Order("(view_count * 0.3 + like_count * 0.5 + share_count * 0.2) DESC, created_at DESC")

	// 执行查询
	var videos []*model.Video
	if err := query.Offset(offset).Limit(pageSize + 1).Find(&videos).Error; err != nil {
		return nil, false, err
	}

	// 检查是否有更多数据
	hasMore := len(videos) > pageSize
	if hasMore {
		videos = videos[:pageSize]
	}

	// 缓存结果
	if len(videos) > 0 {
		videoData, _ := json.Marshal(videos)
		r.redis.Set(ctx, cacheKey, videoData, 5*time.Minute)
	}

	return videos, hasMore, nil
}

// updateVideoScore 更新视频推荐分数
func (r *recommendationRepository) updateVideoScore(videoID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 获取视频信息
	var video model.Video
	if err := r.db.WithContext(ctx).Where("video_id = ?", videoID).First(&video).Error; err != nil {
		return
	}

	// 计算推荐分数（这里使用简单的加权算法）
	// 分数 = 观看次数 * 0.1 + 点赞数 * 0.3 + 分享数 * 0.5 + 评论数 * 0.2 + 时间衰减
	viewScore := float64(video.ViewCount) * 0.1
	likeScore := float64(video.LikeCount) * 0.3
	shareScore := float64(video.ShareCount) * 0.5
	commentScore := float64(video.CommentCount) * 0.2

	// 时间衰减（视频越新，分数越高）
	timeDiff := time.Since(video.CreatedAt).Hours()
	timeDecay := 1.0 / (1.0 + timeDiff/24.0) // 24小时为一个周期

	// 计算总分
	totalScore := (viewScore + likeScore + shareScore + commentScore) * timeDecay

	// 更新分数
	r.db.WithContext(ctx).Model(&video).Update("score", totalScore)
}

// parseStringToArray 将逗号分隔的字符串转换为数组
func parseStringToArray(str string) []string {
	if str == "" {
		return []string{}
	}

	result := []string{}
	for _, s := range strings.Split(str, ",") {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
