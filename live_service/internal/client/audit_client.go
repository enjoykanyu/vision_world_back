package client

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	pb "live_service/proto/proto_gen/audit"
)

// AuditServiceClient 审计服务客户端封装
type AuditServiceClient struct {
	conn   *grpc.ClientConn
	client pb.AuditServiceClient
	mu     sync.RWMutex
}

// NewAuditServiceClient 创建审计服务客户端
func NewAuditServiceClient(serviceAddr string) (*AuditServiceClient, error) {
	// gRPC连接配置
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(4*1024*1024),
			grpc.MaxCallSendMsgSize(4*1024*1024),
		),
	}

	// 建立连接
	conn, err := grpc.Dial(serviceAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to audit service at %s: %w", serviceAddr, err)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 等待连接状态变为Ready或者超时
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			break
		}
		if !conn.WaitForStateChange(ctx, state) {
			conn.Close()
			return nil, fmt.Errorf("failed to establish connection to audit service: connection timeout")
		}
	}

	log.Printf("Successfully connected to audit service at %s", serviceAddr)

	return &AuditServiceClient{
		conn:   conn,
		client: pb.NewAuditServiceClient(conn),
	}, nil
}

// Close 关闭连接
func (c *AuditServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsConnected 检查连接状态
func (c *AuditServiceClient) IsConnected() bool {
	if c.conn == nil {
		return false
	}
	state := c.conn.GetState()
	return state == connectivity.Ready || state == connectivity.Idle
}

// SubmitContent 提交内容审核
func (c *AuditServiceClient) SubmitContent(ctx context.Context, req *pb.SubmitContentRequest) (*pb.SubmitContentResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.SubmitContent(ctx, req)
}

// GetAuditResult 获取审核结果
func (c *AuditServiceClient) GetAuditResult(ctx context.Context, req *pb.GetAuditResultRequest) (*pb.GetAuditResultResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetAuditResult(ctx, req)
}
