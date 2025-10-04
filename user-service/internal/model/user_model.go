package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// UserModel 用户模型
type UserModel struct {
	db *gorm.DB
}

// NewUserModel 创建用户模型
func NewUserModel(db *sql.DB) *UserModel {
	// 将sql.DB转换为gorm.DB
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn: db,
	}), &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("failed to open gorm db: %v", err))
	}

	return &UserModel{
		db: gormDB,
	}
}

// GetByID 根据用户ID获取用户信息
func (m *UserModel) GetByID(ctx context.Context, userID string) (*User, error) {
	var user User
	result := m.db.WithContext(ctx).Where("user_id = ?", userID).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, sql.ErrNoRows
		}
		return nil, result.Error
	}
	return &user, nil
}

// GetByPhone 根据手机号获取用户信息
func (m *UserModel) GetByPhone(ctx context.Context, phone string) (*User, error) {
	var user User
	result := m.db.WithContext(ctx).Where("phone = ?", phone).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, sql.ErrNoRows
		}
		return nil, result.Error
	}
	return &user, nil
}

// UpdateLastLoginTime 更新最后登录时间
func (m *UserModel) UpdateLastLoginTime(ctx context.Context, userID uint64) error {
	now := time.Now()
	result := m.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"last_login_at": now,
		"login_count":   gorm.Expr("login_count + 1"),
	})
	return result.Error
}

// Create 创建用户
func (m *UserModel) Create(ctx context.Context, user *User) error {
	result := m.db.WithContext(ctx).Create(user)
	return result.Error
}

// Update 更新用户信息
func (m *UserModel) Update(ctx context.Context, userID string, updateData map[string]interface{}) error {
	result := m.db.WithContext(ctx).Model(&User{}).Where("user_id = ?", userID).Updates(updateData)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
