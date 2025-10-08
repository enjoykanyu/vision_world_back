package model

import (
	"time"
)

// LiveStream 直播流表
type LiveStream struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement;comment:直播流ID"`
	StreamKey    string `gorm:"uniqueIndex;size:64;not null;comment:直播流密钥"`
	Title        string `gorm:"size:200;not null;comment:直播标题"`
	Description  string `gorm:"type:text;comment:直播描述"`
	UserID       uint64 `gorm:"index;not null;comment:主播用户ID"`
	RoomID       uint64 `gorm:"index;not null;comment:直播间ID"`
	CategoryID   uint32 `gorm:"index;default:0;comment:直播分类ID"`
	Status       uint8  `gorm:"index;default:0;comment:直播状态:0-准备中,1-直播中,2-暂停,3-结束,4-封禁"`
	StreamType   string `gorm:"size:20;default:'rtmp';comment:直播流类型:rtmp,webrtc"`
	StreamURL    string `gorm:"size:500;comment:直播流URL"`
	PlaybackURL  string `gorm:"size:500;comment:回放URL"`
	ThumbnailURL string `gorm:"size:500;comment:缩略图URL"`

	// 直播统计
	ViewerCount  uint32 `gorm:"default:0;comment:观看人数"`
	LikeCount    uint32 `gorm:"default:0;comment:点赞数"`
	GiftCount    uint32 `gorm:"default:0;comment:礼物数"`
	CommentCount uint32 `gorm:"default:0;comment:评论数"`
	ShareCount   uint32 `gorm:"default:0;comment:分享数"`

	// 直播设置
	IsPublic      bool `gorm:"default:true;comment:是否公开"`
	IsRecord      bool `gorm:"default:false;comment:是否录制"`
	IsChatEnabled bool `gorm:"default:true;comment:是否开启聊天"`
	IsGiftEnabled bool `gorm:"default:true;comment:是否开启礼物"`

	// 直播质量
	VideoQuality string `gorm:"size:20;default:'720p';comment:视频质量"`
	AudioQuality string `gorm:"size:20;default:'high';comment:音频质量"`
	Bitrate      uint32 `gorm:"default:0;comment:码率"`
	Framerate    uint8  `gorm:"default:30;comment:帧率"`

	// 时间信息
	StartedAt    *time.Time `gorm:"comment:开始时间"`
	EndedAt      *time.Time `gorm:"comment:结束时间"`
	LastActiveAt *time.Time `gorm:"comment:最后活跃时间"`
	Duration     uint32     `gorm:"default:0;comment:直播时长(秒)"`

	// 状态信息
	IsRecommended bool  `gorm:"default:false;index;comment:是否推荐"`
	IsFeatured    bool  `gorm:"default:false;index;comment:是否精选"`
	Weight        int32 `gorm:"default:0;index;comment:权重排序"`

	// 时间戳
	CreatedAt time.Time  `gorm:"comment:创建时间"`
	UpdatedAt time.Time  `gorm:"comment:更新时间"`
	DeletedAt *time.Time `gorm:"index;comment:删除时间"`
}

// TableName 设置表名
func (LiveStream) TableName() string {
	return "live_streams"
}

// LiveRoom 直播间表
type LiveRoom struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement;comment:直播间ID"`
	RoomNumber  string `gorm:"uniqueIndex;size:20;not null;comment:房间号"`
	Name        string `gorm:"size:100;not null;comment:直播间名称"`
	Description string `gorm:"type:text;comment:直播间描述"`
	UserID      uint64 `gorm:"uniqueIndex;not null;comment:主播用户ID"`

	// 直播间设置
	CoverImage      string `gorm:"size:500;comment:封面图片URL"`
	BackgroundImage string `gorm:"size:500;comment:背景图片URL"`
	Announcement    string `gorm:"type:text;comment:直播间公告"`

	// 直播间状态
	Status   uint8 `gorm:"index;default:0;comment:状态:0-离线,1-在线,2-禁播"`
	IsActive bool  `gorm:"default:true;comment:是否激活"`

	// 直播间配置
	MaxViewers uint32 `gorm:"default:10000;comment:最大观看人数"`
	ChatLevel  uint8  `gorm:"default:0;comment:聊天等级限制"`
	GiftLevel  uint8  `gorm:"default:0;comment:礼物等级限制"`

	// 统计信息
	TotalStreams  uint32 `gorm:"default:0;comment:总直播次数"`
	TotalDuration uint64 `gorm:"default:0;comment:总直播时长"`
	TotalViewers  uint64 `gorm:"default:0;comment:总观看人数"`
	TotalLikes    uint64 `gorm:"default:0;comment:总点赞数"`
	TotalGifts    uint64 `gorm:"default:0;comment:总礼物数"`

	// 时间戳
	CreatedAt time.Time  `gorm:"comment:创建时间"`
	UpdatedAt time.Time  `gorm:"comment:更新时间"`
	DeletedAt *time.Time `gorm:"index;comment:删除时间"`
}

// TableName 设置表名
func (LiveRoom) TableName() string {
	return "live_rooms"
}

// LiveViewer 直播观看者表
type LiveViewer struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement;comment:观看记录ID"`
	StreamID uint64 `gorm:"index;not null;comment:直播流ID"`
	UserID   uint64 `gorm:"index;not null;comment:用户ID"`
	RoomID   uint64 `gorm:"index;not null;comment:直播间ID"`

	// 观看信息
	EnterTime     time.Time  `gorm:"comment:进入时间"`
	ExitTime      *time.Time `gorm:"comment:离开时间"`
	WatchDuration uint32     `gorm:"default:0;comment:观看时长(秒)"`

	// 互动信息
	IsLiked      bool       `gorm:"default:false;comment:是否点赞"`
	LikeTime     *time.Time `gorm:"comment:点赞时间"`
	GiftValue    uint64     `gorm:"default:0;comment:礼物价值"`
	CommentCount uint32     `gorm:"default:0;comment:评论数"`

	// 观看质量
	IP         string `gorm:"size:45;comment:IP地址"`
	UserAgent  string `gorm:"size:500;comment:用户代理"`
	DeviceType string `gorm:"size:50;comment:设备类型"`

	// 时间戳
	CreatedAt time.Time  `gorm:"comment:创建时间"`
	UpdatedAt time.Time  `gorm:"comment:更新时间"`
	DeletedAt *time.Time `gorm:"index;comment:删除时间"`
}

// TableName 设置表名
func (LiveViewer) TableName() string {
	return "live_viewers"
}

// LiveGift 直播礼物表
type LiveGift struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement;comment:礼物记录ID"`
	StreamID uint64 `gorm:"index;not null;comment:直播流ID"`
	UserID   uint64 `gorm:"index;not null;comment:送礼用户ID"`
	AnchorID uint64 `gorm:"index;not null;comment:主播用户ID"`
	GiftID   uint32 `gorm:"not null;comment:礼物ID"`

	// 礼物信息
	GiftName   string `gorm:"size:100;not null;comment:礼物名称"`
	GiftIcon   string `gorm:"size:500;comment:礼物图标"`
	GiftValue  uint64 `gorm:"default:0;comment:礼物价值(金币)"`
	GiftCount  uint32 `gorm:"default:1;comment:礼物数量"`
	TotalValue uint64 `gorm:"default:0;comment:总价值"`

	// 特效信息
	EffectType string `gorm:"size:50;comment:特效类型"`
	EffectData string `gorm:"type:text;comment:特效数据"`

	// 状态信息
	Status   uint8     `gorm:"default:1;comment:状态:0-失败,1-成功"`
	SendTime time.Time `gorm:"comment:发送时间"`

	// 时间戳
	CreatedAt time.Time  `gorm:"comment:创建时间"`
	UpdatedAt time.Time  `gorm:"comment:更新时间"`
	DeletedAt *time.Time `gorm:"index;comment:删除时间"`
}

// TableName 设置表名
func (LiveGift) TableName() string {
	return "live_gifts"
}

// LiveChat 直播聊天消息表
type LiveChat struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement;comment:聊天消息ID"`
	StreamID uint64 `gorm:"index;not null;comment:直播流ID"`
	UserID   uint64 `gorm:"index;not null;comment:用户ID"`
	RoomID   uint64 `gorm:"index;not null;comment:直播间ID"`

	// 消息内容
	Content     string `gorm:"type:text;not null;comment:消息内容"`
	ContentType string `gorm:"size:20;default:'text';comment:内容类型:text,image,emoji"`

	// 用户信息
	UserNickname string `gorm:"size:100;comment:用户昵称"`
	UserAvatar   string `gorm:"size:500;comment:用户头像"`
	UserLevel    uint8  `gorm:"default:0;comment:用户等级"`

	// 消息属性
	IsAnchor bool `gorm:"default:false;comment:是否主播消息"`
	IsAdmin  bool `gorm:"default:false;comment:是否管理员消息"`
	IsSystem bool `gorm:"default:false;comment:是否系统消息"`

	// 互动信息
	IsGift    bool   `gorm:"default:false;comment:是否礼物消息"`
	GiftID    uint32 `gorm:"default:0;comment:礼物ID"`
	GiftName  string `gorm:"size:100;comment:礼物名称"`
	GiftValue uint64 `gorm:"default:0;comment:礼物价值"`

	// 状态信息
	Status uint8 `gorm:"default:1;comment:状态:0-删除,1-正常"`

	// 时间戳
	CreatedAt time.Time  `gorm:"index;comment:创建时间"`
	UpdatedAt time.Time  `gorm:"comment:更新时间"`
	DeletedAt *time.Time `gorm:"index;comment:删除时间"`
}

// TableName 设置表名
func (LiveChat) TableName() string {
	return "live_chats"
}

// 直播状态常量
const (
	LiveStatusPreparing = 0 // 准备中
	LiveStatusStreaming = 1 // 直播中
	LiveStatusPaused    = 2 // 暂停
	LiveStatusEnded     = 3 // 结束
	LiveStatusBanned    = 4 // 封禁
)

// 直播间状态常量
const (
	RoomStatusOffline = 0 // 离线
	RoomStatusOnline  = 1 // 在线
	RoomStatusBanned  = 2 // 禁播
)

// 直播流类型常量
const (
	StreamTypeRTMP   = "rtmp"
	StreamTypeWebRTC = "webrtc"
)

// 内容类型常量
const (
	ContentTypeText  = "text"
	ContentTypeImage = "image"
	ContentTypeEmoji = "emoji"
)

// LiveStatus 直播状态类型
type LiveStatus uint8
