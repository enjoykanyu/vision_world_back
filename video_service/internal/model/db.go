package model

import (
	"gorm.io/gorm"
)

// DB 数据库连接相关模型
type DB struct {
	*gorm.DB
}

func NewDB(db *gorm.DB) *DB {
	return &DB{DB: db}
}

// InitTables 初始化数据表
func (db *DB) InitTables() error {
	return db.AutoMigrate(
		&Video{},
		&VideoLike{},
		&VideoComment{},
		&VideoShare{},
		&VideoFavorite{},
		&VideoView{},
		&VideoCategory{},
		&VideoTag{},
		&VideoTagRelation{},
	)
}
