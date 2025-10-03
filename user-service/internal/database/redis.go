package database

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/visionworld/user-service/internal/config"
	"github.com/visionworld/user-service/pkg/logger"
)

var RedisClient *redis.Client

// InitRedis 初始化Redis连接
func InitRedis(cfg *config.RedisConfig) error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("连接Redis失败: %v", err)
	}

	logger.Info("Redis连接初始化成功")
	return nil
}

// CloseRedis 关闭Redis连接
func CloseRedis() error {
	if RedisClient != nil {
		return RedisClient.Close()
	}
	return nil
}

// GetRedisClient 获取Redis客户端
func GetRedisClient() *redis.Client {
	return RedisClient
}

// GetRedis 获取Redis连接
func GetRedis() *redis.Client {
	return RedisClient
}

// RedisKey Redis键名常量
type RedisKey string

const (
	// 用户相关
	UserInfoKey       RedisKey = "user:info:%s"           // 用户信息
	UserTokenKey      RedisKey = "user:token:%s"          // 用户Token
	UserRefreshKey    RedisKey = "user:refresh:%s"        // 刷新Token
	UserFollowKey     RedisKey = "user:follow:%s"         // 用户关注列表
	UserFollowerKey   RedisKey = "user:follower:%s"       // 用户粉丝列表
	UserLoginAttempts RedisKey = "user:login_attempts:%s" // 登录尝试次数
	UserLockout       RedisKey = "user:lockout:%s"        // 用户锁定状态

	// 验证码相关
	SMSCodeKey     RedisKey = "sms:code:%s:%s"  // 短信验证码
	SMSCooldownKey RedisKey = "sms:cooldown:%s" // 短信冷却时间
	SMSCountKey    RedisKey = "sms:count:%s:%s" // 短信发送次数

	// 统计相关
	UserStatsKey   RedisKey = "user:stats:%s"      // 用户统计
	DailyActiveKey RedisKey = "stats:daily_active" // 日活用户

	// 限流相关
	RateLimitKey RedisKey = "rate_limit:%s:%s" // 限流计数
)

// GetRedisKey 获取Redis键名
func (k RedisKey) GetKey(args ...interface{}) string {
	return fmt.Sprintf(string(k), args...)
}

// SetUserInfo 设置用户信息缓存
func SetUserInfo(ctx context.Context, userID string, userInfo interface{}, expire time.Duration) error {
	key := UserInfoKey.GetKey(userID)
	return RedisClient.Set(ctx, key, userInfo, expire).Err()
}

// GetUserInfo 获取用户信息缓存
func GetUserInfo(ctx context.Context, userID string) (string, error) {
	key := UserInfoKey.GetKey(userID)
	return RedisClient.Get(ctx, key).Result()
}

// DeleteUserInfo 删除用户信息缓存
func DeleteUserInfo(ctx context.Context, userID string) error {
	key := UserInfoKey.GetKey(userID)
	return RedisClient.Del(ctx, key).Err()
}

// SetToken 设置Token缓存
func SetToken(ctx context.Context, token string, userID string, expire time.Duration) error {
	key := UserTokenKey.GetKey(token)
	return RedisClient.Set(ctx, key, userID, expire).Err()
}

// GetToken 获取Token缓存
func GetToken(ctx context.Context, token string) (string, error) {
	key := UserTokenKey.GetKey(token)
	return RedisClient.Get(ctx, key).Result()
}

// DeleteToken 删除Token缓存
func DeleteToken(ctx context.Context, token string) error {
	key := UserTokenKey.GetKey(token)
	return RedisClient.Del(ctx, key).Err()
}

// SetSMSCode 设置短信验证码
func SetSMSCode(ctx context.Context, phone, code string, expire time.Duration) error {
	key := SMSCodeKey.GetKey(phone, code)
	return RedisClient.Set(ctx, key, code, expire).Err()
}

// GetSMSCode 获取短信验证码
func GetSMSCode(ctx context.Context, phone, code string) (string, error) {
	key := SMSCodeKey.GetKey(phone, code)
	return RedisClient.Get(ctx, key).Result()
}

// DeleteSMSCode 删除短信验证码
func DeleteSMSCode(ctx context.Context, phone, code string) error {
	key := SMSCodeKey.GetKey(phone, code)
	return RedisClient.Del(ctx, key).Err()
}

// SetSMSCooldown 设置短信冷却时间
func SetSMSCooldown(ctx context.Context, phone string, expire time.Duration) error {
	key := SMSCooldownKey.GetKey(phone)
	return RedisClient.Set(ctx, key, "1", expire).Err()
}

// GetSMSCooldown 获取短信冷却时间
func GetSMSCooldown(ctx context.Context, phone string) (bool, error) {
	key := SMSCooldownKey.GetKey(phone)
	result, err := RedisClient.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

// IncrementLoginAttempts 增加登录尝试次数
func IncrementLoginAttempts(ctx context.Context, identifier string, expire time.Duration) (int64, error) {
	key := UserLoginAttempts.GetKey(identifier)
	return RedisClient.Incr(ctx, key).Result()
}

// GetLoginAttempts 获取登录尝试次数
func GetLoginAttempts(ctx context.Context, identifier string) (int64, error) {
	key := UserLoginAttempts.GetKey(identifier)
	return RedisClient.Get(ctx, key).Int64()
}

// ResetLoginAttempts 重置登录尝试次数
func ResetLoginAttempts(ctx context.Context, identifier string) error {
	key := UserLoginAttempts.GetKey(identifier)
	return RedisClient.Del(ctx, key).Err()
}

// SetUserLockout 设置用户锁定状态
func SetUserLockout(ctx context.Context, identifier string, expire time.Duration) error {
	key := UserLockout.GetKey(identifier)
	return RedisClient.Set(ctx, key, "1", expire).Err()
}

// GetUserLockout 获取用户锁定状态
func GetUserLockout(ctx context.Context, identifier string) (bool, error) {
	key := UserLockout.GetKey(identifier)
	result, err := RedisClient.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}
