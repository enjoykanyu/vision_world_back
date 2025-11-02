package client

import (
	"time"
)

// SubmitContentRequest 提交内容审核请求（适配器）
type SubmitContentRequest struct {
	ContentId   string
	ContentType int32
	UploaderId  uint64
	Title       string
	Content     string
	CreateTime  string
}

// SubmitContentResponse 提交内容审核响应（适配器）
type SubmitContentResponse struct {
	AuditId   uint64
	Status    int32
	Reason    string
	Level     int32
	CreatedAt string
}

// GetAuditResultRequest 获取审核结果请求（适配器）
type GetAuditResultRequest struct {
	AuditId uint64
}

// GetAuditResultResponse 获取审核结果响应（适配器）
type GetAuditResultResponse struct {
	AuditId     uint64
	ContentId   string
	ContentType int32
	Status      int32
	Reason      string
	Level       int32
	ReviewerId  uint64
	ReviewedAt  string
	CreatedAt   string
}

// UpdateAuditStatusRequest 更新审核状态请求（适配器）
type UpdateAuditStatusRequest struct {
	AuditId    uint64
	Status     int32
	ReviewerId uint64
	Reason     string
}

// UpdateAuditStatusResponse 更新审核状态响应（适配器）
type UpdateAuditStatusResponse struct {
	Success bool
	Message string
}

// ListAuditRecordsRequest 获取审核记录列表请求（适配器）
type ListAuditRecordsRequest struct {
	ContentType int32
	Status      int32
	Level       int32
	UploaderId  uint64
	ReviewerId  uint64
	StartDate   string
	EndDate     string
	Page        int32
	PageSize    int32
}

// ListAuditRecordsResponse 获取审核记录列表响应（适配器）
type ListAuditRecordsResponse struct {
	Total    int64
	Page     int32
	PageSize int32
	Records  []AuditRecord
}

// AuditRecord 审核记录（适配器）
type AuditRecord struct {
	AuditId     uint64
	ContentId   string
	ContentType int32
	Status      int32
	Reason      string
	Level       int32
	UploaderId  uint64
	ReviewerId  uint64
	CreatedAt   string
	ReviewedAt  string
}

// AddToWhitelistRequest 添加到白名单请求（适配器）
type AddToWhitelistRequest struct {
	ContentId   string
	ContentType int32
	Reason      string
	CreatedBy   uint64
}

// AddToWhitelistResponse 添加到白名单响应（适配器）
type AddToWhitelistResponse struct {
	Success bool
	Message string
}

// RemoveFromWhitelistRequest 从白名单移除请求（适配器）
type RemoveFromWhitelistRequest struct {
	ContentId string
}

// RemoveFromWhitelistResponse 从白名单移除响应（适配器）
type RemoveFromWhitelistResponse struct {
	Success bool
	Message string
}

// AddToBlacklistRequest 添加到黑名单请求（适配器）
type AddToBlacklistRequest struct {
	ContentId   string
	ContentType int32
	Reason      string
	CreatedBy   uint64
}

// AddToBlacklistResponse 添加到黑名单响应（适配器）
type AddToBlacklistResponse struct {
	Success bool
	Message string
}

// RemoveFromBlacklistRequest 从黑名单移除请求（适配器）
type RemoveFromBlacklistRequest struct {
	ContentId string
}

// RemoveFromBlacklistResponse 从黑名单移除响应（适配器）
type RemoveFromBlacklistResponse struct {
	Success bool
	Message string
}
