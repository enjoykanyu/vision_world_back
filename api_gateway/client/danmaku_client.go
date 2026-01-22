package client

import (
	"context"
	"fmt"
	"log"
	"time"

	danmakupb "api_gateway/proto_gen/danmaku"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// DanmakuServiceClient 弹幕服务客户端封装
type DanmakuServiceClient struct {
	conn   *grpc.ClientConn
	client danmakupb.DanmakuServiceClient
}

// NewDanmakuServiceClient 创建弹幕服务客户端
func NewDanmakuServiceClient(serviceAddr string) (*DanmakuServiceClient, error) {
	// gRPC连接配置
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                60 * time.Second, // 每60秒发送一次keepalive ping
			Timeout:             time.Second,      // ping超时时间
			PermitWithoutStream: true,             // 允许在没有活跃stream时发送keepalive ping
		}),
	}

	// 建立连接
	conn, err := grpc.Dial(serviceAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to danmaku service at %s: %w", serviceAddr, err)
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
			return nil, fmt.Errorf("failed to establish connection to danmaku service: connection timeout")
		}
	}

	log.Printf("Successfully connected to danmaku service at %s", serviceAddr)

	return &DanmakuServiceClient{
		conn:   conn,
		client: danmakupb.NewDanmakuServiceClient(conn),
	}, nil
}

// Close 关闭连接
func (c *DanmakuServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetConnection 获取gRPC连接
func (c *DanmakuServiceClient) GetConnection() *grpc.ClientConn {
	return c.conn
}

// IsConnected 检查连接状态
func (c *DanmakuServiceClient) IsConnected() bool {
	if c.conn == nil {
		return false
	}
	state := c.conn.GetState()
	return state == connectivity.Ready || state == connectivity.Idle
}

// SendDanmaku 发送弹幕
func (c *DanmakuServiceClient) SendDanmaku(ctx context.Context, req *danmakupb.SendDanmakuRequest) (*danmakupb.SendDanmakuResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.SendDanmaku(ctx, req)
}

// GetDanmakus 获取视频弹幕列表
func (c *DanmakuServiceClient) GetDanmakus(ctx context.Context, req *danmakupb.GetDanmakusRequest) (*danmakupb.GetDanmakusResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetDanmakus(ctx, req)
}
