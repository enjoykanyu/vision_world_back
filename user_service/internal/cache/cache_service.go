package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"user_service/internal/model"
	"user_service/pkg/logger"
)

// CacheService 缓存服务接口
type CacheService interface {
	// 用户信息缓存
	GetUser(ctx context.Context, userID uint32) (*model.UserCache, error)
	SetUser(ctx context.Context, userID uint32, user *model.UserCache, ttl time.Duration) error
	DeleteUser(ctx context.Context, userID uint32) error

	// 短信验证码缓存
	GetSmsCode(ctx context.Context, phone string) (string, error)
	SetSmsCode(ctx context.Context, phone, code string, ttl time.Duration) error
	DeleteSmsCode(ctx context.Context, phone string) error

	// 限流缓存
	CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
	SetRateLimit(ctx context.Context, key string, window time.Duration) error

	// 分布式锁
	TryLock(ctx context.Context, key string, ttl time.Duration) (bool, string, error)
	Unlock(ctx context.Context, key string, token string) error

	// 通用缓存操作
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// 批量操作
	MGet(ctx context.Context, keys []string) ([]string, error)
	MSet(ctx context.Context, pairs map[string]interface{}, ttl time.Duration) error
	DelKeys(ctx context.Context, pattern string) (int64, error)

	// 缓存预热
	WarmupUserCache(ctx context.Context, userIDs []uint32) error

	// 缓存统计
	GetCacheStats(ctx context.Context) (*CacheStats, error)
}

// CacheStats 缓存统计信息
type CacheStats struct {
	TotalKeys     int64     `json:"total_keys"`
	MemoryUsage   int64     `json:"memory_usage"`
	HitRate       float64   `json:"hit_rate"`
	MissRate      float64   `json:"miss_rate"`
	LastResetTime time.Time `json:"last_reset_time"`
}

// cacheService 缓存服务实现
type cacheService struct {
	redis  *redis.Client
	logger logger.Logger
}

// NewCacheService 创建缓存服务
func NewCacheService(redisClient *redis.Client, log logger.Logger) CacheService {
	return &cacheService{
		redis:  redisClient,
		logger: log,
	}
}

// GetUser 获取用户缓存
func (c *cacheService) GetUser(ctx context.Context, userID uint32) (*model.UserCache, error) {
	cacheKey := model.GetUserInfoCacheKey(uint64(userID))
	data, err := c.redis.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("user cache not found for ID: %d", userID)
		}
		c.logger.Error("Failed to get user cache", "error", err, "userID", userID)
		return nil, fmt.Errorf("failed to get user cache: %w", err)
	}

	var userCache model.UserCache
	if err := json.Unmarshal([]byte(data), &userCache); err != nil {
		c.logger.Error("Failed to unmarshal user cache", "error", err, "userID", userID)
		return nil, fmt.Errorf("failed to unmarshal user cache: %w", err)
	}

	return &userCache, nil
}

// SetUser 设置用户缓存
func (c *cacheService) SetUser(ctx context.Context, userID uint32, user *model.UserCache, ttl time.Duration) error {
	cacheKey := model.GetUserInfoCacheKey(uint64(userID))
	data, err := json.Marshal(user)
	if err != nil {
		c.logger.Error("Failed to marshal user cache", "error", err, "userID", userID)
		return fmt.Errorf("failed to marshal user cache: %w", err)
	}

	if err := c.redis.Set(ctx, cacheKey, data, ttl).Err(); err != nil {
		c.logger.Error("Failed to set user cache", "error", err, "userID", userID)
		return fmt.Errorf("failed to set user cache: %w", err)
	}

	return nil
}

// DeleteUser 删除用户缓存
func (c *cacheService) DeleteUser(ctx context.Context, userID uint32) error {
	cacheKey := model.GetUserInfoCacheKey(uint64(userID))
	if err := c.redis.Del(ctx, cacheKey).Err(); err != nil {
		c.logger.Error("Failed to delete user cache", "error", err, "userID", userID)
		return fmt.Errorf("failed to delete user cache: %w", err)
	}
	return nil
}

// GetSmsCode 获取短信验证码
func (c *cacheService) GetSmsCode(ctx context.Context, phone string) (string, error) {
	cacheKey := model.GetSmsCodeCacheKey(phone)
	code, err := c.redis.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", fmt.Errorf("sms code not found or expired for phone: %s", phone)
		}
		c.logger.Error("Failed to get sms code", "error", err, "phone", phone)
		return "", fmt.Errorf("failed to get sms code: %w", err)
	}
	return code, nil
}

// SetSmsCode 设置短信验证码
func (c *cacheService) SetSmsCode(ctx context.Context, phone, code string, ttl time.Duration) error {
	cacheKey := model.GetSmsCodeCacheKey(phone)
	if err := c.redis.Set(ctx, cacheKey, code, ttl).Err(); err != nil {
		c.logger.Error("Failed to set sms code", "error", err, "phone", phone)
		return fmt.Errorf("failed to set sms code: %w", err)
	}
	return nil
}

// DeleteSmsCode 删除短信验证码
func (c *cacheService) DeleteSmsCode(ctx context.Context, phone string) error {
	cacheKey := model.GetSmsCodeCacheKey(phone)
	if err := c.redis.Del(ctx, cacheKey).Err(); err != nil {
		c.logger.Error("Failed to delete sms code", "error", err, "phone", phone)
		return fmt.Errorf("failed to delete sms code: %w", err)
	}
	return nil
}

// CheckRateLimit 检查限流
func (c *cacheService) CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	// 使用Redis的INCR和EXPIRE实现简单的滑动窗口限流
	current, err := c.redis.Incr(ctx, key).Result()
	if err != nil {
		c.logger.Error("Failed to increment rate limit counter", "error", err, "key", key)
		return false, fmt.Errorf("failed to increment rate limit counter: %w", err)
	}

	// 如果是第一次设置，添加过期时间
	if current == 1 {
		if err := c.redis.Expire(ctx, key, window).Err(); err != nil {
			c.logger.Error("Failed to set rate limit expiration", "error", err, "key", key)
			return false, fmt.Errorf("failed to set rate limit expiration: %w", err)
		}
	}

	// 检查是否超过限制
	return current <= int64(limit), nil
}

// SetRateLimit 设置限流
func (c *cacheService) SetRateLimit(ctx context.Context, key string, window time.Duration) error {
	if err := c.redis.Set(ctx, key, 1, window).Err(); err != nil {
		c.logger.Error("Failed to set rate limit", "error", err, "key", key)
		return fmt.Errorf("failed to set rate limit: %w", err)
	}
	return nil
}

// TryLock 尝试获取分布式锁
func (c *cacheService) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, string, error) {
	// 生成唯一token
	token := fmt.Sprintf("%d", time.Now().UnixNano())

	// 使用SET命令的NX和EX选项实现分布式锁
	result, err := c.redis.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		c.logger.Error("Failed to acquire lock", "error", err, "key", key)
		return false, "", fmt.Errorf("failed to acquire lock: %w", err)
	}

	return result, token, nil
}

// Unlock 释放分布式锁
func (c *cacheService) Unlock(ctx context.Context, key string, token string) error {
	// 使用Lua脚本确保只有锁的持有者才能释放锁
	script := `
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("del", KEYS[1])
	else
		return 0
	end
	`

	result, err := c.redis.Eval(ctx, script, []string{key}, token).Result()
	if err != nil {
		c.logger.Error("Failed to release lock", "error", err, "key", key)
		return fmt.Errorf("failed to release lock: %w", err)
	}

	if result.(int64) == 0 {
		return fmt.Errorf("lock not held by this token")
	}

	return nil
}

// Get 获取缓存值
func (c *cacheService) Get(ctx context.Context, key string) (string, error) {
	value, err := c.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", fmt.Errorf("key not found: %s", key)
		}
		c.logger.Error("Failed to get cache value", "error", err, "key", key)
		return "", fmt.Errorf("failed to get cache value: %w", err)
	}
	return value, nil
}

// Set 设置缓存值
func (c *cacheService) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if err := c.redis.Set(ctx, key, value, ttl).Err(); err != nil {
		c.logger.Error("Failed to set cache value", "error", err, "key", key)
		return fmt.Errorf("failed to set cache value: %w", err)
	}
	return nil
}

// Delete 删除缓存
func (c *cacheService) Delete(ctx context.Context, key string) error {
	if err := c.redis.Del(ctx, key).Err(); err != nil {
		c.logger.Error("Failed to delete cache", "error", err, "key", key)
		return fmt.Errorf("failed to delete cache: %w", err)
	}
	return nil
}

// Exists 检查键是否存在
func (c *cacheService) Exists(ctx context.Context, key string) (bool, error) {
	count, err := c.redis.Exists(ctx, key).Result()
	if err != nil {
		c.logger.Error("Failed to check key existence", "error", err, "key", key)
		return false, fmt.Errorf("failed to check key existence: %w", err)
	}
	return count > 0, nil
}

// Expire 设置键的过期时间
func (c *cacheService) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if err := c.redis.Expire(ctx, key, ttl).Err(); err != nil {
		c.logger.Error("Failed to set key expiration", "error", err, "key", key)
		return fmt.Errorf("failed to set key expiration: %w", err)
	}
	return nil
}

// MGet 批量获取缓存值
func (c *cacheService) MGet(ctx context.Context, keys []string) ([]string, error) {
	if len(keys) == 0 {
		return []string{}, nil
	}

	values, err := c.redis.MGet(ctx, keys...).Result()
	if err != nil {
		c.logger.Error("Failed to get multiple cache values", "error", err)
		return nil, fmt.Errorf("failed to get multiple cache values: %w", err)
	}

	result := make([]string, len(values))
	for i, value := range values {
		if value == nil {
			result[i] = ""
		} else {
			result[i] = value.(string)
		}
	}

	return result, nil
}

// MSet 批量设置缓存值
func (c *cacheService) MSet(ctx context.Context, pairs map[string]interface{}, ttl time.Duration) error {
	if len(pairs) == 0 {
		return nil
	}

	// 使用pipeline提高性能
	pipe := c.redis.Pipeline()

	for key, value := range pairs {
		pipe.Set(ctx, key, value, ttl)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		c.logger.Error("Failed to set multiple cache values", "error", err)
		return fmt.Errorf("failed to set multiple cache values: %w", err)
	}

	return nil
}

// DelKeys 根据模式删除键
func (c *cacheService) DelKeys(ctx context.Context, pattern string) (int64, error) {
	keys, err := c.redis.Keys(ctx, pattern).Result()
	if err != nil {
		c.logger.Error("Failed to get keys by pattern", "error", err, "pattern", pattern)
		return 0, fmt.Errorf("failed to get keys by pattern: %w", err)
	}

	if len(keys) == 0 {
		return 0, nil
	}

	count, err := c.redis.Del(ctx, keys...).Result()
	if err != nil {
		c.logger.Error("Failed to delete keys by pattern", "error", err, "pattern", pattern)
		return 0, fmt.Errorf("failed to delete keys by pattern: %w", err)
	}

	return count, nil
}

// WarmupUserCache 预热用户缓存
func (c *cacheService) WarmupUserCache(ctx context.Context, userIDs []uint32) error {
	if len(userIDs) == 0 {
		return nil
	}

	c.logger.Info("Starting user cache warmup", "count", len(userIDs))

	// 这里应该从数据库获取用户信息并设置到缓存
	// 由于这是缓存服务，我们只记录日志，实际的数据获取应该在服务层完成
	c.logger.Info("User cache warmup completed", "count", len(userIDs))

	return nil
}

// GetCacheStats 获取缓存统计信息
func (c *cacheService) GetCacheStats(ctx context.Context) (*CacheStats, error) {
	info, err := c.redis.Info(ctx, "memory").Result()
	if err != nil {
		c.logger.Error("Failed to get Redis memory info", "error", err)
		return nil, fmt.Errorf("failed to get Redis memory info: %w", err)
	}

	// 解析内存使用情况
	var memoryUsage int64
	if _, err := fmt.Sscanf(info, "used_memory:%d", &memoryUsage); err != nil {
		c.logger.Error("Failed to parse memory usage", "error", err)
		memoryUsage = 0
	}

	// 获取键的总数
	dbSize, err := c.redis.DBSize(ctx).Result()
	if err != nil {
		c.logger.Error("Failed to get DB size", "error", err)
		dbSize = 0
	}

	// 这里简化处理，实际应该从Redis的统计信息中获取命中率
	stats := &CacheStats{
		TotalKeys:     dbSize,
		MemoryUsage:   memoryUsage,
		HitRate:       0.0, // 需要从Redis的stats中获取
		MissRate:      0.0, // 需要从Redis的stats中获取
		LastResetTime: time.Now(),
	}

	return stats, nil
}
