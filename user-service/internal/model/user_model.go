package model

import (
	"context"
	"database/sql"
	"time"
)

// User 用户模型
type User struct {
	ID          string     `db:"user_id"`
	UserID      string     `db:"user_id"`
	Username    string     `db:"username"`
	Phone       string     `db:"phone"`
	Password    string     `db:"password"`
	Avatar      string     `db:"avatar"`
	Status      int        `db:"status"`
	LastLoginAt *time.Time `db:"last_login_at"`
	LastLoginIP string     `db:"last_login_ip"`
	LoginCount  int64      `db:"login_count"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`

	// 关联字段
	Nickname string `db:"nickname"`
	Gender   int    `db:"gender"`
	Birthday string `db:"birthday"`
}

// UserModel 用户数据模型
type UserModel struct {
	db *sql.DB
}

// NewUserModel 创建用户模型
func NewUserModel(db *sql.DB) *UserModel {
	return &UserModel{db: db}
}

// Create 创建用户
func (m *UserModel) Create(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (user_id, username, phone, password, avatar, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := m.db.ExecContext(ctx, query,
		user.UserID, user.Username, user.Phone, user.Password, user.Avatar,
		user.Status, user.CreatedAt, user.UpdatedAt)
	return err
}

// GetByID 根据ID获取用户
func (m *UserModel) GetByID(ctx context.Context, id string) (*User, error) {
	user := &User{}
	query := `
		SELECT u.user_id, u.username, u.phone, u.password, u.avatar, u.status, 
		       u.last_login_at, u.created_at, u.updated_at,
		       COALESCE(p.nickname, '') as nickname, 
		       COALESCE(p.gender, 0) as gender, 
		       COALESCE(p.birthday, '') as birthday
		FROM users u
		LEFT JOIN user_profiles p ON u.user_id = p.user_id
		WHERE u.user_id = ?
	`
	err := m.db.QueryRowContext(ctx, query, id).Scan(
		&user.UserID, &user.Username, &user.Phone, &user.Password, &user.Avatar,
		&user.Status, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
		&user.Nickname, &user.Gender, &user.Birthday)
	user.ID = user.UserID
	return user, err
}

// GetByPhone 根据手机号获取用户
func (m *UserModel) GetByPhone(ctx context.Context, phone string) (*User, error) {
	user := &User{}
	query := `
		SELECT u.user_id, u.username, u.phone, u.password, u.avatar, u.status, 
		       u.last_login_at, u.created_at, u.updated_at,
		       COALESCE(p.nickname, '') as nickname, 
		       COALESCE(p.gender, 0) as gender, 
		       COALESCE(p.birthday, '') as birthday
		FROM users u
		LEFT JOIN user_profiles p ON u.user_id = p.user_id
		WHERE u.phone = ?
	`
	err := m.db.QueryRowContext(ctx, query, phone).Scan(
		&user.UserID, &user.Username, &user.Phone, &user.Password, &user.Avatar,
		&user.Status, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
		&user.Nickname, &user.Gender, &user.Birthday)
	user.ID = user.UserID
	return user, err
}

// Update 更新用户信息
func (m *UserModel) Update(ctx context.Context, id string, data map[string]interface{}) error {
	if len(data) == 0 {
		return nil
	}

	// 构建更新语句
	setClause := ""
	args := []interface{}{}
	for key, value := range data {
		if setClause != "" {
			setClause += ", "
		}
		setClause += key + " = ?"
		args = append(args, value)
	}
	setClause += ", updated_at = ?"
	args = append(args, time.Now())
	args = append(args, id)

	query := "UPDATE users SET " + setClause + " WHERE user_id = ?"
	_, err := m.db.ExecContext(ctx, query, args...)
	return err
}

// UpdateLastLoginTime 更新最后登录时间
func (m *UserModel) UpdateLastLoginTime(ctx context.Context, id string) error {
	query := "UPDATE users SET last_login_at = ? WHERE user_id = ?"
	_, err := m.db.ExecContext(ctx, query, time.Now(), id)
	return err
}
