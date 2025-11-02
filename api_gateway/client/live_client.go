package client

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	livepb "api_gateway/proto_gen/live"
)

// LiveServiceClient 直播服务客户端封装
type LiveServiceClient struct {
	conn   *grpc.ClientConn
	client livepb.LiveServiceClient
}

// NewLiveServiceClient 创建直播服务客户端
func NewLiveServiceClient(serviceAddr string) (*LiveServiceClient, error) {
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
		return nil, fmt.Errorf("failed to connect to live service at %s: %w", serviceAddr, err)
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
			return nil, fmt.Errorf("failed to establish connection to live service: connection timeout")
		}
	}

	log.Printf("Successfully connected to live service at %s", serviceAddr)

	return &LiveServiceClient{
		conn:   conn,
		client: livepb.NewLiveServiceClient(conn),
	}, nil
}

// Close 关闭连接
func (c *LiveServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetConnection 获取gRPC连接
func (c *LiveServiceClient) GetConnection() *grpc.ClientConn {
	return c.conn
}

// IsConnected 检查连接状态
func (c *LiveServiceClient) IsConnected() bool {
	if c.conn == nil {
		return false
	}
	state := c.conn.GetState()
	return state == connectivity.Ready || state == connectivity.Idle
}

// StartLive 开始直播
func (c *LiveServiceClient) StartLive(ctx context.Context, req *livepb.StartLiveRequest) (*livepb.StartLiveResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.StartLive(ctx, req)
}

// StopLive 结束直播
func (c *LiveServiceClient) StopLive(ctx context.Context, req *livepb.StopLiveRequest) (*livepb.StopLiveResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.StopLive(ctx, req)
}

// GetLiveStream 获取直播流信息
func (c *LiveServiceClient) GetLiveStream(ctx context.Context, req *livepb.GetLiveStreamRequest) (*livepb.GetLiveStreamResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetLiveStream(ctx, req)
}

// GetLiveList 获取直播列表
func (c *LiveServiceClient) GetLiveList(ctx context.Context, req *livepb.GetLiveListRequest) (*livepb.GetLiveListResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetLiveList(ctx, req)
}

// GetHotLiveList 获取热门直播列表
func (c *LiveServiceClient) GetHotLiveList(ctx context.Context, req *livepb.GetHotLiveListRequest) (*livepb.GetHotLiveListResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetHotLiveList(ctx, req)
}

// JoinLiveRoom 加入直播间
func (c *LiveServiceClient) JoinLiveRoom(ctx context.Context, req *livepb.JoinLiveRoomRequest) (*livepb.JoinLiveRoomResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.JoinLiveRoom(ctx, req)
}

// LeaveLiveRoom 离开直播间
func (c *LiveServiceClient) LeaveLiveRoom(ctx context.Context, req *livepb.LeaveLiveRoomRequest) (*livepb.LeaveLiveRoomResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.LeaveLiveRoom(ctx, req)
}

// SendLiveChat 发送直播聊天消息
func (c *LiveServiceClient) SendLiveChat(ctx context.Context, req *livepb.SendLiveChatRequest) (*livepb.SendLiveChatResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.SendLiveChat(ctx, req)
}

// GetLiveChatList 获取直播聊天列表
func (c *LiveServiceClient) GetLiveChatList(ctx context.Context, req *livepb.GetLiveChatListRequest) (*livepb.GetLiveChatListResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetLiveChatList(ctx, req)
}
