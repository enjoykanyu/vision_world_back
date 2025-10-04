package response

import (
	pb "github.com/visionworld/user-service/proto"
	"google.golang.org/grpc/codes"
)

// Login响应
func NewLoginSuccessResponse(accessToken, refreshToken string, userInfo *pb.UserInfo) *pb.LoginByPhoneResponse {
	return &pb.LoginByPhoneResponse{
		User: userInfo,
		Tokens: &pb.TokenInfo{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    7200, // 2小时
		},
	}
}

func NewLoginErrorResponse(code codes.Code, message string) *pb.LoginByPhoneResponse {
	return &pb.LoginByPhoneResponse{
		User:   nil,
		Tokens: nil,
	}
}

// RefreshToken响应
func NewRefreshTokenSuccessResponse(accessToken string) *pb.RefreshTokenResponse {
	return &pb.RefreshTokenResponse{
		Tokens: &pb.TokenInfo{
			AccessToken: accessToken,
			ExpiresIn:   7200, // 2小时
		},
	}
}

func NewRefreshTokenErrorResponse(code codes.Code, message string) *pb.RefreshTokenResponse {
	return &pb.RefreshTokenResponse{
		Tokens: nil,
	}
}

// GetUserInfo响应
func NewGetUserInfoSuccessResponse(userInfo *pb.UserInfo) *pb.GetUserInfoResponse {
	return &pb.GetUserInfoResponse{
		User:       userInfo,
		IsFollowed: false,
	}
}

func NewGetUserInfoErrorResponse(code codes.Code, message string) *pb.GetUserInfoResponse {
	return &pb.GetUserInfoResponse{
		User:       nil,
		IsFollowed: false,
	}
}

// UpdateUserInfo响应
func NewUpdateUserInfoSuccessResponse() *pb.UpdateUserInfoResponse {
	return &pb.UpdateUserInfoResponse{
		User: nil,
	}
}

func NewUpdateUserInfoErrorResponse(code codes.Code, message string) *pb.UpdateUserInfoResponse {
	return &pb.UpdateUserInfoResponse{
		User: nil,
	}
}

// SendVerificationCode响应
func NewSendVerificationCodeSuccessResponse() *pb.SendVerificationCodeResponse {
	return &pb.SendVerificationCodeResponse{
		Success: true,
	}
}

func NewSendVerificationCodeErrorResponse(code codes.Code, message string) *pb.SendVerificationCodeResponse {
	return &pb.SendVerificationCodeResponse{
		Success: false,
	}
}
