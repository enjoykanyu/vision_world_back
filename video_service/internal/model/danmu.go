package model

import "time"

type Danmuku struct {
	ID        uint32  `gorm:"primaryKey;autoIncrement" json:"id"`
	VideoID   uint32  `gorm:"index;not null;comment:视频ID" json:"video_id"`
	UserID    uint32  `gorm:"index;not null;comment:用户ID" json:"user_id"`
	Content   string  `gorm:"size:200;not null;comment:弹幕内容" json:"content"`
	VideoTime float32 `gorm:"not null;comment:视频时间" json:"video_time"`
	Color     string  `gorm:"size:10;default:#FFFFFF;comment:弹幕颜色" json:"color"`
	FontSize  uint32  `gorm:"default:25;comment:字体大小" json:"font_size"`
	Position  uint32  `gorm:"default:1;comment:弹幕位置" json:"position"`
	Speed     uint32  `gorm:"default:1;comment:弹幕速度" json:"speed"`
	Status    uint32  `gorm:"default:1;comment:弹幕状态" json:"status"`
	// IsDeleted bool      `gorm:"default:false;comment:是否删除" json:"is_deleted"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定表名为 danmaku
func (Danmuku) TableName() string {
	return "danmuku"
}
