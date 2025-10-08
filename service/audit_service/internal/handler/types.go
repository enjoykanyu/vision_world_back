package handler

import (
	"audit_service/internal/repository"
	"time"
)

// SubmitContentRequest 提交内容审核请求
type SubmitContentRequest struct {
	ContentID   string            `json:"content_id" validate:"required"`   // 内容ID
	ContentType string            `json:"content_type" validate:"required"` // 内容类型
	Content     string            `json:"content" validate:"required"`      // 内容
	UploaderID  uint64            `json:"uploader_id"`                      // 上传者ID
	Metadata    map[string]string `json:"metadata"`                         // 元数据
}

// SubmitContentResponse 提交内容审核响应
type SubmitContentResponse struct {
	AuditID   uint64    `json:"audit_id"`   // 审核ID
	Status    string    `json:"status"`     // 审核状态
	Reason    string    `json:"reason"`     // 审核原因
	Level     string    `json:"level"`      // 违规等级
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// GetAuditResultRequest 获取审核结果请求
type GetAuditResultRequest struct {
	AuditID uint64 `json:"audit_id" validate:"required,min=1"` // 审核ID
}

// GetAuditResultResponse 获取审核结果响应
type GetAuditResultResponse struct {
	AuditID     uint64    `json:"audit_id"`     // 审核ID
	ContentID   string    `json:"content_id"`   // 内容ID
	ContentType string    `json:"content_type"` // 内容类型
	Status      string    `json:"status"`       // 审核状态
	Reason      string    `json:"reason"`       // 审核原因
	Level       string    `json:"level"`        // 违规等级
	ReviewerID  uint64    `json:"reviewer_id"`  // 审核员ID
	ReviewedAt  time.Time `json:"reviewed_at"`  // 审核时间
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
}

// UpdateAuditStatusRequest 更新审核状态请求
type UpdateAuditStatusRequest struct {
	AuditID    uint64 `json:"audit_id" validate:"required,min=1"`    // 审核ID
	Status     string `json:"status" validate:"required"`            // 审核状态
	ReviewerID uint64 `json:"reviewer_id" validate:"required,min=1"` // 审核员ID
	Reason     string `json:"reason"`                                // 审核原因
}

// UpdateAuditStatusResponse 更新审核状态响应
type UpdateAuditStatusResponse struct {
	Success bool   `json:"success"` // 是否成功
	Message string `json:"message"` // 消息
}

// ListAuditRecordsRequest 获取审核记录列表请求
type ListAuditRecordsRequest struct {
	ContentType string `json:"content_type"` // 内容类型
	Status      string `json:"status"`       // 审核状态
	Level       string `json:"level"`        // 违规等级
	UploaderID  uint64 `json:"uploader_id"`  // 上传者ID
	ReviewerID  uint64 `json:"reviewer_id"`  // 审核员ID
	StartDate   string `json:"start_date"`   // 开始日期
	EndDate     string `json:"end_date"`     // 结束日期
	Page        int    `json:"page"`         // 页码
	PageSize    int    `json:"page_size"`    // 每页数量
}

// ListAuditRecordsResponse 获取审核记录列表响应
type ListAuditRecordsResponse struct {
	Total    int64                                  `json:"total"`     // 总数
	Page     int                                    `json:"page"`      // 当前页
	PageSize int                                    `json:"page_size"` // 每页数量
	Records  []*repository.ListAuditRecordsResponse `json:"records"`   // 审核记录列表
}

// AddToWhitelistRequest 添加到白名单请求
type AddToWhitelistRequest struct {
	ContentID   string `json:"content_id" validate:"required"`       // 内容ID
	ContentType string `json:"content_type" validate:"required"`     // 内容类型
	Reason      string `json:"reason"`                               // 原因
	CreatedBy   uint64 `json:"created_by" validate:"required,min=1"` // 创建者ID
}

// AddToWhitelistResponse 添加到白名单响应
type AddToWhitelistResponse struct {
	Success bool   `json:"success"` // 是否成功
	Message string `json:"message"` // 消息
}

// RemoveFromWhitelistRequest 从白名单移除请求
type RemoveFromWhitelistRequest struct {
	ContentID string `json:"content_id" validate:"required"` // 内容ID
}

// RemoveFromWhitelistResponse 从白名单移除响应
type RemoveFromWhitelistResponse struct {
	Success bool   `json:"success"` // 是否成功
	Message string `json:"message"` // 消息
}

// AddToBlacklistRequest 添加到黑名单请求
type AddToBlacklistRequest struct {
	ContentID   string `json:"content_id" validate:"required"`       // 内容ID
	ContentType string `json:"content_type" validate:"required"`     // 内容类型
	Reason      string `json:"reason"`                               // 原因
	CreatedBy   uint64 `json:"created_by" validate:"required,min=1"` // 创建者ID
}

// AddToBlacklistResponse 添加到黑名单响应
type AddToBlacklistResponse struct {
	Success bool   `json:"success"` // 是否成功
	Message string `json:"message"` // 消息
}

// RemoveFromBlacklistRequest 从黑名单移除请求
type RemoveFromBlacklistRequest struct {
	ContentID string `json:"content_id" validate:"required"` // 内容ID
}

// RemoveFromBlacklistResponse 从黑名单移除响应
type RemoveFromBlacklistResponse struct {
	Success bool   `json:"success"` // 是否成功
	Message string `json:"message"` // 消息
}

// GetManualReviewQueueRequest 获取人工审核队列请求
type GetManualReviewQueueRequest struct {
	ContentType string `json:"content_type"` // 内容类型
	Level       string `json:"level"`        // 违规等级
	Priority    int    `json:"priority"`     // 优先级
	Page        int    `json:"page"`         // 页码
	PageSize    int    `json:"page_size"`    // 每页数量
}

// GetManualReviewQueueResponse 获取人工审核队列响应
type GetManualReviewQueueResponse struct {
	Total    int64                                      `json:"total"`     // 总数
	Page     int                                        `json:"page"`      // 当前页
	PageSize int                                        `json:"page_size"` // 每页数量
	Records  []*repository.GetManualReviewQueueResponse `json:"records"`   // 审核记录列表
}

// AssignManualReviewRequest 分配人工审核请求
type AssignManualReviewRequest struct {
	AuditID    uint64 `json:"audit_id" validate:"required,min=1"`    // 审核ID
	ReviewerID uint64 `json:"reviewer_id" validate:"required,min=1"` // 审核员ID
}

// AssignManualReviewResponse 分配人工审核响应
type AssignManualReviewResponse struct {
	Success bool   `json:"success"` // 是否成功
	Message string `json:"message"` // 消息
}

// GetAuditStatisticsRequest 获取审核统计请求
type GetAuditStatisticsRequest struct {
	StartDate string `json:"start_date"` // 开始日期
	EndDate   string `json:"end_date"`   // 结束日期
}

// GetAuditStatisticsResponse 获取审核统计响应
type GetAuditStatisticsResponse struct {
	TotalCount  int64                    `json:"total_count"`  // 总审核数
	PassRate    float64                  `json:"pass_rate"`    // 通过率
	StatusStats []repository.StatusCount `json:"status_stats"` // 按状态统计
	LevelStats  []repository.LevelCount  `json:"level_stats"`  // 按违规等级统计
	TypeStats   []repository.TypeCount   `json:"type_stats"`   // 按内容类型统计
}

// GetViolationTrendsRequest 获取违规趋势请求
type GetViolationTrendsRequest struct {
	StartDate string `json:"start_date"` // 开始日期
	EndDate   string `json:"end_date"`   // 结束日期
}

// GetViolationTrendsResponse 获取违规趋势响应
type GetViolationTrendsResponse struct {
	Trends []repository.ViolationTrend `json:"trends"` // 违规趋势
}
