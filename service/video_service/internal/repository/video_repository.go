package repository

import (
	"fmt"

	"github.com/vision_world/video_service/internal/config"
	"github.com/vision_world/video_service/internal/model"
	"github.com/vision_world/video_service/pkg/database"
	"github.com/vision_world/video_service/pkg/logger"
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

// TODO: 实现具体的数据访问方法
// 这些方法将被service层调用，具体实现由你后续完成
// 例如：
// - CreateVideo()
// - GetVideoByID()
// - GetVideosByIDs()
// - GetUserVideos()
// - GetRecommendVideos()
// - GetFollowVideos()
// - LikeVideo()
// - UnlikeVideo()
// - GetUserLikedVideos()
// - CommentVideo()
// - DeleteComment()
// - GetVideoComments()
// - ShareVideo()
// - UpdateVideoStats()
// - GetVideoByCategory()
// - SearchVideos()
// 等等...
