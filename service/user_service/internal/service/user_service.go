package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"user_service/internal/cache"
	"user_service/internal/config"
	"user_service/internal/model"
	"user_service/internal/repository"
	"user_service/pkg/logger"
) // UserService 用户服务接口

type UserService interface {
	// 用户认证相关
	PhoneLogin(ctx context.Context, phone, password, deviceID, osType, appVersion string) (*model.User, string, error)
	CodeLogin(ctx context.Context, phone, code, deviceID, osType, appVersion string) (*model.User, string, error)
	SendSmsCode(ctx context.Context, phone string) error
	VerifyToken(ctx context.Context, token string) (uint32, error)
	RefreshToken(ctx context.Context, refreshToken string) (string, error)
	Logout(ctx context.Context, token string) error

	// 用户信息相关
	GetUserInfo(ctx context.Context, userID uint32) (*model.User, error)
	GetUserInfos(ctx context.Context, userIDs []uint32) ([]*model.User, error)
	UpdateUserInfo(ctx context.Context, userID uint32, updates map[string]interface{}) error
}

// userService 用户服务实现
type userService struct {
	config       *config.Config
	logger       logger.Logger
	userRepo     repository.UserRepository
	cacheService cache.CacheService
	authService  AuthService
	smsService   SmsService
}

// NewUserService 创建用户服务
func NewUserService(cfg *config.Config, log logger.Logger, userRepo repository.UserRepository, cacheService cache.CacheService, authService AuthService, smsService SmsService) UserService {
	return &userService{
		config:       cfg,
		logger:       log,
		userRepo:     userRepo,
		cacheService: cacheService,
		authService:  authService,
		smsService:   smsService,
	}
}

// PhoneLogin 手机号登录
func (s *userService) PhoneLogin(ctx context.Context, phone, password, deviceID, osType, appVersion string) (*model.User, string, error) {
	s.logger.Info("PhoneLogin service called", "phone", phone)

	// 验证手机号格式
	if err := s.validatePhoneNumber(phone); err != nil {
		return nil, "", fmt.Errorf("phone validation failed: %w", err)
	}

	// 验证密码格式
	if err := s.validatePassword(password); err != nil {
		return nil, "", fmt.Errorf("password validation failed: %w", err)
	}

	// 检查登录频率限制
	rateLimitKey := fmt.Sprintf("login_attempt:%s", phone)
	allowed, err := s.cacheService.CheckRateLimit(ctx, rateLimitKey, 5, time.Minute)
	if err != nil {
		s.logger.Error("Failed to check login rate limit", "phone", phone, "error", err)
		return nil, "", fmt.Errorf("failed to check rate limit: %w", err)
	}

	if !allowed {
		s.logger.Warn("Login attempt rate limit exceeded", "phone", phone)
		return nil, "", fmt.Errorf("登录尝试过于频繁，请稍后再试")
	}

	// 从数据库获取用户
	user, err := s.userRepo.GetByPhone(ctx, phone)
	if err != nil {
		s.logger.Error("Failed to query user", "error", err)
		return nil, "", errors.New("user not found")
	}

	// 将用户信息转换为缓存格式并存储到Redis
	userCache := &model.UserCache{
		UserID:          uint64(user.ID),
		Username:        user.Username,
		Nickname:        user.Nickname,
		AvatarURL:       user.AvatarURL,
		BackgroundImage: user.BackgroundImage,
		Signature:       user.Signature,
		IsVerified:      user.IsVerified,
		UserType:        user.UserType,
		Status:          user.Status,
		UpdatedAt:       user.UpdatedAt,
	}

	if cacheErr := s.cacheService.SetUser(ctx, user.ID, userCache, 30*time.Minute); cacheErr != nil {
		s.logger.Warn("Failed to cache user", "phone", phone, "error", cacheErr)
		// 不影响主流程，只记录警告
	}

	// 检查用户状态
	if user.Status != model.UserStatusActive {
		return nil, "", errors.New("user account is disabled")
	}

	// 验证密码（使用bcrypt加密比较）
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		s.logger.Error("Password verification failed", "error", err)
		return nil, "", errors.New("invalid password")
	}

	// 生成token
	token, err := s.authService.GenerateToken(ctx, user.ID)
	if err != nil {
		s.logger.Error("Failed to generate token", "error", err)
		return nil, "", fmt.Errorf("access token generation failed: %w", err)
	}

	// 更新用户信息（如最后登录时间等）
	updates := map[string]interface{}{
		"last_login_at": time.Now(),
		"updated_at":    time.Now(),
	}
	if err := s.userRepo.Update(ctx, user.ID, updates); err != nil {
		s.logger.Error("Failed to update user login info", "error", err)
	}

	// 清除用户缓存，确保登录状态更新
	if err := s.userRepo.DeleteUserCache(ctx, user.ID); err != nil {
		s.logger.Error("Failed to clear user cache", "error", err)
	}

	return user, token, nil
}

// CodeLogin 验证码登录
func (s *userService) CodeLogin(ctx context.Context, phone, code, deviceID, osType, appVersion string) (*model.User, string, error) {
	s.logger.Info("CodeLogin service called", "phone", phone)

	// 验证手机号格式
	if err := s.validatePhoneNumber(phone); err != nil {
		return nil, "", fmt.Errorf("phone validation failed: %w", err)
	}

	// 验证验证码格式
	if err := s.validateSmsCodeFormat(code); err != nil {
		return nil, "", fmt.Errorf("sms code validation failed: %w", err)
	}

	// 检查登录频率限制
	rateLimitKey := fmt.Sprintf("login_attempt:%s", phone)
	allowed, err := s.cacheService.CheckRateLimit(ctx, rateLimitKey, 5, time.Minute)
	if err != nil {
		s.logger.Error("Failed to check login rate limit", "phone", phone, "error", err)
		return nil, "", fmt.Errorf("failed to check rate limit: %w", err)
	}

	if !allowed {
		s.logger.Warn("Login attempt rate limit exceeded", "phone", phone)
		return nil, "", fmt.Errorf("登录尝试过于频繁，请稍后再试")
	}

	// 从缓存获取验证码
	cachedCode, err := s.cacheService.GetSmsCode(ctx, phone)
	if err != nil {
		s.logger.Error("Failed to get SMS code", "phone", phone, "error", err)
		return nil, "", fmt.Errorf("验证码不存在或已过期")
	}

	// 验证验证码
	if cachedCode != code {
		s.logger.Error("SMS code mismatch", "phone", phone, "cachedCode", cachedCode, "inputCode", code)
		return nil, "", fmt.Errorf("验证码错误")
	}

	// 删除已使用的验证码
	if err := s.cacheService.DeleteSmsCode(ctx, phone); err != nil {
		s.logger.Warn("Failed to delete used SMS code", "phone", phone, "error", err)
		// 不影响主流程，只记录警告
	}

	// 从数据库获取用户
	user, err := s.userRepo.GetByPhone(ctx, phone)
	if err != nil {
		s.logger.Error("没有注册过的用户，直接注册成功", "error", err)
		// 新用户，创建用户
		newUser := &model.User{
			Username:  "user_" + phone[7:], // 默认用户名
			Phone:     phone,
			Nickname:  "用户" + phone[7:], // 默认昵称
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

	// 将用户信息转换为缓存格式并存储到Redis
	userCache := &model.UserCache{
		UserID:          uint64(user.ID),
		Username:        user.Username,
		Nickname:        user.Nickname,
		AvatarURL:       user.AvatarURL,
		BackgroundImage: user.BackgroundImage,
		Signature:       user.Signature,
		IsVerified:      user.IsVerified,
		UserType:        user.UserType,
		Status:          user.Status,
		UpdatedAt:       user.UpdatedAt,
	}

	if cacheErr := s.cacheService.SetUser(ctx, user.ID, userCache, 30*time.Minute); cacheErr != nil {
		s.logger.Warn("Failed to cache user", "phone", phone, "error", cacheErr)
		// 不影响主流程，只记录警告
	} else {
		// 验证用户状态
		if !user.IsActive() {
			return nil, "", errors.New("user account is disabled")
		}
	}

	// 生成token
	token, err := s.authService.GenerateToken(ctx, user.ID)
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

	// 清除用户缓存，确保登录状态更新
	if err := s.userRepo.DeleteUserCache(ctx, user.ID); err != nil {
		s.logger.Error("Failed to clear user cache", "error", err)
	}

	return user, token, nil
}

// SendSmsCode 发送短信验证码
func (s *userService) SendSmsCode(ctx context.Context, phone string) error {
	s.logger.Info("SendSmsCode service called", "phone", phone)

	// 验证手机号格式
	if err := s.validatePhoneNumber(phone); err != nil {
		return fmt.Errorf("phone validation failed: %w", err)
	}

	// 检查发送频率限制
	rateLimitKey := fmt.Sprintf("sms_send:%s", phone)
	allowed, err := s.cacheService.CheckRateLimit(ctx, rateLimitKey, 1, time.Minute)
	if err != nil {
		s.logger.Error("Failed to check rate limit", "phone", phone, "error", err)
		return fmt.Errorf("failed to check rate limit: %w", err)
	}

	if !allowed {
		s.logger.Warn("SMS send rate limit exceeded", "phone", phone)
		return fmt.Errorf("发送过于频繁，请稍后再试")
	}

	// 生成6位验证码
	code := s.smsService.GenerateCode()

	// 发送验证码
	if err := s.smsService.SendCode(ctx, phone, code); err != nil {
		s.logger.Error("Failed to send SMS code", "error", err)
		return fmt.Errorf("sms send failed: %w", err)
	}

	// 使用缓存服务存储验证码，5分钟有效
	if err := s.cacheService.SetSmsCode(ctx, phone, code, 5*time.Minute); err != nil {
		s.logger.Error("Failed to cache SMS code", "error", err)
		return fmt.Errorf("cache set failed: %w", err)
	}

	s.logger.Info("SMS code sent successfully", "phone", phone)
	return nil
}

// VerifyToken 验证Token
func (s *userService) VerifyToken(ctx context.Context, token string) (uint32, error) {
	s.logger.Info("VerifyToken service called")

	// 验证token格式
	if token == "" {
		return 0, errors.New("token cannot be empty")
	}

	// 验证token
	userID, err := s.authService.VerifyToken(token)
	if err != nil {
		s.logger.Error("Token parsing failed", "error", err)
		return 0, fmt.Errorf("token verification failed: %w", err)
	}

	// 从数据库获取用户
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, errors.New("user not found")
		}
		s.logger.Error("Failed to get user", "error", err)
		return 0, errors.New("database error")
	}

	// 将用户信息转换为缓存格式并存储到Redis
	userCache := &model.UserCache{
		UserID:          uint64(user.ID),
		Username:        user.Username,
		Nickname:        user.Nickname,
		AvatarURL:       user.AvatarURL,
		BackgroundImage: user.BackgroundImage,
		Signature:       user.Signature,
		IsVerified:      user.IsVerified,
		UserType:        user.UserType,
		Status:          user.Status,
		UpdatedAt:       user.UpdatedAt,
	}

	if cacheErr := s.cacheService.SetUser(ctx, user.ID, userCache, 30*time.Minute); cacheErr != nil {
		s.logger.Warn("Failed to cache user", "userID", userID, "error", cacheErr)
		// 不影响主流程，只记录警告
	}

	// 检查用户状态
	if user.Status != model.UserStatusActive {
		return 0, errors.New("user account is disabled")
	}

	return userID, nil
}

// RefreshToken 刷新token
func (s *userService) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	// 验证refresh token格式
	if err := s.validateToken(refreshToken); err != nil {
		s.logger.Error("Invalid refresh token format", "error", err)
		return "", fmt.Errorf("invalid refresh token format: %w", err)
	}

	// 解析refresh token
	userID, err := s.authService.ParseRefreshToken(refreshToken)
	if err != nil {
		s.logger.Error("Failed to parse refresh token", "error", err)
		return "", fmt.Errorf("failed to parse refresh token: %w", err)
	}

	// 从数据库获取用户
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user by ID", "userID", userID, "error", err)
		return "", fmt.Errorf("failed to get user: %w", err)
	}

	// 将用户信息转换为缓存格式并存储到Redis
	userCache := &model.UserCache{
		UserID:          uint64(user.ID),
		Username:        user.Username,
		Nickname:        user.Nickname,
		AvatarURL:       user.AvatarURL,
		BackgroundImage: user.BackgroundImage,
		Signature:       user.Signature,
		IsVerified:      user.IsVerified,
		UserType:        user.UserType,
		Status:          user.Status,
		UpdatedAt:       user.UpdatedAt,
	}

	if cacheErr := s.cacheService.SetUser(ctx, user.ID, userCache, 30*time.Minute); cacheErr != nil {
		s.logger.Warn("Failed to cache user", "userID", userID, "error", cacheErr)
		// 不影响主流程，只记录警告
	}

	// 检查用户状态
	if user.Status != model.UserStatusActive {
		s.logger.Error("User account is not active", "userID", userID, "status", user.Status)
		return "", fmt.Errorf("account is not active")
	}

	// 生成新的token
	newToken, err := s.authService.GenerateToken(ctx, user.ID)
	if err != nil {
		s.logger.Error("Failed to generate token", "userID", user.ID, "error", err)
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	// 生成新的refresh token
	newRefreshToken, err := s.authService.GenerateRefreshToken(ctx, user.ID)
	if err != nil {
		s.logger.Error("Failed to generate refresh token", "userID", user.ID, "error", err)
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// 将新的refresh token存储在缓存中，以便后续验证
	refreshTokenKey := fmt.Sprintf("refresh_token:%d", user.ID)
	if err := s.cacheService.Set(ctx, refreshTokenKey, newRefreshToken, 7*24*time.Hour); err != nil {
		s.logger.Warn("Failed to cache refresh token", "userID", user.ID, "error", err)
		// 不影响主流程，只记录警告
	}

	// 返回新的token和refresh token，用特殊分隔符分隔
	return fmt.Sprintf("%s|%s", newToken, newRefreshToken), nil
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

// Logout 用户退出登录
func (s *userService) Logout(ctx context.Context, token string) error {
	s.logger.Info("Logout service called")

	// 验证token格式
	if err := s.validateToken(token); err != nil {
		s.logger.Error("Invalid token format", "error", err)
		return err
	}

	// 从token中解析用户ID
	userID, err := s.authService.VerifyToken(token)
	if err != nil {
		s.logger.Error("Failed to verify token", "error", err)
		return errors.New("invalid token")
	}

	// 将token加入黑名单
	if err := s.authService.InvalidateToken(ctx, token); err != nil {
		s.logger.Error("Failed to invalidate token", "error", err)
		// 不阻断流程，继续执行
	}

	// 清除用户缓存
	if err := s.userRepo.DeleteUserCache(ctx, userID); err != nil {
		s.logger.Error("Failed to clear user cache", "error", err)
		// 不阻断流程，继续执行
	}

	s.logger.Info("User logged out successfully", "userID", userID)
	return nil
}

// 辅助方法

func (s *userService) isValidPhone(phone string) bool {
	// 简单的手机号格式验证（中国大陆手机号）
	if len(phone) != 11 {
		return false
	}
	// 检查是否以1开头，第二位是3-9之间的数字
	if phone[0] != '1' || phone[1] < '3' || phone[1] > '9' {
		return false
	}
	// 检查是否全为数字
	for _, ch := range phone {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// HashPassword 生成密码哈希
func (s *userService) HashPassword(password string) (string, error) {
	// 使用bcrypt生成密码哈希，默认cost为10
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("Failed to hash password", "error", err)
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword 验证密码
func (s *userService) VerifyPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
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

	s.logger.Info("SMS code validated successfully", "phone", phone)
	return nil
}

// validatePhoneNumber 验证手机号格式
func (s *userService) validatePhoneNumber(phone string) error {
	if phone == "" {
		return errors.New("phone number cannot be empty")
	}

	// 中国大陆手机号正则表达式
	pattern := `^1[3-9]\d{9}$`
	matched, err := regexp.MatchString(pattern, phone)
	if err != nil {
		return fmt.Errorf("phone validation regex error: %w", err)
	}
	if !matched {
		return errors.New("invalid phone number format")
	}

	return nil
}

// validatePassword 验证密码格式
func (s *userService) validatePassword(password string) error {
	if password == "" {
		return errors.New("password cannot be empty")
	}

	if len(password) < 6 {
		return errors.New("password must be at least 6 characters")
	}

	if len(password) > 20 {
		return errors.New("password must be less than 20 characters")
	}

	return nil
}

// validateSmsCodeFormat 验证短信验证码格式
func (s *userService) validateSmsCodeFormat(code string) error {
	if code == "" {
		return errors.New("verification code cannot be empty")
	}

	if len(code) != 6 {
		return errors.New("verification code must be 6 digits")
	}

	pattern := `^\d{6}$`
	matched, err := regexp.MatchString(pattern, code)
	if err != nil {
		return fmt.Errorf("code validation regex error: %w", err)
	}
	if !matched {
		return errors.New("verification code must contain only digits")
	}

	return nil
}

// validateToken 验证token格式
func (s *userService) validateToken(token string) error {
	if token == "" {
		return errors.New("token cannot be empty")
	}

	// JWT token通常由三部分组成，用点分隔
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("invalid token format")
	}

	return nil
}
