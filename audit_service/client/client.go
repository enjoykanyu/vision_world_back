package client

import (
	"context"
	"fmt"
	"time"

	pb "github.com/vision_world/audit_service/proto_gen/audit/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AuditServiceClient 审核服务客户端接口
type AuditServiceClient interface {
	SubmitContent(ctx context.Context, req *SubmitContentRequest) (*SubmitContentResponse, error)
	GetAuditResult(ctx context.Context, req *GetAuditResultRequest) (*GetAuditResultResponse, error)
	UpdateAuditStatus(ctx context.Context, req *UpdateAuditStatusRequest) (*UpdateAuditStatusResponse, error)
	ListAuditRecords(ctx context.Context, req *ListAuditRecordsRequest) (*ListAuditRecordsResponse, error)
	AddToWhitelist(ctx context.Context, req *AddToWhitelistRequest) (*AddToWhitelistResponse, error)
	RemoveFromWhitelist(ctx context.Context, req *RemoveFromWhitelistRequest) (*RemoveFromWhitelistResponse, error)
	AddToBlacklist(ctx context.Context, req *AddToBlacklistRequest) (*AddToBlacklistResponse, error)
	RemoveFromBlacklist(ctx context.Context, req *RemoveFromBlacklistRequest) (*RemoveFromBlacklistResponse, error)
}

// auditServiceClient 审核服务客户端实现
type auditServiceClient struct {
	conn   *grpc.ClientConn
	client pb.AuditServiceClient
}

// NewAuditServiceClient 创建审核服务客户端
func NewAuditServiceClient(target string) (AuditServiceClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to audit service: %w", err)
	}

	return &auditServiceClient{
		conn:   conn,
		client: pb.NewAuditServiceClient(conn),
	}, nil
}

// SubmitContent 提交内容审核
func (c *auditServiceClient) SubmitContent(ctx context.Context, req *SubmitContentRequest) (*SubmitContentResponse, error) {
	// 转换请求类型
	pbReq := &pb.SubmitContentRequest{
		ContentId:   req.ContentId,
		ContentType: pb.ContentType(req.ContentType),
		UploaderId:  req.UploaderId,
		Title:       req.Title,
		Content:     req.Content,
		CreateTime:  req.CreateTime,
	}

	pbResp, err := c.client.SubmitContent(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	// 转换响应类型
	return &SubmitContentResponse{
		AuditId:   pbResp.AuditId,
		Status:    int32(pbResp.Status),
		Reason:    pbResp.Reason,
		Level:     int32(pbResp.Level),
		CreatedAt: pbResp.CreatedAt,
	}, nil
}

// GetAuditResult 获取审核结果
func (c *auditServiceClient) GetAuditResult(ctx context.Context, req *GetAuditResultRequest) (*GetAuditResultResponse, error) {
	pbReq := &pb.GetAuditResultRequest{
		AuditId: req.AuditId,
	}

	pbResp, err := c.client.GetAuditResult(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	return &GetAuditResultResponse{
		AuditId:     pbResp.AuditId,
		ContentId:   pbResp.ContentId,
		ContentType: int32(pbResp.ContentType),
		Status:      int32(pbResp.Status),
		Reason:      pbResp.Reason,
		Level:       int32(pbResp.Level),
		ReviewerId:  pbResp.ReviewerId,
		ReviewedAt:  pbResp.ReviewedAt,
		CreatedAt:   pbResp.CreatedAt,
	}, nil
}

// UpdateAuditStatus 更新审核状态
func (c *auditServiceClient) UpdateAuditStatus(ctx context.Context, req *UpdateAuditStatusRequest) (*UpdateAuditStatusResponse, error) {
	pbReq := &pb.UpdateAuditStatusRequest{
		AuditId:    req.AuditId,
		Status:     pb.AuditStatus(req.Status),
		ReviewerId: req.ReviewerId,
		Reason:     req.Reason,
	}

	pbResp, err := c.client.UpdateAuditStatus(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	return &UpdateAuditStatusResponse{
		Success: pbResp.Success,
		Message: pbResp.Message,
	}, nil
}

// ListAuditRecords 获取审核记录列表
func (c *auditServiceClient) ListAuditRecords(ctx context.Context, req *ListAuditRecordsRequest) (*ListAuditRecordsResponse, error) {
	pbReq := &pb.ListAuditRecordsRequest{
		ContentType: pb.ContentType(req.ContentType),
		Status:      pb.AuditStatus(req.Status),
		Level:       pb.AuditLevel(req.Level),
		UploaderId:  req.UploaderId,
		ReviewerId:  req.ReviewerId,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
		Page:        req.Page,
		PageSize:    req.PageSize,
	}

	pbResp, err := c.client.ListAuditRecords(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	records := make([]AuditRecord, len(pbResp.Records))
	for i, record := range pbResp.Records {
		records[i] = AuditRecord{
			AuditId:     record.AuditId,
			ContentId:   record.ContentId,
			ContentType: int32(record.ContentType),
			Status:      int32(record.Status),
			Reason:      record.Reason,
			Level:       int32(record.Level),
			UploaderId:  record.UploaderId,
			ReviewerId:  record.ReviewerId,
			CreatedAt:   record.CreatedAt,
			ReviewedAt:  record.ReviewedAt,
		}
	}

	return &ListAuditRecordsResponse{
		Total:    pbResp.Total,
		Page:     pbResp.Page,
		PageSize: pbResp.PageSize,
		Records:  records,
	}, nil
}

// AddToWhitelist 添加到白名单
func (c *auditServiceClient) AddToWhitelist(ctx context.Context, req *AddToWhitelistRequest) (*AddToWhitelistResponse, error) {
	pbReq := &pb.AddToWhitelistRequest{
		ContentId:   req.ContentId,
		ContentType: pb.ContentType(req.ContentType),
		Reason:      req.Reason,
		CreatedBy:   req.CreatedBy,
	}

	pbResp, err := c.client.AddToWhitelist(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	return &AddToWhitelistResponse{
		Success: pbResp.Success,
		Message: pbResp.Message,
	}, nil
}

// RemoveFromWhitelist 从白名单移除
func (c *auditServiceClient) RemoveFromWhitelist(ctx context.Context, req *RemoveFromWhitelistRequest) (*RemoveFromWhitelistResponse, error) {
	pbReq := &pb.RemoveFromWhitelistRequest{
		ContentId: req.ContentId,
	}

	pbResp, err := c.client.RemoveFromWhitelist(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	return &RemoveFromWhitelistResponse{
		Success: pbResp.Success,
		Message: pbResp.Message,
	}, nil
}

// AddToBlacklist 添加到黑名单
func (c *auditServiceClient) AddToBlacklist(ctx context.Context, req *AddToBlacklistRequest) (*AddToBlacklistResponse, error) {
	pbReq := &pb.AddToBlacklistRequest{
		ContentId:   req.ContentId,
		ContentType: pb.ContentType(req.ContentType),
		Reason:      req.Reason,
		CreatedBy:   req.CreatedBy,
	}

	pbResp, err := c.client.AddToBlacklist(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	return &AddToBlacklistResponse{
		Success: pbResp.Success,
		Message: pbResp.Message,
	}, nil
}

// RemoveFromBlacklist 从黑名单移除
func (c *auditServiceClient) RemoveFromBlacklist(ctx context.Context, req *RemoveFromBlacklistRequest) (*RemoveFromBlacklistResponse, error) {
	pbReq := &pb.RemoveFromBlacklistRequest{
		ContentId: req.ContentId,
	}

	pbResp, err := c.client.RemoveFromBlacklist(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	return &RemoveFromBlacklistResponse{
		Success: pbResp.Success,
		Message: pbResp.Message,
	}, nil
}

// Close 关闭客户端连接
func (c *auditServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
