package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Config Token认证配置
type Config struct {
	SecretKey      string        // 用于签名的密钥
	ExpireDuration time.Duration // Token默认过期时间
	AllowOrigin    []string      // 允许的请求来源
}

// Service Token认证服务接口
type Service interface {
	// GenerateToken 生成播放Token
	GenerateToken(videoID string, expire time.Duration) (string, error)

	// ValidateToken 验证播放Token
	ValidateToken(token string, videoID string) (bool, error)

	// GeneratePlayURL 生成完整的播放URL
	GeneratePlayURL(videoID string, baseURL string, expire time.Duration) (string, error)

	// ValidatePlayURL 验证播放URL的有效性
	ValidatePlayURL(url string) (bool, string, error)
}

// service Token认证服务实现
type service struct {
	config Config
}

// NewService 创建Token认证服务实例
func NewService(cfg Config) Service {
	return &service{
		config: cfg,
	}
}

// GenerateToken 生成播放Token
func (s *service) GenerateToken(videoID string, expire time.Duration) (string, error) {
	// 如果未指定过期时间，使用默认值
	if expire == 0 {
		expire = s.config.ExpireDuration
	}

	// 生成过期时间戳
	expireTime := time.Now().Add(expire).Unix()

	// 构建Token数据
	tokenData := fmt.Sprintf("%s:%d", videoID, expireTime)

	// 使用HMAC-SHA256签名
	mac := hmac.New(sha256.New, []byte(s.config.SecretKey))
	mac.Write([]byte(tokenData))
	signature := hex.EncodeToString(mac.Sum(nil))

	// 组合最终Token
	return fmt.Sprintf("%s:%s", tokenData, signature), nil
}

// ValidateToken 验证播放Token
func (s *service) ValidateToken(token string, videoID string) (bool, error) {
	// 解析Token
	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		return false, fmt.Errorf("invalid token format")
	}

	// 提取Token数据
	parsedVideoID := parts[0]
	expireStr := parts[1]
	signature := parts[2]

	// 验证视频ID是否匹配
	if parsedVideoID != videoID {
		return false, fmt.Errorf("video ID mismatch")
	}

	// 解析过期时间
	expireTime, err := strconv.ParseInt(expireStr, 10, 64)
	if err != nil {
		return false, fmt.Errorf("invalid expire time: %w", err)
	}

	// 检查Token是否过期
	if time.Now().Unix() > expireTime {
		return false, fmt.Errorf("token expired")
	}

	// 重新生成签名并验证
	expectedTokenData := fmt.Sprintf("%s:%d", videoID, expireTime)
	mac := hmac.New(sha256.New, []byte(s.config.SecretKey))
	mac.Write([]byte(expectedTokenData))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if signature != expectedSignature {
		return false, fmt.Errorf("invalid signature")
	}

	return true, nil
}

// GeneratePlayURL 生成完整的播放URL
func (s *service) GeneratePlayURL(videoID string, baseURL string, expire time.Duration) (string, error) {
	// 生成Token
	token, err := s.GenerateToken(videoID, expire)
	if err != nil {
		return "", err
	}

	// 生成过期时间戳
	expireTime := time.Now().Add(expire).Unix()

	// 构建完整URL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	return fmt.Sprintf("%splay/%s?token=%s&expire=%d", baseURL, videoID, token, expireTime), nil
}

// ValidatePlayURL 验证播放URL的有效性
func (s *service) ValidatePlayURL(url string) (bool, string, error) {
	// 提取videoID
	startIdx := strings.Index(url, "/play/")
	if startIdx == -1 {
		return false, "", fmt.Errorf("invalid URL format")
	}

	endIdx := strings.Index(url[startIdx+6:], "?")
	if endIdx == -1 {
		return false, "", fmt.Errorf("invalid URL format")
	}

	videoID := url[startIdx+6 : startIdx+6+endIdx]

	// 提取Token
	tokenStart := strings.Index(url, "token=")
	if tokenStart == -1 {
		return false, "", fmt.Errorf("token not found")
	}

	tokenEnd := strings.Index(url[tokenStart+6:], "&")
	if tokenEnd == -1 {
		// Token是URL的最后一个参数
		token := url[tokenStart+6:]
		valid, err := s.ValidateToken(token, videoID)
		return valid, videoID, err
	} else {
		// Token后面还有其他参数
		token := url[tokenStart+6 : tokenStart+6+tokenEnd]
		valid, err := s.ValidateToken(token, videoID)
		return valid, videoID, err
	}
}

// ExtractVideoID 从URL中提取视频ID
func (s *service) ExtractVideoID(url string) (string, error) {
	// 提取videoID
	startIdx := strings.Index(url, "/play/")
	if startIdx == -1 {
		return "", fmt.Errorf("invalid URL format")
	}

	endIdx := strings.Index(url[startIdx+6:], "?")
	if endIdx == -1 {
		return "", fmt.Errorf("invalid URL format")
	}

	return url[startIdx+6 : startIdx+6+endIdx], nil
}

// ExtractToken 从URL中提取Token
func (s *service) ExtractToken(url string) (string, error) {
	// 提取Token
	tokenStart := strings.Index(url, "token=")
	if tokenStart == -1 {
		return "", fmt.Errorf("token not found")
	}

	tokenEnd := strings.Index(url[tokenStart+6:], "&")
	if tokenEnd == -1 {
		// Token是URL的最后一个参数
		return url[tokenStart+6:], nil
	} else {
		// Token后面还有其他参数
		return url[tokenStart+6 : tokenStart+6+tokenEnd], nil
	}
}
