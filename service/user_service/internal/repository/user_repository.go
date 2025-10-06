package repository

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
	"user_service/internal/model"
)

// UserRepository 用户数据访问接口
type UserRepository interface {
	// 用户相关
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, userID uint32) (*model.User, error)
	GetByPhone(ctx context.Context, phone string) (*model.User, error)
	GetByIDs(ctx context.Context, userIDs []uint32) ([]*model.User, error)
	Update(ctx context.Context, userID uint32, updates map[string]interface{}) error
	Exists(ctx context.Context, userID uint32) (bool, error)

	// 缓存相关
	GetUserFromCache(ctx context.Context, userID uint32) (*model.UserCache, error)
	SetUserCache(ctx context.Context, userID uint32, userCache *model.UserCache, expiration time.Duration) error
	DeleteUserCache(ctx context.Context, userID uint32) error

	// 短信验证码
	SetSmsCode(ctx context.Context, phone, code string, expiration time.Duration) error
	GetSmsCode(ctx context.Context, phone string) (string, error)
	DeleteSmsCode(ctx context.Context, phone string) error
}

// userRepository 用户数据访问实现
type userRepository struct {
	db    *gorm.DB
	redis *redis.Client
}

// NewUserRepository 创建用户数据访问对象
func NewUserRepository(db *gorm.DB, redis *redis.Client) UserRepository {
	return &userRepository{
		db:    db,
		redis: redis,
	}
}

// Create 创建用户
func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return err
	}
	return nil
}

// GetByID 根据ID获取用户
func (r *userRepository) GetByID(ctx context.Context, userID uint32) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("id = ? AND status = ?", userID, model.UserStatusActive).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

// GetByPhone 根据手机号获取用户
func (r *userRepository) GetByPhone(ctx context.Context, phone string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("phone = ? AND status = ?", phone, model.UserStatusActive).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

// GetByIDs 批量获取用户
func (r *userRepository) GetByIDs(ctx context.Context, userIDs []uint32) ([]*model.User, error) {
	if len(userIDs) == 0 {
		return []*model.User{}, nil
	}

	var users []*model.User
	if err := r.db.WithContext(ctx).Where("id IN ? AND status = ?", userIDs, model.UserStatusActive).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// Update 更新用户信息
func (r *userRepository) Update(ctx context.Context, userID uint32, updates map[string]interface{}) error {
	// 验证用户是否存在
	var user model.User
	if err := r.db.WithContext(ctx).Where("id = ? AND status = ?", userID, model.UserStatusActive).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("user not found")
		}
		return err
	}

	// 更新时间
	updates["update_time"] = time.Now()

	if err := r.db.WithContext(ctx).Model(&user).Updates(updates).Error; err != nil {
		return err
	}

	return nil
}

// Exists 检查用户是否存在
func (r *userRepository) Exists(ctx context.Context, userID uint32) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ? AND status = ?", userID, model.UserStatusActive).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetUserFromCache 从缓存获取用户信息
func (r *userRepository) GetUserFromCache(ctx context.Context, userID uint32) (*model.UserCache, error) {
	cacheKey := model.GetUserCacheKey(userID)
	cachedData, err := r.redis.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, errors.New("cache not found")
		}
		return nil, err
	}

	var userCache model.UserCache
	if err := userCache.FromJSONBytes([]byte(cachedData)); err != nil {
		return nil, errors.New("failed to parse cached data")
	}

	return &userCache, nil
}

// SetUserCache 设置用户缓存
func (r *userRepository) SetUserCache(ctx context.Context, userID uint32, userCache *model.UserCache, expiration time.Duration) error {
	cacheData, err := userCache.ToJSON()
	if err != nil {
		return errors.New("failed to serialize user cache")
	}

	cacheKey := model.GetUserCacheKey(userID)
	if err := r.redis.Set(ctx, cacheKey, cacheData, expiration).Err(); err != nil {
		return errors.New("failed to set cache")
	}

	return nil
}

// DeleteUserCache 删除用户缓存
func (r *userRepository) DeleteUserCache(ctx context.Context, userID uint32) error {
	cacheKey := model.GetUserCacheKey(userID)
	if err := r.redis.Del(ctx, cacheKey).Err(); err != nil {
		return errors.New("failed to delete cache")
	}
	return nil
}

// SetSmsCode 设置短信验证码
func (r *userRepository) SetSmsCode(ctx context.Context, phone, code string, expiration time.Duration) error {
	cacheKey := model.GetSmsCodeCacheKey(phone)
	if err := r.redis.Set(ctx, cacheKey, code, expiration).Err(); err != nil {
		return errors.New("failed to set sms code")
	}
	return nil
}

// GetSmsCode 获取短信验证码
func (r *userRepository) GetSmsCode(ctx context.Context, phone string) (string, error) {
	cacheKey := model.GetSmsCodeCacheKey(phone)
	code, err := r.redis.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", errors.New("code not found or expired")
		}
		return "", err
	}
	return code, nil
}

// DeleteSmsCode 删除短信验证码
func (r *userRepository) DeleteSmsCode(ctx context.Context, phone string) error {
	cacheKey := model.GetSmsCodeCacheKey(phone)
	if err := r.redis.Del(ctx, cacheKey).Err(); err != nil {
		return errors.New("failed to delete sms code")
	}
	return nil
}
