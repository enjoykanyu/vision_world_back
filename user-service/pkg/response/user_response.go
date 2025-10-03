package response

import (
	pb "github.com/visionworld/user-service/proto"
	"google.golang.org/grpc/codes"
)

// Login响应
func NewLoginSuccessResponse(accessToken, refreshToken string, userInfo *pb.UserInfo) *pb.LoginResponse {
	return &pb.LoginResponse{
		Code:    0,
		Message: "登录成功",
		Data: &pb.LoginData{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    7200, // 2小时
			UserInfo:     userInfo,
		},
	}
}

func NewLoginErrorResponse(code codes.Code, message string) *pb.LoginResponse {
	return &pb.LoginResponse{
		Code:    int32(code),
		Message: message,
		Data:    nil,
	}
}

// RefreshToken响应
func NewRefreshTokenSuccessResponse(accessToken string) *pb.RefreshTokenResponse {
	return &pb.RefreshTokenResponse{
		Code:    0,
		Message: "刷新Token成功",
		Data: &pb.RefreshTokenData{
			AccessToken: accessToken,
			ExpiresIn:   7200, // 2小时
		},
	}
}

func NewRefreshTokenErrorResponse(code codes.Code, message string) *pb.RefreshTokenResponse {
	return &pb.RefreshTokenResponse{
		Code:    int32(code),
		Message: message,
		Data:    nil,
	}
}

// GetUserInfo响应
func NewGetUserInfoSuccessResponse(userInfo *pb.UserInfo) *pb.GetUserInfoResponse {
	return &pb.GetUserInfoResponse{
		Code:    0,
		Message: "获取用户信息成功",
		Data: &pb.GetUserInfoData{
			UserInfo: userInfo,
		},
	}
}

func NewGetUserInfoErrorResponse(code codes.Code, message string) *pb.GetUserInfoResponse {
	return &pb.GetUserInfoResponse{
		Code:    int32(code),
		Message: message,
		Data:    nil,
	}
}

// UpdateUserInfo响应
func NewUpdateUserInfoSuccessResponse() *pb.UpdateUserInfoResponse {
	return &pb.UpdateUserInfoResponse{
		Code:    0,
		Message: "更新用户信息成功",
	}
}

func NewUpdateUserInfoErrorResponse(code codes.Code, message string) *pb.UpdateUserInfoResponse {
	return &pb.UpdateUserInfoResponse{
		Code:    int32(code),
		Message: message,
	}
}

// Register响应
func NewRegisterSuccessResponse(userId string) *pb.RegisterResponse {
	return &pb.RegisterResponse{
		Code:    0,
		Message: "注册成功",
		Data: &pb.RegisterData{
			UserId: userId,
		},
	}
}

func NewRegisterErrorResponse(code codes.Code, message string) *pb.RegisterResponse {
	return &pb.RegisterResponse{
		Code:    int32(code),
		Message: message,
		Data:    nil,
	}
}
