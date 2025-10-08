package repository

import "audit_service/internal/model"

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
	Total    int64                `json:"total"`     // 总数
	Page     int                  `json:"page"`      // 当前页
	PageSize int                  `json:"page_size"` // 每页数量
	Records  []*model.AuditRecord `json:"records"`   // 审核记录列表
}

// ListTemplatesRequest 获取审核模板列表请求
type ListTemplatesRequest struct {
	ContentType string `json:"content_type"` // 内容类型
	Level       string `json:"level"`        // 违规等级
	IsActive    bool   `json:"is_active"`    // 是否激活
	Page        int    `json:"page"`         // 页码
	PageSize    int    `json:"page_size"`    // 每页数量
}

// ListTemplatesResponse 获取审核模板列表响应
type ListTemplatesResponse struct {
	Total     int64                  `json:"total"`     // 总数
	Page      int                    `json:"page"`      // 当前页
	PageSize  int                    `json:"page_size"` // 每页数量
	Templates []*model.AuditTemplate `json:"templates"` // 审核模板列表
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
	Total    int64                `json:"total"`     // 总数
	Page     int                  `json:"page"`      // 当前页
	PageSize int                  `json:"page_size"` // 每页数量
	Records  []*model.AuditRecord `json:"records"`   // 审核记录列表
}

// GetAuditStatisticsRequest 获取审核统计请求
type GetAuditStatisticsRequest struct {
	StartDate string `json:"start_date"` // 开始日期
	EndDate   string `json:"end_date"`   // 结束日期
}

// GetAuditStatisticsResponse 获取审核统计响应
type GetAuditStatisticsResponse struct {
	TotalCount  int64         `json:"total_count"`  // 总审核数
	PassRate    float64       `json:"pass_rate"`    // 通过率
	StatusStats []StatusCount `json:"status_stats"` // 按状态统计
	LevelStats  []LevelCount  `json:"level_stats"`  // 按违规等级统计
	TypeStats   []TypeCount   `json:"type_stats"`   // 按内容类型统计
}

// StatusCount 按状态统计
type StatusCount struct {
	Status string `json:"status"` // 审核状态
	Count  int64  `json:"count"`  // 数量
}

// LevelCount 按违规等级统计
type LevelCount struct {
	Level string `json:"level"` // 违规等级
	Count int64  `json:"count"` // 数量
}

// TypeCount 按内容类型统计
type TypeCount struct {
	ContentType string `json:"content_type"` // 内容类型
	Count       int64  `json:"count"`        // 数量
}

// GetViolationTrendsRequest 获取违规趋势请求
type GetViolationTrendsRequest struct {
	StartDate string `json:"start_date"` // 开始日期
	EndDate   string `json:"end_date"`   // 结束日期
}

// GetViolationTrendsResponse 获取违规趋势响应
type GetViolationTrendsResponse struct {
	Trends []ViolationTrend `json:"trends"` // 违规趋势
}

// ViolationTrend 违规趋势
type ViolationTrend struct {
	Date  string `json:"date"`  // 日期
	Count int64  `json:"count"` // 数量
}
