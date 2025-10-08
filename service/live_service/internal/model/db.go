package model

import (
	"gorm.io/gorm"
)

// DB 数据库连接实例
var DB *gorm.DB

// SetDB 设置数据库连接
func SetDB(db *gorm.DB) {
	DB = db
}

// GetDB 获取数据库连接
func GetDB() *gorm.DB {
	return DB
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

// LiveTabler 直播表接口
type LiveTabler interface {
	TableName() string
}

// 确保模型实现了接口
var (
	_ LiveTabler = (*LiveStream)(nil)
	_ LiveTabler = (*LiveRoom)(nil)
	_ LiveTabler = (*LiveViewer)(nil)
	_ LiveTabler = (*LiveGift)(nil)
	_ LiveTabler = (*LiveChat)(nil)
)
