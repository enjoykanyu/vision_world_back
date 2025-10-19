package service

import (
	"context"
	"github.com/vision_world/video_service/internal/config"
	"github.com/vision_world/video_service/internal/model"
	"github.com/vision_world/video_service/internal/repository"
)

// VideoService 视频服务业务逻辑层
type VideoService struct {
	config *config.Config
	repo   *repository.VideoRepository
}

// NewVideoService 创建视频服务
func NewVideoService(cfg *config.Config) (*VideoService, error) {
	repo, err := repository.NewVideoRepository(cfg)
	if err != nil {
		return nil, err
	}

	return &VideoService{
		config: cfg,
		repo:   repo,
	}, nil
}

// Close 关闭服务
func (s *VideoService) Close() error {
	if s.repo != nil {
		return s.repo.Close()
	}
	return nil
}

// GetVideoByID 根据ID获取视频详情
func (s *VideoService) GetVideoByID(ctx context.Context, videoID string) (*model.RecommendationVideo, error) {
	return s.repo.GetVideoByID(ctx, videoID)
}

// GetVideosByIDs 根据ID列表获取视频详情
func (s *VideoService) GetVideosByIDs(ctx context.Context, videoIDs []string) ([]*model.RecommendationVideo, error) {
	return s.repo.GetVideosByIDs(ctx, videoIDs)
}

// GetHotVideos 获取热门视频
func (s *VideoService) GetHotVideos(ctx context.Context, page, pageSize int, category string) ([]*model.RecommendationVideo, bool, error) {
	return s.repo.GetHotVideos(ctx, page, pageSize, category)
}

// GetCategoryVideos 获取分类视频
func (s *VideoService) GetCategoryVideos(ctx context.Context, category string, page, pageSize int) ([]*model.RecommendationVideo, bool, error) {
	return s.repo.GetCategoryVideos(ctx, category, page, pageSize)
}

// SearchVideos 搜索视频
func (s *VideoService) SearchVideos(ctx context.Context, keyword string, page, pageSize int, category string) ([]*model.RecommendationVideo, bool, error) {
	return s.repo.SearchVideos(ctx, keyword, page, pageSize, category)
}

// UpdateVideoViewCount 更新视频播放量
func (s *VideoService) UpdateVideoViewCount(ctx context.Context, videoID string, increment int64) (int64, error) {
	return s.repo.UpdateVideoViewCount(ctx, videoID, increment)
}

// UpdateVideoLikeCount 更新视频点赞数
func (s *VideoService) UpdateVideoLikeCount(ctx context.Context, videoID string, increment int32) (int64, error) {
	return s.repo.UpdateVideoLikeCount(ctx, videoID, increment)
}

// GetVideosByAuthor 根据作者获取视频
func (s *VideoService) GetVideosByAuthor(ctx context.Context, author string, page, pageSize int) ([]*model.RecommendationVideo, bool, error) {
	return s.repo.GetVideosByAuthor(ctx, author, page, pageSize)
}

// GetVideosByTags 根据标签获取视频
func (s *VideoService) GetVideosByTags(ctx context.Context, tags []string, page, pageSize int) ([]*model.RecommendationVideo, bool, error) {
	return s.repo.GetVideosByTags(ctx, tags, page, pageSize)
}
