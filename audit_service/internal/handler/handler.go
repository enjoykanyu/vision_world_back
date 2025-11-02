package handler

import (
	"context"
	"fmt"
	"net"

	"github.com/vision_world/audit_service/internal/config"
	"github.com/vision_world/audit_service/internal/service"
	"github.com/vision_world/audit_service/pkg/logger"
	auditpb "github.com/vision_world/audit_service/proto_gen/audit/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AuditHandler 审核服务处理器
type AuditHandler struct {
	auditpb.UnimplementedAuditServiceServer
	config       *config.Config
	logger       logger.Logger
	auditService service.AuditService
}

// NewAuditHandler 创建审核服务处理器
func NewAuditHandler(cfg *config.Config, log logger.Logger, auditService service.AuditService) *AuditHandler {
	return &AuditHandler{
		config:       cfg,
		logger:       log,
		auditService: auditService,
	}
}

// SubmitContent 提交内容审核
func (h *AuditHandler) SubmitContent(ctx context.Context, req *auditpb.SubmitContentRequest) (*auditpb.SubmitContentResponse, error) {
	// 参数验证
	if req.ContentId == "" {
		return nil, status.Error(codes.InvalidArgument, "content_id is required")
	}

	// 调用审核服务
	auditReq := &service.SubmitContentRequest{
		ContentID:   req.ContentId,
		ContentType: req.ContentType.String(),
		Content:     req.Content,
		UploaderID:  fmt.Sprint(req.UploaderId),
		Metadata:    req.Metadata,
	}

	resp, err := h.auditService.SubmitContent(ctx, auditReq)
	if err != nil {
		h.logger.Error("Failed to submit content for audit", "error", err)
		return nil, status.Error(codes.Internal, "failed to submit content for audit")
	}

	// 转换审核状态
	var auditStatus auditpb.AuditStatus
	switch resp.Status {
	case "pending":
		auditStatus = auditpb.AuditStatus_AUDIT_STATUS_PENDING
	case "approved":
		auditStatus = auditpb.AuditStatus_AUDIT_STATUS_PASSED
	case "rejected":
		auditStatus = auditpb.AuditStatus_AUDIT_STATUS_REJECTED
	default:
		auditStatus = auditpb.AuditStatus_AUDIT_STATUS_PENDING
	}

	// 转换违规等级
	var auditLevel auditpb.AuditLevel
	switch resp.Level {
	case "low":
		auditLevel = auditpb.AuditLevel_AUDIT_LEVEL_LOW
	case "medium":
		auditLevel = auditpb.AuditLevel_AUDIT_LEVEL_MEDIUM
	case "high":
		auditLevel = auditpb.AuditLevel_AUDIT_LEVEL_HIGH
	case "critical":
		auditLevel = auditpb.AuditLevel_AUDIT_LEVEL_CRITICAL
	default:
		auditLevel = auditpb.AuditLevel_AUDIT_LEVEL_UNSPECIFIED
	}

	return &auditpb.SubmitContentResponse{
		AuditId:   resp.AuditID,
		Status:    auditStatus,
		Reason:    resp.Reason,
		Level:     auditLevel,
		CreatedAt: timestamppb.New(resp.CreatedAt),
	}, nil
}

// GetAuditResult 获取审核结果
func (h *AuditHandler) GetAuditResult(ctx context.Context, req *auditpb.GetAuditResultRequest) (*auditpb.GetAuditResultResponse, error) {
	h.logger.Info("GetAuditResult called", "audit_id", req.AuditId)

	result, err := h.auditService.GetAuditResult(ctx, fmt.Sprint(req.AuditId))
	if err != nil {
		h.logger.Error("GetAuditResult failed", "error", err, "audit_id", req.AuditId)
		return nil, status.Errorf(codes.Internal, "failed to get audit result: %v", err)
	}

	// 转换内容类型
	var contentType auditpb.ContentType
	switch result.ContentType {
	case "text":
		contentType = auditpb.ContentType_CONTENT_TYPE_TEXT
	case "image":
		contentType = auditpb.ContentType_CONTENT_TYPE_IMAGE
	case "video":
		contentType = auditpb.ContentType_CONTENT_TYPE_VIDEO
	case "audio":
		contentType = auditpb.ContentType_CONTENT_TYPE_AUDIO
	default:
		contentType = auditpb.ContentType_CONTENT_TYPE_UNSPECIFIED
	}

	// 转换审核状态
	var auditStatus auditpb.AuditStatus
	switch result.Status {
	case "pending":
		auditStatus = auditpb.AuditStatus_AUDIT_STATUS_PENDING
	case "approved":
		auditStatus = auditpb.AuditStatus_AUDIT_STATUS_PASSED
	case "rejected":
		auditStatus = auditpb.AuditStatus_AUDIT_STATUS_REJECTED
	case "under_review":
		auditStatus = auditpb.AuditStatus_AUDIT_STATUS_UNDER_REVIEW
	default:
		auditStatus = auditpb.AuditStatus_AUDIT_STATUS_UNSPECIFIED
	}

	// 转换违规等级
	var auditLevel auditpb.AuditLevel
	switch result.Level {
	case "low":
		auditLevel = auditpb.AuditLevel_AUDIT_LEVEL_LOW
	case "medium":
		auditLevel = auditpb.AuditLevel_AUDIT_LEVEL_MEDIUM
	case "high":
		auditLevel = auditpb.AuditLevel_AUDIT_LEVEL_HIGH
	case "critical":
		auditLevel = auditpb.AuditLevel_AUDIT_LEVEL_CRITICAL
	default:
		auditLevel = auditpb.AuditLevel_AUDIT_LEVEL_UNSPECIFIED
	}

	var reviewTime *timestamppb.Timestamp
	if result.ReviewTime != nil {
		reviewTime = timestamppb.New(*result.ReviewTime)
	}

	return &auditpb.GetAuditResultResponse{
		AuditId:     result.AuditID,
		ContentId:   result.ContentID,
		ContentType: contentType,
		Status:      auditStatus,
		Reason:      result.Reason,
		Level:       auditLevel,
		ReviewerId:  0, // 如果model中有reviewer_id可以设置
		ReviewedAt:  reviewTime,
		CreatedAt:   timestamppb.New(result.CreatedAt),
	}, nil
}

// UpdateAuditStatus 更新审核状态
func (h *AuditHandler) UpdateAuditStatus(ctx context.Context, req *auditpb.UpdateAuditStatusRequest) (*auditpb.UpdateAuditStatusResponse, error) {
	h.logger.Info("UpdateAuditStatus called", "audit_id", req.AuditId, "status", req.Status)

	serviceReq := &service.UpdateAuditStatusRequest{
		AuditID:    req.AuditId,
		Status:     req.Status,
		Reason:     req.Reason,
		Details:    req.Details,
		Violations: req.Violations,
		ReviewerID: req.ReviewerId,
	}

	resp, err := h.auditService.UpdateAuditStatus(ctx, serviceReq)
	if err != nil {
		h.logger.Error("UpdateAuditStatus failed", "error", err, "audit_id", req.AuditId)
		return nil, status.Errorf(codes.Internal, "failed to update audit status: %v", err)
	}

	return &auditpb.UpdateAuditStatusResponse{
		Success: resp.Success,
		Message: resp.Message,
	}, nil
}

// StartGRPCServer 启动gRPC服务器
func StartGRPCServer(cfg *config.Config, log logger.Logger, auditService service.AuditService) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.Port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer()
	handler := NewAuditHandler(cfg, log, auditService)
	auditpb.RegisterAuditServiceServer(grpcServer, handler)

	log.Info("Starting gRPC server", "port", cfg.Server.Port)
	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}
