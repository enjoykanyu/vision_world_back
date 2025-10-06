package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// SmsService 短信服务接口
type SmsService interface {
	SendCode(ctx context.Context, phone, code string) error
	GenerateCode() string
}

// smsService 短信服务实现
type smsService struct {
	accessKey    string
	secretKey    string
	signName     string
	templateCode string
}

// NewSmsService 创建短信服务
func NewSmsService(accessKey, secretKey, signName, templateCode string) SmsService {
	return &smsService{
		accessKey:    accessKey,
		secretKey:    secretKey,
		signName:     signName,
		templateCode: templateCode,
	}
}

// SendCode 发送验证码
func (s *smsService) SendCode(ctx context.Context, phone, code string) error {
	// 这里应该集成真实的短信服务商API，如阿里云短信、腾讯云短信等
	// 目前只是模拟发送

	fmt.Printf("模拟发送短信验证码 - 手机号: %s, 验证码: %s\n", phone, code)

	// 实际集成时需要：
	// 1. 调用短信服务商API
	// 2. 处理API响应
	// 3. 错误处理和重试机制
	// 4. 发送频率限制

	return nil
}

// GenerateCode 生成6位随机验证码
func (s *smsService) GenerateCode() string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}
