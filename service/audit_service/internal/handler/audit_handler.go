package handler

import (
	"audit_service/internal/config"
	"audit_service/internal/service"
	"audit_service/pkg/logger"
	"context"
	"fmt"
	"strconv"
	"time"

	auditv1 "audit_service/proto_gen/audit/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AuditServiceHandler implements the auditv1.AuditServiceServer interface
type AuditServiceHandler struct {
	auditv1.UnimplementedAuditServiceServer
	config  *config.Config
	logger  logger.Logger
	service service.AuditService
}

// NewAuditServiceHandler creates a new audit service handler
func NewAuditServiceHandler(service service.AuditService, logger logger.Logger) *AuditServiceHandler {
	return &AuditServiceHandler{
		service: service,
		logger:  logger,
	}
}

// SubmitContent submits content for audit
func (h *AuditServiceHandler) SubmitContent(ctx context.Context, req *auditv1.SubmitContentRequest) (*auditv1.SubmitContentResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Convert proto request to service request
	serviceReq := service.SubmitContentRequest{
		ContentID:       req.ContentId,
		ContentType:     string(req.ContentType),
		ContentTitle:    "",                                // 这个字段在proto中不存在
		ContentURL:      "",                                // 这个字段在proto中不存在
		ContentMetadata: "",                                // 这个字段在proto中不存在
		UploaderID:      fmt.Sprintf("%d", req.UploaderId), // uint64转string
		UploaderName:    "",                                // 这个字段在proto中不存在
	}

	// Call service layer
	result, err := h.service.SubmitContent(ctx, &serviceReq)
	if err != nil {
		h.logger.Error("Failed to submit content for audit", "error", err)
		return nil, status.Error(codes.Internal, "failed to submit content for audit")
	}

	// Convert service response to proto response
	// 将字符串状态转换为枚举类型
	var status auditv1.AuditStatus
	switch result.Status {
	case "pending":
		status = auditv1.AuditStatus_AUDIT_STATUS_PENDING
	case "under_review":
		status = auditv1.AuditStatus_AUDIT_STATUS_UNDER_REVIEW
	case "pending_manual":
		status = auditv1.AuditStatus_AUDIT_STATUS_PENDING_MANUAL
	case "passed":
		status = auditv1.AuditStatus_AUDIT_STATUS_PASSED
	case "rejected":
		status = auditv1.AuditStatus_AUDIT_STATUS_REJECTED
	case "expired":
		status = auditv1.AuditStatus_AUDIT_STATUS_EXPIRED
	default:
		status = auditv1.AuditStatus_AUDIT_STATUS_UNSPECIFIED
	}

	resp := &auditv1.SubmitContentResponse{
		AuditId: result.AuditID,
		Status:  status,
		Reason:  result.Message, // 使用Message字段作为Reason
		// Level和CreatedAt在service层没有对应字段，暂时留空
	}

	return resp, nil
}

// GetAuditResult retrieves audit result
func (h *AuditServiceHandler) GetAuditResult(ctx context.Context, req *auditv1.GetAuditResultRequest) (*auditv1.GetAuditResultResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Call service layer
	result, err := h.service.GetAuditResult(ctx, fmt.Sprintf("%d", req.AuditId))
	if err != nil {
		h.logger.Error("Failed to get audit result", "error", err, "audit_id", req.AuditId)
		return nil, status.Error(codes.Internal, "failed to get audit result")
	}

	// Convert service response to proto response
	// 将字符串内容类型转换为枚举类型
	var contentType auditv1.ContentType
	switch result.ContentType {
	case "text":
		contentType = auditv1.ContentType_CONTENT_TYPE_TEXT
	case "image":
		contentType = auditv1.ContentType_CONTENT_TYPE_IMAGE
	case "video":
		contentType = auditv1.ContentType_CONTENT_TYPE_VIDEO
	case "audio":
		contentType = auditv1.ContentType_CONTENT_TYPE_AUDIO
	case "document":
		contentType = auditv1.ContentType_CONTENT_TYPE_DOCUMENT
	case "live":
		contentType = auditv1.ContentType_CONTENT_TYPE_LIVE
	case "comment":
		contentType = auditv1.ContentType_CONTENT_TYPE_COMMENT
	case "profile":
		contentType = auditv1.ContentType_CONTENT_TYPE_PROFILE
	default:
		contentType = auditv1.ContentType_CONTENT_TYPE_UNSPECIFIED
	}

	// 将字符串状态转换为枚举类型
	var status auditv1.AuditStatus
	switch result.Status {
	case "pending":
		status = auditv1.AuditStatus_AUDIT_STATUS_PENDING
	case "under_review":
		status = auditv1.AuditStatus_AUDIT_STATUS_UNDER_REVIEW
	case "pending_manual":
		status = auditv1.AuditStatus_AUDIT_STATUS_PENDING_MANUAL
	case "passed":
		status = auditv1.AuditStatus_AUDIT_STATUS_PASSED
	case "rejected":
		status = auditv1.AuditStatus_AUDIT_STATUS_REJECTED
	case "expired":
		status = auditv1.AuditStatus_AUDIT_STATUS_EXPIRED
	default:
		status = auditv1.AuditStatus_AUDIT_STATUS_UNSPECIFIED
	}

	resp := &auditv1.GetAuditResultResponse{
		AuditId:     result.AuditID,
		ContentId:   result.ContentID,
		ContentType: contentType,
		Status:      status,
		Reason:      result.Reason,
		// Level, ReviewerId, ReviewedAt在service层没有对应字段，暂时留空
		CreatedAt: timestamppb.New(time.Now()), // 使用当前时间，因为service层没有提供
	}

	return resp, nil
}

// UpdateAuditStatus updates audit status
func (h *AuditServiceHandler) UpdateAuditStatus(ctx context.Context, req *auditv1.UpdateAuditStatusRequest) (*auditv1.UpdateAuditStatusResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Convert proto request to service request
	// 将枚举状态转换为字符串
	var statusStr string
	switch req.Status {
	case auditv1.AuditStatus_AUDIT_STATUS_PENDING:
		statusStr = "pending"
	case auditv1.AuditStatus_AUDIT_STATUS_UNDER_REVIEW:
		statusStr = "under_review"
	case auditv1.AuditStatus_AUDIT_STATUS_PENDING_MANUAL:
		statusStr = "pending_manual"
	case auditv1.AuditStatus_AUDIT_STATUS_PASSED:
		statusStr = "passed"
	case auditv1.AuditStatus_AUDIT_STATUS_REJECTED:
		statusStr = "rejected"
	case auditv1.AuditStatus_AUDIT_STATUS_EXPIRED:
		statusStr = "expired"
	default:
		statusStr = "unspecified"
	}

	serviceReq := service.UpdateAuditStatusRequest{
		AuditID:    req.AuditId,
		Status:     statusStr,
		ReviewerID: req.ReviewerId,
		Reason:     req.Reason,
		// Details和Violations在proto中不存在
	}

	// Call service layer
	_, err := h.service.UpdateAuditStatus(ctx, &serviceReq)
	if err != nil {
		h.logger.Error("Failed to update audit status", "error", err, "audit_id", req.AuditId)
		return nil, status.Error(codes.Internal, "failed to update audit status")
	}

	// Return success response
	return &auditv1.UpdateAuditStatusResponse{
		Success: true,
		Message: "Audit status updated successfully",
	}, nil
}

// ListAuditRecords retrieves audit records list
func (h *AuditServiceHandler) ListAuditRecords(ctx context.Context, req *auditv1.ListAuditRecordsRequest) (*auditv1.ListAuditRecordsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Convert proto request to service request
	// 将枚举类型转换为字符串
	var contentTypeStr string
	switch req.ContentType {
	case auditv1.ContentType_CONTENT_TYPE_TEXT:
		contentTypeStr = "text"
	case auditv1.ContentType_CONTENT_TYPE_IMAGE:
		contentTypeStr = "image"
	case auditv1.ContentType_CONTENT_TYPE_VIDEO:
		contentTypeStr = "video"
	case auditv1.ContentType_CONTENT_TYPE_AUDIO:
		contentTypeStr = "audio"
	case auditv1.ContentType_CONTENT_TYPE_DOCUMENT:
		contentTypeStr = "document"
	case auditv1.ContentType_CONTENT_TYPE_LIVE:
		contentTypeStr = "live"
	case auditv1.ContentType_CONTENT_TYPE_COMMENT:
		contentTypeStr = "comment"
	case auditv1.ContentType_CONTENT_TYPE_PROFILE:
		contentTypeStr = "profile"
	default:
		contentTypeStr = "unspecified"
	}

	var statusStr string
	switch req.Status {
	case auditv1.AuditStatus_AUDIT_STATUS_PENDING:
		statusStr = "pending"
	case auditv1.AuditStatus_AUDIT_STATUS_UNDER_REVIEW:
		statusStr = "under_review"
	case auditv1.AuditStatus_AUDIT_STATUS_PENDING_MANUAL:
		statusStr = "pending_manual"
	case auditv1.AuditStatus_AUDIT_STATUS_PASSED:
		statusStr = "passed"
	case auditv1.AuditStatus_AUDIT_STATUS_REJECTED:
		statusStr = "rejected"
	case auditv1.AuditStatus_AUDIT_STATUS_EXPIRED:
		statusStr = "expired"
	default:
		statusStr = "unspecified"
	}

	var levelStr string
	switch req.Level {
	case auditv1.AuditLevel_AUDIT_LEVEL_LOW:
		levelStr = "low"
	case auditv1.AuditLevel_AUDIT_LEVEL_MEDIUM:
		levelStr = "medium"
	case auditv1.AuditLevel_AUDIT_LEVEL_HIGH:
		levelStr = "high"
	case auditv1.AuditLevel_AUDIT_LEVEL_CRITICAL:
		levelStr = "critical"
	default:
		levelStr = "unspecified"
	}

	serviceReq := service.ListAuditRecordsRequest{
		ContentType: contentTypeStr,
		Status:      statusStr,
		Level:       levelStr,
		UploaderID:  fmt.Sprintf("%d", req.UploaderId),
		// ReviewerID在service层不存在
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		Page:      int(req.Page),
		PageSize:  int(req.PageSize),
	}

	// Call service layer
	result, err := h.service.ListAuditRecords(ctx, &serviceReq)
	if err != nil {
		h.logger.Error("Failed to list audit records", "error", err)
		return nil, status.Error(codes.Internal, "failed to list audit records")
	}

	// Convert service response to proto response
	records := make([]*auditv1.AuditRecord, len(result.Records))
	for i, record := range result.Records {
		// 将字符串转换为枚举类型
		var contentType auditv1.ContentType
		switch record.ContentType {
		case "text":
			contentType = auditv1.ContentType_CONTENT_TYPE_TEXT
		case "image":
			contentType = auditv1.ContentType_CONTENT_TYPE_IMAGE
		case "video":
			contentType = auditv1.ContentType_CONTENT_TYPE_VIDEO
		case "audio":
			contentType = auditv1.ContentType_CONTENT_TYPE_AUDIO
		case "document":
			contentType = auditv1.ContentType_CONTENT_TYPE_DOCUMENT
		case "live":
			contentType = auditv1.ContentType_CONTENT_TYPE_LIVE
		case "comment":
			contentType = auditv1.ContentType_CONTENT_TYPE_COMMENT
		case "profile":
			contentType = auditv1.ContentType_CONTENT_TYPE_PROFILE
		default:
			contentType = auditv1.ContentType_CONTENT_TYPE_UNSPECIFIED
		}

		var status auditv1.AuditStatus
		switch record.Status {
		case "pending":
			status = auditv1.AuditStatus_AUDIT_STATUS_PENDING
		case "under_review":
			status = auditv1.AuditStatus_AUDIT_STATUS_UNDER_REVIEW
		case "pending_manual":
			status = auditv1.AuditStatus_AUDIT_STATUS_PENDING_MANUAL
		case "passed":
			status = auditv1.AuditStatus_AUDIT_STATUS_PASSED
		case "rejected":
			status = auditv1.AuditStatus_AUDIT_STATUS_REJECTED
		case "expired":
			status = auditv1.AuditStatus_AUDIT_STATUS_EXPIRED
		default:
			status = auditv1.AuditStatus_AUDIT_STATUS_UNSPECIFIED
		}

		var level auditv1.AuditLevel
		switch record.Level {
		case "low":
			level = auditv1.AuditLevel_AUDIT_LEVEL_LOW
		case "medium":
			level = auditv1.AuditLevel_AUDIT_LEVEL_MEDIUM
		case "high":
			level = auditv1.AuditLevel_AUDIT_LEVEL_HIGH
		case "critical":
			level = auditv1.AuditLevel_AUDIT_LEVEL_CRITICAL
		default:
			level = auditv1.AuditLevel_AUDIT_LEVEL_UNSPECIFIED
		}

		// 转换UploaderID为uint64
		var uploaderID uint64
		if id, err := strconv.ParseUint(record.UploaderID, 10, 64); err == nil {
			uploaderID = id
		}

		// 处理ReviewTime可能为nil的情况
		var reviewedAt *timestamppb.Timestamp
		if record.ReviewTime != nil {
			reviewedAt = timestamppb.New(*record.ReviewTime)
		}

		records[i] = &auditv1.AuditRecord{
			AuditId:     record.ID,
			ContentId:   record.ContentID,
			ContentType: contentType,
			Status:      status,
			Reason:      record.Reason,
			Level:       level,
			UploaderId:  uploaderID,
			CreatedAt:   timestamppb.New(record.CreatedAt),
			ReviewedAt:  reviewedAt,
		}
	}

	return &auditv1.ListAuditRecordsResponse{
		Total:    result.Total,
		Page:     int32(result.Page),
		PageSize: int32(result.PageSize),
		Records:  records,
	}, nil
}

// AddToWhitelist adds content to whitelist
func (h *AuditServiceHandler) AddToWhitelist(ctx context.Context, req *auditv1.AddToWhitelistRequest) (*auditv1.AddToWhitelistResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Convert proto request to service request
	serviceReq := service.AddToWhitelistRequest{
		ContentID:   req.ContentId,
		ContentType: fmt.Sprintf("%d", req.ContentType),
		Reason:      req.Reason,
		CreatedBy:   req.CreatedBy,
	}

	// Call service layer
	_, err := h.service.AddToWhitelist(ctx, &serviceReq)
	if err != nil {
		h.logger.Error("Failed to add to whitelist", "error", err)
		return nil, status.Error(codes.Internal, "failed to add to whitelist")
	}

	// Return success response
	return &auditv1.AddToWhitelistResponse{
		Success: true,
		Message: "Added to whitelist successfully",
	}, nil
}

// RemoveFromWhitelist removes content from whitelist
func (h *AuditServiceHandler) RemoveFromWhitelist(ctx context.Context, req *auditv1.RemoveFromWhitelistRequest) (*auditv1.RemoveFromWhitelistResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Call service layer
	err := h.service.RemoveFromWhitelist(ctx, req.ContentId)
	if err != nil {
		h.logger.Error("Failed to remove from whitelist", "error", err)
		return nil, status.Error(codes.Internal, "failed to remove from whitelist")
	}

	// Return success response
	return &auditv1.RemoveFromWhitelistResponse{
		Success: true,
		Message: "Removed from whitelist successfully",
	}, nil
}

// AddToBlacklist adds content to blacklist
func (h *AuditServiceHandler) AddToBlacklist(ctx context.Context, req *auditv1.AddToBlacklistRequest) (*auditv1.AddToBlacklistResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Convert proto request to service request
	serviceReq := service.AddToBlacklistRequest{
		ContentID:   req.ContentId,
		ContentType: fmt.Sprintf("%d", req.ContentType),
		Reason:      req.Reason,
		CreatedBy:   req.CreatedBy,
	}

	// Call service layer
	_, err := h.service.AddToBlacklist(ctx, &serviceReq)
	if err != nil {
		h.logger.Error("Failed to add to blacklist", "error", err)
		return nil, status.Error(codes.Internal, "failed to add to blacklist")
	}

	// Return success response
	return &auditv1.AddToBlacklistResponse{
		Success: true,
		Message: "Added to blacklist successfully",
	}, nil
}

// RemoveFromBlacklist removes content from blacklist
func (h *AuditServiceHandler) RemoveFromBlacklist(ctx context.Context, req *auditv1.RemoveFromBlacklistRequest) (*auditv1.RemoveFromBlacklistResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Call service layer
	err := h.service.RemoveFromBlacklist(ctx, req.ContentId)
	if err != nil {
		h.logger.Error("Failed to remove from blacklist", "error", err)
		return nil, status.Error(codes.Internal, "failed to remove from blacklist")
	}

	// Return success response
	return &auditv1.RemoveFromBlacklistResponse{
		Success: true,
		Message: "Removed from blacklist successfully",
	}, nil
}

// GetManualReviewQueue retrieves manual review queue
func (h *AuditServiceHandler) GetManualReviewQueue(ctx context.Context, req *auditv1.GetManualReviewQueueRequest) (*auditv1.GetManualReviewQueueResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Convert proto request to service request
	// 将枚举类型转换为字符串
	var contentTypeStr string
	switch req.ContentType {
	case auditv1.ContentType_CONTENT_TYPE_TEXT:
		contentTypeStr = "text"
	case auditv1.ContentType_CONTENT_TYPE_IMAGE:
		contentTypeStr = "image"
	case auditv1.ContentType_CONTENT_TYPE_VIDEO:
		contentTypeStr = "video"
	case auditv1.ContentType_CONTENT_TYPE_AUDIO:
		contentTypeStr = "audio"
	case auditv1.ContentType_CONTENT_TYPE_DOCUMENT:
		contentTypeStr = "document"
	case auditv1.ContentType_CONTENT_TYPE_LIVE:
		contentTypeStr = "live"
	case auditv1.ContentType_CONTENT_TYPE_COMMENT:
		contentTypeStr = "comment"
	case auditv1.ContentType_CONTENT_TYPE_PROFILE:
		contentTypeStr = "profile"
	default:
		contentTypeStr = "unspecified"
	}

	var levelStr string
	switch req.Level {
	case auditv1.AuditLevel_AUDIT_LEVEL_LOW:
		levelStr = "low"
	case auditv1.AuditLevel_AUDIT_LEVEL_MEDIUM:
		levelStr = "medium"
	case auditv1.AuditLevel_AUDIT_LEVEL_HIGH:
		levelStr = "high"
	case auditv1.AuditLevel_AUDIT_LEVEL_CRITICAL:
		levelStr = "critical"
	default:
		levelStr = "unspecified"
	}

	serviceReq := service.GetManualReviewQueueRequest{
		ContentType: contentTypeStr,
		Level:       levelStr,
		ReviewerID:  0, // proto中没有ReviewerId字段，使用默认值
		Page:        int(req.Page),
		PageSize:    int(req.PageSize),
	}

	// Call service layer
	result, err := h.service.GetManualReviewQueue(ctx, &serviceReq)
	if err != nil {
		h.logger.Error("Failed to get manual review queue", "error", err)
		return nil, status.Error(codes.Internal, "failed to get manual review queue")
	}

	// Convert service response to proto response
	records := make([]*auditv1.AuditRecord, len(result.Queue))
	for i, record := range result.Queue {
		// 将字符串转换为枚举类型
		var contentType auditv1.ContentType
		switch record.ContentType {
		case "text":
			contentType = auditv1.ContentType_CONTENT_TYPE_TEXT
		case "image":
			contentType = auditv1.ContentType_CONTENT_TYPE_IMAGE
		case "video":
			contentType = auditv1.ContentType_CONTENT_TYPE_VIDEO
		case "audio":
			contentType = auditv1.ContentType_CONTENT_TYPE_AUDIO
		case "document":
			contentType = auditv1.ContentType_CONTENT_TYPE_DOCUMENT
		case "live":
			contentType = auditv1.ContentType_CONTENT_TYPE_LIVE
		case "comment":
			contentType = auditv1.ContentType_CONTENT_TYPE_COMMENT
		case "profile":
			contentType = auditv1.ContentType_CONTENT_TYPE_PROFILE
		default:
			contentType = auditv1.ContentType_CONTENT_TYPE_UNSPECIFIED
		}

		var status auditv1.AuditStatus
		switch record.Status {
		case "pending":
			status = auditv1.AuditStatus_AUDIT_STATUS_PENDING
		case "under_review":
			status = auditv1.AuditStatus_AUDIT_STATUS_UNDER_REVIEW
		case "pending_manual":
			status = auditv1.AuditStatus_AUDIT_STATUS_PENDING_MANUAL
		case "passed":
			status = auditv1.AuditStatus_AUDIT_STATUS_PASSED
		case "rejected":
			status = auditv1.AuditStatus_AUDIT_STATUS_REJECTED
		case "expired":
			status = auditv1.AuditStatus_AUDIT_STATUS_EXPIRED
		default:
			status = auditv1.AuditStatus_AUDIT_STATUS_UNSPECIFIED
		}

		var level auditv1.AuditLevel
		switch record.Level {
		case "low":
			level = auditv1.AuditLevel_AUDIT_LEVEL_LOW
		case "medium":
			level = auditv1.AuditLevel_AUDIT_LEVEL_MEDIUM
		case "high":
			level = auditv1.AuditLevel_AUDIT_LEVEL_HIGH
		case "critical":
			level = auditv1.AuditLevel_AUDIT_LEVEL_CRITICAL
		default:
			level = auditv1.AuditLevel_AUDIT_LEVEL_UNSPECIFIED
		}

		// 转换UploaderID为uint64
		var uploaderID uint64
		if id, err := strconv.ParseUint(record.UploaderID, 10, 64); err == nil {
			uploaderID = id
		}

		// 转换ReviewerID为uint64
		var reviewerID uint64
		if record.ReviewerID != nil {
			reviewerID = *record.ReviewerID
		}

		// 处理ReviewTime可能为nil的情况
		var reviewedAt *timestamppb.Timestamp
		if record.ReviewTime != nil {
			reviewedAt = timestamppb.New(*record.ReviewTime)
		}

		records[i] = &auditv1.AuditRecord{
			AuditId:     record.ID,
			ContentId:   record.ContentID,
			ContentType: contentType,
			Status:      status,
			Reason:      record.Reason,
			Level:       level,
			UploaderId:  uploaderID,
			ReviewerId:  reviewerID,
			CreatedAt:   timestamppb.New(record.CreatedAt),
			ReviewedAt:  reviewedAt,
		}
	}

	return &auditv1.GetManualReviewQueueResponse{
		Total:    result.Total,
		Page:     int32(result.Page),
		PageSize: int32(result.PageSize),
		Records:  records,
	}, nil
}

// AssignManualReview assigns manual review
func (h *AuditServiceHandler) AssignManualReview(ctx context.Context, req *auditv1.AssignManualReviewRequest) (*auditv1.AssignManualReviewResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Convert proto request to service request
	serviceReq := service.AssignManualReviewRequest{
		AuditID:    req.AuditId,
		ReviewerID: req.ReviewerId,
	}

	// Call service layer
	_, err := h.service.AssignManualReview(ctx, &serviceReq)
	if err != nil {
		h.logger.Error("Failed to assign manual review", "error", err)
		return nil, status.Error(codes.Internal, "failed to assign manual review")
	}

	// Return success response
	return &auditv1.AssignManualReviewResponse{
		Success: true,
		Message: "Manual review assigned successfully",
	}, nil
}

// GetAuditStatistics retrieves audit statistics
func (h *AuditServiceHandler) GetAuditStatistics(ctx context.Context, req *auditv1.GetAuditStatisticsRequest) (*auditv1.GetAuditStatisticsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Convert proto request to service request
	serviceReq := service.GetAuditStatisticsRequest{
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
	}

	// Call service layer
	result, err := h.service.GetAuditStatistics(ctx, &serviceReq)
	if err != nil {
		h.logger.Error("Failed to get audit statistics", "error", err)
		return nil, status.Error(codes.Internal, "failed to get audit statistics")
	}

	// Convert service response to proto response
	resp := &auditv1.GetAuditStatisticsResponse{
		TotalCount: result.TotalAudited,
		PassRate:   float64(result.AutoPassed+result.ManualPassed) / float64(result.TotalAudited),
	}

	// 转换状态统计
	for _, stat := range result.StatusCounts {
		status := auditv1.AuditStatus_AUDIT_STATUS_UNSPECIFIED
		switch stat.Status {
		case "pending":
			status = auditv1.AuditStatus_AUDIT_STATUS_PENDING
		case "passed":
			status = auditv1.AuditStatus_AUDIT_STATUS_PASSED
		case "rejected":
			status = auditv1.AuditStatus_AUDIT_STATUS_REJECTED
		case "under_review":
			status = auditv1.AuditStatus_AUDIT_STATUS_UNDER_REVIEW
		case "pending_manual":
			status = auditv1.AuditStatus_AUDIT_STATUS_PENDING_MANUAL
		}
		resp.StatusStats = append(resp.StatusStats, &auditv1.StatusCount{
			Status: status,
			Count:  stat.Count,
		})
	}

	// 转换级别统计
	for _, stat := range result.LevelCounts {
		level := auditv1.AuditLevel_AUDIT_LEVEL_UNSPECIFIED
		switch stat.Level {
		case "low":
			level = auditv1.AuditLevel_AUDIT_LEVEL_LOW
		case "medium":
			level = auditv1.AuditLevel_AUDIT_LEVEL_MEDIUM
		case "high":
			level = auditv1.AuditLevel_AUDIT_LEVEL_HIGH
		case "critical":
			level = auditv1.AuditLevel_AUDIT_LEVEL_CRITICAL
		}
		resp.LevelStats = append(resp.LevelStats, &auditv1.LevelCount{
			Level: level,
			Count: stat.Count,
		})
	}

	// 转换类型统计
	for _, stat := range result.TypeCounts {
		contentType := auditv1.ContentType_CONTENT_TYPE_UNSPECIFIED
		switch stat.Type {
		case "text":
			contentType = auditv1.ContentType_CONTENT_TYPE_TEXT
		case "image":
			contentType = auditv1.ContentType_CONTENT_TYPE_IMAGE
		case "video":
			contentType = auditv1.ContentType_CONTENT_TYPE_VIDEO
		case "audio":
			contentType = auditv1.ContentType_CONTENT_TYPE_AUDIO
		}
		resp.TypeStats = append(resp.TypeStats, &auditv1.TypeCount{
			ContentType: contentType,
			Count:       stat.Count,
		})
	}

	return resp, nil
}

// GetViolationTrends retrieves violation trends
func (h *AuditServiceHandler) GetViolationTrends(ctx context.Context, req *auditv1.GetViolationTrendsRequest) (*auditv1.GetViolationTrendsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Convert proto request to service request
	serviceReq := service.GetViolationTrendsRequest{
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
	}

	// Call service layer
	result, err := h.service.GetViolationTrends(ctx, &serviceReq)
	if err != nil {
		h.logger.Error("Failed to get violation trends", "error", err)
		return nil, status.Error(codes.Internal, "failed to get violation trends")
	}

	// Convert service response to proto response
	trends := make([]*auditv1.ViolationTrend, len(result.Trends))
	for i, trend := range result.Trends {
		trends[i] = &auditv1.ViolationTrend{
			Date:  trend.Date,
			Count: trend.Violation,
		}
	}

	return &auditv1.GetViolationTrendsResponse{
		Trends: trends,
	}, nil
}
