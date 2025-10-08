package handler

import (
	"context"
	"audit_service/internal/config"
	"audit_service/internal/model"
	"audit_service/internal/service"
	"audit_service/pkg/logger"

	"audit_service/proto_gen/audit/v1"
	auditv1 "audit_service/proto_gen/audit/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AuditServiceHandler implements the auditv1.AuditServiceServer interface
type AuditServiceHandler struct {
	auditv1.UnimplementedAuditServiceServer
	config     *config.Config
	logger     logger.Logger
	service    service.AuditService
	repository service.AuditRepository
}

// NewAuditServiceHandler creates a new audit service handler
func NewAuditServiceHandler(service service.AuditService, logger *logger.Logger) *AuditServiceHandler {
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
		ContentID:   req.ContentId,
		ContentType: int(req.ContentType),
		Content:     req.Content,
		UploaderID:  req.UploaderId,
		Metadata:    req.Metadata,
	}

	// Call service layer
	result, err := h.service.SubmitContent(ctx, serviceReq)
	if err != nil {
		h.logger.Error("Failed to submit content for audit", "error", err)
		return nil, status.Error(codes.Internal, "failed to submit content for audit")
	}

	// Convert service response to proto response
	resp := &auditv1.SubmitContentResponse{
		AuditId:   result.AuditID,
		Status:    auditv1.AuditStatus(result.Status),
		Reason:    result.Reason,
		Level:     auditv1.AuditLevel(result.Level),
		CreatedAt: timestamppb.New(result.CreatedAt),
	}

	return resp, nil
}

// GetAuditResult retrieves audit result
func (h *AuditServiceHandler) GetAuditResult(ctx context.Context, req *auditv1.GetAuditResultRequest) (*auditv1.GetAuditResultResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Call service layer
	result, err := h.service.GetAuditResult(ctx, req.AuditId)
	if err != nil {
		h.logger.Error("Failed to get audit result", "error", err, "audit_id", req.AuditId)
		return nil, status.Error(codes.Internal, "failed to get audit result")
	}

	// Convert service response to proto response
	resp := &auditv1.GetAuditResultResponse{
		AuditId:     result.AuditID,
		ContentId:   result.ContentID,
		ContentType: auditv1.ContentType(result.ContentType),
		Content:     result.Content,
		UploaderId:  result.UploaderID,
		Status:      auditv1.AuditStatus(result.Status),
		Reason:      result.Reason,
		Level:       auditv1.AuditLevel(result.Level),
		CreatedAt:   timestamppb.New(result.CreatedAt),
		UpdatedAt:   timestamppb.New(result.UpdatedAt),
		Metadata:    result.Metadata,
	}

	return resp, nil
}

// UpdateAuditStatus updates audit status
func (h *AuditServiceHandler) UpdateAuditStatus(ctx context.Context, req *auditv1.UpdateAuditStatusRequest) (*auditv1.UpdateAuditStatusResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Convert proto request to service request
	serviceReq := service.UpdateAuditStatusRequest{
		AuditID:    req.AuditId,
		Status:     int(req.Status),
		Reason:     req.Reason,
		ReviewerID: req.ReviewerId,
	}

	// Call service layer
	err := h.service.UpdateAuditStatus(ctx, serviceReq)
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
	serviceReq := service.ListAuditRecordsRequest{
		ContentType: int(req.ContentType),
		Status:      int(req.Status),
		Level:       int(req.Level),
		UploaderID:  req.UploaderId,
		ReviewerID:  req.ReviewerId,
		StartDate:   req.StartDate.AsTime(),
		EndDate:     req.EndDate.AsTime(),
		Page:        req.Page,
		PageSize:    req.PageSize,
	}

	// Call service layer
	result, err := h.service.ListAuditRecords(ctx, serviceReq)
	if err != nil {
		h.logger.Error("Failed to list audit records", "error", err)
		return nil, status.Error(codes.Internal, "failed to list audit records")
	}

	// Convert service response to proto response
	records := make([]*auditv1.AuditRecord, len(result.Records))
	for i, record := range result.Records {
		records[i] = &auditv1.AuditRecord{
			AuditId:     record.AuditID,
			ContentId:   record.ContentID,
			ContentType: auditv1.ContentType(record.ContentType),
			Content:     record.Content,
			UploaderId:  record.UploaderID,
			Status:      auditv1.AuditStatus(record.Status),
			Reason:      record.Reason,
			Level:       auditv1.AuditLevel(record.Level),
			CreatedAt:   timestamppb.New(record.CreatedAt),
			UpdatedAt:   timestamppb.New(record.UpdatedAt),
		}
	}

	return &auditv1.ListAuditRecordsResponse{
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
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
		ContentType: int(req.ContentType),
		Reason:      req.Reason,
		CreatedBy:   req.CreatedBy,
	}

	// Call service layer
	err := h.service.AddToWhitelist(ctx, serviceReq)
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
		ContentType: int(req.ContentType),
		Reason:      req.Reason,
		CreatedBy:   req.CreatedBy,
	}

	// Call service layer
	err := h.service.AddToBlacklist(ctx, serviceReq)
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
	serviceReq := service.GetManualReviewQueueRequest{
		ContentType: int(req.ContentType),
		Level:       int(req.Level),
		Priority:    int(req.Priority),
		Page:        req.Page,
		PageSize:    req.PageSize,
	}

	// Call service layer
	result, err := h.service.GetManualReviewQueue(ctx, serviceReq)
	if err != nil {
		h.logger.Error("Failed to get manual review queue", "error", err)
		return nil, status.Error(codes.Internal, "failed to get manual review queue")
	}

	// Convert service response to proto response
	records := make([]*auditv1.AuditRecord, len(result.Records))
	for i, record := range result.Records {
		records[i] = &auditv1.AuditRecord{
			AuditId:     record.AuditID,
			ContentId:   record.ContentID,
			ContentType: auditv1.ContentType(record.ContentType),
			Content:     record.Content,
			UploaderId:  record.UploaderID,
			Status:      auditv1.AuditStatus(record.Status),
			Reason:      record.Reason,
			Level:       auditv1.AuditLevel(record.Level),
			CreatedAt:   timestamppb.New(record.CreatedAt),
			UpdatedAt:   timestamppb.New(record.UpdatedAt),
		}
	}

	return &auditv1.GetManualReviewQueueResponse{
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
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
	err := h.service.AssignManualReview(ctx, serviceReq)
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
		StartDate: req.StartDate.AsTime(),
		EndDate:   req.EndDate.AsTime(),
	}

	// Call service layer
	result, err := h.service.GetAuditStatistics(ctx, serviceReq)
	if err != nil {
		h.logger.Error("Failed to get audit statistics", "error", err)
		return nil, status.Error(codes.Internal, "failed to get audit statistics")
	}

	// Convert service response to proto response
	statusStats := make(map[int32]int32)
	for k, v := range result.StatusStats {
		statusStats[int32(k)] = int32(v)
	}

	levelStats := make(map[int32]int32)
	for k, v := range result.LevelStats {
		levelStats[int32(k)] = int32(v)
	}

	typeStats := make(map[int32]int32)
	for k, v := range result.TypeStats {
		typeStats[int32(k)] = int32(v)
	}

	return &auditv1.GetAuditStatisticsResponse{
		TotalCount:  result.TotalCount,
		PassRate:    result.PassRate,
		StatusStats: statusStats,
		LevelStats:  levelStats,
		TypeStats:   typeStats,
	}, nil
}

// GetViolationTrends retrieves violation trends
func (h *AuditServiceHandler) GetViolationTrends(ctx context.Context, req *auditv1.GetViolationTrendsRequest) (*auditv1.GetViolationTrendsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Convert proto request to service request
	serviceReq := service.GetViolationTrendsRequest{
		StartDate: req.StartDate.AsTime(),
		EndDate:   req.EndDate.AsTime(),
	}

	// Call service layer
	result, err := h.service.GetViolationTrends(ctx, serviceReq)
	if err != nil {
		h.logger.Error("Failed to get violation trends", "error", err)
		return nil, status.Error(codes.Internal, "failed to get violation trends")
	}

	// Convert service response to proto response
	trends := make([]*auditv1.ViolationTrend, len(result.Trends))
	for i, trend := range result.Trends {
		trends[i] = &auditv1.ViolationTrend{
			Date:   timestamppb.New(trend.Date),
			Count:  trend.Count,
			Level:  auditv1.AuditLevel(trend.Level),
			Type:   auditv1.ContentType(trend.Type),
		}
	}

	return &auditv1.GetViolationTrendsResponse{
		Trends: trends,
	}, nil
}

	return &GetViolationTrendsResponse{
		Trends: result.Trends,
	}, nil
}