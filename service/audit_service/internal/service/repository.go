package service

import (
	"context"
	"fmt"

	"github.com/vision_world/audit_service/internal/config"
	"github.com/vision_world/audit_service/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// AuditRepository 审核数据访问层
type AuditRepository struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewAuditRepository 创建审核数据访问层
func NewAuditRepository(cfg *config.Config, logger *zap.Logger) (*AuditRepository, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Database,
		cfg.Database.Charset,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 自动迁移表结构
	if err := db.AutoMigrate(
		&model.AuditRecord{},
		&model.AuditTemplate{},
		&model.AuditWhitelist{},
		&model.AuditBlacklist{},
		&model.AuditStatistics{},
	); err != nil {
		return nil, fmt.Errorf("failed to auto migrate: %w", err)
	}

	logger.Info("Database connected and migrated successfully",
		zap.String("database", cfg.Database.Database))

	return &AuditRepository{
		db:     db,
		logger: logger,
	}, nil
}

// CreateAuditRecord 创建审核记录
func (r *AuditRepository) CreateAuditRecord(ctx context.Context, record *model.AuditRecord) error {
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		r.logger.Error("Failed to create audit record", zap.Error(err))
		return fmt.Errorf("failed to create audit record: %w", err)
	}
	return nil
}

// GetAuditRecordByAuditID 根据审核ID获取审核记录
func (r *AuditRepository) GetAuditRecordByAuditID(ctx context.Context, auditID string) (*model.AuditRecord, error) {
	var record model.AuditRecord
	if err := r.db.WithContext(ctx).Where("audit_id = ?", auditID).First(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		r.logger.Error("Failed to get audit record by audit ID", zap.Error(err))
		return nil, fmt.Errorf("failed to get audit record: %w", err)
	}
	return &record, nil
}

// GetAuditRecordByContentID 根据内容ID获取审核记录
func (r *AuditRepository) GetAuditRecordByContentID(ctx context.Context, contentID string) (*model.AuditRecord, error) {
	var record model.AuditRecord
	if err := r.db.WithContext(ctx).Where("content_id = ?", contentID).First(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		r.logger.Error("Failed to get audit record by content ID", zap.Error(err))
		return nil, fmt.Errorf("failed to get audit record: %w", err)
	}
	return &record, nil
}

// UpdateAuditRecord 更新审核记录
func (r *AuditRepository) UpdateAuditRecord(ctx context.Context, record *model.AuditRecord) error {
	if err := r.db.WithContext(ctx).Save(record).Error; err != nil {
		r.logger.Error("Failed to update audit record", zap.Error(err))
		return fmt.Errorf("failed to update audit record: %w", err)
	}
	return nil
}

// UpdateAuditStatus 更新审核状态
func (r *AuditRepository) UpdateAuditStatus(ctx context.Context, auditID string, status model.AuditStatus, reason string) error {
	result := r.db.WithContext(ctx).Model(&model.AuditRecord{}).
		Where("audit_id = ?", auditID).
		Updates(map[string]interface{}{
			"status": status,
			"reason": reason,
		})

	if result.Error != nil {
		r.logger.Error("Failed to update audit status", zap.Error(result.Error))
		return fmt.Errorf("failed to update audit status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("audit record not found")
	}

	return nil
}

// GetAuditRecords 获取审核记录列表
func (r *AuditRepository) GetAuditRecords(ctx context.Context, req interface{}) ([]*model.AuditRecord, int64, error) {
	// 这里简化处理，实际应该根据req参数进行查询
	var records []*model.AuditRecord
	var total int64

	query := r.db.WithContext(ctx).Model(&model.AuditRecord{})

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		r.logger.Error("Failed to count audit records", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to count audit records: %w", err)
	}

	// 获取记录
	if err := query.Find(&records).Error; err != nil {
		r.logger.Error("Failed to get audit records", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to get audit records: %w", err)
	}

	return records, total, nil
}

// Close 关闭数据库连接
func (r *AuditRepository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		r.logger.Error("Failed to close database connection", zap.Error(err))
		return fmt.Errorf("failed to close database connection: %w", err)
	}

	r.logger.Info("Database connection closed")
	return nil
}
