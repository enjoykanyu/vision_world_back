package service

import (
	"github.com/vision_world/video_service/internal/config"
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

// TODO: 实现具体的业务逻辑方法
// 这些方法将被handler层调用，具体实现由你后续完成
// 例如：
// - PublishVideo()
// - DeleteVideo()
// - GetVideoInfo()
// - GetVideoInfos()
// - GetUserVideos()
// - GetRecommendVideos()
// - GetFollowVideos()
// - LikeVideo()
// - GetUserLikedVideos()
// - ShareVideo()
// - CommentVideo()
// - DeleteComment()
// - GetVideoComments()
// 等等...
