package repository

import (
	"context"

	"github.com/go-redis/redis/v8"
	"github.com/vision_world/video_service/internal/model"
	"gorm.io/gorm"
)

// DanmakuRepository 弹幕仓库接口
type DanmakuRepository interface {
	// 创建弹幕
	CreateDanmaku(ctx context.Context, danmaku *model.Danmuku) error
	// 获取视频弹幕
	GetDanmakuByVideoID(ctx context.Context, videoID uint32) ([]*model.Danmuku, error)
}

// danmakuRepository 弹幕数据访问实现
type danmakuRepository struct {
	db    *gorm.DB
	redis *redis.Client
}

// NewDanmakuRepository 创建弹幕数据访问对象
func NewDanmakuRepository(db *gorm.DB, redis *redis.Client) DanmakuRepository {
	return &danmakuRepository{
		db:    db,
		redis: redis,
	}
}

// CreateDanmaku 创建弹幕
func (r *danmakuRepository) CreateDanmaku(ctx context.Context, danmaku *model.Danmuku) error {
	return r.db.WithContext(ctx).Create(danmaku).Error
}

func (r *danmakuRepository) GetDanmakuByVideoID(ctx context.Context, videoID uint32) ([]*model.Danmuku, error) {
	var danmaku []*model.Danmuku
	return danmaku, r.db.WithContext(ctx).Where("video_id = ?", videoID).Find(&danmaku).Error
}
