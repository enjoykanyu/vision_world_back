package crypto

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/visionworld/user-service/internal/config"
	"github.com/visionworld/user-service/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// PasswordManager 密码管理器
type PasswordManager struct {
	config *config.SecurityConfig
}

// NewPasswordManager 创建密码管理器
func NewPasswordManager(cfg *config.SecurityConfig) *PasswordManager {
	return &PasswordManager{
		config: cfg,
	}
}

// HashPassword 密码加密
func (pm *PasswordManager) HashPassword(password string) (string, error) {
	// 验证密码强度
	if err := pm.ValidatePasswordStrength(password); err != nil {
		return "", err
	}

	// 生成密码哈希
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), pm.config.BcryptCost)
	if err != nil {
		logger.Error("密码加密失败", zap.Error(err))
		return "", fmt.Errorf("密码加密失败: %v", err)
	}

	return string(hashedPassword), nil
}

// VerifyPassword 验证密码
func (pm *PasswordManager) VerifyPassword(hashedPassword, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return errors.New("密码错误")
		}
		logger.Error("密码验证失败", zap.Error(err))
		return fmt.Errorf("密码验证失败: %v", err)
	}
	return nil
}

// ValidatePasswordStrength 验证密码强度
func (pm *PasswordManager) ValidatePasswordStrength(password string) error {
	// 检查最小长度
	if len(password) < pm.config.PasswordMinLength {
		return fmt.Errorf("密码长度至少为%d位", pm.config.PasswordMinLength)
	}

	// 检查是否包含大写字母
	if pm.config.PasswordRequireUppercase {
		if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
			return errors.New("密码必须包含至少一个大写字母")
		}
	}

	// 检查是否包含小写字母
	if pm.config.PasswordRequireLowercase {
		if !regexp.MustCompile(`[a-z]`).MatchString(password) {
			return errors.New("密码必须包含至少一个小写字母")
		}
	}

	// 检查是否包含数字
	if pm.config.PasswordRequireDigit {
		if !regexp.MustCompile(`[0-9]`).MatchString(password) {
			return errors.New("密码必须包含至少一个数字")
		}
	}

	// 检查是否包含特殊字符
	if pm.config.PasswordRequireSpecial {
		if !regexp.MustCompile(`[!@#$%^&*(),.?":{}|<>]`).MatchString(password) {
			return errors.New("密码必须包含至少一个特殊字符")
		}
	}

	return nil
}

// GenerateRandomPassword 生成随机密码
func GenerateRandomPassword(length int) (string, error) {
	if length < 8 {
		return "", errors.New("密码长度至少为8位")
	}

	const (
		lowercase = "abcdefghijklmnopqrstuvwxyz"
		uppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		digits    = "0123456789"
		special   = "!@#$%^&*(),.?\":{}|<>"
	)

	allChars := lowercase + uppercase + digits + special

	// 确保密码包含各种字符类型
	password := make([]byte, length)
	password[0] = lowercase[generateRandomInt(len(lowercase))]
	password[1] = uppercase[generateRandomInt(len(uppercase))]
	password[2] = digits[generateRandomInt(len(digits))]
	password[3] = special[generateRandomInt(len(special))]

	// 填充剩余字符
	for i := 4; i < length; i++ {
		password[i] = allChars[generateRandomInt(len(allChars))]
	}

	// 打乱字符顺序
	for i := len(password) - 1; i > 0; i-- {
		j := generateRandomInt(i + 1)
		password[i], password[j] = password[j], password[i]
	}

	return string(password), nil
}

// generateRandomInt 生成随机整数
func generateRandomInt(max int) int {
	return int(time.Now().UnixNano() % int64(max))
}

// ValidateUsername 验证用户名
func ValidateUsername(username string) error {
	if strings.TrimSpace(username) == "" {
		return errors.New("用户名不能为空")
	}

	if len(username) < 3 || len(username) > 20 {
		return errors.New("用户名长度必须在3-20位之间")
	}

	// 只允许字母、数字、下划线
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, username); !matched {
		return errors.New("用户名只能包含字母、数字和下划线")
	}

	// 不能以数字开头
	if matched, _ := regexp.MatchString(`^[0-9]`, username); matched {
		return errors.New("用户名不能以数字开头")
	}

	return nil
}

// ValidateEmail 验证邮箱格式
func ValidateEmail(email string) error {
	if strings.TrimSpace(email) == "" {
		return errors.New("邮箱不能为空")
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return errors.New("邮箱格式不正确")
	}

	return nil
}

// ValidatePhone 验证手机号格式（中国大陆）
func ValidatePhone(phone string) error {
	if strings.TrimSpace(phone) == "" {
		return errors.New("手机号不能为空")
	}

	phoneRegex := regexp.MustCompile(`^1[3-9]\d{9}$`)
	if !phoneRegex.MatchString(phone) {
		return errors.New("手机号格式不正确")
	}

	return nil
}
