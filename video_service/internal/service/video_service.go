package service

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/vision_world/video_service/internal/config"
	"github.com/vision_world/video_service/internal/model"
	"github.com/vision_world/video_service/internal/queue"
	"github.com/vision_world/video_service/internal/repository"
	"github.com/vision_world/video_service/pkg/auth"
	"github.com/vision_world/video_service/pkg/logger"
	"github.com/vision_world/video_service/pkg/minio"
	"github.com/vision_world/video_service/pkg/transcode"
	"github.com/vision_world/video_service/proto/proto_gen/user"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

// VideoService 视频服务业务逻辑层
type VideoService struct {
	config           *config.Config
	repo             repository.VideoRepository
	minioClient      *minio.Client
	queueClient      *queue.RabbitMQClient
	logger           logger.Logger
	userClient       user.UserServiceClient
	authService      auth.Service
	transcodeService transcode.Service
}

// NewVideoService 创建视频服务
func NewVideoService(cfg *config.Config, db *gorm.DB, redis *redis.Client, minioClient *minio.Client, queueClient *queue.RabbitMQClient, log logger.Logger) (*VideoService, error) {
	repo := repository.NewVideoRepository(db, redis)

	// 初始化用户服务客户端
	var userClient user.UserServiceClient
	userConn, err := grpc.Dial(cfg.Services.UserService.Address, grpc.WithInsecure())
	if err != nil {
		log.Warn("Failed to connect to user service, will use mock user data", "error", err)
		// 继续执行，使用默认的用户信息
	} else {
		userClient = user.NewUserServiceClient(userConn)
	}

	return &VideoService{
		config:      cfg,
		repo:        repo,
		minioClient: minioClient,
		queueClient: queueClient,
		logger:      log,
		userClient:  userClient,
		authService: auth.NewService(auth.Config{
			SecretKey:      "your-secret-key", // 从配置中读取
			ExpireDuration: 24 * time.Hour,
			AllowOrigin:    []string{"localhost", "127.0.0.1"},
		}),
		transcodeService: transcode.NewService(transcode.Config{
			FFmpegPath:      "ffmpeg",
			WorkDir:         "/tmp/transcode",
			OutputDir:       "/tmp/output",
			Preset:          "medium",
			SegmentDuration: 10,
			Timeout:         2 * time.Hour,
			LogLevel:        "info",
		}, log),
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

// GetVideoDetail 根据ID获取完整视频详情，包括用户信息
func (s *VideoService) GetVideoDetail(ctx context.Context, videoID string) (*model.VideoDetail, error) {
	// 从仓库层获取视频详情
	videoDetail, err := s.repo.GetVideoDetailByID(ctx, videoID)
	if err != nil {
		return nil, err
	}

	// 获取视频作者信息
	if err := s.populateUserInfo(ctx, videoDetail); err != nil {
		s.logger.Warn("Failed to populate user info for video",
			"video_id", videoID,
			"user_id", videoDetail.UserInfo.UserID,
			"error", err)
		// 继续执行，使用默认的用户信息
	}

	// 生成HLS播放URL
	if videoDetail.PlaylistURL != "" {
		// 使用API网关端点生成HLS播放URL
		baseURL := s.config.APIGateway.Endpoint + "/api/video"
		playURL, err := s.authService.GeneratePlayURL(videoID, baseURL, 24*time.Hour)
		if err != nil {
			s.logger.Warn("Failed to generate play URL",
				"video_id", videoID,
				"error", err)
			// 继续执行，使用原始URL
		} else {
			// 修改生成的URL，添加/stream路径
			playURL = strings.Replace(playURL, "/play/", "/play/stream/", 1)
			videoDetail.PlayURL = playURL
		}
	} else {
		// 如果HLS播放列表不存在，使用原始URL
		// 但我们需要重新生成预签名URL，有效期24小时
		// 解析原始URL，提取完整的对象名
		objectName := "video.mp4" // 默认值
		if videoDetail.PlayURL != "" {
			// 查找bucket名称后的路径
			bucketName := s.minioClient.GetBucketName()
			bucketIndex := strings.Index(videoDetail.PlayURL, "/"+bucketName+"/")
			if bucketIndex != -1 {
				// 提取bucket名称后的路径，包括文件名
				pathWithQuery := videoDetail.PlayURL[bucketIndex+len("/"+bucketName+"/"):]
				// 去除查询参数
				path := strings.Split(pathWithQuery, "?")[0]
				// 检查路径是否包含重复的bucket名称前缀（兼容旧URL）
				if strings.HasPrefix(path, bucketName+"/") {
					// 去除重复的bucket名称前缀
					objectName = path[len(bucketName+"/"):]
				} else {
					objectName = path
				}
			} else {
				// 备用方案：使用videoID直接构建对象名
				// 从URL中提取文件名
				fileName := "video.mp4"
				lastSlashIndex := strings.LastIndex(videoDetail.PlayURL, "/")
				if lastSlashIndex != -1 && lastSlashIndex < len(videoDetail.PlayURL)-1 {
					fileNameWithQuery := videoDetail.PlayURL[lastSlashIndex+1:]
					fileName = strings.Split(fileNameWithQuery, "?")[0]
				}
				objectName = fmt.Sprintf("%s/%s", videoID, fileName)
			}
		} else {
			// 使用videoID直接构建对象名
			objectName = fmt.Sprintf("%s/video.mp4", videoID)
		}

		// 生成新的预签名URL，有效期24小时
		presignedURL, err := s.minioClient.GeneratePresignedURL(ctx, objectName, 24*time.Hour)
		if err != nil {
			s.logger.Warn("Failed to generate presigned URL for video",
				"video_id", videoID,
				"error", err)
			// 继续执行，使用数据库中存储的旧URL
		} else {
			// 使用新生成的URL替换旧URL
			videoDetail.PlayURL = presignedURL
		}
	}

	return videoDetail, nil
}

// populateUserInfo 填充用户信息
func (s *VideoService) populateUserInfo(ctx context.Context, videoDetail *model.VideoDetail) error {
	if s.userClient == nil {
		// 用户服务客户端未初始化，使用默认值
		videoDetail.UserInfo.Username = fmt.Sprintf("用户%d", videoDetail.UserInfo.UserID)
		videoDetail.UserInfo.AvatarURL = fmt.Sprintf("https://picsum.photos/seed/user%d/200/200.jpg", videoDetail.UserInfo.UserID)
		videoDetail.UserInfo.FollowersCount = 0
		return nil
	}

	// 调用用户服务获取用户信息
	req := &user.GetUserInfoRequest{
		UserId: videoDetail.UserInfo.UserID,
	}

	resp, err := s.userClient.GetUserInfo(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get user info: %w", err)
	}

	if resp.User == nil {
		// 用户不存在，使用默认值
		videoDetail.UserInfo.Username = fmt.Sprintf("用户%d", videoDetail.UserInfo.UserID)
		videoDetail.UserInfo.AvatarURL = fmt.Sprintf("https://picsum.photos/seed/user%d/200/200.jpg", videoDetail.UserInfo.UserID)
		videoDetail.UserInfo.FollowersCount = 0
		return nil
	}

	// 填充用户信息
	videoDetail.UserInfo.Username = resp.User.Name
	if resp.User.Avatar != nil {
		videoDetail.UserInfo.AvatarURL = *resp.User.Avatar
	} else {
		videoDetail.UserInfo.AvatarURL = fmt.Sprintf("https://picsum.photos/seed/user%d/200/200.jpg", videoDetail.UserInfo.UserID)
	}
	if resp.User.FollowerCount != nil {
		videoDetail.UserInfo.FollowersCount = int64(*resp.User.FollowerCount)
	} else {
		videoDetail.UserInfo.FollowersCount = 0
	}

	return nil
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
	// 将用户ID转换为uint32类型
	userIDUint32, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		return 0, "", "", fmt.Errorf("invalid user ID: %w", err)
	}

	// 将标签数组转换为逗号分隔的字符串
	tagsStr := ""
	if len(tags) > 0 {
		for i, tag := range tags {
			if i > 0 {
				tagsStr += ","
			}
			tagsStr += tag
		}
	}

	// 1. 先创建视频记录，获取数据库自增ID
	video := &model.Video{
		UserID:          uint32(userIDUint32),
		Title:           title,
		Description:     description,
		CoverURL:        "https://default-cover-url.com/default.jpg", // 使用默认封面URL，确保not null字段有值
		VideoURL:        "",                                          // 稍后更新
		PlaylistURL:     "",                                          // HLS播放列表URL，转码后更新
		TranscodeStatus: "pending",                                   // 初始转码状态
		Duration:        0,                                           // TODO: 可以从视频文件中获取时长
		Resolution:      "1080p",                                     // 设置默认分辨率，确保字段有值
		Size:            uint64(len(videoData)),
		Tags:            tagsStr,
		Category:        category,
		PlayCount:       0,
		LikeCount:       0,
		CommentCount:    0,
		ShareCount:      0,
		FavoriteCount:   0,
		IsPublic:        true,
		Status:          "uploading", // 上传后状态为uploading，等待发布
	}

	// 保存视频信息到数据库，获取自增ID
	if err := s.repo.CreateVideo(ctx, video); err != nil {
		return 0, "", "", fmt.Errorf("failed to save video info: %w", err)
	}

	// 2. 使用数据库生成的ID作为视频ID
	videoID := video.ID

	// 3. 创建对象名称
	objectName := fmt.Sprintf("%d/%s", videoID, fileName)

	// 根据文件扩展名确定MIME类型
	contentType := getContentTypeFromFileName(fileName)

	// 4. 将视频数据上传到MinIO
	videoURL, err := s.minioClient.UploadFileFromReader(ctx, objectName,
		bytes.NewReader(videoData),
		int64(len(videoData)),
		contentType)
	if err != nil {
		return 0, "", "", fmt.Errorf("failed to upload video to MinIO: %w", err)
	}

	// 5. 生成预签名URL，有效期24小时
	presignedURL, err := s.minioClient.GeneratePresignedURL(ctx, objectName, 24*time.Hour)
	if err != nil {
		return 0, "", "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	// 6. 更新视频记录，保存视频URL
	video.VideoURL = presignedURL
	if err := s.repo.UpdateVideoURL(ctx, videoID, presignedURL); err != nil {
		s.logger.Warn("Failed to update video URL", "video_id", videoID, "error", err)
	}

	// 7. 创建临时文件用于转码
	tmpFilePath := fmt.Sprintf("/tmp/%d_%s", videoID, fileName)
	if err := os.WriteFile(tmpFilePath, videoData, 0644); err != nil {
		s.logger.Warn("Failed to write temporary file for transcoding", "video_id", videoID, "error", err)
		// 继续执行，不影响视频上传
	} else {
		// 8. 提交转码任务
		transcodeTask := &transcode.Task{
			ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
			VideoID:    strconv.FormatUint(uint64(videoID), 10),
			InputPath:  tmpFilePath,
			OutputPath: fmt.Sprintf("/tmp/transcode_output/%d", videoID),
			Qualities:  transcode.DefaultQualities(),
			Status:     "pending",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		// 异步执行转码任务
		go func(vid uint32, transcodeTask *transcode.Task, tmpFilePath string) {
			transcodeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
			defer cancel()
			defer os.Remove(tmpFilePath) // 清理临时文件

			s.logger.Info("Starting video transcoding", "video_id", vid)
			if err := s.transcodeService.TranscodeVideo(transcodeCtx, transcodeTask); err != nil {
				s.logger.Error("Failed to transcode video", "video_id", vid, "error", err)
				// 更新转码状态为失败
				if err := s.repo.UpdateVideoTranscodeStatus(transcodeCtx, vid, "failed", ""); err != nil {
					s.logger.Warn("Failed to update transcode status", "video_id", vid, "error", err)
				}
			} else {
				s.logger.Info("Video transcoding completed", "video_id", vid)
				// 上传转码结果到MinIO
				playlistURL, err := s.uploadTranscodeResult(transcodeCtx, vid, transcodeTask.OutputPath)
				if err != nil {
					s.logger.Error("Failed to upload transcode result", "video_id", vid, "error", err)
					// 更新转码状态为失败
					if err := s.repo.UpdateVideoTranscodeStatus(transcodeCtx, vid, "failed", ""); err != nil {
						s.logger.Warn("Failed to update transcode status", "video_id", vid, "error", err)
					}
					return
				}
				// 更新视频记录的PlaylistURL和TranscodeStatus
				if err := s.repo.UpdateVideoTranscodeStatus(transcodeCtx, vid, "completed", playlistURL); err != nil {
					s.logger.Warn("Failed to update transcode status", "video_id", vid, "error", err)
				}
			}
		}(videoID, transcodeTask, tmpFilePath)
	}

	// 添加日志记录，便于调试
	s.logger.Info("Video uploaded successfully",
		"video_id", videoID,
		"title", title,
		"cover_url", video.CoverURL,
		"video_url", video.VideoURL,
		"category", category,
		"tags", tagsStr)

	// 返回数据库生成的ID，以及URL
	return videoID, presignedURL, videoURL, nil
}

// getContentTypeFromFileName 根据文件名获取MIME类型
func getContentTypeFromFileName(fileName string) string {
	// 获取文件扩展名
	ext := filepath.Ext(fileName)
	// 转换为小写
	ext = strings.ToLower(ext)

	// 根据扩展名返回对应的MIME类型
	switch ext {
	case ".mp4":
		return "video/mp4"
	case ".avi":
		return "video/x-msvideo"
	case ".mov", ".qt":
		return "video/quicktime"
	case ".wmv":
		return "video/x-ms-wmv"
	case ".flv":
		return "video/x-flv"
	case ".mkv":
		return "video/x-matroska"
	case ".webm":
		return "video/webm"
	case ".m3u8":
		return "application/x-mpegURL"
	case ".ts":
		return "video/MP2T"
	default:
		// 默认返回MP4类型
		return "application/octet-stream"
	}
}

// uploadTranscodeResult 上传转码结果到MinIO
func (s *VideoService) uploadTranscodeResult(ctx context.Context, videoID uint32, outputPath string) (string, error) {
	// 遍历转码输出目录
	entries, err := os.ReadDir(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to read output directory: %w", err)
	}

	// 定义HLS主播放列表URL
	playlistURL := fmt.Sprintf("%d/hls/index.m3u8", videoID)

	// 上传所有转码文件
	for _, entry := range entries {
		if entry.IsDir() {
			// 递归上传子目录
			subDirPath := filepath.Join(outputPath, entry.Name())
			subEntries, err := os.ReadDir(subDirPath)
			if err != nil {
				s.logger.Warn("Failed to read subdirectory", "path", subDirPath, "error", err)
				continue
			}

			for _, subEntry := range subEntries {
				// 上传子目录中的文件
				filePath := filepath.Join(subDirPath, subEntry.Name())
				objectName := fmt.Sprintf("%d/hls/%s/%s", videoID, entry.Name(), subEntry.Name())
				if err := s.uploadFileToMinIO(ctx, filePath, objectName); err != nil {
					s.logger.Warn("Failed to upload file", "file_path", filePath, "object_name", objectName, "error", err)
					continue
				}
				s.logger.Info("File uploaded successfully", "file_path", filePath, "object_name", objectName)
			}
		} else {
			// 上传单个文件
			filePath := filepath.Join(outputPath, entry.Name())
			objectName := fmt.Sprintf("%d/hls/%s", videoID, entry.Name())
			if err := s.uploadFileToMinIO(ctx, filePath, objectName); err != nil {
				s.logger.Warn("Failed to upload file", "file_path", filePath, "object_name", objectName, "error", err)
				continue
			}
			s.logger.Info("File uploaded successfully", "file_path", filePath, "object_name", objectName)
		}
	}

	return playlistURL, nil
}

// uploadFileToMinIO 上传单个文件到MinIO
func (s *VideoService) uploadFileToMinIO(ctx context.Context, filePath, objectName string) error {
	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// 获取文件大小
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// 确定文件的MIME类型
	contentType := "application/octet-stream"
	ext := filepath.Ext(filePath)
	if ext != "" {
		contentType = getContentTypeFromFileName(filePath)
	}

	// 上传文件到MinIO
	_, err = s.minioClient.UploadFileFromReader(ctx, objectName, file, fileInfo.Size(), contentType)
	if err != nil {
		return fmt.Errorf("failed to upload file to MinIO: %w", err)
	}

	return nil
}

// PublishVideo 发布视频
func (s *VideoService) PublishVideo(ctx context.Context, userID string, videoID uint32, title, description string) error {
	// 添加日志记录
	s.logger.Info("Publishing video",
		"video_id", videoID,
		"title", title,
		"user_id", userID)

	// 先检查视频是否存在
	videoStrID := strconv.FormatUint(uint64(videoID), 10)
	video, err := s.repo.GetVideoByID(ctx, videoStrID)
	if err != nil {
		s.logger.Error("Video not found when publishing",
			"video_id", videoID,
			"error", err)
		return fmt.Errorf("failed to get video info: %w", err)
	}

	// 视频存在，更新状态为发布中
	err = s.repo.UpdateVideoStatus(ctx, videoID, "reviewing")
	if err != nil {
		s.logger.Error("Failed to update video status to reviewing",
			"video_id", videoID,
			"error", err)
		return fmt.Errorf("failed to update video status: %w", err)
	}

	// 发送审核消息到RabbitMQ队列，添加重试逻辑
	auditMessage := &queue.AuditMessage{
		ContentID:    fmt.Sprintf("video_%d", videoID),
		ContentType:  "video",
		Title:        title,
		URL:          video.PlayURL,
		Metadata:     description,
		UploaderID:   userID,
		UploaderName: userID, // TODO: 从用户信息获取用户名
	}

	// 最多重试3次发送审核消息
	maxRetries := 3
	var publishErr error
	for i := 0; i < maxRetries; i++ {
		publishErr = s.queueClient.PublishAuditMessage(ctx, auditMessage)
		if publishErr == nil {
			// 获取发布统计信息
			publishCount, errorCount := s.queueClient.GetPublishStats()
			s.logger.Info("Audit message published successfully",
				"video_id", videoID,
				"message_id", auditMessage.MessageID,
				"attempt", i+1,
				"total_published", publishCount,
				"total_errors", errorCount)
			break // 发送成功，退出重试循环
		}

		s.logger.Warn("Failed to publish audit message, retrying",
			"video_id", videoID,
			"message_id", auditMessage.MessageID,
			"attempt", i+1,
			"max_retries", maxRetries,
			"error", publishErr)

		if i < maxRetries-1 {
			// 等待一段时间后重试
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	if publishErr != nil {
		// 审核消息发送失败，回滚视频状态为uploading
		s.repo.UpdateVideoStatus(ctx, videoID, "uploading")

		// 获取发布统计信息
		publishCount, errorCount := s.queueClient.GetPublishStats()
		s.logger.Error("Failed to publish audit message after retries",
			"video_id", videoID,
			"message_id", auditMessage.MessageID,
			"error", publishErr,
			"total_published", publishCount,
			"total_errors", errorCount)

		return fmt.Errorf("failed to publish audit message: %w", publishErr)
	}

	s.logger.Info("Video published successfully",
		"video_id", videoID,
		"status", "reviewing")

	return nil
}
