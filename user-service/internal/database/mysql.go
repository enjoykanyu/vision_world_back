package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"github.com/visionworld/user-service/internal/config"
	"github.com/visionworld/user-service/internal/model"
	"github.com/visionworld/user-service/pkg/logger"
)

var (
	DB    *gorm.DB
	sqlDB *sql.DB
)

// InitMySQL 初始化MySQL连接
func InitMySQL(cfg *config.DatabaseConfig) error {
	// 配置MySQL驱动
	mysqlConfig := mysql.Config{
		DSN: cfg.GetDSN(),
	}

	// 配置GORM
	gormConfig := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",  // 表名前缀
			SingularTable: true,  // 使用单数表名
			NoLowerCase:   false, // 表名不强制小写
		},
		Logger: logger.NewGormLogger(cfg.LogLevel),
		NowFunc: func() time.Time {
			return time.Now().Local()
		},
		PrepareStmt:            true,
		SkipDefaultTransaction: true,
	}

	// 连接数据库
	db, err := gorm.Open(mysql.New(mysqlConfig), gormConfig)
	if err != nil {
		return fmt.Errorf("连接MySQL失败: %v", err)
	}

	// 获取底层SQL DB
	sqlDB, err = db.DB()
	if err != nil {
		return fmt.Errorf("获取SQL DB失败: %v", err)
	}

	// 设置连接池参数
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("测试MySQL连接失败: %v", err)
	}

	DB = db
	logger.Info("MySQL连接初始化成功")
	return nil
}

// AutoMigrate 自动迁移表结构
func AutoMigrate() error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	err := DB.AutoMigrate(
		&model.User{},
		&model.UserProfile{},
		&model.UserLoginLog{},
		&model.VerificationCode{},
		&model.UserFollow{},
	)
	if err != nil {
		return fmt.Errorf("自动迁移表结构失败: %v", err)
	}

	logger.Info("表结构自动迁移成功")
	return nil
}

// CloseMySQL 关闭MySQL连接
func CloseMySQL() error {
	if sqlDB != nil {
		return sqlDB.Close()
	}
	return nil
}

// GetDB 获取数据库实例
func GetDB() *gorm.DB {
	return DB
}

// GetMySQL 获取MySQL连接
func GetMySQL() *sql.DB {
	return sqlDB
}

// BeginTransaction 开始事务
func BeginTransaction() *gorm.DB {
	return DB.Begin()
}

// Transaction 执行事务
func Transaction(fc func(tx *gorm.DB) error) error {
	return DB.Transaction(fc)
}
