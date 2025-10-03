package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/visionworld/user-service/internal/config"
	"github.com/visionworld/user-service/internal/model"
	"github.com/visionworld/user-service/pkg/crypto"
	"github.com/visionworld/user-service/pkg/jwt"
	"github.com/visionworld/user-service/pkg/logger"
	"github.com/visionworld/user-service/pkg/response"
	pb "github.com/visionworld/user-service/proto"
)

// UserService 用户服务
type UserService struct {
	pb.UnimplementedUserServiceServer
	db         *sql.DB
	redis      *redis.Client
	jwtManager *jwt.Manager
	config     *config.Config
	logger     *zap.Logger
	userModel  *model.UserModel
}

// NewUserService 创建用户服务
func NewUserService(db *sql.DB, redis *redis.Client, jwtManager *jwt.Manager, cfg *config.Config) *UserService {
	return &UserService{
		db:         db,
		redis:      redis,
		jwtManager: jwtManager,
		config:     cfg,
		logger:     logger.GetLogger(),
		userModel:  model.NewUserModel(db),
	}
}

// LoginByPhone 手机号登录
func (s *UserService) LoginByPhone(ctx context.Context, req *pb.LoginByPhoneRequest) (*pb.LoginResponse, error) {
	// 参数验证
	if req.Phone == "" {
		return response.NewLoginErrorResponse(codes.InvalidArgument, "手机号不能为空"), nil
	}
	if req.Password == "" {
		return response.NewLoginErrorResponse(codes.InvalidArgument, "密码不能为空"), nil
	}

	// 获取用户信息
	user, err := s.userModel.GetByPhone(ctx, req.Phone)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return response.NewLoginErrorResponse(codes.NotFound, "用户不存在"), nil
		}
		s.logger.Error("获取用户信息失败", zap.Error(err), zap.String("phone", req.Phone))
		return response.NewLoginErrorResponse(codes.Internal, "系统错误"), nil
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return response.NewLoginErrorResponse(codes.Unauthenticated, "密码错误"), nil
	}

	// 生成Token
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Phone)
	if err != nil {
		s.logger.Error("生成访问Token失败", zap.Error(err), zap.String("userID", user.ID))
		return response.NewLoginErrorResponse(codes.Internal, "系统错误"), nil
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID)
	if err != nil {
		s.logger.Error("生成刷新Token失败", zap.Error(err), zap.String("userID", user.ID))
		return response.NewLoginErrorResponse(codes.Internal, "系统错误"), nil
	}

	// 构建用户信息
	userInfo := &pb.UserInfo{
		UserId:    user.UserID,
		Phone:     user.Phone,
		Nickname:  user.Nickname,
		Avatar:    user.Avatar,
		Gender:    int32(user.Gender),
		Birthday:  user.Birthday,
		Status:    int32(user.Status),
		CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: user.UpdatedAt.Format("2006-01-02 15:04:05"),
	}

	// 更新最后登录时间
	if err := s.userModel.UpdateLastLoginTime(ctx, user.ID); err != nil {
		s.logger.Error("更新最后登录时间失败", zap.Error(err), zap.String("userID", user.ID))
	}

	// 保存刷新Token到Redis
	refreshKey := fmt.Sprintf("refresh_token:%s", user.UserID)
	if err := s.redis.Set(ctx, refreshKey, refreshToken, time.Duration(s.config.JWT.RefreshTokenExpire)*time.Second).Err(); err != nil {
		s.logger.Error("保存刷新Token失败", zap.Error(err), zap.String("userID", user.UserID))
	}

	return response.NewLoginSuccessResponse(accessToken, refreshToken, userInfo), nil
}

// Logout 登出
func (s *UserService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	// 参数验证
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "用户ID不能为空")
	}

	// 删除Redis中的刷新Token
	refreshKey := fmt.Sprintf("refresh_token:%s", req.UserId)
	if err := s.redis.Del(ctx, refreshKey).Err(); err != nil {
		s.logger.Error("删除刷新Token失败", zap.Error(err), zap.String("userID", req.UserId))
		return nil, status.Error(codes.Internal, "系统错误")
	}

	return &pb.LogoutResponse{
		Code:    0,
		Message: "登出成功",
	}, nil
}

// RefreshToken 刷新Token
func (s *UserService) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	// 参数验证
	if req.RefreshToken == "" {
		return response.NewRefreshTokenErrorResponse(codes.InvalidArgument, "刷新Token不能为空"), nil
	}

	// 解析刷新Token
	claims, err := s.jwtManager.ParseRefreshToken(req.RefreshToken)
	if err != nil {
		return response.NewRefreshTokenErrorResponse(codes.Unauthenticated, "无效的刷新Token"), nil
	}

	// 验证Redis中的刷新Token
	userID := claims.UserID
	refreshKey := fmt.Sprintf("refresh_token:%s", userID)
	storedToken, err := s.redis.Get(ctx, refreshKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return response.NewRefreshTokenErrorResponse(codes.Unauthenticated, "刷新Token已过期"), nil
		}
		s.logger.Error("获取刷新Token失败", zap.Error(err), zap.String("userID", userID))
		return response.NewRefreshTokenErrorResponse(codes.Internal, "系统错误"), nil
	}

	if storedToken != req.RefreshToken {
		return response.NewRefreshTokenErrorResponse(codes.Unauthenticated, "刷新Token不匹配"), nil
	}

	// 获取用户信息
	user, err := s.userModel.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return response.NewRefreshTokenErrorResponse(codes.NotFound, "用户不存在"), nil
		}
		s.logger.Error("获取用户信息失败", zap.Error(err), zap.String("userID", userID))
		return response.NewRefreshTokenErrorResponse(codes.Internal, "系统错误"), nil
	}

	// 生成新的访问Token
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Phone)
	if err != nil {
		s.logger.Error("生成访问Token失败", zap.Error(err), zap.String("userID", user.ID))
		return response.NewRefreshTokenErrorResponse(codes.Internal, "系统错误"), nil
	}

	return response.NewRefreshTokenSuccessResponse(accessToken), nil
}

// GetUserInfo 获取用户信息
func (s *UserService) GetUserInfo(ctx context.Context, req *pb.GetUserInfoRequest) (*pb.GetUserInfoResponse, error) {
	// 参数验证
	if req.UserId == "" {
		return response.NewGetUserInfoErrorResponse(codes.InvalidArgument, "用户ID不能为空"), nil
	}

	// 获取用户信息
	user, err := s.userModel.GetByID(ctx, req.UserId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return response.NewGetUserInfoErrorResponse(codes.NotFound, "用户不存在"), nil
		}
		s.logger.Error("获取用户信息失败", zap.Error(err), zap.String("userID", req.UserId))
		return response.NewGetUserInfoErrorResponse(codes.Internal, "系统错误"), nil
	}

	// 构建用户信息
	userInfo := &pb.UserInfo{
		UserId:    user.ID,
		Phone:     user.Phone,
		Nickname:  user.Nickname,
		Avatar:    user.Avatar,
		Gender:    int32(user.Gender),
		Birthday:  user.Birthday,
		Status:    int32(user.Status),
		CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: user.UpdatedAt.Format("2006-01-02 15:04:05"),
	}

	return response.NewGetUserInfoSuccessResponse(userInfo), nil
}

// UpdateUserInfo 更新用户信息
func (s *UserService) UpdateUserInfo(ctx context.Context, req *pb.UpdateUserInfoRequest) (*pb.UpdateUserInfoResponse, error) {
	// 参数验证
	if req.UserId == "" {
		return response.NewUpdateUserInfoErrorResponse(codes.InvalidArgument, "用户ID不能为空"), nil
	}

	// 构建更新数据
	updateData := map[string]interface{}{}
	if req.Nickname != "" {
		updateData["nickname"] = req.Nickname
	}
	if req.Avatar != "" {
		updateData["avatar"] = req.Avatar
	}
	if req.Gender > 0 {
		updateData["gender"] = int(req.Gender)
	}
	if req.Birthday != "" {
		updateData["birthday"] = req.Birthday
	}

	if len(updateData) == 0 {
		return response.NewUpdateUserInfoErrorResponse(codes.InvalidArgument, "没有需要更新的字段"), nil
	}

	// 更新用户信息
	if err := s.userModel.Update(ctx, req.UserId, updateData); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return response.NewUpdateUserInfoErrorResponse(codes.NotFound, "用户不存在"), nil
		}
		s.logger.Error("更新用户信息失败", zap.Error(err), zap.String("userID", req.UserId))
		return response.NewUpdateUserInfoErrorResponse(codes.Internal, "系统错误"), nil
	}

	return response.NewUpdateUserInfoSuccessResponse(), nil
}

// Register 用户注册
func (s *UserService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	// 参数验证
	if req.Phone == "" {
		return response.NewRegisterErrorResponse(codes.InvalidArgument, "手机号不能为空"), nil
	}
	if req.Password == "" {
		return response.NewRegisterErrorResponse(codes.InvalidArgument, "密码不能为空"), nil
	}
	if req.SmsCode == "" {
		return response.NewRegisterErrorResponse(codes.InvalidArgument, "验证码不能为空"), nil
	}
	if req.Nickname == "" {
		return response.NewRegisterErrorResponse(codes.InvalidArgument, "昵称不能为空"), nil
	}

	// 验证短信验证码
	smsKey := fmt.Sprintf("sms_code:%s", req.Phone)
	storedCode, err := s.redis.Get(ctx, smsKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return response.NewRegisterErrorResponse(codes.InvalidArgument, "验证码已过期"), nil
		}
		s.logger.Error("获取验证码失败", zap.Error(err), zap.String("phone", req.Phone))
		return response.NewRegisterErrorResponse(codes.Internal, "系统错误"), nil
	}

	if storedCode != req.SmsCode {
		return response.NewRegisterErrorResponse(codes.InvalidArgument, "验证码错误"), nil
	}

	// 检查手机号是否已注册
	if _, err := s.userModel.GetByPhone(ctx, req.Phone); err == nil {
		return response.NewRegisterErrorResponse(codes.AlreadyExists, "手机号已注册"), nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		s.logger.Error("检查手机号失败", zap.Error(err), zap.String("phone", req.Phone))
		return response.NewRegisterErrorResponse(codes.Internal, "系统错误"), nil
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.config.Security.BcryptCost)
	if err != nil {
		s.logger.Error("加密密码失败", zap.Error(err))
		return response.NewRegisterErrorResponse(codes.Internal, "系统错误"), nil
	}

	// 创建用户
	user := &model.User{
		ID:        crypto.GenerateUUID(),
		Phone:     req.Phone,
		Password:  string(hashedPassword),
		Nickname:  req.Nickname,
		Avatar:    req.Avatar,
		Gender:    int(req.Gender),
		Birthday:  req.Birthday,
		Status:    1, // 正常状态
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.userModel.Create(ctx, user); err != nil {
		s.logger.Error("创建用户失败", zap.Error(err))
		return response.NewRegisterErrorResponse(codes.Internal, "系统错误"), nil
	}

	// 删除验证码
	if err := s.redis.Del(ctx, smsKey).Err(); err != nil {
		s.logger.Error("删除验证码失败", zap.Error(err), zap.String("phone", req.Phone))
	}

	return response.NewRegisterSuccessResponse(user.ID), nil
}
