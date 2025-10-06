package model

import (
	"fmt"
	"user_service/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// DB 数据库连接实例
var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB(cfg *config.DatabaseConfig) error {
	// 构建DSN数据源名称
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.Charset,
	)

	// GORM配置
	gormConfig := &gorm.Config{
		// 命名策略
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   cfg.TablePrefix, // 表前缀
			SingularTable: true,            // 单数表名
		},
		// 日志配置
		Logger: logger.Default.LogMode(getLogLevel(cfg.LogLevel)),
		// 禁用外键约束（提高性能）
		DisableForeignKeyConstraintWhenMigrating: true,
	}

	// 连接数据库
	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// 获取底层SQL数据库连接
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	// 设置连接池参数
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	DB = db
	return nil
}

// AutoMigrate 自动迁移数据库表结构
func AutoMigrate() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	// 自动迁移所有模型
	err := DB.AutoMigrate(
		&User{},
		&UserFollow{},
		&UserStats{},
		&UserStatsDaily{},
	)
	if err != nil {
		return fmt.Errorf("failed to auto migrate: %w", err)
	}

	return nil
}

// getLogLevel 获取GORM日志级别
func getLogLevel(level string) logger.LogLevel {
	switch level {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "warn":
		return logger.Warn
	case "info":
		return logger.Info
	default:
		return logger.Warn
	}
}

// GetDB 获取数据库连接
func GetDB() *gorm.DB {
	return DB
}

// CloseDB 关闭数据库连接
func CloseDB() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

// Transaction 执行数据库事务
func Transaction(fc func(tx *gorm.DB) error) error {
	return DB.Transaction(fc)
}

// Paginate 分页查询辅助函数
func Paginate(page, pageSize int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if page <= 0 {
			page = 1
		}
		switch {
		case pageSize > 100:
			pageSize = 100
		case pageSize <= 0:
			pageSize = 10
		}
		offset := (page - 1) * pageSize
		return db.Offset(offset).Limit(pageSize)
	}
}

// UserTabler 用户表接口
type UserTabler interface {
	TableName() string
}

// 确保模型实现了接口
var (
	_ UserTabler = (*User)(nil)
	_ UserTabler = (*UserFollow)(nil)
	_ UserTabler = (*UserStats)(nil)
	_ UserTabler = (*UserStatsDaily)(nil)
)
