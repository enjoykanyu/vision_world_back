package database

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"user_service/internal/config"
	"user_service/internal/model"
)

// NewMySQLConnection 创建MySQL连接并初始化表结构
func NewMySQLConnection(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.Charset,
	)

	// 获取日志级别
	logLevel := logger.Silent
	switch cfg.LogLevel {
	case "silent":
		logLevel = logger.Silent
	case "error":
		logLevel = logger.Error
	case "warn":
		logLevel = logger.Warn
	case "info":
		logLevel = logger.Info
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   cfg.TablePrefix,
			SingularTable: true,
		},
		NowFunc: func() time.Time {
			return time.Now().Local()
		},
		// 禁用外键约束以提高性能
		DisableForeignKeyConstraintWhenMigrating: true,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to mysql: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// 设置连接池参数
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// 初始化数据库表结构
	if err := InitDatabase(db); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return db, nil
}

// NewRedisClient 创建Redis客户端
func NewRedisClient(cfg config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return client, nil
}

// InitDatabase 初始化数据库表结构
func InitDatabase(db *gorm.DB) error {
	// 自动迁移所有模型
	if err := db.AutoMigrate(
		&model.User{},
		&model.UserFollow{},
		&model.UserStats{},
		&model.UserStatsDaily{},
	); err != nil {
		return fmt.Errorf("failed to auto migrate: %w", err)
	}

	// 创建索引
	if err := createIndexes(db); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

// createIndexes 创建数据库索引
func createIndexes(db *gorm.DB) error {
	// 用户表索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)").Error; err != nil {
		return fmt.Errorf("failed to create idx_users_username: %w", err)
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_status ON users(status)").Error; err != nil {
		return fmt.Errorf("failed to create idx_users_status: %w", err)
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at)").Error; err != nil {
		return fmt.Errorf("failed to create idx_users_created_at: %w", err)
	}

	// 关注表索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_user_follows_user_id ON user_follows(user_id)").Error; err != nil {
		return fmt.Errorf("failed to create idx_user_follows_user_id: %w", err)
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_user_follows_follow_user_id ON user_follows(follow_user_id)").Error; err != nil {
		return fmt.Errorf("failed to create idx_user_follows_follow_user_id: %w", err)
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_user_follows_status ON user_follows(status)").Error; err != nil {
		return fmt.Errorf("failed to create idx_user_follows_status: %w", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uk_user_follows ON user_follows(user_id, follow_user_id)").Error; err != nil {
		return fmt.Errorf("failed to create uk_user_follows: %w", err)
	}

	// 统计表索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_user_stats_user_id ON user_stats(user_id)").Error; err != nil {
		return fmt.Errorf("failed to create idx_user_stats_user_id: %w", err)
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_user_stats_daily_user_id ON user_stats_daily(user_id, date)").Error; err != nil {
		return fmt.Errorf("failed to create idx_user_stats_daily_user_id: %w", err)
	}

	return nil
}
