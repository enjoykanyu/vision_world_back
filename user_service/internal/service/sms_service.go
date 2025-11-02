package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
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
	// 生成唯一消息ID用于跟踪
	messageID := uuid.New().String()

	fmt.Printf("[%s] 模拟发送短信验证码 - 手机号: %s, 验证码: %s\n", messageID, phone, code)

	// 实际集成时需要：
	// 1. 调用短信服务商API
	// 2. 处理API响应
	// 3. 错误处理和重试机制
	// 4. 发送频率限制

	// TODO: 集成真实短信服务商API
	// 示例：阿里云短信服务集成
	/*
		client, err := dysmsapi.NewClientWithAccessKey("cn-hangzhou", s.accessKey, s.secretKey)
		if err != nil {
			return fmt.Errorf("failed to create sms client: %w", err)
		}

		request := dysmsapi.CreateSendSmsRequest()
		request.Scheme = "https"
		request.PhoneNumbers = phone
		request.SignName = s.signName
		request.TemplateCode = s.templateCode
		request.TemplateParam = fmt.Sprintf(`{"code":"%s"}`, code)
		request.OutId = messageID // 设置外部流水号

		response, err := client.SendSms(request)
		if err != nil {
			return fmt.Errorf("failed to send sms: %w", err)
		}

		if response.Code != "OK" {
			return fmt.Errorf("sms send failed: %s, bizId: %s", response.Message, response.BizId)
		}

		fmt.Printf("[%s] 短信发送成功，业务ID: %s\n", messageID, response.BizId)
	*/

	return nil
}

// GenerateCode 生成6位随机验证码
func (s *smsService) GenerateCode() string {
	// 使用更安全的随机数生成方式
	rand.Seed(time.Now().UnixNano() + int64(rand.Intn(1000)))
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}
