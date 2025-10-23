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

// VideoServiceClient 视频服务客户端封装
type VideoServiceClient struct {
	conn   *grpc.ClientConn
	client pb.VideoServiceClient
}

// NewVideoServiceClient 创建视频服务客户端
func NewVideoServiceClient(serviceAddr string) (*VideoServiceClient, error) {
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
		return nil, fmt.Errorf("failed to connect to video service at %s: %w", serviceAddr, err)
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
			return nil, fmt.Errorf("failed to establish connection to video service: connection timeout")
		}
	}

	log.Printf("Successfully connected to video service at %s", serviceAddr)

	return &VideoServiceClient{
		conn:   conn,
		client: pb.NewVideoServiceClient(conn),
	}, nil
}

// Close 关闭连接
func (c *VideoServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetConnection 获取gRPC连接
func (c *VideoServiceClient) GetConnection() *grpc.ClientConn {
	return c.conn
}

// IsConnected 检查连接状态
func (c *VideoServiceClient) IsConnected() bool {
	if c.conn == nil {
		return false
	}
	state := c.conn.GetState()
	return state == connectivity.Ready || state == connectivity.Idle
}

// GetVideoInfo 获取视频信息
func (c *VideoServiceClient) GetVideoInfo(ctx context.Context, req *pb.GetVideoInfoRequest) (*pb.VideoResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetVideoInfo(ctx, req)
}

// GetVideoInfos 批量获取视频信息
func (c *VideoServiceClient) GetVideoInfos(ctx context.Context, req *pb.GetVideoInfosRequest) (*pb.GetVideoInfosResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetVideoInfos(ctx, req)
}

// GetRecommendVideos 获取推荐视频列表
func (c *VideoServiceClient) GetRecommendVideos(ctx context.Context, req *pb.GetRecommendVideosRequest) (*pb.GetRecommendVideosResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetRecommendVideos(ctx, req)
}

// GetFollowVideos 获取关注用户的视频列表
func (c *VideoServiceClient) GetFollowVideos(ctx context.Context, req *pb.GetFollowVideosRequest) (*pb.GetFollowVideosResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetFollowVideos(ctx, req)
}

// GetHotVideos 获取热门视频列表
func (c *VideoServiceClient) GetHotVideos(ctx context.Context, req *pb.GetHotVideosRequest) (*pb.GetHotVideosResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetHotVideos(ctx, req)
}

// GetUserVideos 获取用户发布的视频列表
func (c *VideoServiceClient) GetUserVideos(ctx context.Context, req *pb.GetUserVideosRequest) (*pb.GetUserVideosResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetUserVideos(ctx, req)
}

// LikeVideo 点赞/取消点赞视频
func (c *VideoServiceClient) LikeVideo(ctx context.Context, req *pb.LikeVideoRequest) (*pb.LikeVideoResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.LikeVideo(ctx, req)
}

// GetUserLikedVideos 获取用户点赞的视频列表
func (c *VideoServiceClient) GetUserLikedVideos(ctx context.Context, req *pb.GetUserLikedVideosRequest) (*pb.GetUserLikedVideosResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetUserLikedVideos(ctx, req)
}

// ShareVideo 分享视频
func (c *VideoServiceClient) ShareVideo(ctx context.Context, req *pb.ShareVideoRequest) (*pb.ShareVideoResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.ShareVideo(ctx, req)
}

// CommentVideo 发表评论
func (c *VideoServiceClient) CommentVideo(ctx context.Context, req *pb.CommentRequest) (*pb.CommentResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.CommentVideo(ctx, req)
}

// DeleteComment 删除评论
func (c *VideoServiceClient) DeleteComment(ctx context.Context, req *pb.DeleteCommentRequest) (*pb.DeleteCommentResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.DeleteComment(ctx, req)
}

// GetVideoComments 获取视频评论列表
func (c *VideoServiceClient) GetVideoComments(ctx context.Context, req *pb.GetVideoCommentsRequest) (*pb.GetVideoCommentsResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetVideoComments(ctx, req)
}

// PublishVideo 发布视频
func (c *VideoServiceClient) PublishVideo(ctx context.Context, req *pb.PublishVideoRequest) (*pb.PublishVideoResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.PublishVideo(ctx, req)
}

// DeleteVideo 删除视频
func (c *VideoServiceClient) DeleteVideo(ctx context.Context, req *pb.DeleteVideoRequest) (*pb.DeleteVideoResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.DeleteVideo(ctx, req)
}
