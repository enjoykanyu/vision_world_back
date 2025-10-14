package handler

import (
	"errors"
	"strings"
)

// validateSubmitContentRequest 验证提交内容审核请求
func (h *AuditServiceHandler) validateSubmitContentRequest(req *SubmitContentRequest) error {
	if req.ContentID == "" {
		return errors.New("content_id is required")
	}
	if req.ContentType == "" {
		return errors.New("content_type is required")
	}
	if !isValidContentType(req.ContentType) {
		return errors.New("invalid content_type")
	}
	if req.Content == "" {
		return errors.New("content is required")
	}
	return nil
}

// validateGetAuditResultRequest 验证获取审核结果请求
func (h *AuditServiceHandler) validateGetAuditResultRequest(req *GetAuditResultRequest) error {
	if req.AuditID == 0 {
		return errors.New("audit_id is required and must be greater than 0")
	}
	return nil
}

// validateUpdateAuditStatusRequest 验证更新审核状态请求
func (h *AuditServiceHandler) validateUpdateAuditStatusRequest(req *UpdateAuditStatusRequest) error {
	if req.AuditID == 0 {
		return errors.New("audit_id is required and must be greater than 0")
	}
	if req.Status == "" {
		return errors.New("status is required")
	}
	if !isValidAuditStatus(req.Status) {
		return errors.New("invalid status")
	}
	if req.ReviewerID == 0 {
		return errors.New("reviewer_id is required and must be greater than 0")
	}
	return nil
}

// validateListAuditRecordsRequest 验证获取审核记录列表请求
func (h *AuditServiceHandler) validateListAuditRecordsRequest(req *ListAuditRecordsRequest) error {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 10
	}
	if req.ContentType != "" && !isValidContentType(req.ContentType) {
		return errors.New("invalid content_type")
	}
	if req.Status != "" && !isValidAuditStatus(req.Status) {
		return errors.New("invalid status")
	}
	if req.Level != "" && !isValidAuditLevel(req.Level) {
		return errors.New("invalid level")
	}
	return nil
}

// validateAddToWhitelistRequest 验证添加到白名单请求
func (h *AuditServiceHandler) validateAddToWhitelistRequest(req *AddToWhitelistRequest) error {
	if req.ContentID == "" {
		return errors.New("content_id is required")
	}
	if req.ContentType == "" {
		return errors.New("content_type is required")
	}
	if !isValidContentType(req.ContentType) {
		return errors.New("invalid content_type")
	}
	if req.CreatedBy == 0 {
		return errors.New("created_by is required and must be greater than 0")
	}
	return nil
}

// validateRemoveFromWhitelistRequest 验证从白名单移除请求
func (h *AuditServiceHandler) validateRemoveFromWhitelistRequest(req *RemoveFromWhitelistRequest) error {
	if req.ContentID == "" {
		return errors.New("content_id is required")
	}
	return nil
}

// validateAddToBlacklistRequest 验证添加到黑名单请求
func (h *AuditServiceHandler) validateAddToBlacklistRequest(req *AddToBlacklistRequest) error {
	if req.ContentID == "" {
		return errors.New("content_id is required")
	}
	if req.ContentType == "" {
		return errors.New("content_type is required")
	}
	if !isValidContentType(req.ContentType) {
		return errors.New("invalid content_type")
	}
	if req.CreatedBy == 0 {
		return errors.New("created_by is required and must be greater than 0")
	}
	return nil
}

// validateRemoveFromBlacklistRequest 验证从黑名单移除请求
func (h *AuditServiceHandler) validateRemoveFromBlacklistRequest(req *RemoveFromBlacklistRequest) error {
	if req.ContentID == "" {
		return errors.New("content_id is required")
	}
	return nil
}

// validateGetManualReviewQueueRequest 验证获取人工审核队列请求
func (h *AuditServiceHandler) validateGetManualReviewQueueRequest(req *GetManualReviewQueueRequest) error {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 10
	}
	if req.ContentType != "" && !isValidContentType(req.ContentType) {
		return errors.New("invalid content_type")
	}
	if req.Level != "" && !isValidAuditLevel(req.Level) {
		return errors.New("invalid level")
	}
	return nil
}

// validateAssignManualReviewRequest 验证分配人工审核请求
func (h *AuditServiceHandler) validateAssignManualReviewRequest(req *AssignManualReviewRequest) error {
	if req.AuditID == 0 {
		return errors.New("audit_id is required and must be greater than 0")
	}
	if req.ReviewerID == 0 {
		return errors.New("reviewer_id is required and must be greater than 0")
	}
	return nil
}

// isValidContentType 验证内容类型是否有效
func isValidContentType(contentType string) bool {
	validTypes := []string{
		"text",
		"image",
		"video",
		"audio",
	}

	for _, validType := range validTypes {
		if strings.EqualFold(contentType, validType) {
			return true
		}
	}
	return false
}

// isValidAuditStatus 验证审核状态是否有效
func isValidAuditStatus(status string) bool {
	validStatuses := []string{
		"pending",
		"approved",
		"rejected",
		"auto_passed",
		"auto_blocked",
	}

	for _, validStatus := range validStatuses {
		if strings.EqualFold(status, validStatus) {
			return true
		}
	}
	return false
}

// isValidAuditLevel 验证审核等级是否有效
func isValidAuditLevel(level string) bool {
	validLevels := []string{
		"low",
		"medium",
		"high",
	}

	for _, validLevel := range validLevels {
		if strings.EqualFold(level, validLevel) {
			return true
		}
	}
	return false
}
