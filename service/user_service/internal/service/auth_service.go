package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// TokenClaims JWT claims
type TokenClaims struct {
	UserID uint32 `json:"user_id"`
	jwt.RegisteredClaims
}

// AuthService 认证服务接口
type AuthService interface {
	GenerateToken(ctx context.Context, userID uint32) (string, error)
	GenerateRefreshToken(ctx context.Context, userID uint32) (string, error)
	ParseToken(tokenString string) (uint32, error)
	ParseRefreshToken(tokenString string) (uint32, error)
	VerifyToken(tokenString string) (uint32, error)
	VerifyRefreshToken(tokenString string) (uint32, error)
	InvalidateToken(ctx context.Context, token string) error
	GetTokenExpiration() time.Duration
	GetRefreshTokenExpiration() time.Duration
}

// authService 认证服务实现
type authService struct {
	secretKey         string
	refreshSecretKey  string
	tokenExpiration   time.Duration
	refreshExpiration time.Duration
	issuer            string
	audience          string
}

// NewAuthService 创建认证服务
func NewAuthService(secretKey, refreshSecretKey string, tokenExpiration, refreshExpiration time.Duration) AuthService {
	return &authService{
		secretKey:         secretKey,
		refreshSecretKey:  refreshSecretKey,
		tokenExpiration:   tokenExpiration,
		refreshExpiration: refreshExpiration,
		issuer:            "vision-world-user-service",
		audience:          "vision-world-app",
	}
}

// GenerateToken 生成访问token
func (s *authService) GenerateToken(ctx context.Context, userID uint32) (string, error) {
	claims := TokenClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Audience:  jwt.ClaimStrings{s.audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.tokenExpiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.secretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// GenerateRefreshToken 生成刷新token
func (s *authService) GenerateRefreshToken(ctx context.Context, userID uint32) (string, error) {
	claims := TokenClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Audience:  jwt.ClaimStrings{s.audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.refreshExpiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.refreshSecretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return tokenString, nil
}

// ParseToken 解析访问token
func (s *authService) ParseToken(tokenString string) (uint32, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.secretKey), nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		return claims.UserID, nil
	}

	return 0, errors.New("invalid token")
}

// ParseRefreshToken 解析刷新token
func (s *authService) ParseRefreshToken(tokenString string) (uint32, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.refreshSecretKey), nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to parse refresh token: %w", err)
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		return claims.UserID, nil
	}

	return 0, errors.New("invalid refresh token")
}

// VerifyToken 验证访问token（兼容接口）
func (s *authService) VerifyToken(tokenString string) (uint32, error) {
	return s.ParseToken(tokenString)
}

// VerifyRefreshToken 验证刷新token（兼容接口）
func (s *authService) VerifyRefreshToken(tokenString string) (uint32, error) {
	return s.ParseRefreshToken(tokenString)
}

// GetTokenExpiration 获取访问token过期时间
func (s *authService) GetTokenExpiration() time.Duration {
	return s.tokenExpiration
}

// GetRefreshTokenExpiration 获取刷新token过期时间
func (s *authService) GetRefreshTokenExpiration() time.Duration {
	return s.refreshExpiration
}

// InvalidateToken 使token失效（加入黑名单）
func (s *authService) InvalidateToken(ctx context.Context, token string) error {
	// 解析token获取过期时间
	tokenObj, err := jwt.ParseWithClaims(token, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.secretKey), nil
	})

	if err != nil {
		return fmt.Errorf("failed to parse token for invalidation: %w", err)
	}

	if claims, ok := tokenObj.Claims.(*TokenClaims); ok && tokenObj.Valid {
		// 计算token剩余有效时间
		now := time.Now()
		expiresAt := claims.ExpiresAt.Time
		remainingTime := expiresAt.Sub(now)

		// 如果token已过期，无需加入黑名单
		if remainingTime <= 0 {
			return nil
		}

		// TODO: 将token加入Redis黑名单
		// 使用token的jti（JWT ID）作为key，设置过期时间为token剩余有效时间
		// key格式: "blacklist:token:{jti}"
		// 示例: redis.Set(ctx, fmt.Sprintf("blacklist:token:%s", claims.ID), "1", remainingTime)

		// 暂时只打印日志，表示token已被标记为失效
		fmt.Printf("Token invalidated for user %d, expires in %v\n", claims.UserID, remainingTime)
		return nil
	}

	return errors.New("invalid token for invalidation")
}
