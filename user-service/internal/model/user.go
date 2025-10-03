package model

import (
	"time"

	"gorm.io/gorm"
)

// User 用户基础信息表
type User struct {
	ID          uint64         `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	UserID      string         `gorm:"uniqueIndex;not null;column:user_id;type:varchar(32);comment:用户唯一ID" json:"user_id"`
	Username    string         `gorm:"uniqueIndex;not null;column:username;type:varchar(50);comment:用户名" json:"username"`
	Phone       string         `gorm:"uniqueIndex;not null;column:phone;type:varchar(20);comment:手机号" json:"phone"`
	Password    string         `gorm:"not null;column:password;type:varchar(255);comment:密码哈希" json:"-"`
	Avatar      string         `gorm:"column:avatar;type:varchar(255);comment:头像URL" json:"avatar"`
	Status      int32          `gorm:"not null;default:1;column:status;type:tinyint;comment:状态：1-正常，2-禁用，3-删除" json:"status"`
	LastLoginAt *time.Time     `gorm:"column:last_login_at;type:datetime;comment:最后登录时间" json:"last_login_at"`
	LastLoginIP string         `gorm:"column:last_login_ip;type:varchar(50);comment:最后登录IP" json:"last_login_ip"`
	LoginCount  int64          `gorm:"not null;default:0;column:login_count;type:bigint;comment:登录次数" json:"login_count"`
	CreatedAt   time.Time      `gorm:"column:created_at;type:datetime;comment:创建时间" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"column:updated_at;type:datetime;comment:更新时间" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index;column:deleted_at;type:datetime;comment:删除时间" json:"-"`

	// 关联关系
	Profile *UserProfile `gorm:"foreignKey:UserID;references:UserID" json:"profile,omitempty"`
}

// TableName 设置表名
func (User) TableName() string {
	return "users"
}

// BeforeCreate 创建前的钩子
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.Status == 0 {
		u.Status = 1 // 默认状态为正常
	}
	return nil
}

// UserProfile 用户详细信息表
type UserProfile struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	UserID         string    `gorm:"uniqueIndex;not null;column:user_id;type:varchar(32);comment:用户ID" json:"user_id"`
	Signature      string    `gorm:"column:signature;type:varchar(500);comment:个性签名" json:"signature"`
	Birthday       string    `gorm:"column:birthday;type:varchar(20);comment:生日" json:"birthday"`
	Gender         int32     `gorm:"column:gender;type:tinyint;comment:性别：0-未知，1-男，2-女" json:"gender"`
	Location       string    `gorm:"column:location;type:varchar(100);comment:地区" json:"location"`
	FollowerCount  int64     `gorm:"not null;default:0;column:follower_count;type:bigint;comment:粉丝数" json:"follower_count"`
	FollowingCount int64     `gorm:"not null;default:0;column:following_count;type:bigint;comment:关注数" json:"following_count"`
	VideoCount     int64     `gorm:"not null;default:0;column:video_count;type:bigint;comment:视频数" json:"video_count"`
	LikeCount      int64     `gorm:"not null;default:0;column:like_count;type:bigint;comment:获赞数" json:"like_count"`
	CreatedAt      time.Time `gorm:"column:created_at;type:datetime;comment:创建时间" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;type:datetime;comment:更新时间" json:"updated_at"`
}

// TableName 设置表名
func (UserProfile) TableName() string {
	return "user_profiles"
}

// UserLoginLog 用户登录日志表
type UserLoginLog struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	UserID      string    `gorm:"index;not null;column:user_id;type:varchar(32);comment:用户ID" json:"user_id"`
	LoginAt     time.Time `gorm:"not null;column:login_at;type:datetime;comment:登录时间" json:"login_at"`
	LoginIP     string    `gorm:"column:login_ip;type:varchar(50);comment:登录IP" json:"login_ip"`
	DeviceID    string    `gorm:"column:device_id;type:varchar(100);comment:设备ID" json:"device_id"`
	Platform    int32     `gorm:"column:platform;type:tinyint;comment:平台：1-Web，2-iOS，3-Android" json:"platform"`
	OSVersion   string    `gorm:"column:os_version;type:varchar(50);comment:系统版本" json:"os_version"`
	AppVersion  string    `gorm:"column:app_version;type:varchar(50);comment:应用版本" json:"app_version"`
	DeviceModel string    `gorm:"column:device_model;type:varchar(100);comment:设备型号" json:"device_model"`
	Status      int32     `gorm:"not null;default:1;column:status;type:tinyint;comment:状态：1-成功，2-失败" json:"status"`
	ErrorMsg    string    `gorm:"column:error_msg;type:varchar(500);comment:错误信息" json:"error_msg"`
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime;comment:创建时间" json:"created_at"`
}

// TableName 设置表名
func (UserLoginLog) TableName() string {
	return "user_login_logs"
}

// VerificationCode 验证码表
type VerificationCode struct {
	ID        uint64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	Phone     string     `gorm:"index;not null;column:phone;type:varchar(20);comment:手机号" json:"phone"`
	Code      string     `gorm:"not null;column:code;type:varchar(10);comment:验证码" json:"code"`
	Scene     string     `gorm:"not null;column:scene;type:varchar(50);comment:场景：login-登录，register-注册，reset_pwd-重置密码" json:"scene"`
	ExpireAt  time.Time  `gorm:"index;not null;column:expire_at;type:datetime;comment:过期时间" json:"expire_at"`
	Used      bool       `gorm:"not null;default:false;column:used;type:tinyint(1);comment:是否已使用" json:"used"`
	UsedAt    *time.Time `gorm:"column:used_at;type:datetime;comment:使用时间" json:"used_at"`
	IP        string     `gorm:"column:ip;type:varchar(50);comment:IP地址" json:"ip"`
	DeviceID  string     `gorm:"column:device_id;type:varchar(100);comment:设备ID" json:"device_id"`
	CreatedAt time.Time  `gorm:"column:created_at;type:datetime;comment:创建时间" json:"created_at"`
}

// TableName 设置表名
func (VerificationCode) TableName() string {
	return "verification_codes"
}

// UserFollow 用户关注关系表
type UserFollow struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	FollowerID  string    `gorm:"index;not null;column:follower_id;type:varchar(32);comment:关注者ID" json:"follower_id"`
	FollowingID string    `gorm:"index;not null;column:following_id;type:varchar(32);comment:被关注者ID" json:"following_id"`
	Status      int32     `gorm:"not null;default:1;column:status;type:tinyint;comment:状态：1-正常，2-取消关注" json:"status"`
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime;comment:创建时间" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;type:datetime;comment:更新时间" json:"updated_at"`
}

// TableName 设置表名
func (UserFollow) TableName() string {
	return "user_follows"
}

// 状态常量定义
const (
	UserStatusActive   int32 = 1 // 正常
	UserStatusDisabled int32 = 2 // 禁用
	UserStatusDeleted  int32 = 3 // 删除

	GenderUnknown int32 = 0 // 未知
	GenderMale    int32 = 1 // 男
	GenderFemale  int32 = 2 // 女

	PlatformWeb     int32 = 1 // Web
	PlatformIOS     int32 = 2 // iOS
	PlatformAndroid int32 = 3 // Android

	LoginStatusSuccess int32 = 1 // 登录成功
	LoginStatusFailed  int32 = 2 // 登录失败

	FollowStatusActive   int32 = 1 // 关注
	FollowStatusInactive int32 = 2 // 取消关注
)

// 场景常量定义
const (
	SceneLogin    string = "login"     // 登录
	SceneRegister string = "register"  // 注册
	SceneResetPwd string = "reset_pwd" // 重置密码
)
