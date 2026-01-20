package client

import (
	"context"
	"fmt"
	"log"
	"time"

	videopb "api_gateway/proto/proto_gen/video"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// VideoServiceClient 视频服务客户端封装
type VideoServiceClient struct {
	conn   *grpc.ClientConn
	client videopb.VideoServiceClient
}

// NewVideoServiceClient 创建视频服务客户端
func NewVideoServiceClient(serviceAddr string) (*VideoServiceClient, error) {
	// gRPC连接配置
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                60 * time.Second, // 每60秒发送一次keepalive ping
			Timeout:             time.Second,      // ping超时时间
			PermitWithoutStream: true,             // 允许在没有活跃stream时发送keepalive ping
		}),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(100*1024*1024), // 100MB
			grpc.MaxCallSendMsgSize(100*1024*1024), // 100MB
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
		client: videopb.NewVideoServiceClient(conn),
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

// UploadVideo 上传视频
func (c *VideoServiceClient) UploadVideo(ctx context.Context, req *videopb.UploadVideoRequest) (*videopb.UploadVideoResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.UploadVideo(ctx, req)
}

// CreateVideoRecord 创建视频记录（用于分片上传完成后）
func (c *VideoServiceClient) CreateVideoRecord(ctx context.Context, req *videopb.CreateVideoRecordRequest) (*videopb.CreateVideoRecordResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.CreateVideoRecord(ctx, req)
}

// GetVideoInfo 获取视频信息
func (c *VideoServiceClient) GetVideoInfo(ctx context.Context, req *videopb.GetVideoInfoRequest) (*videopb.VideoResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetVideoInfo(ctx, req)
}

// GetFollowVideos 获取关注用户的视频列表
func (c *VideoServiceClient) GetFollowVideos(ctx context.Context, req *videopb.GetFollowVideosRequest) (*videopb.GetFollowVideosResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetFollowVideos(ctx, req)
}

// GetHotVideos 获取热门视频列表
func (c *VideoServiceClient) GetHotVideos(ctx context.Context, req *videopb.GetHotVideosRequest) (*videopb.GetHotVideosResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetHotVideos(ctx, req)
}

// DeleteVideo 删除视频
func (c *VideoServiceClient) DeleteVideo(ctx context.Context, req *videopb.DeleteVideoRequest) (*videopb.DeleteVideoResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.DeleteVideo(ctx, req)
}

// GetRecommendedVideos 获取推荐视频列表
func (c *VideoServiceClient) GetRecommendedVideos(ctx context.Context, req *videopb.GetRecommendVideosRequest) (*videopb.GetRecommendVideosResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetRecommendVideos(ctx, req)
}

// PublishVideo 发布视频
func (c *VideoServiceClient) PublishVideo(ctx context.Context, req *videopb.PublishVideoRequest) (*videopb.PublishVideoResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.PublishVideo(ctx, req)
}

// RetryTranscode 重试视频转码
func (c *VideoServiceClient) RetryTranscode(ctx context.Context, req *videopb.RetryTranscodeRequest) (*videopb.RetryTranscodeResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.RetryTranscode(ctx, req)
}

// GetUserVideos 获取用户发布的视频列表
func (c *VideoServiceClient) GetUserVideos(ctx context.Context, req *videopb.GetUserVideosRequest) (*videopb.GetUserVideosResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetUserVideos(ctx, req)
}

// LikeVideo 点赞/取消点赞视频
func (c *VideoServiceClient) LikeVideo(ctx context.Context, req *videopb.LikeVideoRequest) (*videopb.LikeVideoResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.LikeVideo(ctx, req)
}

// FavoriteVideo 收藏/取消收藏视频
func (c *VideoServiceClient) FavoriteVideo(ctx context.Context, req *videopb.FavoriteVideoRequest) (*videopb.FavoriteVideoResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.FavoriteVideo(ctx, req)
}

// ShareVideo 分享视频
func (c *VideoServiceClient) ShareVideo(ctx context.Context, req *videopb.ShareVideoRequest) (*videopb.ShareVideoResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.ShareVideo(ctx, req)
}

// CommentVideo 发表评论
func (c *VideoServiceClient) CommentVideo(ctx context.Context, req *videopb.CommentRequest) (*videopb.CommentResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.CommentVideo(ctx, req)
}

// DeleteComment 删除评论
func (c *VideoServiceClient) DeleteComment(ctx context.Context, req *videopb.DeleteCommentRequest) (*videopb.DeleteCommentResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.DeleteComment(ctx, req)
}

// GetVideoComments 获取视频评论列表
func (c *VideoServiceClient) GetVideoComments(ctx context.Context, req *videopb.GetVideoCommentsRequest) (*videopb.GetVideoCommentsResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("connection not ready")
	}
	return c.client.GetVideoComments(ctx, req)
}
