package client

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "api_gateway/proto/proto_gen/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// RecommendationServiceClient 推荐服务客户端封装
type RecommendationServiceClient struct {
	conn   *grpc.ClientConn
	client pb.RecommendationServiceClient
}

// NewRecommendationServiceClient 创建推荐服务客户端
func NewRecommendationServiceClient(serviceAddr string) (*RecommendationServiceClient, error) {
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
		return nil, fmt.Errorf("failed to connect to recommendation service at %s: %w", serviceAddr, err)
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
			return nil, fmt.Errorf("failed to establish connection to recommendation service: connection timeout")
		}
	}

	log.Printf("Successfully connected to recommendation service at %s", serviceAddr)

	return &RecommendationServiceClient{
		conn:   conn,
		client: pb.NewRecommendationServiceClient(conn),
	}, nil
}

// Close 关闭连接
func (c *RecommendationServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetConnection 获取gRPC连接
func (c *RecommendationServiceClient) GetConnection() *grpc.ClientConn {
	return c.conn
}

// IsConnected 检查连接状态
func (c *RecommendationServiceClient) IsConnected() bool {
	if c.conn == nil {
		return false
	}
	state := c.conn.GetState()
	return state == connectivity.Ready || state == connectivity.Idle
}

// GetPersonalizedRecommendations 获取个性化推荐
func (c *RecommendationServiceClient) GetPersonalizedRecommendations(ctx context.Context, req *pb.GetPersonalizedRecommendationsRequest) (*pb.GetPersonalizedRecommendationsResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetPersonalizedRecommendations(ctx, req)
}

// GetGeneralRecommendations 获取通用推荐
func (c *RecommendationServiceClient) GetGeneralRecommendations(ctx context.Context, req *pb.GetGeneralRecommendationsRequest) (*pb.GetGeneralRecommendationsResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetGeneralRecommendations(ctx, req)
}

// UpdateUserPreferences 更新用户偏好
func (c *RecommendationServiceClient) UpdateUserPreferences(ctx context.Context, req *pb.UpdateUserPreferencesRequest) (*pb.UpdateUserPreferencesResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.UpdateUserPreferences(ctx, req)
}

// RecordUserAction 记录用户行为
func (c *RecommendationServiceClient) RecordUserAction(ctx context.Context, req *pb.RecordUserActionRequest) (*pb.RecordUserActionResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.RecordUserAction(ctx, req)
}
