package model

import (
	"time"
)

// User 用户基础信息表
type User struct {
	ID              uint32     `gorm:"primaryKey;autoIncrement;comment:用户ID"`
	Username        string     `gorm:"uniqueIndex;size:50;not null;comment:用户名"`
	Phone           string     `gorm:"uniqueIndex;size:20;comment:手机号"`
	Email           string     `gorm:"size:100;comment:邮箱"`
	PasswordHash    string     `gorm:"size:255;not null;comment:密码哈希"`
	Nickname        string     `gorm:"size:100;not null;comment:昵称"`
	AvatarURL       string     `gorm:"size:500;comment:头像URL"`
	BackgroundImage string     `gorm:"size:500;comment:背景图URL"`
	Signature       string     `gorm:"type:text;comment:个人简介"`
	Gender          uint8      `gorm:"default:0;comment:性别:0-未知,1-男,2-女"`
	Birthday        *time.Time `gorm:"type:date;comment:生日"`

	// 统计数字（冗余存储，用于快速展示）
	FollowingCount uint32 `gorm:"default:0;comment:关注数量"`
	FollowersCount uint32 `gorm:"default:0;comment:粉丝数量"`
	TotalFavorited uint64 `gorm:"default:0;comment:获赞总数"`
	WorkCount      uint32 `gorm:"default:0;comment:作品数量"`
	FavoriteCount  uint32 `gorm:"default:0;comment:点赞数量"`

	// 状态信息
	IsVerified  bool       `gorm:"default:false;comment:是否认证"`
	UserType    string     `gorm:"size:20;default:'normal';comment:用户类型:normal,verified,official"`
	Status      uint8      `gorm:"default:1;index;comment:状态:0-禁用,1-正常"`
	LastLoginAt *time.Time `gorm:"comment:最后登录时间"`

	// 时间戳
	CreatedAt time.Time  `gorm:"comment:创建时间"`
	UpdatedAt time.Time  `gorm:"comment:更新时间"`
	DeletedAt *time.Time `gorm:"index;comment:删除时间"`
}

// TableName 设置表名
func (User) TableName() string {
	return "users"
}

// ToProtoUser 转换为protobuf User结构
type ProtoUser interface {
	GetId() uint32
	GetName() string
	GetPhone() string
	GetFollowCount() uint32
	GetFollowerCount() uint32
	GetIsFollow() bool
	GetAvatar() string
	GetBackgroundImage() string
	GetSignature() string
	GetTotalFavorited() uint32
	GetWorkCount() uint32
	GetFavoriteCount() uint32
	GetCreateTime() int64
	GetLastLoginTime() int64
	GetIsVerified() bool
	GetUserType() string
}

// ToProto 转换为protobuf格式
func (u *User) ToProto() map[string]interface{} {
	return map[string]interface{}{
		"id":               u.ID,
		"username":         u.Username,
		"phone":            u.Phone,
		"nickname":         u.Nickname,
		"avatar_url":       u.AvatarURL,
		"background_image": u.BackgroundImage,
		"signature":        u.Signature,
		"gender":           u.Gender,
		"birthday":         u.Birthday,
		"following_count":  u.FollowingCount,
		"followers_count":  u.FollowersCount,
		"total_favorited":  u.TotalFavorited,
		"work_count":       u.WorkCount,
		"favorite_count":   u.FavoriteCount,
		"is_verified":      u.IsVerified,
		"user_type":        u.UserType,
		"status":           u.Status,
		"last_login_at":    u.LastLoginAt,
		"created_at":       u.CreatedAt,
		"updated_at":       u.UpdatedAt,
	}
}

// GetPublicInfo 获取公开信息（脱敏）
func (u *User) GetPublicInfo() map[string]interface{} {
	// 手机号脱敏处理
	maskedPhone := ""
	if u.Phone != "" && len(u.Phone) >= 11 {
		maskedPhone = u.Phone[:3] + "****" + u.Phone[7:]
	}

	return map[string]interface{}{
		"id":               u.ID,
		"username":         u.Username,
		"phone":            maskedPhone,
		"nickname":         u.Nickname,
		"avatar_url":       u.AvatarURL,
		"background_image": u.BackgroundImage,
		"signature":        u.Signature,
		"following_count":  u.FollowingCount,
		"followers_count":  u.FollowersCount,
		"total_favorited":  u.TotalFavorited,
		"work_count":       u.WorkCount,
		"favorite_count":   u.FavoriteCount,
		"is_verified":      u.IsVerified,
		"user_type":        u.UserType,
		"created_at":       u.CreatedAt.Unix(),
		"last_login_at":    u.getLastLoginTimestamp(),
	}
}

// getLastLoginTimestamp 获取最后登录时间戳
func (u *User) getLastLoginTimestamp() int64 {
	if u.LastLoginAt != nil {
		return u.LastLoginAt.Unix()
	}
	return 0
}

// 用户状态常量
const (
	UserStatusDisabled = 0 // 禁用
	UserStatusActive   = 1 // 正常
)

// IsActive 检查用户是否活跃
func (u *User) IsActive() bool {
	return u.Status == UserStatusActive && u.DeletedAt == nil
}

// IsOfficial 检查是否为官方账号
func (u *User) IsOfficial() bool {
	return u.UserType == "official"
}

// IsVerified 检查是否认证用户
func (u *User) IsVerifiedUser() bool {
	return u.IsVerified || u.UserType == "verified"
}

// UpdateStats 更新统计数字
func (u *User) UpdateStats(field string, delta int32) {
	switch field {
	case "following_count":
		if delta > 0 || u.FollowingCount >= uint32(-delta) {
			u.FollowingCount = uint32(int32(u.FollowingCount) + delta)
		}
	case "followers_count":
		if delta > 0 || u.FollowersCount >= uint32(-delta) {
			u.FollowersCount = uint32(int32(u.FollowersCount) + delta)
		}
	case "total_favorited":
		if delta > 0 || u.TotalFavorited >= uint64(-delta) {
			u.TotalFavorited = uint64(int64(u.TotalFavorited) + int64(delta))
		}
	case "work_count":
		if delta > 0 || u.WorkCount >= uint32(-delta) {
			u.WorkCount = uint32(int32(u.WorkCount) + delta)
		}
	case "favorite_count":
		if delta > 0 || u.FavoriteCount >= uint32(-delta) {
			u.FavoriteCount = uint32(int32(u.FavoriteCount) + delta)
		}
	}
}
