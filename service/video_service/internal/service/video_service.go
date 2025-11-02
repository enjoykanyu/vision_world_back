package service

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/vision_world/video_service/internal/config"
	"github.com/vision_world/video_service/internal/model"
	"github.com/vision_world/video_service/internal/queue"
	"github.com/vision_world/video_service/internal/repository"
	"github.com/vision_world/video_service/pkg/minio"
	"gorm.io/gorm"
)

// VideoService 视频服务业务逻辑层
type VideoService struct {
	config      *config.Config
	repo        repository.VideoRepository
	minioClient *minio.Client
	queueClient *queue.RabbitMQClient
}

// NewVideoService 创建视频服务
func NewVideoService(cfg *config.Config, db *gorm.DB, redis *redis.Client, minioClient *minio.Client, queueClient *queue.RabbitMQClient) (*VideoService, error) {
	repo := repository.NewVideoRepository(db, redis)

	return &VideoService{
		config:      cfg,
		repo:        repo,
		minioClient: minioClient,
		queueClient: queueClient,
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
	err := s.repo.IncrementPlayCount(ctx, videoID)
	if err != nil {
		return 0, err
	}

	// 获取更新后的播放量
	video, err := s.repo.GetVideoByID(ctx, videoID)
	if err != nil {
		return 0, err
	}

	return int64(video.ViewCount), nil
}

// UpdateVideoLikeCount 更新视频点赞数
func (s *VideoService) UpdateVideoLikeCount(ctx context.Context, videoID string, increment int32) (int64, error) {
	if increment > 0 {
		err := s.repo.IncrementLikeCount(ctx, videoID)
		if err != nil {
			return 0, err
		}
	} else {
		err := s.repo.DecrementLikeCount(ctx, videoID)
		if err != nil {
			return 0, err
		}
	}

	// 获取更新后的点赞数
	video, err := s.repo.GetVideoByID(ctx, videoID)
	if err != nil {
		return 0, err
	}

	return int64(video.LikeCount), nil
}

// GetVideosByAuthor 根据作者获取视频
func (s *VideoService) GetVideosByAuthor(ctx context.Context, author string, page, pageSize int) ([]*model.RecommendationVideo, bool, error) {
	return s.repo.GetVideosByAuthor(ctx, author, page, pageSize)
}

// GetVideosByTags 根据标签获取视频
func (s *VideoService) GetVideosByTags(ctx context.Context, tags []string, page, pageSize int) ([]*model.RecommendationVideo, bool, error) {
	// 使用 GetVideosByIDs 方法获取视频
	// 这里需要先根据标签获取视频ID列表，然后再获取视频详情
	// 由于没有直接的方法，我们暂时返回空列表
	return []*model.RecommendationVideo{}, false, nil
}

// UploadVideo 上传视频
func (s *VideoService) UploadVideo(ctx context.Context, userID, fileName, title, description, category string, tags []string, videoData []byte) (uint32, string, string, error) {
	// 生成视频ID (这里简化处理，实际应该从数据库获取)
	videoID := uint32(time.Now().Unix())

	// 创建对象名称
	objectName := fmt.Sprintf("videos/%d/%s", videoID, fileName)

	// 将视频数据上传到MinIO
	videoURL, err := s.minioClient.UploadFileFromReader(ctx, objectName,
		bytes.NewReader(videoData),
		int64(len(videoData)),
		"video/mp4")
	if err != nil {
		return 0, "", videoURL, fmt.Errorf("failed to upload video to MinIO: %w", err)
	}

	// 生成预签名URL，有效期24小时
	presignedURL, err := s.minioClient.GeneratePresignedURL(ctx, objectName, 24*time.Hour)
	if err != nil {
		return 0, "", videoURL, fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	// 保存视频信息到数据库 (简化处理)
	// TODO: 实际应该调用repo保存视频信息

	// 发送审核消息到RabbitMQ队列
	//auditMessage := &queue.AuditMessage{
	//	ContentID:    fmt.Sprintf("video_%d", videoID),
	//	ContentType:  "video",
	//	Title:        title,
	//	URL:          videoURL,
	//	Metadata:     description,
	//	UploaderID:   userID,
	//	UploaderName: userID, // TODO: 从用户信息获取用户名
	//}

	//if err := s.queueClient.PublishAuditMessage(ctx, auditMessage); err != nil {
	//	// 审核消息发送失败，但视频已上传成功，可以继续返回成功状态
	//	return videoID, presignedURL, nil
	//}

	return videoID, presignedURL, videoURL, nil
}
