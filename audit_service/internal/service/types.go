package service

import (
	"time"
)

// SubmitContentRequest 提交内容审核请求
type SubmitContentRequest struct {
	ContentID       string `json:"content_id" binding:"required"`
	ContentType     string `json:"content_type" binding:"required"`
	ContentTitle    string `json:"content_title"`
	ContentURL      string `json:"content_url"`
	ContentMetadata string `json:"content_metadata"`
	UploaderID      string `json:"uploader_id" binding:"required"`
	UploaderName    string `json:"uploader_name"`
}

// SubmitContentResponse 提交内容审核响应
type SubmitContentResponse struct {
	AuditID uint64  `json:"audit_id"`
	Status  string  `json:"status"`
	Score   float64 `json:"score"`
	Message string  `json:"message"`
}

// AuditResult 审核结果
type AuditResult struct {
	AuditID     uint64     `json:"audit_id"`
	ContentID   string     `json:"content_id"`
	ContentType string     `json:"content_type"`
	Status      string     `json:"status"`
	Score       float64    `json:"score"`
	Reason      string     `json:"reason"`
	Details     string     `json:"details"`
	ReviewTime  *time.Time `json:"review_time"`
}

// UpdateAuditStatusRequest 更新审核状态请求
type UpdateAuditStatusRequest struct {
	AuditID    uint64 `json:"audit_id" binding:"required"`
	Status     string `json:"status" binding:"required"`
	ReviewerID uint64 `json:"reviewer_id" binding:"required"`
	Reason     string `json:"reason"`
	Details    string `json:"details"`
	Violations string `json:"violations"`
}

// UpdateAuditStatusResponse 更新审核状态响应
type UpdateAuditStatusResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// BatchSubmitContentRequest 批量提交内容审核请求
type BatchSubmitContentRequest struct {
	ContentIDs  []string `json:"content_ids" binding:"required"`
	ContentType string   `json:"content_type" binding:"required"`
	Content     string   `json:"content"`
	UploaderID  string   `json:"uploader_id" binding:"required"`
	Metadata    string   `json:"metadata"`
}

// BatchSubmitContentResponse 批量提交内容审核响应
type BatchSubmitContentResponse struct {
	Results []*SubmitContentResponse `json:"results"`
	Message string                   `json:"message"`
}

// AssignManualReviewRequest 分配人工审核请求
type AssignManualReviewRequest struct {
	AuditID    uint64 `json:"audit_id" binding:"required"`
	ReviewerID uint64 `json:"reviewer_id" binding:"required"`
}

// AssignManualReviewResponse 分配人工审核响应
type AssignManualReviewResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// CompleteManualReviewRequest 完成人工审核请求
type CompleteManualReviewRequest struct {
	AuditID    uint64 `json:"audit_id" binding:"required"`
	Status     string `json:"status" binding:"required"`
	ReviewerID uint64 `json:"reviewer_id" binding:"required"`
	Reason     string `json:"reason"`
	Details    string `json:"details"`
	Violations string `json:"violations"`
}

// CompleteManualReviewResponse 完成人工审核响应
type CompleteManualReviewResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// CreateTemplateRequest 创建审核模板请求
type CreateTemplateRequest struct {
	Name             string  `json:"name" binding:"required"`
	Description      string  `json:"description"`
	ContentType      string  `json:"content_type" binding:"required"`
	Level            string  `json:"level" binding:"required"`
	Rules            string  `json:"rules"`
	Keywords         string  `json:"keywords"`
	Violations       string  `json:"violations"`
	Sensitivity      float64 `json:"sensitivity"`
	ThirdPartyConfig string  `json:"third_party_config"`
	CreatedBy        uint64  `json:"created_by" binding:"required"`
}

// CreateTemplateResponse 创建审核模板响应
type CreateTemplateResponse struct {
	TemplateID uint64 `json:"template_id"`
	Message    string `json:"message"`
}

// UpdateTemplateRequest 更新审核模板请求
type UpdateTemplateRequest struct {
	TemplateID       uint64  `json:"template_id" binding:"required"`
	Name             string  `json:"name" binding:"required"`
	Description      string  `json:"description"`
	ContentType      string  `json:"content_type" binding:"required"`
	Level            string  `json:"level" binding:"required"`
	Rules            string  `json:"rules"`
	Keywords         string  `json:"keywords"`
	Violations       string  `json:"violations"`
	Sensitivity      float64 `json:"sensitivity"`
	ThirdPartyConfig string  `json:"third_party_config"`
	IsActive         bool    `json:"is_active"`
	UpdatedBy        uint64  `json:"updated_by" binding:"required"`
}

// UpdateTemplateResponse 更新审核模板响应
type UpdateTemplateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Template 审核模板
type Template struct {
	ID               uint64    `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	ContentType      string    `json:"content_type"`
	Level            string    `json:"level"`
	Rules            string    `json:"rules"`
	Keywords         string    `json:"keywords"`
	Violations       string    `json:"violations"`
	Sensitivity      float64   `json:"sensitivity"`
	ThirdPartyConfig string    `json:"third_party_config"`
	IsActive         bool      `json:"is_active"`
	CreatedBy        uint64    `json:"created_by"`
	UpdatedBy        uint64    `json:"updated_by"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ListTemplatesRequest 获取审核模板列表请求
type ListTemplatesRequest struct {
	ContentType string `json:"content_type"`
	Level       string `json:"level"`
	IsActive    *bool  `json:"is_active"`
	Page        int    `json:"page" binding:"min=1"`
	PageSize    int    `json:"page_size" binding:"min=1,max=100"`
}

// ListTemplatesResponse 获取审核模板列表响应
type ListTemplatesResponse struct {
	Templates []*Template `json:"templates"`
	Total     int64       `json:"total"`
	Page      int         `json:"page"`
	PageSize  int         `json:"page_size"`
}

// AddToWhitelistRequest 添加到白名单请求
type AddToWhitelistRequest struct {
	ContentID   string `json:"content_id" binding:"required"`
	ContentType string `json:"content_type" binding:"required"`
	UploaderID  string `json:"uploader_id"`
	Reason      string `json:"reason"`
	IsPermanent bool   `json:"is permanent"`
	ExpiryDate  string `json:"expiry_date"`
	CreatedBy   uint64 `json:"created_by" binding:"required"`
}

// AddToWhitelistResponse 添加到白名单响应
type AddToWhitelistResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// AddToBlacklistRequest 添加到黑名单请求
type AddToBlacklistRequest struct {
	ContentID   string `json:"content_id" binding:"required"`
	ContentType string `json:"content_type" binding:"required"`
	UploaderID  string `json:"uploader_id"`
	Reason      string `json:"reason"`
	Violations  string `json:"violations"`
	IsPermanent bool   `json:"is permanent"`
	ExpiryDate  string `json:"expiry_date"`
	CreatedBy   uint64 `json:"created_by" binding:"required"`
}

// AddToBlacklistResponse 添加到黑名单响应
type AddToBlacklistResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// GetAuditStatisticsRequest 获取审核统计请求
type GetAuditStatisticsRequest struct {
	StartDate string `json:"start_date" binding:"required"`
	EndDate   string `json:"end_date" binding:"required"`
	GroupBy   string `json:"group_by"` // day, week, month
}

// GetAuditStatisticsResponse 获取审核统计响应
type GetAuditStatisticsResponse struct {
	StatusCounts  []StatusCount `json:"status_counts"`
	LevelCounts   []LevelCount  `json:"level_counts"`
	TypeCounts    []TypeCount   `json:"type_counts"`
	TotalAudited  int64         `json:"total_audited"`
	AutoPassed    int64         `json:"auto_passed"`
	AutoBlocked   int64         `json:"auto_blocked"`
	ManualPassed  int64         `json:"manual_passed"`
	ManualBlocked int64         `json:"manual_blocked"`
}

// GetViolationTrendsRequest 获取违规趋势请求
type GetViolationTrendsRequest struct {
	StartDate string `json:"start_date" binding:"required"`
	EndDate   string `json:"end_date" binding:"required"`
	GroupBy   string `json:"group_by"` // day, week, month
}

// GetViolationTrendsResponse 获取违规趋势响应
type GetViolationTrendsResponse struct {
	Trends []ViolationTrend `json:"trends"`
}

// ListAuditRecordsRequest 获取审核记录列表请求
type ListAuditRecordsRequest struct {
	ContentID   string `json:"content_id"`
	ContentType string `json:"content_type"`
	Status      string `json:"status"`
	Level       string `json:"level"`
	UploaderID  string `json:"uploader_id"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Page        int    `json:"page" binding:"min=1"`
	PageSize    int    `json:"page_size" binding:"min=1,max=100"`
}

// ListAuditRecordsResponse 获取审核记录列表响应
type ListAuditRecordsResponse struct {
	Records  []*AuditRecord `json:"records"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

// GetManualReviewQueueRequest 获取人工审核队列请求
type GetManualReviewQueueRequest struct {
	ContentType string `json:"content_type"`
	Level       string `json:"level"`
	ReviewerID  uint64 `json:"reviewer_id"`
	Page        int    `json:"page" binding:"min=1"`
	PageSize    int    `json:"page_size" binding:"min=1,max=100"`
}

// GetManualReviewQueueResponse 获取人工审核队列响应
type GetManualReviewQueueResponse struct {
	Queue    []*AuditRecord `json:"queue"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

// AuditRecord 审核记录
type AuditRecord struct {
	ID              uint64     `json:"id"`
	ContentID       string     `json:"content_id"`
	ContentType     string     `json:"content_type"`
	ContentTitle    string     `json:"content_title"`
	ContentURL      string     `json:"content_url"`
	ContentMetadata string     `json:"content_metadata"`
	UploaderID      string     `json:"uploader_id"`
	UploaderName    string     `json:"uploader_name"`
	Status          string     `json:"status"`
	Level           string     `json:"level"`
	Score           float64    `json:"score"`
	Reason          string     `json:"reason"`
	Details         string     `json:"details"`
	Violations      string     `json:"violations"`
	AIResult        string     `json:"ai_result"`
	AIConfidence    float64    `json:"ai_confidence"`
	ReviewerID      *uint64    `json:"reviewer_id"`
	ReviewerName    string     `json:"reviewer_name"`
	ReviewTime      *time.Time `json:"review_time"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// StatusCount 状态统计
type StatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// LevelCount 级别统计
type LevelCount struct {
	Level string `json:"level"`
	Count int64  `json:"count"`
}

// TypeCount 类型统计
type TypeCount struct {
	Type  string `json:"type"`
	Count int64  `json:"count"`
}

// ViolationTrend 违规趋势
type ViolationTrend struct {
	Date      string `json:"date"`
	Violation int64  `json:"violation"`
}

// AIReviewResult AI审核结果
type AIReviewResult struct {
	Result     string  `json:"result"`
	Confidence float64 `json:"confidence"`
	Score      float64 `json:"score"`
}
