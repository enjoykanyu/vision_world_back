package service

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"social_service/internal/config"
	"social_service/internal/model"
	"social_service/internal/repository"
	"social_service/pkg/logger"
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
	config      *config.Config
	logger      logger.Logger
	userRepo    repository.UserRepository
	authService AuthService
	smsService  SmsService
}

// NewUserService 创建用户服务
func NewUserService(cfg *config.Config, log logger.Logger, userRepo repository.UserRepository, authService AuthService, smsService SmsService) UserService {
	return &userService{
		config:      cfg,
		logger:      log,
		userRepo:    userRepo,
		authService: authService,
		smsService:  smsService,
	}
}

// PhoneLogin 手机号登录
func (s *userService) PhoneLogin(ctx context.Context, phone, password, deviceID, osType, appVersion string) (*model.User, string, error) {
	s.logger.Info("PhoneLogin service called", "phone", phone)

	// 验证手机号格式
	if !s.isValidPhone(phone) {
		return nil, "", errors.New("invalid phone format")
	}

	// 查找用户
	user, err := s.userRepo.GetByPhone(ctx, phone)
	if err != nil {
		s.logger.Error("Failed to query user", "error", err)
		return nil, "", errors.New("user not found")
	}

	// 验证密码（这里应该使用加密后的密码比较）
	if user.PasswordHash != password {
		return nil, "", errors.New("invalid password")
	}

	// 生成token
	token, err := s.authService.GenerateToken(uint32(user.ID))
	if err != nil {
		s.logger.Error("Failed to generate token", "error", err)
		return nil, "", errors.New("token generation failed")
	}

	// 更新用户信息（如最后登录时间等）
	updates := map[string]interface{}{
		"last_login_at": time.Now(),
		"updated_at":    time.Now(),
	}
	if err := s.userRepo.Update(ctx, user.ID, updates); err != nil {
		s.logger.Error("Failed to update user login info", "error", err)
	}

	return user, token, nil
}

// CodeLogin 验证码登录
func (s *userService) CodeLogin(ctx context.Context, phone, code, deviceID, osType, appVersion string) (*model.User, string, error) {
	s.logger.Info("CodeLogin service called", "phone", phone)

	// 验证手机号格式
	if !s.isValidPhone(phone) {
		return nil, "", errors.New("invalid phone format")
	}

	// 验证验证码
	if err := s.validateSmsCode(ctx, phone, code); err != nil {
		return nil, "", err
	}

	// 查找或创建用户
	user, err := s.userRepo.GetByPhone(ctx, phone)
	if err != nil {
		// 新用户，创建用户
		newUser := &model.User{
			Phone:     phone,
			Status:    model.UserStatusActive,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := s.userRepo.Create(ctx, newUser); err != nil {
			s.logger.Error("Failed to create user", "error", err)
			return nil, "", errors.New("user creation failed")
		}
		user = newUser
	}

	// 生成token
	token, err := s.authService.GenerateToken(uint32(user.ID))
	if err != nil {
		s.logger.Error("Failed to generate token", "error", err)
		return nil, "", errors.New("token generation failed")
	}

	// 更新用户信息
	updates := map[string]interface{}{
		"last_login_at": time.Now(),
		"updated_at":    time.Now(),
	}
	if err := s.userRepo.Update(ctx, user.ID, updates); err != nil {
		s.logger.Error("Failed to update user login info", "error", err)
	}

	return user, token, nil
}

// SendSmsCode 发送短信验证码
func (s *userService) SendSmsCode(ctx context.Context, phone string) error {
	s.logger.Info("SendSmsCode service called", "phone", phone)

	// 验证手机号格式
	if !s.isValidPhone(phone) {
		return errors.New("invalid phone format")
	}

	// 生成6位验证码
	code := s.smsService.GenerateCode()

	// 发送验证码
	if err := s.smsService.SendCode(ctx, phone, code); err != nil {
		s.logger.Error("Failed to send SMS code", "error", err)
		return errors.New("sms send failed")
	}

	// 缓存验证码，5分钟有效
	if err := s.userRepo.SetSmsCode(ctx, phone, code, 5*time.Minute); err != nil {
		s.logger.Error("Failed to cache SMS code", "error", err)
		return errors.New("cache error")
	}

	return nil
}

// VerifyToken 验证Token
func (s *userService) VerifyToken(ctx context.Context, token string) (uint32, error) {
	s.logger.Info("VerifyToken service called")

	// 验证token
	userID, err := s.authService.ParseToken(token)
	if err != nil {
		return 0, errors.New("invalid token")
	}

	return userID, nil
}

// RefreshToken 刷新Token
func (s *userService) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	s.logger.Info("RefreshToken service called")

	// 验证refresh token
	userID, err := s.authService.VerifyRefreshToken(refreshToken)
	if err != nil {
		return "", errors.New("invalid refresh token")
	}

	// 生成新的token
	token, err := s.authService.GenerateToken(userID)
	if err != nil {
		return "", errors.New("token generation failed")
	}

	return token, nil
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

// 辅助方法

func (s *userService) isValidPhone(phone string) bool {
	// 简单的手机号格式验证
	if len(phone) != 11 {
		return false
	}
	// 这里应该使用更严格的正则表达式验证
	return true
}

func (s *userService) validateSmsCode(ctx context.Context, phone, code string) error {
	cachedCode, err := s.userRepo.GetSmsCode(ctx, phone)
	if err != nil {
		return errors.New("code expired or not found")
	}

	if cachedCode != code {
		return errors.New("invalid code")
	}

	// 验证成功后删除验证码
	if err := s.userRepo.DeleteSmsCode(ctx, phone); err != nil {
		s.logger.Error("Failed to delete SMS code", "error", err)
	}

	return nil
}
