package main

import (
	"context"
	"fmt"
	"log"
	"time"
	"user_service/internal/cache"
	"user_service/internal/config"
	"user_service/internal/model"
	"user_service/internal/service"
	"user_service/pkg/database"
	"user_service/pkg/logger"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// MockAuthService 模拟认证服务
type MockAuthService struct{}

func (m *MockAuthService) GenerateTokenPair(userID uint32) (string, string, error) {
	accessToken := fmt.Sprintf("access_token_%d", userID)
	refreshToken := fmt.Sprintf("refresh_token_%d", userID)
	return accessToken, refreshToken, nil
}

func (m *MockAuthService) ParseToken(token string) (interface{}, error) {
	// 简单解析，实际应该解析JWT
	if len(token) > 12 && token[:12] == "access_token_" {
		userIDStr := token[12:]
		return struct{ UserID uint32 }{UserID: uint32(userIDStr[0])}, nil
	}
	return nil, fmt.Errorf("invalid token")
}

// MockSmsService 模拟短信服务
type MockSmsService struct{}

func (m *MockSmsService) SendSms(ctx context.Context, phone, message string) error {
	log.Printf("Mock SMS sent to %s: %s", phone, message)
	return nil
}

func main() {
	// 1. 加载配置
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. 初始化日志
	logger, err := logger.NewLogger(logger.Config{
		Level:      cfg.Logger.Level,
		Format:     cfg.Logger.Format,
		OutputPath: cfg.Logger.OutputPath,
	})
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	logger.Info("Starting user service test")

	// 3. 初始化数据库连接
	db, err := database.NewMySQLConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}()

	// 4. 初始化Redis连接
	redisClient, err := database.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to redis: %v", err)
	}
	defer redisClient.Close()

	// 5. 创建模拟服务
	authService := &MockAuthService{}
	smsService := &MockSmsService{}

	// 6. 创建仓库
	userRepo := NewMockUserRepository(db, redisClient)

	// 7. 创建缓存服务
	cacheService := cache.NewCacheService(redisClient, logger)

	// 8. 创建用户服务
	userService := service.NewUserService(cfg, logger, userRepo, cacheService, authService, smsService)

	// 9. 创建测试用户
	user := &model.User{
		Phone:     "13800138000",
		Nickname:  "测试用户",
		AvatarURL: "https://example.com/avatar.jpg",
	}
	createdUser, err := userRepo.CreateUser(ctx, user)
	if err != nil {
		log.Printf("Failed to create test user: %v", err)
	} else {
		fmt.Printf("创建测试用户成功: ID=%d, Phone=%s, Nickname=%s\n",
			createdUser.ID, createdUser.Phone, createdUser.Nickname)
	}

	// 10. 测试短信验证码登录
	testPhone := "13800138000"

	// 发送验证码
	fmt.Println("=== 测试发送短信验证码 ===")
	ctx := context.Background()

	// 清理可能存在的验证码
	redisClient.Del(ctx, "sms_code:"+testPhone).Result()

	// 发送验证码
	err = userService.SendSmsCode(ctx, &service.SendSmsCodeRequest{
		Phone: testPhone,
	})
	if err != nil {
		log.Printf("Failed to send SMS code: %v", err)
	} else {
		fmt.Println("短信验证码发送成功")
	}

	// 获取验证码
	code, err := redisClient.Get(ctx, "sms_code:"+testPhone).Result()
	if err != nil {
		log.Printf("Failed to get SMS code from Redis: %v", err)
	} else {
		fmt.Printf("获取到验证码: %s\n", code)
	}

	// 使用验证码登录
	fmt.Println("\n=== 测试短信验证码登录 ===")
	loginResp, err := userService.SmsCodeLogin(ctx, &service.SmsCodeLoginRequest{
		Phone: testPhone,
		Code:  code,
	})
	if err != nil {
		log.Printf("SMS code login failed: %v", err)
	} else {
		fmt.Printf("登录成功! Access Token: %s\n", loginResp.AccessToken)
		fmt.Printf("用户信息: ID=%d, Phone=%s, Nickname=%s\n",
			loginResp.User.ID, loginResp.User.Phone, loginResp.User.Nickname)
	}

	// 测试Token验证
	fmt.Println("\n=== 测试Token验证 ===")
	verifyResp, err := userService.VerifyToken(ctx, &service.VerifyTokenRequest{
		Token: loginResp.AccessToken,
	})
	if err != nil {
		log.Printf("Token verification failed: %v", err)
	} else {
		fmt.Printf("Token验证成功! 用户信息: ID=%d, Phone=%s, Nickname=%s\n",
			verifyResp.User.ID, verifyResp.User.Phone, verifyResp.User.Nickname)
	}

	// 测试Token刷新
	fmt.Println("\n=== 测试Token刷新 ===")
	refreshResp, err := userService.RefreshToken(ctx, &service.RefreshTokenRequest{
		RefreshToken: loginResp.RefreshToken,
	})
	if err != nil {
		log.Printf("Token refresh failed: %v", err)
	} else {
		fmt.Printf("Token刷新成功! 新Access Token: %s\n", refreshResp.AccessToken)
	}

	// 测试缓存效果
	fmt.Println("\n=== 测试缓存效果 ===")
	// 再次获取用户信息，应该从缓存获取
	verifyResp2, err := userService.VerifyToken(ctx, &service.VerifyTokenRequest{
		Token: loginResp.AccessToken,
	})
	if err != nil {
		log.Printf("Second token verification failed: %v", err)
	} else {
		fmt.Printf("第二次Token验证成功! 用户信息: ID=%d, Phone=%s, Nickname=%s\n",
			verifyResp2.User.ID, verifyResp2.User.Phone, verifyResp2.User.Nickname)
	}

	fmt.Println("\n=== 测试完成 ===")
}

// MockUserRepository 模拟用户仓库
type MockUserRepository struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewMockUserRepository(db *gorm.DB, redis *redis.Client) *MockUserRepository {
	return &MockUserRepository{
		db:    db,
		redis: redis,
	}
}

func (r *MockUserRepository) GetUserByPhone(ctx context.Context, phone string) (*model.User, error) {
	// 实际应该从数据库查询
	return nil, fmt.Errorf("user not found")
}

func (r *MockUserRepository) CreateUser(ctx context.Context, user *model.User) (*model.User, error) {
	// 实际应该插入数据库
	user.ID = 1
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	return user, nil
}

func (r *MockUserRepository) GetUserByID(ctx context.Context, userID uint32) (*model.User, error) {
	// 实际应该从数据库查询
	return &model.User{
		ID:       userID,
		Phone:    "13800138000",
		Nickname: "测试用户",
		Status:   model.UserStatusActive,
	}, nil
}

func (r *MockUserRepository) UpdateUser(ctx context.Context, user *model.User) error {
	// 实际应该更新数据库
	return nil
}

// MockCacheService 模拟缓存服务
type MockCacheService struct {
	redis  *redis.Client
	logger logger.Logger
}

func NewMockCacheService(redis *redis.Client, logger logger.Logger) *MockCacheService {
	return &MockCacheService{
		redis:  redis,
		logger: logger,
	}
}

func (c *MockCacheService) GetUser(ctx context.Context, phone string) (*model.User, error) {
	// 实际应该从Redis获取
	return nil, fmt.Errorf("user not found in cache")
}

func (c *MockCacheService) SetUser(ctx context.Context, user *model.User, expiration time.Duration) error {
	// 实际应该设置到Redis
	return nil
}

func (c *MockCacheService) GetUserByID(ctx context.Context, userID uint32) (*model.User, error) {
	// 实际应该从Redis获取
	return nil, fmt.Errorf("user not found in cache")
}

func (c *MockCacheService) SetUserByID(ctx context.Context, userID uint32, user *model.User, expiration time.Duration) error {
	// 实际应该设置到Redis
	return nil
}

func (c *MockCacheService) DeleteUser(ctx context.Context, phone string) error {
	// 实际应该从Redis删除
	return nil
}

func (c *MockCacheService) DeleteUserByID(ctx context.Context, userID uint32) error {
	// 实际应该从Redis删除
	return nil
}

func (c *MockCacheService) GetSmsCode(ctx context.Context, phone string) (string, error) {
	// 从Redis获取验证码
	key := "sms_code:" + phone
	return c.redis.Get(ctx, key).Result()
}

func (c *MockCacheService) SetSmsCode(ctx context.Context, phone, code string, expiration time.Duration) error {
	// 设置验证码到Redis
	key := "sms_code:" + phone
	return c.redis.Set(ctx, key, code, expiration).Err()
}

func (c *MockCacheService) DeleteSmsCode(ctx context.Context, phone string) error {
	// 从Redis删除验证码
	key := "sms_code:" + phone
	return c.redis.Del(ctx, key).Err()
}

func (c *MockCacheService) CheckRateLimit(ctx context.Context, keyPrefix, identifier string, limit int, window time.Duration) (bool, error) {
	// 实际应该检查Redis中的限流计数器
	return true, nil
}

func (c *MockCacheService) AcquireLock(ctx context.Context, key string, expiration time.Duration) (bool, string, error) {
	// 实际应该获取分布式锁
	return true, "mock-lock-token", nil
}

func (c *MockCacheService) ReleaseLock(ctx context.Context, key, token string) (bool, error) {
	// 实际应该释放分布式锁
	return true, nil
}

func (c *MockCacheService) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	// 实际应该设置到Redis
	return nil
}

func (c *MockCacheService) Get(ctx context.Context, key string) (string, error) {
	// 实际应该从Redis获取
	return "", fmt.Errorf("key not found")
}

func (c *MockCacheService) Delete(ctx context.Context, key string) error {
	// 实际应该从Redis删除
	return nil
}

func (c *MockCacheService) Exists(ctx context.Context, key string) (bool, error) {
	// 实际应该检查Redis中是否存在
	return false, nil
}

func (c *MockCacheService) Expire(ctx context.Context, key string, expiration time.Duration) error {
	// 实际应该设置过期时间
	return nil
}

func (c *MockCacheService) Increment(ctx context.Context, key string) (int64, error) {
	// 实际应该增加计数器
	return 1, nil
}

func (c *MockCacheService) BatchSet(ctx context.Context, items map[string]interface{}, expiration time.Duration) error {
	// 实际应该批量设置到Redis
	return nil
}

func (c *MockCacheService) BatchGet(ctx context.Context, keys []string) (map[string]string, error) {
	// 实际应该批量从Redis获取
	return make(map[string]string), nil
}

func (c *MockCacheService) BatchDelete(ctx context.Context, keys []string) error {
	// 实际应该批量从Redis删除
	return nil
}

func (c *MockCacheService) PreWarmCache(ctx context.Context) error {
	// 实际应该预热缓存
	return nil
}

func (c *MockCacheService) GetCacheStats(ctx context.Context) (map[string]interface{}, error) {
	// 实际应该获取缓存统计
	return make(map[string]interface{}), nil
}
