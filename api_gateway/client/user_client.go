package client

import (
	"context"
	"fmt"
	"log"
	"time"

	userpb "api_gateway/proto/proto_gen/user"
	// videopb "api_gateway/proto_gen/video"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// UserServiceClient 用户服务客户端封装
type UserServiceClient struct {
	conn   *grpc.ClientConn
	client userpb.UserServiceClient
}

// NewUserServiceClient 创建用户服务客户端
func NewUserServiceClient(serviceAddr string) (*UserServiceClient, error) {
	// gRPC连接配置
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                60 * time.Second, // 每60秒发送一次keepalive ping
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
		return nil, fmt.Errorf("failed to connect to user service at %s: %w", serviceAddr, err)
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
			return nil, fmt.Errorf("failed to establish connection to user service: connection timeout")
		}
	}

	log.Printf("Successfully connected to user service at %s", serviceAddr)

	return &UserServiceClient{
		conn:   conn,
		client: userpb.NewUserServiceClient(conn),
	}, nil
}

// Close 关闭连接
func (c *UserServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetConnection 获取gRPC连接
func (c *UserServiceClient) GetConnection() *grpc.ClientConn {
	return c.conn
}

// IsConnected 检查连接状态
func (c *UserServiceClient) IsConnected() bool {
	if c.conn == nil {
		return false
	}
	state := c.conn.GetState()
	return state == connectivity.Ready || state == connectivity.Idle
}

// PhoneLogin 手机号登录
func (c *UserServiceClient) PhoneLogin(ctx context.Context, req *userpb.PhoneLoginRequest) (*userpb.LoginResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.PhoneLogin(ctx, req)
}

// CodeLogin 验证码登录
func (c *UserServiceClient) CodeLogin(ctx context.Context, req *userpb.CodeLoginRequest) (*userpb.LoginResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.CodeLogin(ctx, req)
}

// SendSmsCode 发送短信验证码
func (c *UserServiceClient) SendSmsCode(ctx context.Context, req *userpb.SendSmsRequest) (*userpb.SendSmsResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.SendSmsCode(ctx, req)
}

// GetUserInfo 获取用户信息
func (c *UserServiceClient) GetUserInfo(ctx context.Context, req *userpb.GetUserInfoRequest) (*userpb.UserResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetUserInfo(ctx, req)
}

// VerifyToken 验证Token
func (c *UserServiceClient) VerifyToken(ctx context.Context, req *userpb.VerifyTokenRequest) (*userpb.VerifyTokenResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.VerifyToken(ctx, req)
}

// RefreshToken 刷新Token
func (c *UserServiceClient) RefreshToken(ctx context.Context, req *userpb.RefreshTokenRequest) (*userpb.RefreshTokenResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.RefreshToken(ctx, req)
}

// 退出登录
func (c *UserServiceClient) LogOut(ctx context.Context, req *userpb.LogoutRequest) (*userpb.LogoutResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.Logout(ctx, req)
}
