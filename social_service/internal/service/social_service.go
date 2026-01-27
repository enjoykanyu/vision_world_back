package service

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/vision_world/social_service/internal/config"
	"github.com/vision_world/social_service/internal/model"
	"github.com/vision_world/social_service/internal/repository"
	"github.com/vision_world/social_service/pkg/logger"
)

// UserService 用户服务接口
type UserService interface {
	// 用户信息相关
	GetUserInfo(ctx context.Context, userID uint32) (*model.User, error)
	GetUserInfos(ctx context.Context, userIDs []uint32) ([]*model.User, error)
	UpdateUserInfo(ctx context.Context, userID uint32, updates map[string]interface{}) error

	// 弹幕相关
	SendDanmaku(ctx context.Context, userID uint32, videoID uint32, text string, color string, videoTimestamp float32, speed string) (*model.Danmaku, error)
	GetDanmakus(ctx context.Context, videoID uint32, page, pageSize int) ([]*model.Danmaku, int64, error)
}

// DanmakuService 弹幕服务接口
type DanmakuService interface {
	// 发送弹幕
	SendDanmaku(ctx context.Context, userID uint32, videoID uint32, text string, color string, videoTimestamp float32, speed string) (*model.Danmaku, error)
	// 获取视频弹幕列表
	GetDanmakus(ctx context.Context, videoID uint32, page, pageSize int) ([]*model.Danmaku, int64, error)
}

// userService 用户服务实现
type userService struct {
	config   *config.Config
	logger   logger.Logger
	userRepo repository.UserRepository
}

// NewUserService 创建用户服务
func NewUserService(cfg *config.Config, log logger.Logger, userRepo repository.UserRepository) *userService {
	return &userService{
		config:   cfg,
		logger:   log,
		userRepo: userRepo,
	}
}

// GetUserInfo 获取用户信息
func (s *userService) GetUserInfo(ctx context.Context, userID uint32) (*model.User, error) {
	s.logger.Info("GetUserInfo service called", "userID", userID)

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("user not found")
		}
		s.logger.Error("Failed to get user", "error", err)
		return nil, errors.New("database error")
	}

	return user, nil
}

// GetUserInfos 批量获取用户信息
func (s *userService) GetUserInfos(ctx context.Context, userIDs []uint32) ([]*model.User, error) {
	s.logger.Info("GetUserInfos service called", "count", len(userIDs))

	users, err := s.userRepo.GetByIDs(ctx, userIDs)
	if err != nil {
		s.logger.Error("Failed to get users", "error", err)
		return nil, errors.New("database error")
	}

	// 将map转换为slice
	result := make([]*model.User, 0, len(users))
	for _, user := range users {
		result = append(result, user)
	}

	return result, nil
}

// UpdateUserInfo 更新用户信息
func (s *userService) UpdateUserInfo(ctx context.Context, userID uint32, updates map[string]interface{}) error {
	s.logger.Info("UpdateUserInfo service called", "userID", userID)

	// 验证用户是否存在
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("user not found")
		}
		s.logger.Error("Failed to get user", "error", err)
		return errors.New("database error")
	}

	// 更新用户信息
	updates["updated_at"] = time.Now()
	if err := s.userRepo.Update(ctx, uint32(userID), updates); err != nil {
		s.logger.Error("Failed to update user", "error", err)
		return errors.New("update failed")
	}

	// 清除用户缓存
	if err := s.userRepo.DeleteUserCache(ctx, userID); err != nil {
		s.logger.Error("Failed to clear user cache", "error", err)
	}

	return nil
}

// GetUserExistInformation 检查用户是否存在
func (s *userService) GetUserExistInformation(ctx context.Context, userID uint32) (bool, error) {
	s.logger.Info("GetUserExistInformation service called", "userID", userID)

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		s.logger.Error("Failed to check user existence", "error", err)
		return false, errors.New("database error")
	}

	return user != nil && user.Status == model.UserStatusActive, nil
}

// Danmaku 相关方法

// SendDanmaku 发送弹幕
func (s *userService) SendDanmaku(ctx context.Context, userID uint32, videoID uint32, text string, color string, videoTimestamp float32, speed string) (*model.Danmaku, error) {
	s.logger.Info("SendDanmaku service called", "userID", userID, "videoID", videoID)

	// 参数验证
	if text == "" {
		return nil, errors.New("danmaku text cannot be empty")
	}

	if len(text) > 200 {
		return nil, errors.New("danmaku text too long")
	}

	// 设置默认值
	if color == "" {
		color = "#FFFFFF" // 默认白色
	}

	if speed == "" {
		speed = "normal" // 默认正常速度
	}

	// 创建弹幕
	danmaku := &model.Danmaku{
		UserID:         userID,
		VideoID:        videoID,
		Text:           text,
		Color:          color,
		VideoTimestamp: videoTimestamp,
		Speed:          speed,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// 保存到数据库
	if err := s.userRepo.CreateDanmaku(ctx, danmaku); err != nil {
		s.logger.Error("Failed to create danmaku", "error", err)
		return nil, errors.New("failed to send danmaku")
	}

	return danmaku, nil
}

// GetDanmakus 获取视频弹幕列表
func (s *userService) GetDanmakus(ctx context.Context, videoID uint32, page, pageSize int) ([]*model.Danmaku, int64, error) {
	s.logger.Info("GetDanmakus service called", "videoID", videoID, "page", page, "pageSize", pageSize)

	// 设置默认值
	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20 // 默认20条，最大100条
	}

	// 从数据库获取弹幕列表
	danmakus, total, err := s.userRepo.GetDanmakusByVideoID(ctx, videoID, page, pageSize)
	if err != nil {
		s.logger.Error("Failed to get danmakus", "error", err)
		return nil, 0, errors.New("failed to get danmakus")
	}

	return danmakus, total, nil
}
