package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/visionworld/user-service/internal/config"
	"github.com/visionworld/user-service/pkg/logger"
	"go.uber.org/zap"
)

// CustomClaims 自定义Claims
type CustomClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

// JWTManager JWT管理器
type JWTManager struct {
	secret             []byte
	accessTokenExpire  time.Duration
	refreshTokenExpire time.Duration
	issuer             string
}

// NewJWTManager 创建JWT管理器
func NewJWTManager(cfg *config.JWTConfig) *JWTManager {
	return &JWTManager{
		secret:             []byte(cfg.Secret),
		accessTokenExpire:  time.Duration(cfg.AccessTokenExpire) * time.Second,
		refreshTokenExpire: time.Duration(cfg.RefreshTokenExpire) * time.Second,
		issuer:             cfg.Issuer,
	}
}

// GenerateAccessToken 生成访问Token
func (j *JWTManager) GenerateAccessToken(userID, username, email string) (string, error) {
	claims := CustomClaims{
		UserID:   userID,
		Username: username,
		Email:    email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   "access_token",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.accessTokenExpire)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(j.secret)
	if err != nil {
		logger.Error("生成访问Token失败", zap.Error(err), zap.String("userID", userID))
		return "", fmt.Errorf("生成访问Token失败: %v", err)
	}

	return tokenString, nil
}

// GenerateRefreshToken 生成刷新Token
func (j *JWTManager) GenerateRefreshToken(userID string) (string, error) {
	claims := jwt.RegisteredClaims{
		Issuer:    j.issuer,
		Subject:   "refresh_token",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.refreshTokenExpire)),
		ID:        userID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(j.secret)
	if err != nil {
		logger.Error("生成刷新Token失败", zap.Error(err), zap.String("userID", userID))
		return "", fmt.Errorf("生成刷新Token失败: %v", err)
	}

	return tokenString, nil
}

// ParseToken 解析Token
func (j *JWTManager) ParseToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secret, nil
	})

	if err != nil {
		logger.Error("解析Token失败", zap.Error(err))
		return nil, fmt.Errorf("解析Token失败: %v", err)
	}

	if !token.Valid {
		logger.Error("Token无效")
		return nil, errors.New("Token无效")
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		logger.Error("Token声明类型错误")
		return nil, errors.New("Token声明类型错误")
	}

	return claims, nil
}

// ValidateToken 验证Token
func (j *JWTManager) ValidateToken(tokenString string) error {
	_, err := j.ParseToken(tokenString)
	return err
}

// RefreshAccessToken 刷新访问Token
func (j *JWTManager) RefreshAccessToken(refreshToken string) (string, error) {
	// 解析刷新Token
	token, err := jwt.ParseWithClaims(refreshToken, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secret, nil
	})

	if err != nil {
		logger.Error("解析刷新Token失败", zap.Error(err))
		return "", fmt.Errorf("解析刷新Token失败: %v", err)
	}

	if !token.Valid {
		logger.Error("刷新Token无效")
		return "", errors.New("刷新Token无效")
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok {
		logger.Error("刷新Token声明类型错误")
		return "", errors.New("刷新Token声明类型错误")
	}

	// 检查过期时间
	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
		logger.Error("刷新Token已过期")
		return "", errors.New("刷新Token已过期")
	}

	// 获取用户ID
	userID := claims.ID
	if userID == "" {
		logger.Error("刷新Token中用户ID为空")
		return "", errors.New("刷新Token中用户ID为空")
	}

	// 生成新的访问Token
	// 注意：这里需要获取用户信息，在实际应用中应该从数据库或缓存中获取
	// 这里简化处理，实际使用时需要传入用户信息
	return j.GenerateAccessToken(userID, "", "")
}

// GetTokenRemainingTime 获取Token剩余时间
func (j *JWTManager) GetTokenRemainingTime(tokenString string) (time.Duration, error) {
	claims, err := j.ParseToken(tokenString)
	if err != nil {
		return 0, err
	}

	if claims.ExpiresAt == nil {
		return 0, errors.New("Token没有过期时间")
	}

	remainingTime := claims.ExpiresAt.Time.Sub(time.Now())
	if remainingTime < 0 {
		return 0, errors.New("Token已过期")
	}

	return remainingTime, nil
}

// GetUserIDFromToken 从Token中获取用户ID
func (j *JWTManager) GetUserIDFromToken(tokenString string) (string, error) {
	claims, err := j.ParseToken(tokenString)
	if err != nil {
		return "", err
	}

	return claims.UserID, nil
}
