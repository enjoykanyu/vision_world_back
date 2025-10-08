package repository

import (
	"audit_service/internal/model"
	"context"
	"fmt"
)

// AddToManualReviewQueue 添加到人工审核队列
func (r *auditRepository) AddToManualReviewQueue(ctx context.Context, auditID uint64) error {
	// 这里可以添加更复杂的队列逻辑，比如使用Redis队列
	// 目前简单地将审核状态更新为待人工审核
	if err := r.db.WithContext(ctx).
		Model(&model.AuditRecord{}).
		Where("id = ?", auditID).
		Update("status", model.AuditStatusPending).Error; err != nil {
		return fmt.Errorf("failed to add to manual review queue: %w", err)
	}
	return nil
}

// GetManualReviewQueue 获取人工审核队列
func (r *auditRepository) GetManualReviewQueue(ctx context.Context, req *GetManualReviewQueueRequest) (*GetManualReviewQueueResponse, error) {
	query := r.db.WithContext(ctx).
		Model(&model.AuditRecord{}).
		Where("status = ?", model.AuditStatusPending)

	// 应用过滤条件
	if req.ContentType != "" {
		query = query.Where("content_type = ?", req.ContentType)
	}
	if req.Level != "" {
		query = query.Where("level = ?", req.Level)
	}
	if req.Priority != 0 {
		query = query.Where("priority = ?", req.Priority)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count manual review queue: %w", err)
	}

	// 分页查询
	var records []*model.AuditRecord
	offset := (req.Page - 1) * req.PageSize
	if err := query.Offset(offset).Limit(req.PageSize).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("failed to get manual review queue: %w", err)
	}

	return &GetManualReviewQueueResponse{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Records:  records,
	}, nil
}

// AssignManualReview 分配人工审核
func (r *auditRepository) AssignManualReview(ctx context.Context, auditID uint64, reviewerID uint64) error {
	if err := r.db.WithContext(ctx).
		Model(&model.AuditRecord{}).
		Where("id = ?", auditID).
		Updates(map[string]interface{}{
			"reviewer_id": reviewerID,
			"status":      model.AuditStatusPending,
		}).Error; err != nil {
		return fmt.Errorf("failed to assign manual review: %w", err)
	}
	return nil
}

// GetAuditStatistics 获取审核统计
func (r *auditRepository) GetAuditStatistics(ctx context.Context, req *GetAuditStatisticsRequest) (*GetAuditStatisticsResponse, error) {
	var stats GetAuditStatisticsResponse

	// 总审核数
	var totalCount int64
	if err := r.db.WithContext(ctx).Model(&model.AuditRecord{}).Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}
	stats.TotalCount = totalCount

	// 按状态统计
	var statusStats []StatusCount
	if err := r.db.WithContext(ctx).
		Model(&model.AuditRecord{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&statusStats).Error; err != nil {
		return nil, fmt.Errorf("failed to get status statistics: %w", err)
	}
	stats.StatusStats = statusStats

	// 按违规等级统计
	var levelStats []LevelCount
	if err := r.db.WithContext(ctx).
		Model(&model.AuditRecord{}).
		Select("level, COUNT(*) as count").
		Group("level").
		Scan(&levelStats).Error; err != nil {
		return nil, fmt.Errorf("failed to get level statistics: %w", err)
	}
	stats.LevelStats = levelStats

	// 按内容类型统计
	var typeStats []TypeCount
	if err := r.db.WithContext(ctx).
		Model(&model.AuditRecord{}).
		Select("content_type, COUNT(*) as count").
		Group("content_type").
		Scan(&typeStats).Error; err != nil {
		return nil, fmt.Errorf("failed to get type statistics: %w", err)
	}
	stats.TypeStats = typeStats

	// 通过率计算
	if totalCount > 0 {
		var passedCount int64
		r.db.WithContext(ctx).
			Model(&model.AuditRecord{}).
			Where("status = ?", model.AuditStatusApproved).
			Count(&passedCount)
		stats.PassRate = float64(passedCount) / float64(totalCount) * 100
	}

	return &stats, nil
}

// GetViolationTrends 获取违规趋势
func (r *auditRepository) GetViolationTrends(ctx context.Context, req *GetViolationTrendsRequest) (*GetViolationTrendsResponse, error) {
	var trends []ViolationTrend

	// 按日期分组统计违规数量
	query := r.db.WithContext(ctx).
		Model(&model.AuditRecord{}).
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("status = ?", model.AuditStatusRejected)

	if req.StartDate != "" {
		query = query.Where("created_at >= ?", req.StartDate)
	}
	if req.EndDate != "" {
		query = query.Where("created_at <= ?", req.EndDate)
	}

	if err := query.
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&trends).Error; err != nil {
		return nil, fmt.Errorf("failed to get violation trends: %w", err)
	}

	return &GetViolationTrendsResponse{
		Trends: trends,
	}, nil
}
