package repository

import (
	"audit_service/internal/model"
	"context"
	"fmt"

	"gorm.io/gorm"
)

// AuditRepository 审核仓库接口
type AuditRepository interface {
	// 审核记录操作
	CreateAuditRecord(ctx context.Context, record *model.AuditRecord) (uint64, error)
	GetAuditRecord(ctx context.Context, auditID uint64) (*model.AuditRecord, error)
	GetAuditRecordByContentID(ctx context.Context, contentID string) (*model.AuditRecord, error)
	UpdateAuditRecord(ctx context.Context, record *model.AuditRecord) error
	ListAuditRecords(ctx context.Context, req *ListAuditRecordsRequest) (*ListAuditRecordsResponse, error)

	// 批量操作
	BatchCreateAuditRecords(ctx context.Context, records []*model.AuditRecord) error
	GetAuditRecordsByContentIDs(ctx context.Context, contentIDs []string) ([]*model.AuditRecord, error)

	// 模板操作
	CreateTemplate(ctx context.Context, template *model.AuditTemplate) (uint64, error)
	GetTemplate(ctx context.Context, templateID uint64) (*model.AuditTemplate, error)
	UpdateTemplate(ctx context.Context, template *model.AuditTemplate) error
	ListTemplates(ctx context.Context, req *ListTemplatesRequest) (*ListTemplatesResponse, error)
	DeleteTemplate(ctx context.Context, templateID uint64) error

	// 黑白名单操作
	AddToWhitelist(ctx context.Context, whitelist *model.AuditWhitelist) error
	RemoveFromWhitelist(ctx context.Context, contentID string) error
	IsWhitelisted(ctx context.Context, contentID string, contentType model.ContentType) (bool, error)

	AddToBlacklist(ctx context.Context, blacklist *model.AuditBlacklist) error
	RemoveFromBlacklist(ctx context.Context, contentID string) error
	IsBlacklisted(ctx context.Context, contentID string, contentType model.ContentType) (bool, error)

	// 人工审核队列
	AddToManualReviewQueue(ctx context.Context, auditID uint64) error
	GetManualReviewQueue(ctx context.Context, req *GetManualReviewQueueRequest) (*GetManualReviewQueueResponse, error)
	AssignManualReview(ctx context.Context, auditID uint64, reviewerID uint64) error

	// 统计操作
	GetAuditStatistics(ctx context.Context, req *GetAuditStatisticsRequest) (*GetAuditStatisticsResponse, error)
	GetViolationTrends(ctx context.Context, req *GetViolationTrendsRequest) (*GetViolationTrendsResponse, error)
}

// auditRepository 审核仓库实现
type auditRepository struct {
	db *gorm.DB
}

// NewAuditRepository 创建审核仓库
func NewAuditRepository(db *gorm.DB) AuditRepository {
	return &auditRepository{db: db}
}

// CreateAuditRecord 创建审核记录
func (r *auditRepository) CreateAuditRecord(ctx context.Context, record *model.AuditRecord) (uint64, error) {
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return 0, fmt.Errorf("failed to create audit record: %w", err)
	}
	return record.ID, nil
}

// GetAuditRecord 获取审核记录
func (r *auditRepository) GetAuditRecord(ctx context.Context, auditID uint64) (*model.AuditRecord, error) {
	var record model.AuditRecord
	if err := r.db.WithContext(ctx).First(&record, auditID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("audit record not found: %d", auditID)
		}
		return nil, fmt.Errorf("failed to get audit record: %w", err)
	}
	return &record, nil
}

// GetAuditRecordByContentID 根据内容ID获取审核记录
func (r *auditRepository) GetAuditRecordByContentID(ctx context.Context, contentID string) (*model.AuditRecord, error) {
	var record model.AuditRecord
	if err := r.db.WithContext(ctx).Where("content_id = ?", contentID).First(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("audit record not found for content: %s", contentID)
		}
		return nil, fmt.Errorf("failed to get audit record by content ID: %w", err)
	}
	return &record, nil
}

// UpdateAuditRecord 更新审核记录
func (r *auditRepository) UpdateAuditRecord(ctx context.Context, record *model.AuditRecord) error {
	if err := r.db.WithContext(ctx).Save(record).Error; err != nil {
		return fmt.Errorf("failed to update audit record: %w", err)
	}
	return nil
}

// ListAuditRecords 获取审核记录列表
func (r *auditRepository) ListAuditRecords(ctx context.Context, req *ListAuditRecordsRequest) (*ListAuditRecordsResponse, error) {
	query := r.db.WithContext(ctx).Model(&model.AuditRecord{})

	// 应用过滤条件
	if req.ContentType != "" {
		query = query.Where("content_type = ?", req.ContentType)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.Level != "" {
		query = query.Where("level = ?", req.Level)
	}
	if req.UploaderID != 0 {
		query = query.Where("uploader_id = ?", req.UploaderID)
	}
	if req.ReviewerID != 0 {
		query = query.Where("reviewer_id = ?", req.ReviewerID)
	}
	if req.StartDate != "" {
		query = query.Where("created_at >= ?", req.StartDate)
	}
	if req.EndDate != "" {
		query = query.Where("created_at <= ?", req.EndDate)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count audit records: %w", err)
	}

	// 分页查询
	var records []*model.AuditRecord
	offset := (req.Page - 1) * req.PageSize
	if err := query.Offset(offset).Limit(req.PageSize).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("failed to list audit records: %w", err)
	}

	return &ListAuditRecordsResponse{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Records:  records,
	}, nil
}

// BatchCreateAuditRecords 批量创建审核记录
func (r *auditRepository) BatchCreateAuditRecords(ctx context.Context, records []*model.AuditRecord) error {
	if err := r.db.WithContext(ctx).CreateInBatches(records, 100).Error; err != nil {
		return fmt.Errorf("failed to batch create audit records: %w", err)
	}
	return nil
}

// GetAuditRecordsByContentIDs 根据内容ID列表获取审核记录
func (r *auditRepository) GetAuditRecordsByContentIDs(ctx context.Context, contentIDs []string) ([]*model.AuditRecord, error) {
	var records []*model.AuditRecord
	if err := r.db.WithContext(ctx).Where("content_id IN ?", contentIDs).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("failed to get audit records by content IDs: %w", err)
	}
	return records, nil
}

// CreateTemplate 创建审核模板
func (r *auditRepository) CreateTemplate(ctx context.Context, template *model.AuditTemplate) (uint64, error) {
	if err := r.db.WithContext(ctx).Create(template).Error; err != nil {
		return 0, fmt.Errorf("failed to create audit template: %w", err)
	}
	return template.ID, nil
}

// GetTemplate 获取审核模板
func (r *auditRepository) GetTemplate(ctx context.Context, templateID uint64) (*model.AuditTemplate, error) {
	var template model.AuditTemplate
	if err := r.db.WithContext(ctx).First(&template, templateID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("audit template not found: %d", templateID)
		}
		return nil, fmt.Errorf("failed to get audit template: %w", err)
	}
	return &template, nil
}

// UpdateTemplate 更新审核模板
func (r *auditRepository) UpdateTemplate(ctx context.Context, template *model.AuditTemplate) error {
	if err := r.db.WithContext(ctx).Save(template).Error; err != nil {
		return fmt.Errorf("failed to update audit template: %w", err)
	}
	return nil
}

// ListTemplates 获取审核模板列表
func (r *auditRepository) ListTemplates(ctx context.Context, req *ListTemplatesRequest) (*ListTemplatesResponse, error) {
	query := r.db.WithContext(ctx).Model(&model.AuditTemplate{})

	// 应用过滤条件
	if req.ContentType != "" {
		query = query.Where("content_type = ?", req.ContentType)
	}
	if req.Level != "" {
		query = query.Where("level = ?", req.Level)
	}
	if req.IsActive {
		query = query.Where("is_active = ?", true)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count audit templates: %w", err)
	}

	// 分页查询
	var templates []*model.AuditTemplate
	offset := (req.Page - 1) * req.PageSize
	if err := query.Offset(offset).Limit(req.PageSize).Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("failed to list audit templates: %w", err)
	}

	return &ListTemplatesResponse{
		Total:     total,
		Page:      req.Page,
		PageSize:  req.PageSize,
		Templates: templates,
	}, nil
}

// DeleteTemplate 删除审核模板
func (r *auditRepository) DeleteTemplate(ctx context.Context, templateID uint64) error {
	if err := r.db.WithContext(ctx).Delete(&model.AuditTemplate{}, templateID).Error; err != nil {
		return fmt.Errorf("failed to delete audit template: %w", err)
	}
	return nil
}

// AddToWhitelist 添加到白名单
func (r *auditRepository) AddToWhitelist(ctx context.Context, whitelist *model.AuditWhitelist) error {
	if err := r.db.WithContext(ctx).Create(whitelist).Error; err != nil {
		return fmt.Errorf("failed to add to whitelist: %w", err)
	}
	return nil
}

// RemoveFromWhitelist 从白名单移除
func (r *auditRepository) RemoveFromWhitelist(ctx context.Context, contentID string) error {
	if err := r.db.WithContext(ctx).Where("content_id = ?", contentID).Delete(&model.AuditWhitelist{}).Error; err != nil {
		return fmt.Errorf("failed to remove from whitelist: %w", err)
	}
	return nil
}

// IsWhitelisted 检查是否在白名单中
func (r *auditRepository) IsWhitelisted(ctx context.Context, contentID string, contentType model.ContentType) (bool, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&model.AuditWhitelist{}).Where("content_id = ?", contentID)
	if contentType != "" {
		query = query.Where("content_type = ?", contentType)
	}

	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check whitelist: %w", err)
	}

	return count > 0, nil
}

// AddToBlacklist 添加到黑名单
func (r *auditRepository) AddToBlacklist(ctx context.Context, blacklist *model.AuditBlacklist) error {
	if err := r.db.WithContext(ctx).Create(blacklist).Error; err != nil {
		return fmt.Errorf("failed to add to blacklist: %w", err)
	}
	return nil
}

// RemoveFromBlacklist 从黑名单移除
func (r *auditRepository) RemoveFromBlacklist(ctx context.Context, contentID string) error {
	if err := r.db.WithContext(ctx).Where("content_id = ?", contentID).Delete(&model.AuditBlacklist{}).Error; err != nil {
		return fmt.Errorf("failed to remove from blacklist: %w", err)
	}
	return nil
}

// IsBlacklisted 检查是否在黑名单中
func (r *auditRepository) IsBlacklisted(ctx context.Context, contentID string, contentType model.ContentType) (bool, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&model.AuditBlacklist{}).Where("content_id = ?", contentID)
	if contentType != "" {
		query = query.Where("content_type = ?", contentType)
	}

	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check blacklist: %w", err)
	}

	return count > 0, nil
}
