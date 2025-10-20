package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"recommendation_service/internal/config"
	"recommendation_service/internal/model"
	"recommendation_service/internal/repository"
	"recommendation_service/pkg/logger"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// VideoRecommendationService 视频推荐服务接口
type VideoRecommendationService interface {
	// 获取个性化推荐视频
	GetPersonalizedRecommendations(ctx context.Context, userID string, page, pageSize int, category string, userTags string) ([]*model.Video, bool, error)

	// 获取通用推荐视频
	GetGeneralRecommendations(ctx context.Context, page, pageSize int, category string) ([]*model.Video, bool, error)

	// 更新用户偏好
	UpdateUserPreferences(ctx context.Context, userID string, categories, tags []string, categoryWeights, tagWeights map[string]float64) error

	// 记录用户行为
	RecordUserAction(ctx context.Context, userID, videoID, actionType string, duration, totalDuration float64, timestamp int64) error
}

// videoRecommendationService 视频推荐服务实现
type videoRecommendationService struct {
	config             *config.Config
	logger             logger.Logger
	recommendationRepo repository.RecommendationRepository
}

// NewVideoRecommendationService 创建视频推荐服务
func NewVideoRecommendationService(cfg *config.Config, log logger.Logger, recommendationRepo repository.RecommendationRepository) VideoRecommendationService {
	return &videoRecommendationService{
		config:             cfg,
		logger:             log,
		recommendationRepo: recommendationRepo,
	}
}

// GetPersonalizedRecommendations 获取个性化推荐视频
func (s *videoRecommendationService) GetPersonalizedRecommendations(ctx context.Context, userID string, page, pageSize int, category string, userTags string) ([]*model.Video, bool, error) {
	s.logger.Info("GetPersonalizedRecommendations called", "user_id", userID, "page", page, "page_size", pageSize, "category", category)

	// 获取用户行为历史
	actions, err := s.recommendationRepo.GetUserActions(ctx, userID, 50)
	if err != nil {
		s.logger.Error("Failed to get user actions", "error", err, "user_id", userID)
		// 如果获取用户行为失败，回退到通用推荐
		return s.GetGeneralRecommendations(ctx, page, pageSize, category)
	}

	// 如果用户行为记录不足，回退到通用推荐
	if len(actions) < 5 {
		s.logger.Info("User has insufficient actions, falling back to general recommendations", "user_id", userID, "actions_count", len(actions))
		return s.GetGeneralRecommendations(ctx, page, pageSize, category)
	}

	// 分析用户偏好
	userPreferences := s.analyzeUserPreferences(actions)

	// 获取个性化推荐视频
	videos, hasMore, err := s.recommendationRepo.GetPersonalizedVideos(ctx, userID, page, pageSize, category, userTags)
	if err != nil {
		s.logger.Error("Failed to get personalized videos", "error", err, "user_id", userID)
		// 如果获取个性化推荐失败，回退到通用推荐
		return s.GetGeneralRecommendations(ctx, page, pageSize, category)
	}

	// 如果没有足够的个性化推荐视频，补充通用推荐
	if len(videos) < pageSize {
		generalVideos, _, err := s.recommendationRepo.GetGeneralVideos(ctx, 1, pageSize-len(videos), category)
		if err != nil {
			s.logger.Error("Failed to get general videos for supplement", "error", err)
			return videos, hasMore, nil
		}

		// 合并视频列表
		for _, video := range generalVideos {
			// 检查是否已存在
			exists := false
			for _, v := range videos {
				if v.VideoID == video.VideoID {
					exists = true
					break
				}
			}
			if !exists {
				videos = append(videos, video)
			}
		}
	}

	// 根据用户偏好调整视频分数
	s.adjustVideoScores(videos, userPreferences)

	// 按调整后的分数排序
	sort.Slice(videos, func(i, j int) bool {
		return videos[i].Score > videos[j].Score
	})

	// 记录推荐行为（用于后续分析）
	go s.recordRecommendationActions(userID, videos)

	return videos, hasMore, nil
}

// GetGeneralRecommendations 获取通用推荐视频
func (s *videoRecommendationService) GetGeneralRecommendations(ctx context.Context, page, pageSize int, category string) ([]*model.Video, bool, error) {
	s.logger.Info("GetGeneralRecommendations called", "page", page, "page_size", pageSize, "category", category)

	// 获取通用推荐视频
	videos, hasMore, err := s.recommendationRepo.GetGeneralVideos(ctx, page, pageSize, category)
	if err != nil {
		s.logger.Error("Failed to get general videos", "error", err, "page", page, "category", category)
		return nil, false, err
	}

	return videos, hasMore, nil
}

// UpdateUserPreferences 更新用户偏好
func (s *videoRecommendationService) UpdateUserPreferences(ctx context.Context, userID string, categories, tags []string, categoryWeights, tagWeights map[string]float64) error {
	s.logger.Info("UpdateUserPreferences called", "user_id", userID, "categories", categories, "tags", tags)

	// 将数组转换为逗号分隔的字符串
	categoriesStr := strings.Join(categories, ",")
	tagsStr := strings.Join(tags, ",")

	// 更新用户偏好
	err := s.recommendationRepo.UpdateUserPreferences(ctx, userID, categoriesStr, tagsStr, categoryWeights, tagWeights)
	if err != nil {
		s.logger.Error("Failed to update user preferences", "error", err, "user_id", userID)
		return err
	}

	return nil
}

// RecordUserAction 记录用户行为
func (s *videoRecommendationService) RecordUserAction(ctx context.Context, userID, videoID, actionType string, duration, totalDuration float64, timestamp int64) error {
	s.logger.Info("RecordUserAction called", "user_id", userID, "video_id", videoID, "action_type", actionType)

	// 记录用户行为
	err := s.recommendationRepo.RecordUserAction(ctx, userID, videoID, actionType, duration, totalDuration, timestamp)
	if err != nil {
		s.logger.Error("Failed to record user action", "error", err, "user_id", userID, "video_id", videoID, "action_type", actionType)
		return err
	}

	// 异步更新用户偏好（基于最新行为）
	go s.updateUserPreferencesFromActions(userID)

	return nil
}

// analyzeUserPreferences 分析用户偏好
func (s *videoRecommendationService) analyzeUserPreferences(actions []*model.UserAction) map[string]float64 {
	preferences := make(map[string]float64)

	// 统计用户行为
	categoryCount := make(map[string]int)
	tagCount := make(map[string]int)

	// 这里简化处理，实际项目中需要从视频信息中获取分类和标签
	for _, action := range actions {
		// 根据行为类型给予不同权重
		weight := 1.0
		switch action.ActionType {
		case "view":
			weight = 1.0
		case "like":
			weight = 3.0
		case "share":
			weight = 5.0
		case "comment":
			weight = 4.0
		case "complete":
			weight = 2.0
		}

		// 这里简化处理，实际需要从视频信息中获取分类和标签
		// 在实际实现中，需要查询视频信息并提取分类和标签
	}

	// 计算偏好分数
	for category, count := range categoryCount {
		preferences[category] = float64(count) / float64(len(actions))
	}

	for tag, count := range tagCount {
		preferences[tag] = float64(count) / float64(len(actions))
	}

	return preferences
}

// adjustVideoScores 根据用户偏好调整视频分数
func (s *videoRecommendationService) adjustVideoScores(videos []*model.Video, userPreferences map[string]float64) {
	for _, video := range videos {
		// 获取视频的分类和标签
		categories := parseStringToArray(video.Category)
		tags := parseStringToArray(video.Tags)

		// 计算偏好匹配分数
		preferenceScore := 0.0

		// 分类匹配
		for _, category := range categories {
			if weight, exists := userPreferences[category]; exists {
				preferenceScore += weight
			}
		}

		// 标签匹配
		for _, tag := range tags {
			if weight, exists := userPreferences[tag]; exists {
				preferenceScore += weight * 0.5 // 标签权重较低
			}
		}

		// 调整视频分数
		video.Score += preferenceScore
	}
}

// recordRecommendationActions 记录推荐行为（用于后续分析）
func (s *videoRecommendationService) recordRecommendationActions(userID string, videos []*model.Video) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 记录推荐的视频ID列表，用于后续分析推荐效果
	videoIDs := make([]string, len(videos))
	for i, video := range videos {
		videoIDs[i] = video.VideoID
	}

	// 这里简化处理，实际项目中可以使用更复杂的记录方式
	// 例如记录到专门的推荐日志表中
	s.logger.Info("Recorded recommendation actions", "user_id", userID, "video_count", len(videos))
}

// updateUserPreferencesFromActions 基于用户行为更新用户偏好
func (s *videoRecommendationService) updateUserPreferencesFromActions(userID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 获取用户最近的行为
	actions, err := s.recommendationRepo.GetUserActions(ctx, userID, 100)
	if err != nil {
		s.logger.Error("Failed to get user actions for preference update", "error", err, "user_id", userID)
		return
	}

	// 分析用户偏好
	preferences := s.analyzeUserPreferences(actions)

	// 提取分类和标签
	categories := []string{}
	tags := []string{}

	for preference := range preferences {
		// 简化处理，实际需要更复杂的分类和标签提取逻辑
		categories = append(categories, preference)
	}

	// 更新用户偏好
	err = s.UpdateUserPreferences(ctx, userID, categories, tags, preferences, make(map[string]float64))
	if err != nil {
		s.logger.Error("Failed to update user preferences from actions", "error", err, "user_id", userID)
		return
	}

	s.logger.Info("Updated user preferences from actions", "user_id", userID, "preferences_count", len(preferences))
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
