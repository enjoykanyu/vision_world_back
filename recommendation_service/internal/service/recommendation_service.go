package service

import (
	"context"
	"recommendation_service/internal/config"
	"recommendation_service/internal/model"
	"recommendation_service/internal/repository"
	"recommendation_service/pkg/logger"
) // UserService 用户服务接口

type UserService interface {
	// 用户认证相关
	PhoneLogin(ctx context.Context, phone, password, deviceID, osType, appVersion string) (*model.User, string, error)
	CodeLogin(ctx context.Context, phone, code, deviceID, osType, appVersion string) (*model.User, string, error)
	SendSmsCode(ctx context.Context, phone string) error
	VerifyToken(ctx context.Context, token string) (uint32, error)
	RefreshToken(ctx context.Context, refreshToken string) (string, error)

	// 用户信息相关
	GetUserInfo(ctx context.Context, userID uint32) (*model.User, error)
	GetUserInfos(ctx context.Context, userIDs []uint32) ([]*model.User, error)
	UpdateUserInfo(ctx context.Context, userID uint32, updates map[string]interface{}) error
}

// userService 用户服务实现
type userService struct {
	config   *config.Config
	logger   logger.Logger
	userRepo repository.UserRepository
}
