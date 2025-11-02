package client

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	pb "github.com/visionworld/service/audit_service/proto_gen/audit/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	failCount    int
	lastFailTime time.Time
	isOpen       bool
	mutex        sync.Mutex
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		lastFailTime: time.Now(),
	}
}

// CanExecute 检查是否可以执行请求
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if cb.isOpen {
		// 熔断器开启，检查是否过了冷却时间（30秒）
		if time.Since(cb.lastFailTime) > 30*time.Second {
			cb.isOpen = false
			cb.failCount = 0
			return true
		}
		return false
	}
	return true
}

// RecordSuccess 记录成功
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.failCount = 0
	cb.isOpen = false
}

// RecordFailure 记录失败
func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.failCount++
	cb.lastFailTime = time.Now()

	// 连续失败3次开启熔断器
	if cb.failCount >= 3 {
		cb.isOpen = true
		log.Printf("Audit service circuit breaker opened due to %d consecutive failures", cb.failCount)
	}
}

// AuditServiceClient 审核服务客户端封装
type AuditServiceClient struct {
	conn           *grpc.ClientConn
	client         pb.AuditServiceClient
	serviceAddr    string
	mu             sync.RWMutex
	circuitBreaker *CircuitBreaker
}

// NewAuditServiceClient 创建审核服务客户端
func NewAuditServiceClient(serviceAddr string) (*AuditServiceClient, error) {
	// gRPC连接配置
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second, // 每10秒发送一次keepalive ping
			Timeout:             time.Second,      // ping超时时间
			PermitWithoutStream: true,             // 允许在没有活跃stream时发送keepalive ping
		}),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(4*1024*1024), // 4MB
			grpc.MaxCallSendMsgSize(4*1024*1024), // 4MB
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
			// 超时或上下文取消
			conn.Close()
			return nil, fmt.Errorf("failed to establish connection to audit service: connection timeout")
		}
	}

	log.Printf("Successfully connected to audit service at %s", serviceAddr)

	return &AuditServiceClient{
		conn:           conn,
		client:         pb.NewAuditServiceClient(conn),
		serviceAddr:    serviceAddr,
		circuitBreaker: NewCircuitBreaker(),
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

// getClient 获取客户端（带熔断检查）
func (c *AuditServiceClient) getClient() (pb.AuditServiceClient, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}

	if !c.circuitBreaker.CanExecute() {
		return nil, fmt.Errorf("circuit breaker is open, please try again later")
	}

	return c.client, nil
}

// SubmitContent 提交内容审核
func (c *AuditServiceClient) SubmitContent(ctx context.Context, req *pb.SubmitContentRequest) (*pb.SubmitContentResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.SubmitContent(ctx, req)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to submit content for audit: %w", err)
	}

	c.circuitBreaker.RecordSuccess()
	return resp, nil
}

// GetAuditResult 获取审核结果
func (c *AuditServiceClient) GetAuditResult(ctx context.Context, req *pb.GetAuditResultRequest) (*pb.GetAuditResultResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetAuditResult(ctx, req)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to get audit result: %w", err)
	}

	c.circuitBreaker.RecordSuccess()
	return resp, nil
}

// UpdateAuditStatus 更新审核状态
func (c *AuditServiceClient) UpdateAuditStatus(ctx context.Context, req *pb.UpdateAuditStatusRequest) (*pb.UpdateAuditStatusResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.UpdateAuditStatus(ctx, req)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to update audit status: %w", err)
	}

	c.circuitBreaker.RecordSuccess()
	return resp, nil
}

// ListAuditRecords 获取审核记录列表
func (c *AuditServiceClient) ListAuditRecords(ctx context.Context, req *pb.ListAuditRecordsRequest) (*pb.ListAuditRecordsResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.ListAuditRecords(ctx, req)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to list audit records: %w", err)
	}

	c.circuitBreaker.RecordSuccess()
	return resp, nil
}

// AddToWhitelist 添加到白名单
func (c *AuditServiceClient) AddToWhitelist(ctx context.Context, req *pb.AddToWhitelistRequest) (*pb.AddToWhitelistResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.AddToWhitelist(ctx, req)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to add to whitelist: %w", err)
	}

	c.circuitBreaker.RecordSuccess()
	return resp, nil
}

// RemoveFromWhitelist 从白名单移除
func (c *AuditServiceClient) RemoveFromWhitelist(ctx context.Context, req *pb.RemoveFromWhitelistRequest) (*pb.RemoveFromWhitelistResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.RemoveFromWhitelist(ctx, req)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to remove from whitelist: %w", err)
	}

	c.circuitBreaker.RecordSuccess()
	return resp, nil
}

// AddToBlacklist 添加到黑名单
func (c *AuditServiceClient) AddToBlacklist(ctx context.Context, req *pb.AddToBlacklistRequest) (*pb.AddToBlacklistResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.AddToBlacklist(ctx, req)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to add to blacklist: %w", err)
	}

	c.circuitBreaker.RecordSuccess()
	return resp, nil
}

// RemoveFromBlacklist 从黑名单移除
func (c *AuditServiceClient) RemoveFromBlacklist(ctx context.Context, req *pb.RemoveFromBlacklistRequest) (*pb.RemoveFromBlacklistResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.RemoveFromBlacklist(ctx, req)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to remove from blacklist: %w", err)
	}

	c.circuitBreaker.RecordSuccess()
	return resp, nil
}
