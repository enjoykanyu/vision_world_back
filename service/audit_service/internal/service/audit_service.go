package service

import (
	"audit_service/internal/config"
	"audit_service/internal/model"
	"audit_service/internal/repository"
	"audit_service/pkg/logger"
	"context"
	"fmt"
	"time"
)

// AuditService 审核服务接口
type AuditService interface {
	// 内容审核
	SubmitContent(ctx context.Context, req *SubmitContentRequest) (*SubmitContentResponse, error)
	GetAuditResult(ctx context.Context, contentID string) (*AuditResult, error)
	UpdateAuditStatus(ctx context.Context, req *UpdateAuditStatusRequest) (*UpdateAuditStatusResponse, error)

	// 批量审核
	BatchSubmitContent(ctx context.Context, req *BatchSubmitContentRequest) (*BatchSubmitContentResponse, error)
	GetBatchAuditResults(ctx context.Context, contentIDs []string) ([]*AuditResult, error)

	// 人工审核
	AssignManualReview(ctx context.Context, req *AssignManualReviewRequest) (*AssignManualReviewResponse, error)
	CompleteManualReview(ctx context.Context, req *CompleteManualReviewRequest) (*CompleteManualReviewResponse, error)

	// 模板管理
	CreateTemplate(ctx context.Context, req *CreateTemplateRequest) (*CreateTemplateResponse, error)
	UpdateTemplate(ctx context.Context, req *UpdateTemplateRequest) (*UpdateTemplateResponse, error)
	GetTemplate(ctx context.Context, templateID uint64) (*Template, error)
	ListTemplates(ctx context.Context, req *ListTemplatesRequest) (*ListTemplatesResponse, error)

	// 黑白名单管理
	AddToWhitelist(ctx context.Context, req *AddToWhitelistRequest) (*AddToWhitelistResponse, error)
	RemoveFromWhitelist(ctx context.Context, contentID string) error
	AddToBlacklist(ctx context.Context, req *AddToBlacklistRequest) (*AddToBlacklistResponse, error)
	RemoveFromBlacklist(ctx context.Context, contentID string) error

	// 统计报表
	GetAuditStatistics(ctx context.Context, req *GetAuditStatisticsRequest) (*GetAuditStatisticsResponse, error)
	GetViolationTrends(ctx context.Context, req *GetViolationTrendsRequest) (*GetViolationTrendsResponse, error)
}

// auditService 审核服务实现
type auditService struct {
	config     *config.Config
	logger     logger.Logger
	repository repository.AuditRepository
}

// NewAuditService 创建审核服务
func NewAuditService(cfg *config.Config, log logger.Logger, repo repository.AuditRepository) AuditService {
	return &auditService{
		config:     cfg,
		logger:     log,
		repository: repo,
	}
}

// SubmitContent 提交内容审核
func (s *auditService) SubmitContent(ctx context.Context, req *SubmitContentRequest) (*SubmitContentResponse, error) {
	s.logger.Info("Submitting content for audit", "content_id", req.ContentID, "content_type", req.ContentType)

	// 检查黑白名单
	if whitelisted, err := s.repository.IsWhitelisted(ctx, req.ContentID, model.ContentType(req.ContentType)); err != nil {
		return nil, fmt.Errorf("failed to check whitelist: %w", err)
	} else if whitelisted {
		return &SubmitContentResponse{
			AuditID: 0,
			Status:  string(model.AuditStatusAutoPassed),
			Message: "Content is whitelisted",
		}, nil
	}

	if blacklisted, err := s.repository.IsBlacklisted(ctx, req.ContentID, model.ContentType(req.ContentType)); err != nil {
		return nil, fmt.Errorf("failed to check blacklist: %w", err)
	} else if blacklisted {
		return &SubmitContentResponse{
			AuditID: 0,
			Status:  string(model.AuditStatusAutoBlocked),
			Message: "Content is blacklisted",
		}, nil
	}

	// 创建审核记录
	// Convert string UploaderID to uint64 (assuming it's a numeric string)
	var uploaderID uint64
	fmt.Sscanf(req.UploaderID, "%d", &uploaderID)

	auditRecord := &model.AuditRecord{
		ContentID:       req.ContentID,
		ContentType:     model.ContentType(req.ContentType),
		ContentTitle:    req.ContentTitle,
		ContentURL:      req.ContentURL,
		ContentMetadata: req.ContentMetadata,
		UploaderID:      uploaderID,
		UploaderName:    req.UploaderName,
		Status:          model.AuditStatusPending,
		Level:           s.determineAuditLevel(model.ContentType(req.ContentType), req.ContentMetadata),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// 执行AI审核
	aiResult, err := s.performAIReview(ctx, auditRecord)
	if err != nil {
		s.logger.Error("AI review failed", "error", err, "content_id", req.ContentID)
	} else {
		auditRecord.AIResult = aiResult.Result
		auditRecord.AIConfidence = aiResult.Confidence
		auditRecord.Score = aiResult.Score

		// 根据AI结果决定审核状态
		if aiResult.Score >= s.config.Audit.Strategies.Content.AutoBlockThreshold {
			auditRecord.Status = model.AuditStatusAutoBlocked
		} else if aiResult.Score <= 0.2 {
			auditRecord.Status = model.AuditStatusAutoPassed
		}
	}

	// 保存审核记录
	auditID, err := s.repository.CreateAuditRecord(ctx, auditRecord)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit record: %w", err)
	}

	// 如果需要人工审核，添加到队列
	if auditRecord.Status == model.AuditStatusPending {
		if err := s.repository.AddToManualReviewQueue(ctx, auditID); err != nil {
			s.logger.Error("Failed to add to manual review queue", "error", err, "audit_id", auditID)
		}
	}

	return &SubmitContentResponse{
		AuditID: auditID,
		Status:  string(auditRecord.Status),
		Score:   auditRecord.Score,
		Message: "Content submitted for audit successfully",
	}, nil
}

// GetAuditResult 获取审核结果
func (s *auditService) GetAuditResult(ctx context.Context, contentID string) (*AuditResult, error) {
	auditRecord, err := s.repository.GetAuditRecordByContentID(ctx, contentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit record: %w", err)
	}

	return &AuditResult{
		AuditID:     auditRecord.ID,
		ContentID:   auditRecord.ContentID,
		ContentType: string(auditRecord.ContentType),
		Status:      string(auditRecord.Status),
		Score:       auditRecord.Score,
		Reason:      auditRecord.Reason,
		Details:     auditRecord.Details,
		ReviewTime:  auditRecord.ReviewTime,
	}, nil
}

// UpdateAuditStatus 更新审核状态
func (s *auditService) UpdateAuditStatus(ctx context.Context, req *UpdateAuditStatusRequest) (*UpdateAuditStatusResponse, error) {
	auditRecord, err := s.repository.GetAuditRecord(ctx, req.AuditID)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit record: %w", err)
	}

	// 更新审核状态
	auditRecord.Status = model.AuditStatus(req.Status)
	auditRecord.Reason = req.Reason
	auditRecord.Details = req.Details
	auditRecord.Violations = req.Violations
	auditRecord.ReviewerID = &req.ReviewerID
	// ReviewerName is not available in the request, so we'll leave it empty
	now := time.Now()
	auditRecord.ReviewTime = &now
	auditRecord.UpdatedAt = time.Now()

	if err := s.repository.UpdateAuditRecord(ctx, auditRecord); err != nil {
		return nil, fmt.Errorf("failed to update audit record: %w", err)
	}

	// 更新黑名单（如果是拒绝状态）
	if req.Status == string(model.AuditStatusRejected) {
		blacklistRecord := &model.AuditBlacklist{
			ContentID:   auditRecord.ContentID,
			ContentType: auditRecord.ContentType,
			UploaderID:  auditRecord.UploaderID,
			Reason:      req.Reason,
			Violations:  req.Violations,
			CreatedAt:   time.Now(),
			CreatedBy:   req.ReviewerID,
		}

		if err := s.repository.AddToBlacklist(ctx, blacklistRecord); err != nil {
			s.logger.Error("Failed to add to blacklist", "error", err, "content_id", auditRecord.ContentID)
		}
	}

	return &UpdateAuditStatusResponse{
		Success: true,
		Message: "Audit status updated successfully",
	}, nil
}

// BatchSubmitContent 批量提交内容审核
func (s *auditService) BatchSubmitContent(ctx context.Context, req *BatchSubmitContentRequest) (*BatchSubmitContentResponse, error) {
	s.logger.Info("Batch submitting content for audit", "count", len(req.ContentIDs))

	results := make([]*SubmitContentResponse, len(req.ContentIDs))

	for i, contentID := range req.ContentIDs {
		contentReq := &SubmitContentRequest{
			ContentID:   contentID,
			ContentType: req.ContentType,
			// Content and Metadata are not available in BatchSubmitContentRequest
			// Content:     req.Content,
			// Metadata:    req.Metadata,
			UploaderID: req.UploaderID,
		}

		result, err := s.SubmitContent(ctx, contentReq)
		if err != nil {
			s.logger.Error("Failed to submit content in batch", "error", err, "content_id", contentID)
			results[i] = &SubmitContentResponse{
				AuditID: 0,
				Status:  string(model.AuditStatusRejected),
				Message: fmt.Sprintf("Failed to submit content: %v", err),
			}
		} else {
			results[i] = result
		}
	}

	return &BatchSubmitContentResponse{
		Results: results,
		Message: fmt.Sprintf("Batch submitted %d contents for audit", len(req.ContentIDs)),
	}, nil
}

// GetBatchAuditResults 批量获取审核结果
func (s *auditService) GetBatchAuditResults(ctx context.Context, contentIDs []string) ([]*AuditResult, error) {
	s.logger.Info("Getting batch audit results", "count", len(contentIDs))

	results := make([]*AuditResult, len(contentIDs))

	for i, contentID := range contentIDs {
		result, err := s.GetAuditResult(ctx, contentID)
		if err != nil {
			s.logger.Error("Failed to get audit result in batch", "error", err, "content_id", contentID)
			results[i] = &AuditResult{
				AuditID:   0,
				ContentID: contentID,
				Status:    string(model.AuditStatusRejected),
				Reason:    fmt.Sprintf("Failed to get audit result: %v", err),
			}
		} else {
			results[i] = result
		}
	}

	return results, nil
}

// AssignManualReview 分配人工审核
func (s *auditService) AssignManualReview(ctx context.Context, req *AssignManualReviewRequest) (*AssignManualReviewResponse, error) {
	s.logger.Info("Assigning manual review", "audit_id", req.AuditID, "reviewer_id", req.ReviewerID)

	// 获取审核记录
	auditRecord, err := s.repository.GetAuditRecord(ctx, req.AuditID)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit record: %w", err)
	}

	// 更新审核记录
	auditRecord.ReviewerID = &req.ReviewerID
	auditRecord.UpdatedAt = time.Now()

	if err := s.repository.UpdateAuditRecord(ctx, auditRecord); err != nil {
		return nil, fmt.Errorf("failed to update audit record: %w", err)
	}

	return &AssignManualReviewResponse{
		Success: true,
		Message: "Manual review assigned successfully",
	}, nil
}

// CompleteManualReview 完成人工审核
func (s *auditService) CompleteManualReview(ctx context.Context, req *CompleteManualReviewRequest) (*CompleteManualReviewResponse, error) {
	s.logger.Info("Completing manual review", "audit_id", req.AuditID, "status", req.Status)

	// 更新审核状态
	updateReq := &UpdateAuditStatusRequest{
		AuditID:    req.AuditID,
		Status:     req.Status,
		ReviewerID: req.ReviewerID,
		Reason:     req.Reason,
		Details:    req.Details,
		Violations: req.Violations,
	}

	updateResp, err := s.UpdateAuditStatus(ctx, updateReq)
	if err != nil {
		return nil, err
	}

	return &CompleteManualReviewResponse{
		Success: updateResp.Success,
		Message: updateResp.Message,
	}, nil
}

// CreateTemplate 创建审核模板
func (s *auditService) CreateTemplate(ctx context.Context, req *CreateTemplateRequest) (*CreateTemplateResponse, error) {
	s.logger.Info("Creating audit template", "name", req.Name, "content_type", req.ContentType)
	
	// 转换UploaderID从string到uint64
	var uploaderID uint64
	if req.UploaderID != "" {
		_, err := fmt.Sscanf(req.UploaderID, "%d", &uploaderID)
		if err != nil {
			return nil, fmt.Errorf("invalid uploader ID format: %w", err)
		}
	}
	
	template := &model.AuditTemplate{
		Name:        req.Name,
		Description: req.Description,
		ContentType: model.ContentType(req.ContentType),
		Level:       model.AuditLevel(req.Level),
		Rules:       req.Rules,
		Keywords:    req.Keywords,
		Violations:  req.Violations,
		Sensitivity: req.Sensitivity,
		ThirdPartyConfig: req.ThirdPartyConfig,
		IsActive:    true,
		CreatedBy:   req.CreatedBy,
		UpdatedBy:   req.CreatedBy,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		UploaderID:  uploaderID,
	}
	
	templateID, err := s.repository.CreateTemplate(ctx, template)
	if err != nil {
		return nil, fmt.Errorf("failed to create template: %w", err)
	}
	
	return &CreateTemplateResponse{
		TemplateID: templateID,
		Message:    "Template created successfully",
	}, nil
}

// UpdateTemplate 更新审核模板
func (s *auditService) UpdateTemplate(ctx context.Context, req *UpdateTemplateRequest) (*UpdateTemplateResponse, error) {
	s.logger.Info("Updating audit template", "template_id", req.TemplateID)

	// 获取模板
	template, err := s.repository.GetTemplate(ctx, req.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	// 更新模板
	template.Name = req.Name
	template.Description = req.Description
	template.ContentType = model.ContentType(req.ContentType)
	template.Level = model.AuditLevel(req.Level)
	template.Rules = req.Rules
	template.Keywords = req.Keywords
	template.Violations = req.Violations
	template.Sensitivity = req.Sensitivity
	template.ThirdPartyConfig = req.ThirdPartyConfig
	template.IsActive = req.IsActive
	template.UpdatedBy = req.UpdatedBy
	template.UpdatedAt = time.Now()

	if err := s.repository.UpdateTemplate(ctx, template); err != nil {
		return nil, fmt.Errorf("failed to update template: %w", err)
	}

	return &UpdateTemplateResponse{
		Success: true,
		Message: "Template updated successfully",
	}, nil
}

// GetTemplate 获取审核模板
func (s *auditService) GetTemplate(ctx context.Context, templateID uint64) (*Template, error) {
	s.logger.Info("Getting audit template", "template_id", templateID)

	template, err := s.repository.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	return &Template{
		ID:               template.ID,
		Name:             template.Name,
		Description:      template.Description,
		ContentType:      string(template.ContentType),
		Level:            string(template.Level),
		Rules:            template.Rules,
		Keywords:         template.Keywords,
		Violations:       template.Violations,
		Sensitivity:      template.Sensitivity,
		ThirdPartyConfig: template.ThirdPartyConfig,
		IsActive:         template.IsActive,
		CreatedBy:        template.CreatedBy,
		UpdatedBy:        template.UpdatedBy,
		CreatedAt:        template.CreatedAt,
		UpdatedAt:        template.UpdatedAt,
	}, nil
}

// ListTemplates 获取审核模板列表
func (s *auditService) ListTemplates(ctx context.Context, req *ListTemplatesRequest) (*ListTemplatesResponse, error) {
	s.logger.Info("Listing audit templates", "content_type", req.ContentType, "page", req.Page)

	// 转换为repository层的请求类型
	repoReq := &repository.ListTemplatesRequest{
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	// 调用repository层的方法
	templates, err := s.repository.ListTemplates(ctx, repoReq)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}

	// 转换为service层的响应类型
	result := &ListTemplatesResponse{
		Success: true,
		Message: "Templates retrieved successfully",
		Total:   templates.Total,
	}

	// 转换模板列表
	for _, template := range templates.Templates {
		result.Templates = append(result.Templates, &Template{
			ID:          template.ID,
			Name:        template.Name,
			Description: template.Description,
			Category:    template.Category,
			Rules:       template.Rules,
			CreatedAt:   template.CreatedAt,
			UpdatedAt:   template.UpdatedAt,
		})
	}

	return result, nil
}

// AddToWhitelist 添加到白名单
func (s *auditService) AddToWhitelist(ctx context.Context, req *AddToWhitelistRequest) (*AddToWhitelistResponse, error) {
	s.logger.Info("Adding to whitelist", "content_id", req.ContentID, "content_type", req.ContentType)

	whitelist := &model.AuditWhitelist{
		ContentID:   req.ContentID,
		ContentType: model.ContentType(req.ContentType),
		UploaderID:  req.UploaderID,
		Reason:      req.Reason,
		IsPermanent: req.IsPermanent,
		CreatedAt:   time.Now(),
		CreatedBy:   req.CreatedBy,
	}

	if req.ExpiryDate != "" {
		expiryTime, err := time.Parse("2006-01-02 15:04:05", req.ExpiryDate)
		if err != nil {
			return nil, fmt.Errorf("invalid expiry date format: %w", err)
		}
		whitelist.ExpiryDate = &expiryTime
	}

	if err := s.repository.AddToWhitelist(ctx, whitelist); err != nil {
		return nil, fmt.Errorf("failed to add to whitelist: %w", err)
	}

	return &AddToWhitelistResponse{
		Success: true,
		Message: "Successfully added to whitelist",
	}, nil
}

// RemoveFromWhitelist 从白名单移除
func (s *auditService) RemoveFromWhitelist(ctx context.Context, contentID string) error {
	s.logger.Info("Removing from whitelist", "content_id", contentID)

	if err := s.repository.RemoveFromWhitelist(ctx, contentID); err != nil {
		return fmt.Errorf("failed to remove from whitelist: %w", err)
	}

	return nil
}

// AddToBlacklist 添加到黑名单
func (s *auditService) AddToBlacklist(ctx context.Context, req *AddToBlacklistRequest) (*AddToBlacklistResponse, error) {
	s.logger.Info("Adding to blacklist", "content_id", req.ContentID, "content_type", req.ContentType)

	blacklist := &model.AuditBlacklist{
		ContentID:   req.ContentID,
		ContentType: model.ContentType(req.ContentType),
		UploaderID:  req.UploaderID,
		Reason:      req.Reason,
		Violations:  req.Violations,
		IsPermanent: req.IsPermanent,
		CreatedAt:   time.Now(),
		CreatedBy:   req.CreatedBy,
	}

	if req.ExpiryDate != "" {
		expiryTime, err := time.Parse("2006-01-02 15:04:05", req.ExpiryDate)
		if err != nil {
			return nil, fmt.Errorf("invalid expiry date format: %w", err)
		}
		blacklist.ExpiryDate = &expiryTime
	}

	if err := s.repository.AddToBlacklist(ctx, blacklist); err != nil {
		return nil, fmt.Errorf("failed to add to blacklist: %w", err)
	}

	return &AddToBlacklistResponse{
		Success: true,
		Message: "Successfully added to blacklist",
	}, nil
}

// RemoveFromBlacklist 从黑名单移除
func (s *auditService) RemoveFromBlacklist(ctx context.Context, contentID string) error {
	s.logger.Info("Removing from blacklist", "content_id", contentID)

	if err := s.repository.RemoveFromBlacklist(ctx, contentID); err != nil {
		return fmt.Errorf("failed to remove from blacklist: %w", err)
	}

	return nil
}

// GetAuditStatistics 获取审核统计
func (s *auditService) GetAuditStatistics(ctx context.Context, req *GetAuditStatisticsRequest) (*GetAuditStatisticsResponse, error) {
	s.logger.Info("Getting audit statistics", "start_date", req.StartDate, "end_date", req.EndDate)

	// 调用repository获取统计数据
	stats, err := s.repository.GetAuditStatistics(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit statistics: %w", err)
	}

	return stats, nil
}

// GetViolationTrends 获取违规趋势
func (s *auditService) GetViolationTrends(ctx context.Context, req *GetViolationTrendsRequest) (*GetViolationTrendsResponse, error) {
	s.logger.Info("Getting violation trends", "start_date", req.StartDate, "end_date", req.EndDate)

	// 调用repository获取趋势数据
	trends, err := s.repository.GetViolationTrends(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get violation trends: %w", err)
	}

	return trends, nil
}

// ListAuditRecords 获取审核记录列表
func (s *auditService) ListAuditRecords(ctx context.Context, req *ListAuditRecordsRequest) (*ListAuditRecordsResponse, error) {
	s.logger.Info("Listing audit records", "content_type", req.ContentType, "page", req.Page)

	// 调用repository获取审核记录列表
	records, err := s.repository.ListAuditRecords(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit records: %w", err)
	}

	return records, nil
}

// GetManualReviewQueue 获取人工审核队列
func (s *auditService) GetManualReviewQueue(ctx context.Context, req *GetManualReviewQueueRequest) (*GetManualReviewQueueResponse, error) {
	s.logger.Info("Getting manual review queue", "content_type", req.ContentType, "page", req.Page)

	// 调用repository获取人工审核队列
	queue, err := s.repository.GetManualReviewQueue(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get manual review queue: %w", err)
	}

	return queue, nil
}

// determineAuditLevel 确定审核级别
func (s *auditService) determineAuditLevel(contentType model.ContentType, metadata string) model.AuditLevel {
	// 根据内容类型和元数据确定审核级别
	switch contentType {
	case model.ContentTypeVideo:
		return model.AuditLevelHigh
	case model.ContentTypeImage:
		return model.AuditLevelMedium
	case model.ContentTypeText:
		return model.AuditLevelLow
	case model.ContentTypeAudio:
		return model.AuditLevelMedium
	default:
		return model.AuditLevelMedium
	}
}

// performAIReview 执行AI审核
func (s *auditService) performAIReview(ctx context.Context, record *model.AuditRecord) (*AIReviewResult, error) {
	// 这里应该调用实际的AI审核服务
	// 现在返回模拟结果
	return &AIReviewResult{
		Result:     `{"violations": [], "keywords": [], "risk_level": "low"}`,
		Confidence: 0.95,
		Score:      0.1, // 低风险分数
	}, nil
}
