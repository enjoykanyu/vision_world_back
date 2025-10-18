package handler

import (
	"context"
	"strings"
	"user_service/proto/proto_gen"

	"user_service/internal/cache"
	"user_service/internal/config"
	"user_service/internal/converter"
	"user_service/internal/repository"
	"user_service/internal/service"
	"user_service/pkg/logger"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// UserServiceHandler 用户服务处理器
type UserServiceHandler struct {
	proto_gen.UnimplementedUserServiceServer
	config      *config.Config
	logger      logger.Logger
	userService service.UserService
	converter   *converter.UserConverter
}

// NewUserServiceHandler 创建用户服务处理器
func NewUserServiceHandler(cfg *config.Config, log logger.Logger, db *gorm.DB, redis *redis.Client) *UserServiceHandler {
	// 创建认证服务
	refreshSecret := cfg.JWT.RefreshSecret
	if refreshSecret == "" {
		refreshSecret = cfg.JWT.Secret // 如果没有配置refresh_secret，使用secret作为替代
	}
	authService := service.NewAuthService(
		cfg.JWT.Secret,
		refreshSecret,
		cfg.JWT.TokenExpiration,
		cfg.JWT.RefreshExpiration,
	)

	// 创建短信服务
	smsService := service.NewSmsService(
		cfg.SMS.AccessKey,
		cfg.SMS.SecretKey,
		cfg.SMS.SignName,
		cfg.SMS.TemplateCode,
	)

	// 创建用户仓库
	userRepo := repository.NewUserRepository(db, redis)

	// 创建缓存服务
	cacheService := cache.NewCacheService(redis, log)

	// 创建用户服务
	userService := service.NewUserService(cfg, log, userRepo, cacheService, authService, smsService)

	return &UserServiceHandler{
		config:      cfg,
		logger:      log,
		userService: userService,
		converter:   converter.NewUserConverter(),
	}
}

// PhoneLogin 手机号登录
func (h *UserServiceHandler) PhoneLogin(ctx context.Context, req *proto_gen.PhoneLoginRequest) (*proto_gen.LoginResponse, error) {
	h.logger.Info("PhoneLogin called", "phone", req.Phone)

	// 调用用户服务进行登录
	_, token, err := h.userService.PhoneLogin(ctx, req.Phone, req.Password, req.DeviceId, req.OsType, req.AppVersion)
	if err != nil {
		h.logger.Error("PhoneLogin failed", "error", err, "phone", req.Phone)
		return &proto_gen.LoginResponse{
			StatusCode: 400,
			StatusMsg:  err.Error(),
		}, nil
	}

	return &proto_gen.LoginResponse{
		StatusCode: 0,
		StatusMsg:  "登录成功",
		//User:     user,
		Token: token,
	}, nil
}

// CodeLogin 验证码登录
func (h *UserServiceHandler) CodeLogin(ctx context.Context, req *proto_gen.CodeLoginRequest) (*proto_gen.LoginResponse, error) {
	h.logger.Info("CodeLogin called", "phone", req.Phone)

	// 调用用户服务进行验证码登录
	_, token, err := h.userService.CodeLogin(ctx, req.Phone, req.Code, req.DeviceId, req.OsType, req.AppVersion)
	if err != nil {
		h.logger.Error("CodeLogin failed", "error", err, "phone", req.Phone)
		return &proto_gen.LoginResponse{
			StatusCode: 400,
			StatusMsg:  err.Error(),
		}, nil
	}

	return &proto_gen.LoginResponse{
		StatusCode: 0,
		StatusMsg:  "登录成功",
		//UserId:     user.ID,
		Token: token,
	}, nil
}

// SendSmsCode 发送短信验证码
func (h *UserServiceHandler) SendSmsCode(ctx context.Context, req *proto_gen.SendSmsRequest) (*proto_gen.SendSmsResponse, error) {
	h.logger.Info("SendSmsCode called", "phone", req.Phone)

	// 调用用户服务发送短信验证码
	if err := h.userService.SendSmsCode(ctx, req.Phone); err != nil {
		h.logger.Error("SendSmsCode failed", "error", err, "phone", req.Phone)
		return &proto_gen.SendSmsResponse{
			StatusCode: 400,
			StatusMsg:  err.Error(),
		}, nil
	}

	return &proto_gen.SendSmsResponse{
		StatusCode: 0,
		StatusMsg:  "验证码发送成功",
	}, nil
}

// VerifyToken 验证Token
func (h *UserServiceHandler) VerifyToken(ctx context.Context, req *proto_gen.VerifyTokenRequest) (*proto_gen.VerifyTokenResponse, error) {
	h.logger.Info("VerifyToken called", "token", req.Token)

	// 调用用户服务验证token
	userID, err := h.userService.VerifyToken(ctx, req.Token)
	if err != nil {
		h.logger.Error("VerifyToken failed", "error", err)
		return &proto_gen.VerifyTokenResponse{
			StatusCode: 400,
			StatusMsg:  err.Error(),
			UserId:     0,
		}, nil
	}

	return &proto_gen.VerifyTokenResponse{
		StatusCode: 0,
		StatusMsg:  "token验证成功",
		UserId:     userID,
	}, nil
}

// RefreshToken 刷新Token
func (h *UserServiceHandler) RefreshToken(ctx context.Context, req *proto_gen.RefreshTokenRequest) (*proto_gen.RefreshTokenResponse, error) {
	h.logger.Info("RefreshToken called", "refresh_token", req.RefreshToken)

	// 调用用户服务刷新token
	tokenResponse, err := h.userService.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		h.logger.Error("RefreshToken failed", "error", err)
		return &proto_gen.RefreshTokenResponse{
			StatusCode: 400,
			StatusMsg:  err.Error(),
			Token:      "",
		}, nil
	}

	// 解析返回的token和refresh_token
	parts := strings.Split(tokenResponse, "|")
	if len(parts) != 2 {
		h.logger.Error("Invalid token response format", "response", tokenResponse)
		return &proto_gen.RefreshTokenResponse{
			StatusCode: 500,
			StatusMsg:  "服务器内部错误",
			Token:      "",
		}, nil
	}

	newToken := parts[0]
	newRefreshToken := parts[1]

	return &proto_gen.RefreshTokenResponse{
		StatusCode:   0,
		StatusMsg:    "token刷新成功",
		Token:        newToken,
		RefreshToken: newRefreshToken,
	}, nil
}

// GetUserInfo 获取用户信息
func (h *UserServiceHandler) GetUserInfo(ctx context.Context, req *proto_gen.GetUserInfoRequest) (*proto_gen.UserResponse, error) {
	h.logger.Info("GetUserInfo called", "user_id", req.UserId)
	converter := converter.NewUserConverter()

	// 调用用户服务获取用户信息
	user, err := h.userService.GetUserInfo(ctx, req.UserId)
	if err != nil {
		h.logger.Error("GetUserInfo failed", "error", err, "user_id", req.UserId)
		return &proto_gen.UserResponse{
			StatusCode: 404,
			StatusMsg:  err.Error(),
			User:       nil,
		}, nil
	}

	return &proto_gen.UserResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		User:       converter.ModelToProto(user),
	}, nil
}

// GetUserInfos 批量获取用户信息
func (h *UserServiceHandler) GetUserInfos(ctx context.Context, req *proto_gen.GetUserInfosRequest) (*proto_gen.GetUserInfosResponse, error) {
	h.logger.Info("GetUserInfos called", "user_ids", req.UserIds)

	// 调用用户服务批量获取用户信息
	_, err := h.userService.GetUserInfos(ctx, req.UserIds)
	if err != nil {
		h.logger.Error("GetUserInfos failed", "error", err)
		return &proto_gen.GetUserInfosResponse{
			StatusCode: 400,
			StatusMsg:  err.Error(),
			Users:      nil,
		}, nil
	}

	// 转换用户列表到protobuf格式
	//protoUsers := make([]*proto_gen.User, len(users))
	//for i, user := range users {
	//	//protoUsers[i] = user.ToProto()
	//}

	return &proto_gen.GetUserInfosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		//Users:      protoUsers,
	}, nil
}

// UpdateUserInfo 更新用户信息
func (h *UserServiceHandler) UpdateUserInfo(ctx context.Context, req *proto_gen.UpdateUserRequest) (*proto_gen.UpdateUserResponse, error) {
	//h.logger.Info("UpdateUserInfo called", "user_id", req.UserId)

	//// 调用用户服务更新用户信息
	//if err := h.userService.UpdateUserInfo(ctx, req.UserId, req); err != nil {
	//	h.logger.Error("UpdateUserInfo failed", "error", err, "user_id", req.UserId)
	//	return &proto_gen.UpdateUserResponse{
	//		StatusCode: 400,
	//		StatusMsg:  err.Error(),
	//	}, nil
	//}

	return &proto_gen.UpdateUserResponse{
		StatusCode: 0,
		StatusMsg:  "用户信息更新成功",
	}, nil
}

// GetUserExistInformation 检查用户是否存在
func (h *UserServiceHandler) GetUserExistInformation(ctx context.Context, req *proto_gen.UserExistRequest) (*proto_gen.UserExistResponse, error) {
	h.logger.Info("GetUserExistInformation called", "user_id", req.UserId)

	// 调用用户服务检查用户是否存在
	//exists, err := h.userService.CheckUserExists(ctx, req.UserId)
	//if err != nil {
	//	h.logger.Error("GetUserExistInformation failed", "error", err, "user_id", req.UserId)
	//	return &proto_gen.UserExistResponse{
	//		StatusCode: 500,
	//		//StatusMsg:  err.Error(),
	//		//Exist:      false,
	//	}, nil
	//}

	return &proto_gen.UserExistResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		//Exist:      exists,
	}, nil
}

// Logout 用户退出登录
func (h *UserServiceHandler) Logout(ctx context.Context, req *proto_gen.LogoutRequest) (*proto_gen.LogoutResponse, error) {
	h.logger.Info("Logout called", "token", req.Token)

	// 调用用户服务进行退出登录
	if err := h.userService.Logout(ctx, req.Token); err != nil {
		h.logger.Error("Logout failed", "error", err)
		return &proto_gen.LogoutResponse{
			StatusCode: 400,
			StatusMsg:  err.Error(),
		}, nil
	}

	return &proto_gen.LogoutResponse{
		StatusCode: 0,
		StatusMsg:  "退出登录成功",
	}, nil
}
