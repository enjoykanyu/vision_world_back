package server

import (
	"audit_service/internal/handler"
	pb "audit_service/proto/audit/v1"
	"context"
)

// auditServiceServer 审核服务gRPC服务器实现
type auditServiceServer struct {
	pb.UnimplementedAuditServiceServer
	handler *handler.AuditServiceHandler
}

// NewAuditServiceServer 创建审核服务gRPC服务器
func NewAuditServiceServer(handler *handler.AuditServiceHandler) pb.AuditServiceServer {
	return &auditServiceServer{
		handler: handler,
	}
}

// SubmitContent 提交内容审核
func (s *auditServiceServer) SubmitContent(ctx context.Context, req *pb.SubmitContentRequest) (*pb.SubmitContentResponse, error) {
	return s.handler.SubmitContent(ctx, req)
}

// GetAuditResult 获取审核结果
func (s *auditServiceServer) GetAuditResult(ctx context.Context, req *pb.GetAuditResultRequest) (*pb.GetAuditResultResponse, error) {
	return s.handler.GetAuditResult(ctx, req)
}

// UpdateAuditStatus 更新审核状态
func (s *auditServiceServer) UpdateAuditStatus(ctx context.Context, req *pb.UpdateAuditStatusRequest) (*pb.UpdateAuditStatusResponse, error) {
	return s.handler.UpdateAuditStatus(ctx, req)
}

// ListAuditRecords 获取审核记录列表
func (s *auditServiceServer) ListAuditRecords(ctx context.Context, req *pb.ListAuditRecordsRequest) (*pb.ListAuditRecordsResponse, error) {
	return s.handler.ListAuditRecords(ctx, req)
}

// GetAuditStatistics 获取审核统计信息
func (s *auditServiceServer) GetAuditStatistics(ctx context.Context, req *pb.GetAuditStatisticsRequest) (*pb.GetAuditStatisticsResponse, error) {
	return s.handler.GetAuditStatistics(ctx, req)
}

// CreateAuditTemplate 创建审核模板
func (s *auditServiceServer) CreateAuditTemplate(ctx context.Context, req *pb.CreateAuditTemplateRequest) (*pb.CreateAuditTemplateResponse, error) {
	return s.handler.CreateAuditTemplate(ctx, req)
}

// UpdateAuditTemplate 更新审核模板
func (s *auditServiceServer) UpdateAuditTemplate(ctx context.Context, req *pb.UpdateAuditTemplateRequest) (*pb.UpdateAuditTemplateResponse, error) {
	return s.handler.UpdateAuditTemplate(ctx, req)
}

// DeleteAuditTemplate 删除审核模板
func (s *auditServiceServer) DeleteAuditTemplate(ctx context.Context, req *pb.DeleteAuditTemplateRequest) (*pb.DeleteAuditTemplateResponse, error) {
	return s.handler.DeleteAuditTemplate(ctx, req)
}

// ListAuditTemplates 获取审核模板列表
func (s *auditServiceServer) ListAuditTemplates(ctx context.Context, req *pb.ListAuditTemplatesRequest) (*pb.ListAuditTemplatesResponse, error) {
	return s.handler.ListAuditTemplates(ctx, req)
}

// AddToWhitelist 添加到白名单
func (s *auditServiceServer) AddToWhitelist(ctx context.Context, req *pb.AddToWhitelistRequest) (*pb.AddToWhitelistResponse, error) {
	return s.handler.AddToWhitelist(ctx, req)
}

// RemoveFromWhitelist 从白名单移除
func (s *auditServiceServer) RemoveFromWhitelist(ctx context.Context, req *pb.RemoveFromWhitelistRequest) (*pb.RemoveFromWhitelistResponse, error) {
	return s.handler.RemoveFromWhitelist(ctx, req)
}

// AddToBlacklist 添加到黑名单
func (s *auditServiceServer) AddToBlacklist(ctx context.Context, req *pb.AddToBlacklistRequest) (*pb.AddToBlacklistResponse, error) {
	return s.handler.AddToBlacklist(ctx, req)
}

// RemoveFromBlacklist 从黑名单移除
func (s *auditServiceServer) RemoveFromBlacklist(ctx context.Context, req *pb.RemoveFromBlacklistRequest) (*pb.RemoveFromBlacklistResponse, error) {
	return s.handler.RemoveFromBlacklist(ctx, req)
}

// GetManualReviewQueue 获取人工审核队列
func (s *auditServiceServer) GetManualReviewQueue(ctx context.Context, req *pb.GetManualReviewQueueRequest) (*pb.GetManualReviewQueueResponse, error) {
	return s.handler.GetManualReviewQueue(ctx, req)
}

// AssignManualReview 分配人工审核任务
func (s *auditServiceServer) AssignManualReview(ctx context.Context, req *pb.AssignManualReviewRequest) (*pb.AssignManualReviewResponse, error) {
	return s.handler.AssignManualReview(ctx, req)
}

// GetViolationTrends 获取违规趋势
func (s *auditServiceServer) GetViolationTrends(ctx context.Context, req *pb.GetViolationTrendsRequest) (*pb.GetViolationTrendsResponse, error) {
	return s.handler.GetViolationTrends(ctx, req)
}
