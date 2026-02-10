package handler

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"

	// auditpb "github.com/vision_world/video_service/github.com/vision_world/audit_service/proto/proto_gen/audit/v1"
	"github.com/vision_world/video_service/internal/config"
	"github.com/vision_world/video_service/internal/discovery"
	"github.com/vision_world/video_service/internal/model"
	"github.com/vision_world/video_service/internal/queue"
	"github.com/vision_world/video_service/internal/service"
	"github.com/vision_world/video_service/pkg/logger"
	"github.com/vision_world/video_service/pkg/minio"
	auditpb "github.com/vision_world/video_service/proto/proto_gen/audit"

	//"github.com/vision_world/video_service/proto/proto_gen/user"
	userpb "github.com/vision_world/video_service/proto/proto_gen/user"
	pb "github.com/vision_world/video_service/proto/proto_gen/video"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// VideoHandler 视频服务处理器
type VideoHandler struct {
	pb.UnimplementedVideoServiceServer
	config          *config.Config
	videoService    *service.VideoService
	commentService  *service.CommentService
	auditClient     auditpb.AuditServiceClient
	auditConn       *grpc.ClientConn
	discoveryClient discovery.ServiceDiscovery
	serviceID       string
	logger          logger.Logger
	db              *gorm.DB
	userClient      userpb.UserServiceClient
	userConn        *grpc.ClientConn
	// 添加互斥锁保护audit连接
	auditMutex sync.RWMutex
}

// NewVideoHandler 创建视频处理器
func NewVideoHandler(cfg *config.Config, log logger.Logger, db *gorm.DB, redis *redis.Client) (*VideoHandler, error) {
	// 创建MinIO客户端
	minioClient, err := minio.NewClient(minio.Config{
		Endpoint:        cfg.MinIO.Endpoint,
		AccessKeyID:     cfg.MinIO.AccessKeyID,
		SecretAccessKey: cfg.MinIO.SecretAccessKey,
		UseSSL:          cfg.MinIO.UseSSL,
		BucketName:      cfg.MinIO.BucketName,
		Location:        cfg.MinIO.Location,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	// 创建RabbitMQ客户端
	queueClient, err := queue.NewRabbitMQClient(cfg, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create RabbitMQ client: %w", err)
	}

	videoService, err := service.NewVideoService(cfg, db, redis, minioClient, queueClient, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create video service: %w", err)
	}

	// 创建model.DB实例
	modelDB := model.NewDB(db)
	// 创建服务发现客户端 - 参考api_gateway的实现方式
	var discoveryClient discovery.ServiceDiscovery
	if cfg.Discovery.Type != "" && cfg.Discovery.Type == "etcd" && cfg.Discovery.Address != "" {
		// 解析etcd端点
		etcdEndpoints := strings.Split(cfg.Discovery.Address, ",")
		for i, endpoint := range etcdEndpoints {
			etcdEndpoints[i] = strings.TrimSpace(endpoint)
		}

		// 使用与api_gateway相同的方式创建etcd服务发现
		etcdDiscovery, err := discovery.NewEtcdServiceDiscovery(etcdEndpoints, "audit-service")
		if err != nil {
			log.Error("Failed to create etcd service discovery client", "error", err)
			// 服务发现创建失败不影响服务启动，只记录错误
		} else {
			discoveryClient = etcdDiscovery
			log.Info("Etcd service discovery client created",
				"type", cfg.Discovery.Type,
				"endpoints", etcdEndpoints)
		}
	} else if cfg.Discovery.Type != "" {
		// 对于其他类型的服务发现，使用原有方式
		discoveryClient, err = discovery.NewServiceDiscovery(&cfg.Discovery, cfg.Server.Name)
		if err != nil {
			log.Error("Failed to create service discovery client", "error", err)
			// 服务发现创建失败不影响服务启动，只记录错误
		} else {
			log.Info("Service discovery client created",
				"type", cfg.Discovery.Type)
		}
	}

	// 创建audit_service客户端连接 - 使用服务发现机制
	var auditClient auditpb.AuditServiceClient
	var auditConn *grpc.ClientConn

	// 如果配置了静态地址且没有服务发现，使用静态地址
	if cfg.Services.AuditService.Address != "" && discoveryClient == nil {
		// 使用较短的超时时间进行初始连接尝试
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

		conn, err := grpc.DialContext(ctx, cfg.Services.AuditService.Address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		cancel()

		if err != nil {
			// 连接失败，记录警告但不阻止服务启动
			log.Warn("Failed to connect to audit service on startup, will retry later",
				"address", cfg.Services.AuditService.Address,
				"error", err)

			// 设置为nil，后续可以通过重连机制建立连接
			auditClient = nil
			auditConn = nil
		} else {
			// 连接成功
			auditClient = auditpb.NewAuditServiceClient(conn)
			auditConn = conn

			log.Info("Connected to audit service",
				"address", cfg.Services.AuditService.Address)
		}
	} else if discoveryClient != nil {
		// 使用服务发现机制连接audit服务
		log.Info("Audit service will be discovered via service discovery")
		// auditClient和auditConn将在需要时通过服务发现获取
	} else {
		log.Info("Audit service not configured, audit features will be disabled")
	}

	// 生成服务ID
	host, port, err := getHostAndPort(cfg.Server.Address)
	if err != nil {
		log.Error("Failed to parse server address", "error", err)
		host = "localhost"
		port = 50052 // 默认端口
	}

	serviceID := fmt.Sprintf("%s-%s-%d", cfg.Server.Name, host, port)

	// 创建user_service客户端连接
	var userClient userpb.UserServiceClient
	var userConn *grpc.ClientConn
	userConn, err = grpc.Dial(cfg.Services.UserService.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Warn("Failed to connect to user service, token verification will fail",
			"address", cfg.Services.UserService.Address,
			"error", err)
		userClient = nil
		userConn = nil
	} else {
		userClient = userpb.NewUserServiceClient(userConn)
		log.Info("Connected to user service",
			"address", cfg.Services.UserService.Address)
	}
	// 创建CommentService实例
	commentService := service.NewCommentService(modelDB, userClient)
	// 更新CommentService的userClient
	//commentService.SetUserClient(userClient)

	return &VideoHandler{
		config:          cfg,
		videoService:    videoService,
		commentService:  commentService,
		auditClient:     auditClient,
		auditConn:       auditConn,
		discoveryClient: discoveryClient,
		serviceID:       serviceID,
		logger:          log,
		userClient:      userClient,
		userConn:        userConn,
	}, nil
}

// getHostAndPort 从地址中解析主机和端口
func getHostAndPort(address string) (string, int, error) {
	// 移除地址前缀
	address = strings.TrimPrefix(address, ":")

	// 如果地址只包含端口，使用localhost作为主机
	if !strings.Contains(address, ":") {
		port, err := strconv.Atoi(address)
		if err != nil {
			return "", 0, fmt.Errorf("invalid port: %s", address)
		}
		return "localhost", port, nil
	}

	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, fmt.Errorf("failed to split host and port: %w", err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port: %s", portStr)
	}

	return host, port, nil
}

// getAuditClient 获取audit服务客户端，如果未连接则通过服务发现建立连接
func (h *VideoHandler) getAuditClient(ctx context.Context) (auditpb.AuditServiceClient, error) {
	// 使用读锁检查是否已有连接
	h.auditMutex.RLock()
	if h.auditClient != nil {
		client := h.auditClient
		h.auditMutex.RUnlock()
		return client, nil
	}
	h.auditMutex.RUnlock()

	// 使用写锁建立新连接
	h.auditMutex.Lock()
	defer h.auditMutex.Unlock()

	// 双重检查，防止在获取写锁期间其他goroutine已经建立了连接
	if h.auditClient != nil {
		return h.auditClient, nil
	}

	// 如果没有服务发现客户端，返回错误
	if h.discoveryClient == nil {
		return nil, fmt.Errorf("service discovery not available for audit service")
	}

	// 通过服务发现获取audit服务实例
	instances, err := h.discoveryClient.Discover(ctx, "audit-service")
	if err != nil {
		return nil, fmt.Errorf("failed to discover audit service: %w", err)
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no audit service instances found")
	}

	// 选择第一个可用的实例
	instance := instances[0]
	address := fmt.Sprintf("%s:%d", instance.Host, instance.Port)

	// 建立连接
	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to audit service at %s: %w", address, err)
	}

	// 创建客户端并保存连接
	h.auditClient = auditpb.NewAuditServiceClient(conn)
	h.auditConn = conn

	h.logger.Info("Connected to audit service via service discovery",
		"address", address,
		"instance_id", instance.ID)

	return h.auditClient, nil
}

// RegisterService 注册服务到服务发现
func (h *VideoHandler) RegisterService() error {
	if h.discoveryClient == nil {
		h.logger.Info("Service discovery not configured, skipping registration")
		return nil
	}

	// 解析服务器地址
	host, port, err := getHostAndPort(h.config.Server.Address)
	if err != nil {
		return fmt.Errorf("failed to parse server address: %w", err)
	}

	// 构建服务信息
	serviceInfo := &discovery.ServiceInfo{
		ID:   h.serviceID,
		Name: h.config.Server.Name,
		Host: host,
		Port: port,
		Tags: []string{"video", "grpc", h.config.Server.Environment},
		Meta: map[string]string{
			"version":     h.config.Server.Version,
			"environment": h.config.Server.Environment,
		},
		Check: &discovery.HealthCheck{
			GRPC:                           fmt.Sprintf("%s:%d", host, port),
			Interval:                       fmt.Sprintf("%ds", h.config.Discovery.Interval),
			Timeout:                        "5s",
			DeregisterCriticalServiceAfter: "30s",
		},
	}

	// 注册服务
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.discoveryClient.Register(ctx, serviceInfo); err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	h.logger.Info("Service registered successfully",
		"service_id", h.serviceID,
		"service_name", h.config.Server.Name,
		"address", fmt.Sprintf("%s:%d", host, port))

	return nil
}

// Close 关闭处理器
func (h *VideoHandler) Close() error {
	var errs []error

	// 注销服务
	if h.discoveryClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := h.discoveryClient.Deregister(ctx, h.serviceID); err != nil {
			h.logger.Error("Failed to deregister service", "error", err)
			errs = append(errs, err)
		} else {
			h.logger.Info("Service deregistered successfully", "service_id", h.serviceID)
		}

		if err := h.discoveryClient.Close(); err != nil {
			h.logger.Error("Failed to close discovery client", "error", err)
			errs = append(errs, err)
		}
	}

	// 关闭audit_service连接
	h.auditMutex.Lock()
	if h.auditConn != nil {
		if err := h.auditConn.Close(); err != nil {
			h.logger.Error("Failed to close audit service connection", "error", err)
			errs = append(errs, err)
		}
		h.auditConn = nil
		h.auditClient = nil
	}
	h.auditMutex.Unlock()

	// 关闭video_service连接
	if h.videoService != nil {
		if err := h.videoService.Close(); err != nil {
			h.logger.Error("Failed to close video service", "error", err)
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiple errors occurred during close: %v", errs)
	}

	return nil
}

// reconnectAuditService 重连audit服务
func (h *VideoHandler) reconnectAuditService(ctx context.Context) error {
	h.auditMutex.Lock()
	defer h.auditMutex.Unlock()

	// 如果已经存在连接，先关闭它
	if h.auditConn != nil {
		h.auditConn.Close()
		h.auditConn = nil
		h.auditClient = nil
	}

	// 优先使用服务发现
	if h.discoveryClient != nil {
		// 通过服务发现获取audit服务实例
		instances, err := h.discoveryClient.Discover(ctx, "audit-service")
		if err != nil {
			return fmt.Errorf("failed to discover audit service: %w", err)
		}

		if len(instances) == 0 {
			return fmt.Errorf("no audit service instances found")
		}

		// 选择第一个可用的实例
		instance := instances[0]
		address := fmt.Sprintf("%s:%d", instance.Host, instance.Port)

		// 建立连接
		conn, err := grpc.DialContext(ctx, address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			return fmt.Errorf("failed to connect to audit service at %s: %w", address, err)
		}

		// 创建客户端并保存连接
		h.auditClient = auditpb.NewAuditServiceClient(conn)
		h.auditConn = conn

		h.logger.Info("Successfully reconnected to audit service via service discovery",
			"address", address,
			"instance_id", instance.ID)

		return nil
	}

	// 如果没有服务发现，使用静态配置
	if h.config.Services.AuditService.Address == "" {
		return fmt.Errorf("audit service address not configured and service discovery not available")
	}

	// 使用配置的超时时间进行连接
	timeout := time.Duration(h.config.Services.AuditService.Timeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Second // 默认5秒超时
	}

	connCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := grpc.DialContext(connCtx, h.config.Services.AuditService.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)

	if err != nil {
		h.logger.Warn("Failed to reconnect to audit service",
			"address", h.config.Services.AuditService.Address,
			"error", err)
		return err
	}

	// 连接成功，更新连接和客户端
	h.auditConn = conn
	h.auditClient = auditpb.NewAuditServiceClient(conn)

	h.logger.Info("Successfully reconnected to audit service",
		"address", h.config.Services.AuditService.Address)

	return nil
}

// isAuditServiceAvailable 检查audit服务是否可用
func (h *VideoHandler) isAuditServiceAvailable() bool {
	h.auditMutex.RLock()
	defer h.auditMutex.RUnlock()
	return h.auditClient != nil
}

// verifyTokenAndGetUserID 验证用户token并返回用户ID
func (h *VideoHandler) verifyTokenAndGetUserID(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", fmt.Errorf("token is empty")
	}

	if h.userClient == nil {
		h.logger.Warn("User service client not available, using default user ID")
		return "1", nil
	}

	resp, err := h.userClient.VerifyToken(ctx, &userpb.VerifyTokenRequest{Token: token})
	if err != nil {
		h.logger.Error("Failed to verify token", "error", err)
		return "", fmt.Errorf("failed to verify token: %w", err)
	}

	if resp.StatusCode != 0 {
		h.logger.Error("Token verification failed", "status_code", resp.StatusCode, "status_msg", resp.StatusMsg)
		return "", fmt.Errorf("token verification failed: %s", resp.StatusMsg)
	}

	return strconv.FormatUint(uint64(resp.UserId), 10), nil
}

// ==================== 视频发布相关接口 ====================

// UploadVideo 上传视频
func (h *VideoHandler) UploadVideo(ctx context.Context, req *pb.UploadVideoRequest) (*pb.UploadVideoResponse, error) {
	h.logger.Info("UploadVideo called", "token", req.Token, "file_name", req.FileName)

	// 验证用户token并获取用户ID
	userID, err := h.verifyTokenAndGetUserID(ctx, req.Token)
	if err != nil {
		h.logger.Error("Failed to verify token", "error", err)
		return &pb.UploadVideoResponse{
			StatusCode: 401,
			StatusMsg:  "用户认证失败: " + err.Error(),
			VideoId:    0,
		}, nil
	}

	h.logger.Info("User authenticated successfully", "user_id", userID)

	h.logger.Info("Calling videoService.UploadVideo",
		zap.String("user_id", userID),
		zap.String("file_name", req.FileName),
		zap.String("title", req.Title),
		zap.String("description", req.Description),
		zap.String("category", req.Category),
		zap.Strings("tags", req.Tags),
		zap.Int("video_data_size", len(req.VideoData)))

	// 调用service层的UploadVideo方法
	videoID, videoURL, video, err := h.videoService.UploadVideo(ctx, userID, req.FileName, req.Title, req.Description, req.Category, req.Tags, req.VideoData)
	if err != nil {
		h.logger.Error("Failed to upload video",
			zap.Error(err),
			zap.String("user_id", userID),
			zap.String("file_name", req.FileName),
			zap.String("title", req.Title))
		return &pb.UploadVideoResponse{
			StatusCode: 500,
			StatusMsg:  "视频上传失败: " + err.Error(),
			VideoId:    0,
		}, nil
	}
	h.logger.Info("video", zap.Any("video", video), zap.String("videoUrl", videoURL))
	h.logger.Info("Video uploaded successfully",
		zap.Uint32("video_id", videoID),
		zap.String("video_url", videoURL))

	// 视频上传成功，状态为uploading
	statusCode := int32(0)
	statusMsg := "视频上传成功"

	return &pb.UploadVideoResponse{
		StatusCode: statusCode,
		StatusMsg:  statusMsg,
		VideoId:    videoID,
		VideoUrl:   videoURL,
	}, nil
}

// CreateVideoRecord 创建视频记录（用于分片上传完成后）
func (h *VideoHandler) CreateVideoRecord(ctx context.Context, req *pb.CreateVideoRecordRequest) (*pb.CreateVideoRecordResponse, error) {
	h.logger.Info("CreateVideoRecord called",
		zap.String("token", req.Token),
		zap.String("file_name", req.FileName),
		zap.String("video_url", req.VideoUrl),
		zap.String("uuid", req.Uuid))

	// 验证用户token并获取用户ID
	userID, err := h.verifyTokenAndGetUserID(ctx, req.Token)
	if err != nil {
		h.logger.Error("Failed to verify token", zap.Error(err))
		return &pb.CreateVideoRecordResponse{
			StatusCode: 401,
			StatusMsg:  "用户认证失败: " + err.Error(),
			VideoId:    0,
		}, nil
	}

	h.logger.Info("User authenticated successfully", zap.String("user_id", userID))

	videoID, videoURL, err := h.videoService.CreateVideoRecord(
		ctx,
		userID,
		req.FileName,
		req.Title,
		req.Description,
		req.Category,
		req.Tags,
		req.VideoUrl,
		req.Uuid,
	)
	if err != nil {
		h.logger.Error("Failed to create video record", zap.Error(err))
		return &pb.CreateVideoRecordResponse{
			StatusCode: 500,
			StatusMsg:  "创建视频记录失败",
			VideoId:    0,
		}, nil
	}

	h.logger.Info("Video record created successfully",
		zap.Uint32("video_id", videoID),
		zap.String("video_url", videoURL))

	return &pb.CreateVideoRecordResponse{
		StatusCode: 0,
		StatusMsg:  "视频记录创建成功",
		VideoId:    videoID,
		VideoUrl:   videoURL,
	}, nil
}

// PublishVideo 发布视频
func (h *VideoHandler) PublishVideo(ctx context.Context, req *pb.PublishVideoRequest) (*pb.PublishVideoResponse, error) {
	h.logger.Info("PublishVideo called",
		zap.String("title", req.Title),
		zap.String("token", req.Token),
		zap.String("video_id", req.VideoId))

	// 验证用户token并获取用户ID
	userID, err := h.verifyTokenAndGetUserID(ctx, req.Token)
	if err != nil {
		h.logger.Error("Failed to verify token", zap.Error(err))
		return &pb.PublishVideoResponse{
			StatusCode: 401,
			StatusMsg:  "用户认证失败: " + err.Error(),
			VideoId:    0,
		}, nil
	}

	h.logger.Info("User authenticated successfully", zap.String("user_id", userID))

	// 直接使用前端传来的video_id（UUID格式）
	videoID := req.VideoId
	if videoID == "" {
		h.logger.Error("Video ID is empty")
		return &pb.PublishVideoResponse{
			StatusCode: 400,
			StatusMsg:  "视频ID不能为空",
			VideoId:    0,
		}, nil
	}

	// 调用service层的发布方法
	err = h.videoService.PublishVideo(ctx, userID, videoID, req.Title, req.Description)
	if err != nil {
		h.logger.Error("Failed to publish video",
			"video_id", videoID,
			"error", err)
		return &pb.PublishVideoResponse{
			StatusCode: 500,
			StatusMsg:  fmt.Sprintf("发布失败: %s", err.Error()),
			VideoId:    0,
		}, nil
	}

	// 发布成功，视频进入审核中状态
	statusCode := int32(0)
	statusMsg := "视频发布成功，正在审核中"

	return &pb.PublishVideoResponse{
		StatusCode: statusCode,
		StatusMsg:  statusMsg,
		VideoId:    0, // UUID格式无法返回数字ID
	}, nil
}

// DeleteVideo 删除视频
func (h *VideoHandler) DeleteVideo(ctx context.Context, req *pb.DeleteVideoRequest) (*pb.DeleteVideoResponse, error) {
	h.logger.Info("DeleteVideo called", zap.Uint32("video_id", req.VideoId))

	// TODO: 验证用户token和权限
	// TODO: 实现视频删除逻辑

	return &pb.DeleteVideoResponse{
		StatusCode: 0,
		StatusMsg:  "success",
	}, nil
}

// RetryTranscode 重试转码
func (h *VideoHandler) RetryTranscode(ctx context.Context, req *pb.RetryTranscodeRequest) (*pb.RetryTranscodeResponse, error) {
	h.logger.Info("RetryTranscode called", zap.Uint32("video_id", req.VideoId))

	// TODO: 验证用户token和权限

	// 调用service层的重试转码方法
	err := h.videoService.RetryTranscode(ctx, req.VideoId)
	if err != nil {
		h.logger.Error("Failed to retry transcode", zap.Uint32("video_id", req.VideoId), zap.Error(err))
		return &pb.RetryTranscodeResponse{
			StatusCode: 500,
			StatusMsg:  fmt.Sprintf("转码失败: %s", err.Error()),
		}, nil
	}

	return &pb.RetryTranscodeResponse{
		StatusCode: 0,
		StatusMsg:  "转码任务已提交",
	}, nil
}

// ==================== 分片上传接口 ====================

// InitChunkUpload 初始化分片上传
// func (h *VideoHandler) InitChunkUpload(ctx context.Context, req *pb.InitChunkUploadRequest) (*pb.InitChunkUploadResponse, error) {
// 	h.logger.Info("InitChunkUpload called",
// 		zap.String("file_name", req.FileName),
// 		zap.Int64("file_size", req.FileSize))

// 	// 验证用户token并获取用户ID
// 	userID, err := h.verifyTokenAndGetUserID(ctx, req.Token)
// 	if err != nil {
// 		h.logger.Error("Failed to verify token", zap.Error(err))
// 		return &pb.InitChunkUploadResponse{
// 			StatusCode: 401,
// 			StatusMsg:  "用户认证失败: " + err.Error(),
// 		}, nil
// 	}

// 	h.logger.Info("User authenticated successfully", zap.String("user_id", userID))

// 	// 调用service层初始化分片上传
// 	uploadID, objectName, chunkSize, totalChunks, err := h.videoService.InitChunkUpload(ctx, userID, req.FileName, req.FileSize, req.Title, req.Description, req.Category, req.Tags)
// 	if err != nil {
// 		h.logger.Error("Failed to init chunk upload", zap.Error(err))
// 		return &pb.InitChunkUploadResponse{
// 			StatusCode: 500,
// 			StatusMsg:  "初始化分片上传失败: " + err.Error(),
// 		}, nil
// 	}

// 	h.logger.Info("Chunk upload initialized successfully",
// 		zap.String("upload_id", uploadID),
// 		zap.String("object_name", objectName),
// 		zap.Int("chunk_size", chunkSize),
// 		zap.Int("total_chunks", totalChunks))

// 	return &pb.InitChunkUploadResponse{
// 		StatusCode:  0,
// 		StatusMsg:   "初始化成功",
// 		UploadId:    uploadID,
// 		ObjectName:  objectName,
// 		ChunkSize:   int32(chunkSize),
// 		TotalChunks: int32(totalChunks),
// 	}, nil
// }

// UploadChunk 上传分片
func (h *VideoHandler) UploadChunk(ctx context.Context, req *pb.UploadChunkRequest) (*pb.UploadChunkResponse, error) {
	h.logger.Info("UploadChunk called",
		zap.String("upload_id", req.UploadId),
		zap.Int32("chunk_number", req.ChunkNumber))

	// 验证用户token
	_, err := h.verifyTokenAndGetUserID(ctx, req.Token)
	if err != nil {
		h.logger.Error("Failed to verify token", zap.Error(err))
		return &pb.UploadChunkResponse{
			StatusCode: 401,
			StatusMsg:  "用户认证失败: " + err.Error(),
		}, nil
	}

	// 调用service层上传分片
	etag, err := h.videoService.UploadChunk(ctx, req.UploadId, req.ObjectName, int(req.ChunkNumber), req.ChunkData)
	if err != nil {
		h.logger.Error("Failed to upload chunk", zap.Error(err))
		return &pb.UploadChunkResponse{
			StatusCode: 500,
			StatusMsg:  "上传分片失败: " + err.Error(),
		}, nil
	}

	h.logger.Info("Chunk uploaded successfully",
		zap.Int32("chunk_number", req.ChunkNumber),
		zap.String("etag", etag))

	return &pb.UploadChunkResponse{
		StatusCode:  0,
		StatusMsg:   "分片上传成功",
		ChunkNumber: req.ChunkNumber,
		Etag:        etag,
	}, nil
}

// CompleteChunkUpload 完成分片上传
// func (h *VideoHandler) CompleteChunkUpload(ctx context.Context, req *pb.CompleteChunkUploadRequest) (*pb.CompleteChunkUploadResponse, error) {
// 	h.logger.Info("CompleteChunkUpload called",
// 		zap.String("upload_id", req.UploadId),
// 		zap.String("object_name", req.ObjectName))

// 	// 验证用户token并获取用户ID
// 	userID, err := h.verifyTokenAndGetUserID(ctx, req.Token)
// 	if err != nil {
// 		h.logger.Error("Failed to verify token", zap.Error(err))
// 		return &pb.CompleteChunkUploadResponse{
// 			StatusCode: 401,
// 			StatusMsg:  "用户认证失败: " + err.Error(),
// 		}, nil
// 	}

// 	// 转换分片列表
// 	parts := make([]miniogo.CompletePart, 0, len(req.Parts))
// 	for _, part := range req.Parts {
// 		parts = append(parts, miniogo.CompletePart{
// 			PartNumber: int(part.PartNumber),
// 			ETag:       part.Etag,
// 		})
// 	}

// 	// 调用service层完成分片上传
// 	videoID, videoURL, err := h.videoService.CompleteChunkUpload(ctx, userID, req.UploadId, req.ObjectName, parts)
// 	if err != nil {
// 		h.logger.Error("Failed to complete chunk upload", zap.Error(err))
// 		return &pb.CompleteChunkUploadResponse{
// 			StatusCode: 500,
// 			StatusMsg:  "完成分片上传失败: " + err.Error(),
// 		}, nil
// 	}

// 	h.logger.Info("Chunk upload completed successfully",
// 		zap.Uint32("video_id", videoID),
// 		zap.String("video_url", videoURL))

// 	return &pb.CompleteChunkUploadResponse{
// 		StatusCode: 0,
// 		StatusMsg:  "上传完成",
// 		VideoId:    videoID,
// 		VideoUrl:   videoURL,
// 	}, nil
// }

// AbortChunkUpload 取消分片上传
func (h *VideoHandler) AbortChunkUpload(ctx context.Context, req *pb.AbortChunkUploadRequest) (*pb.AbortChunkUploadResponse, error) {
	h.logger.Info("AbortChunkUpload called",
		zap.String("upload_id", req.UploadId),
		zap.String("object_name", req.ObjectName))

	// 验证用户token
	_, err := h.verifyTokenAndGetUserID(ctx, req.Token)
	if err != nil {
		h.logger.Error("Failed to verify token", zap.Error(err))
		return &pb.AbortChunkUploadResponse{
			StatusCode: 401,
			StatusMsg:  "用户认证失败: " + err.Error(),
		}, nil
	}

	// 调用service层取消分片上传
	err = h.videoService.AbortChunkUpload(ctx, req.UploadId, req.ObjectName)
	if err != nil {
		h.logger.Error("Failed to abort chunk upload", zap.Error(err))
		return &pb.AbortChunkUploadResponse{
			StatusCode: 500,
			StatusMsg:  "取消分片上传失败: " + err.Error(),
		}, nil
	}

	h.logger.Info("Chunk upload aborted successfully")

	return &pb.AbortChunkUploadResponse{
		StatusCode: 0,
		StatusMsg:  "取消成功",
	}, nil
}

// ==================== 视频信息获取接口 ====================

// GetVideoInfo 获取单个视频信息
func (h *VideoHandler) GetVideoInfo(ctx context.Context, req *pb.GetVideoInfoRequest) (*pb.VideoResponse, error) {
	h.logger.Info("GetVideoInfo called", zap.Uint32("video_id", req.VideoId))

	// 调用service层获取视频信息
	videoID := strconv.FormatUint(uint64(req.VideoId), 10)
	videoDetail, err := h.videoService.GetVideoDetail(ctx, videoID)
	if err != nil {
		h.logger.Error("Failed to get video info", zap.Error(err))
		return &pb.VideoResponse{
			StatusCode: 500,
			StatusMsg:  "获取视频信息失败",
		}, nil
	}

	if videoDetail == nil {
		return &pb.VideoResponse{
			StatusCode: 404,
			StatusMsg:  "视频不存在",
		}, nil
	}

	// 转换为protobuf格式
	pbVideo := h.convertToProtoVideoDetail(videoDetail)

	// 如果有token，判断用户是否点赞和收藏
	if req.Token != "" {
		// 验证token并获取用户ID
		userIDStr, err := h.verifyTokenAndGetUserID(ctx, req.Token)
		if err == nil {
			userID, err := strconv.ParseUint(userIDStr, 10, 32)
			if err == nil {
				// 判断是否点赞
				isLiked, err := h.videoService.IsVideoLiked(ctx, uint32(userID), req.VideoId)
				if err == nil {
					pbVideo.IsLiked = isLiked
				}

				// 判断是否收藏
				isFavorite, err := h.videoService.IsVideoFavorited(ctx, uint32(userID), req.VideoId)
				if err == nil {
					pbVideo.IsFavorite = isFavorite
				}
			}
		}
	}

	return &pb.VideoResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Video:      pbVideo,
	}, nil
}

// GetVideoInfos 批量获取视频信息
func (h *VideoHandler) GetVideoInfos(ctx context.Context, req *pb.GetVideoInfosRequest) (*pb.GetVideoInfosResponse, error) {
	h.logger.Info("GetVideoInfos called", zap.Int("video_count", len(req.VideoIds)))

	// 转换视频ID列表
	videoIDs := make([]string, 0, len(req.VideoIds))
	for _, videoId := range req.VideoIds {
		videoIDs = append(videoIDs, strconv.FormatUint(uint64(videoId), 10))
	}

	// 调用service层获取视频信息
	videos, err := h.videoService.GetVideosByIDs(ctx, videoIDs)
	if err != nil {
		h.logger.Error("Failed to get videos info", zap.Error(err))
		return &pb.GetVideoInfosResponse{
			StatusCode: 500,
			StatusMsg:  "获取视频信息失败",
		}, nil
	}

	// 转换为protobuf格式
	pbVideos := make([]*pb.Video, 0, len(videos))
	for _, video := range videos {
		pbVideos = append(pbVideos, h.convertToProtoVideo(video))
	}

	return &pb.GetVideoInfosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     pbVideos,
	}, nil
}

// ==================== 视频列表相关接口 ====================

// GetUserVideos 获取用户发布的视频列表
func (h *VideoHandler) GetUserVideos(ctx context.Context, req *pb.GetUserVideosRequest) (*pb.GetUserVideosResponse, error) {
	h.logger.Info("GetUserVideos called", zap.Uint32("user_id", req.UserId), zap.Uint32("page", req.Page), zap.Uint32("page_size", req.PageSize))

	// 调用服务层获取用户视频
	videos, hasMore, err := h.videoService.GetVideosByAuthor(ctx, strconv.FormatUint(uint64(req.UserId), 10), int(req.Page), int(req.PageSize))
	if err != nil {
		h.logger.Error("Failed to get user videos", zap.Error(err))
		return &pb.GetUserVideosResponse{
			StatusCode: 500,
			StatusMsg:  "获取视频列表失败",
		}, nil
	}

	// 转换为protobuf格式
	pbVideos := make([]*pb.Video, 0, len(videos))
	for _, video := range videos {
		// 将标签字符串转换为数组
		var tags []string
		if video.Tags != "" {
			tags = strings.Split(video.Tags, ",")
		}

		// 解析视频ID
		videoID, err := strconv.ParseUint(video.VideoID, 10, 32)
		if err != nil {
			h.logger.Warn("Invalid video ID format", zap.String("video_id", video.VideoID), zap.Error(err))
			videoID = 0
		}

		pbVideo := &pb.Video{
			Id:            uint32(videoID),
			Title:         video.Title,
			Description:   video.Description,
			CoverUrl:      video.CoverURL,
			VideoUrl:      video.PlayURL,
			PlayCount:     uint32(video.ViewCount),
			LikeCount:     uint32(video.LikeCount),
			CommentCount:  0, // TODO: 从数据库获取真实评论数
			ShareCount:    0, // TODO: 从数据库获取真实分享数
			FavoriteCount: 0, // TODO: 从数据库获取真实收藏数
			Tags:          tags,
			Category:      video.Category,
			CreateTime:    video.CreatedAt.Unix(),
			UpdateTime:    video.UpdatedAt.Unix(),
			Duration:      uint32(video.Duration), // 转换为uint32类型
			Resolution:    "1080p",                // TODO: 从数据库获取真实分辨率
			IsPublic:      true,
			Status:        "normal",
			IsLiked:       false, // TODO: 根据用户token判断是否点赞
			IsFavorite:    false, // TODO: 根据用户token判断是否收藏
		}

		pbVideos = append(pbVideos, pbVideo)
	}

	// TODO: 实现真实总数计算，暂时使用当前列表长度
	total := uint32(len(pbVideos))

	return &pb.GetUserVideosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     pbVideos,
		Total:      total,
		HasMore:    hasMore,
	}, nil
}

// GetRecommendVideos 获取推荐视频列表
func (h *VideoHandler) GetRecommendVideos(ctx context.Context, req *pb.GetRecommendVideosRequest) (*pb.GetRecommendVideosResponse, error) {
	category := ""
	if req.Category != nil {
		category = *req.Category
	}
	h.logger.Info("GetRecommendVideos called", zap.Uint32("page", req.Page), zap.String("category", category))

	// 调用service层获取热门视频作为推荐视频
	page := int(req.Page)
	pageSize := int(req.PageSize)

	videos, hasMore, err := h.videoService.GetHotVideos(ctx, page, pageSize, category)
	if err != nil {
		h.logger.Error("Failed to get recommend videos", zap.Error(err))
		return &pb.GetRecommendVideosResponse{
			StatusCode: 500,
			StatusMsg:  "获取推荐视频失败",
		}, nil
	}

	// 转换为protobuf格式
	pbVideos := make([]*pb.Video, 0, len(videos))
	for _, video := range videos {
		pbVideos = append(pbVideos, h.convertToProtoVideo(video))
	}

	return &pb.GetRecommendVideosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     pbVideos,
		HasMore:    hasMore,
	}, nil
}

// GetFollowVideos 获取关注用户的视频列表
func (h *VideoHandler) GetFollowVideos(ctx context.Context, req *pb.GetFollowVideosRequest) (*pb.GetFollowVideosResponse, error) {
	h.logger.Info("GetFollowVideos called", zap.Uint32("page", req.Page))

	// TODO: 验证用户token
	// TODO: 实现获取关注用户视频逻辑

	videos := make([]*pb.Video, 0)
	for i := uint32(0); i < req.PageSize; i++ {
		videos = append(videos, &pb.Video{
			Id:         uint32(i + 1),
			Title:      "TODO: Followed User Video Title",
			CoverUrl:   "TODO: Cover URL",
			VideoUrl:   "TODO: Video URL",
			PlayCount:  200,
			LikeCount:  100,
			CreateTime: time.Now().Unix(),
		})
	}

	return &pb.GetFollowVideosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     videos,
		HasMore:    true,
	}, nil
}

// GetHotVideos 获取热门视频列表
func (h *VideoHandler) GetHotVideos(ctx context.Context, req *pb.GetHotVideosRequest) (*pb.GetHotVideosResponse, error) {
	category := ""
	if req.Category != nil {
		category = *req.Category
	}
	h.logger.Info("GetHotVideos called", zap.Uint32("page", req.Page), zap.String("category", category))

	// 调用service层获取热门视频
	page := int(req.Page)
	pageSize := int(req.PageSize)

	videos, hasMore, err := h.videoService.GetHotVideos(ctx, page, pageSize, category)
	if err != nil {
		h.logger.Error("Failed to get hot videos", zap.Error(err))
		return &pb.GetHotVideosResponse{
			StatusCode: 500,
			StatusMsg:  "获取热门视频失败",
		}, nil
	}

	// 转换为protobuf格式
	pbVideos := make([]*pb.Video, 0, len(videos))
	for _, video := range videos {
		pbVideos = append(pbVideos, h.convertToProtoVideo(video))
	}

	return &pb.GetHotVideosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     pbVideos,
		HasMore:    hasMore,
	}, nil
}

// GetCategoryVideos 获取分类视频列表
func (h *VideoHandler) GetCategoryVideos(ctx context.Context, req *pb.GetCategoryVideosRequest) (*pb.GetCategoryVideosResponse, error) {
	h.logger.Info("GetCategoryVideos called", zap.String("category", req.Category), zap.Uint32("page", req.Page))

	// 调用service层获取分类视频
	page := int(req.Page)
	pageSize := int(req.PageSize)

	videos, hasMore, err := h.videoService.GetCategoryVideos(ctx, req.Category, page, pageSize)
	if err != nil {
		h.logger.Error("Failed to get category videos", zap.Error(err))
		return &pb.GetCategoryVideosResponse{
			StatusCode: 500,
			StatusMsg:  "获取分类视频失败",
		}, nil
	}

	// 转换为protobuf格式
	pbVideos := make([]*pb.Video, 0, len(videos))
	for _, video := range videos {
		pbVideos = append(pbVideos, h.convertToProtoVideo(video))
	}

	return &pb.GetCategoryVideosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     pbVideos,
		HasMore:    hasMore,
	}, nil
}

// SearchVideos 搜索视频
func (h *VideoHandler) SearchVideos(ctx context.Context, req *pb.SearchVideosRequest) (*pb.SearchVideosResponse, error) {
	category := ""
	if req.Category != nil {
		category = *req.Category
	}
	h.logger.Info("SearchVideos called", zap.String("keyword", req.Keyword), zap.String("category", category), zap.Uint32("page", req.Page))

	// 调用service层搜索视频
	page := int(req.Page)
	pageSize := int(req.PageSize)

	videos, hasMore, err := h.videoService.SearchVideos(ctx, req.Keyword, page, pageSize, category)
	if err != nil {
		h.logger.Error("Failed to search videos", zap.Error(err))
		return &pb.SearchVideosResponse{
			StatusCode: 500,
			StatusMsg:  "搜索视频失败",
		}, nil
	}

	// 转换为protobuf格式
	pbVideos := make([]*pb.Video, 0, len(videos))
	for _, video := range videos {
		pbVideos = append(pbVideos, h.convertToProtoVideo(video))
	}

	return &pb.SearchVideosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     pbVideos,
		HasMore:    hasMore,
	}, nil
}

// ==================== 视频互动相关接口 ====================

// LikeVideo 点赞/取消点赞视频
func (h *VideoHandler) LikeVideo(ctx context.Context, req *pb.LikeVideoRequest) (*pb.LikeVideoResponse, error) {
	actionType := "like"
	if !req.ActionType {
		actionType = "unlike"
	}
	h.logger.Info("LikeVideo called", zap.Uint32("video_id", req.VideoId), zap.String("action_type", actionType))

	// 验证用户token并获取用户ID
	userIDStr, err := h.verifyTokenAndGetUserID(ctx, req.Token)
	if err != nil {
		h.logger.Error("Failed to verify token", zap.Error(err))
		return &pb.LikeVideoResponse{
			StatusCode: 401,
			StatusMsg:  "用户认证失败",
			LikeCount:  0,
		}, nil
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		h.logger.Error("Failed to parse user ID", zap.Error(err))
		return &pb.LikeVideoResponse{
			StatusCode: 500,
			StatusMsg:  "用户ID解析失败",
			LikeCount:  0,
		}, nil
	}

	// 调用service层处理点赞逻辑
	likeCount, err := h.videoService.LikeVideo(ctx, uint32(userID), req.VideoId, req.ActionType)
	if err != nil {
		h.logger.Error("Failed to like video", zap.Error(err))
		return &pb.LikeVideoResponse{
			StatusCode: 500,
			StatusMsg:  "点赞失败",
			LikeCount:  0,
		}, nil
	}

	return &pb.LikeVideoResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		LikeCount:  uint32(likeCount),
	}, nil
}

// GetUserLikedVideos 获取用户点赞的视频列表
func (h *VideoHandler) GetUserLikedVideos(ctx context.Context, req *pb.GetUserLikedVideosRequest) (*pb.GetUserLikedVideosResponse, error) {
	h.logger.Info("GetUserLikedVideos called", zap.Uint32("user_id", req.UserId), zap.Uint32("page", req.Page))

	// TODO: 实现获取用户点赞视频逻辑

	videos := make([]*pb.Video, 0)
	for i := uint32(0); i < req.PageSize; i++ {
		videos = append(videos, &pb.Video{
			Id:         uint32(i + 1),
			Title:      "TODO: Liked Video Title",
			CoverUrl:   "TODO: Cover URL",
			VideoUrl:   "TODO: Video URL",
			PlayCount:  300,
			LikeCount:  200,
			CreateTime: time.Now().Unix(),
		})
	}

	return &pb.GetUserLikedVideosResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Videos:     videos,
		Total:      50, // TODO: 真实的总数
		HasMore:    true,
	}, nil
}

// ShareVideo 分享视频
func (h *VideoHandler) ShareVideo(ctx context.Context, req *pb.ShareVideoRequest) (*pb.ShareVideoResponse, error) {
	h.logger.Info("ShareVideo called", zap.Uint32("video_id", req.VideoId), zap.String("share_type", req.ShareType))

	// 验证用户token并获取用户ID
	userIDStr, err := h.verifyTokenAndGetUserID(ctx, req.Token)
	if err != nil {
		h.logger.Error("Failed to verify token", zap.Error(err))
		return &pb.ShareVideoResponse{
			StatusCode: 401,
			StatusMsg:  "用户认证失败",
			ShareUrl:   "",
		}, nil
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		h.logger.Error("Failed to parse user ID", zap.Error(err))
		return &pb.ShareVideoResponse{
			StatusCode: 500,
			StatusMsg:  "用户ID解析失败",
			ShareUrl:   "",
		}, nil
	}

	// 调用service层处理分享逻辑
	_, err = h.videoService.ShareVideo(ctx, uint32(userID), req.VideoId, req.ShareType)
	if err != nil {
		h.logger.Error("Failed to share video", zap.Error(err))
		return &pb.ShareVideoResponse{
			StatusCode: 500,
			StatusMsg:  "分享失败",
			ShareUrl:   "",
		}, nil
	}

	// 生成分享链接
	shareUrl := fmt.Sprintf("/video/%d", req.VideoId)

	return &pb.ShareVideoResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		ShareUrl:   shareUrl,
	}, nil
}

// FavoriteVideo 收藏/取消收藏视频
func (h *VideoHandler) FavoriteVideo(ctx context.Context, req *pb.FavoriteVideoRequest) (*pb.FavoriteVideoResponse, error) {
	actionType := "favorite"
	if !req.ActionType {
		actionType = "unfavorite"
	}
	h.logger.Info("FavoriteVideo called", zap.Uint32("video_id", req.VideoId), zap.String("action_type", actionType))

	// 验证用户token并获取用户ID
	userIDStr, err := h.verifyTokenAndGetUserID(ctx, req.Token)
	if err != nil {
		h.logger.Error("Failed to verify token", zap.Error(err))
		return &pb.FavoriteVideoResponse{
			StatusCode:    401,
			StatusMsg:     "用户认证失败",
			FavoriteCount: 0,
		}, nil
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		h.logger.Error("Failed to parse user ID", zap.Error(err))
		return &pb.FavoriteVideoResponse{
			StatusCode:    500,
			StatusMsg:     "用户ID解析失败",
			FavoriteCount: 0,
		}, nil
	}

	// 调用service层处理收藏逻辑
	favoriteCount, err := h.videoService.FavoriteVideo(ctx, uint32(userID), req.VideoId, req.ActionType)
	if err != nil {
		h.logger.Error("Failed to favorite video", zap.Error(err))
		return &pb.FavoriteVideoResponse{
			StatusCode:    500,
			StatusMsg:     "收藏失败",
			FavoriteCount: 0,
		}, nil
	}

	return &pb.FavoriteVideoResponse{
		StatusCode:    0,
		StatusMsg:     "success",
		FavoriteCount: uint32(favoriteCount),
	}, nil
}

// ==================== 视频评论相关接口 ====================

// CommentVideo 发表评论
func (h *VideoHandler) CommentVideo(ctx context.Context, req *pb.CommentRequest) (*pb.CommentResponse, error) {
	h.logger.Info("CommentVideo called", zap.Uint32("video_id", req.VideoId), zap.String("content", req.Content))

	return h.commentService.CommentVideo(ctx, req)
}

// DeleteComment 删除评论
func (h *VideoHandler) DeleteComment(ctx context.Context, req *pb.DeleteCommentRequest) (*pb.DeleteCommentResponse, error) {
	h.logger.Info("DeleteComment called", zap.Uint32("comment_id", req.CommentId))

	return h.commentService.DeleteComment(ctx, req)
}

// GetVideoComments 获取视频评论列表
func (h *VideoHandler) GetVideoComments(ctx context.Context, req *pb.GetVideoCommentsRequest) (*pb.GetVideoCommentsResponse, error) {
	h.logger.Info("GetVideoComments called", zap.Uint32("video_id", req.VideoId), zap.Uint32("page", req.Page), zap.String("sort_order", req.SortOrder))

	return h.commentService.GetVideoComments(ctx, req)
}

// LikeComment 点赞评论
func (h *VideoHandler) LikeComment(ctx context.Context, req *pb.LikeCommentRequest) (*pb.LikeCommentResponse, error) {
	h.logger.Info("LikeComment called", zap.Uint32("comment_id", req.CommentId), zap.Bool("action_type", req.ActionType))

	return h.commentService.LikeComment(ctx, req)
}

// RecordPlay 记录视频播放
func (h *VideoHandler) RecordPlay(ctx context.Context, req *pb.RecordPlayRequest) (*pb.RecordPlayResponse, error) {
	h.logger.Info("RecordPlay called",
		zap.Uint32("video_id", req.VideoId),
		zap.Uint32("user_id", req.UserId),
		zap.String("session_id", req.SessionId),
		zap.String("device_id", req.DeviceId),
		zap.String("view_source", req.ViewSource))

	// 调用 videoService 记录播放
	playCount, isRecorded, realPlayCount, err := h.videoService.RecordPlay(ctx, req.VideoId, req.UserId, req.SessionId, req.DeviceId, req.ViewSource)
	if err != nil {
		h.logger.Error("Failed to record play", zap.Error(err))
		return &pb.RecordPlayResponse{
			StatusCode: 500,
			StatusMsg:  "Failed to record play: " + err.Error(),
		}, nil
	}

	return &pb.RecordPlayResponse{
		StatusCode:    0,
		StatusMsg:     "success",
		PlayCount:     playCount,
		IsRecorded:    isRecorded,
		RealPlayCount: realPlayCount,
	}, nil
}

// ReportProgress 上报视频观看进度
func (h *VideoHandler) ReportProgress(ctx context.Context, req *pb.ReportProgressRequest) (*pb.ReportProgressResponse, error) {
	h.logger.Info("ReportProgress called",
		zap.Uint32("video_id", req.VideoId),
		zap.Uint32("user_id", req.UserId),
		zap.String("session_id", req.SessionId),
		zap.Float64("current_time", req.CurrentTime),
		zap.Float64("progress", req.Progress),
		zap.String("action", req.Action))

	// 调用 videoService 上报进度
	isComplete, watchTime, err := h.videoService.ReportProgress(ctx, req.VideoId, req.UserId, req.SessionId, req.CurrentTime, req.Progress, req.Action)
	if err != nil {
		h.logger.Error("Failed to report progress", zap.Error(err))
		return &pb.ReportProgressResponse{
			StatusCode: 500,
			StatusMsg:  "Failed to report progress: " + err.Error(),
		}, nil
	}

	return &pb.ReportProgressResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		IsComplete: isComplete,
		WatchTime:  watchTime,
	}, nil
}

// GetVideoInteractionStats 获取视频互动数据统计
func (h *VideoHandler) GetVideoInteractionStats(ctx context.Context, req *pb.GetVideoInteractionStatsRequest) (*pb.GetVideoInteractionStatsResponse, error) {
	log.Printf("GetVideoInteractionStats called, video_id: %d", req.VideoId)

	// 获取视频详情
	videoID := strconv.FormatUint(uint64(req.VideoId), 10)
	videoDetail, err := h.videoService.GetVideoDetail(ctx, videoID)
	if err != nil {
		log.Printf("Failed to get video detail, video_id: %d, error: %v", req.VideoId, err)
		return &pb.GetVideoInteractionStatsResponse{
			StatusCode: 404,
			StatusMsg:  "视频不存在",
		}, nil
	}

	// 初始化响应
	resp := &pb.GetVideoInteractionStatsResponse{
		StatusCode:    0,
		StatusMsg:     "success",
		VideoId:       req.VideoId,
		LikeCount:     uint32(videoDetail.LikeCount),
		FavoriteCount: uint32(videoDetail.FavoriteCount),
		CoinCount:     0, // TODO: 实现投币功能后更新
		ShareCount:    uint32(videoDetail.ShareCount),
		PlayCount:     uint32(videoDetail.PlayCount),
		DanmakuCount:  0, // TODO: 从弹幕服务获取
		CommentCount:  uint32(videoDetail.CommentCount),
		IsLiked:       false,
		IsFavorited:   false,
		IsCoined:      false,
	}

	// 如果提供了token，检查用户是否已点赞/收藏/投币
	if req.Token != "" {
		userIDStr, err := h.verifyTokenAndGetUserID(ctx, req.Token)
		if err == nil {
			userID, err := strconv.ParseUint(userIDStr, 10, 32)
			if err == nil {
				// 检查是否已点赞
				isLiked, _ := h.videoService.IsVideoLiked(ctx, uint32(userID), req.VideoId)
				resp.IsLiked = isLiked

				// 检查是否已收藏
				isFavorited, _ := h.videoService.IsVideoFavorited(ctx, uint32(userID), req.VideoId)
				resp.IsFavorited = isFavorited

				// TODO: 检查是否已投币
				resp.IsCoined = false
			}
		}
	}

	return resp, nil
}

// ==================== 辅助方法 ====================

// convertToProtoVideo 将模型转换为protobuf格式
func (h *VideoHandler) convertToProtoVideo(video *model.RecommendationVideo) *pb.Video {
	if video == nil {
		return nil
	}

	// 解析视频ID
	videoID, err := strconv.ParseUint(video.VideoID, 10, 32)
	if err != nil {
		h.logger.Error("Failed to parse video ID", zap.String("video_id", video.VideoID), zap.Error(err))
		videoID = 0
	}

	// 解析标签
	tags := make([]string, 0)
	if video.Tags != "" {
		tags = strings.Split(video.Tags, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}

	// 创建作者信息
	author := &userpb.User{
		Id:   0, // TODO: 从其他地方获取作者ID
		Name: video.Author,
	}

	// 设置默认视频类型为原创
	videoType := "original"
	// 如果有来源信息，则设置为转载
	if video.Source != "" {
		videoType = "repost"
	}
	fmt.Printf(videoType)
	return &pb.Video{
		Id:           uint32(videoID),
		Title:        video.Title,
		Description:  video.Description,
		CoverUrl:     video.CoverURL,
		VideoUrl:     video.PlayURL,
		PlayCount:    uint32(video.ViewCount),
		LikeCount:    uint32(video.LikeCount),
		CommentCount: 0, // RecommendationVideo 中没有这个字段
		ShareCount:   0, // RecommendationVideo 中没有这个字段
		CreateTime:   video.CreatedAt.Unix(),
		UpdateTime:   video.UpdatedAt.Unix(),
		Duration:     uint32(video.Duration),
		Resolution:   "",       // RecommendationVideo 中没有这个字段
		Status:       "normal", // 默认状态
		IsPublic:     true,     // 默认公开
		Category:     video.Category,
		Author:       author,
		Tags:         tags,
	}
}

// convertToProtoVideoDetail 将 VideoDetail 转换为 protobuf 格式
func (h *VideoHandler) convertToProtoVideoDetail(videoDetail *model.VideoDetail) *pb.Video {
	if videoDetail == nil {
		return nil
	}

	// 解析视频ID
	videoID, err := strconv.ParseUint(videoDetail.VideoID, 10, 32)
	if err != nil {
		h.logger.Error("Failed to parse video ID", zap.String("video_id", videoDetail.VideoID), zap.Error(err))
		videoID = 0
	}

	// 解析标签
	tags := make([]string, 0)
	if videoDetail.Tags != "" {
		// 处理特殊情况：如果标签是"[]"（空数组字符串），则返回空数组
		if strings.TrimSpace(videoDetail.Tags) == "[]" {
			tags = []string{}
		} else {
			tags = strings.Split(videoDetail.Tags, ",")
			// 过滤掉空标签和无效标签
			validTags := make([]string, 0, len(tags))
			for i := range tags {
				tag := strings.TrimSpace(tags[i])
				if tag != "" && tag != "[]" {
					validTags = append(validTags, tag)
				}
			}
			tags = validTags
		}
	}

	// 创建作者信息，包含ID、名称、头像和粉丝量
	followerCount := uint32(videoDetail.UserInfo.FollowersCount)
	author := &userpb.User{
		Id:            videoDetail.UserInfo.UserID,
		Name:          videoDetail.UserInfo.Username,
		Avatar:        &videoDetail.UserInfo.AvatarURL,
		FollowerCount: &followerCount,
	}

	// 设置默认视频类型为原创
	videoType := "original"
	// 如果有来源信息，则设置为转载
	if videoDetail.Source != "" {
		videoType = "repost"
	}

	// 处理Location字段，转换为*string
	var location *string
	if videoDetail.Location != "" {
		location = &videoDetail.Location
	}

	return &pb.Video{
		Id:            uint32(videoID),
		Title:         videoDetail.Title,
		Description:   videoDetail.Description,
		CoverUrl:      videoDetail.CoverURL,
		VideoUrl:      videoDetail.PlayURL,     // API Gateway HLS 代理 URL
		PlaylistUrl:   videoDetail.PlaylistURL, // HLS播放列表URL
		PlayCount:     uint32(videoDetail.PlayCount),
		LikeCount:     uint32(videoDetail.LikeCount),
		CommentCount:  uint32(videoDetail.CommentCount),
		ShareCount:    uint32(videoDetail.ShareCount),
		FavoriteCount: uint32(videoDetail.FavoriteCount),
		// 默认未点赞和未收藏
		IsLiked:    false,
		IsFavorite: false,
		Tags:       tags,
		Location:   location,
		Category:   videoDetail.Category,
		CreateTime: videoDetail.CreatedAt.Unix(),
		UpdateTime: videoDetail.UpdatedAt.Unix(),
		Duration:   uint32(videoDetail.Duration),
		// 分辨率在VideoDetail中没有，使用默认值
		Resolution: "1080p",
		Status:     "normal",
		IsPublic:   true,
		Author:     author,
		Type:       &videoType,
		Source:     &videoDetail.Source,
	}
}

// ==================== 弹幕相关接口 ====================

// SendDanmaku 发送弹幕
func (h *VideoHandler) SendDanmaku(ctx context.Context, req *pb.SendDanmakuRequest) (*pb.SendDanmakuResponse, error) {
	log.Println("SendDanmaku called", zap.Uint32("video_id", req.VideoId))

	userID := req.UserId

	//调用弹幕服务存储弹幕
	//调用service->db
	err := h.videoService.SendDanmaku(ctx, userID, req.VideoId, req.Text, req.Color, req.VideoTimestamp, req.Speed)
	if err != nil {
		h.logger.Error("Failed to send danmaku", zap.Error(err))
		return &pb.SendDanmakuResponse{
			StatusCode: 500,
			StatusMsg:  "发送弹幕失败",
		}, err
	}

	now := time.Now().Unix()
	danmaku := &pb.Danmaku{
		Id:             uint32(now), // 使用时间戳作为临时ID
		UserId:         userID,
		VideoId:        req.VideoId,
		Text:           req.Text,
		Color:          req.Color,
		VideoTimestamp: req.VideoTimestamp,
		Speed:          req.Speed,
		CreatedAt:      now,
	}

	return &pb.SendDanmakuResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Danmaku:    danmaku,
	}, nil
}

// GetDanmakus 获取视频弹幕列表
func (h *VideoHandler) GetDanmakus(ctx context.Context, req *pb.GetDanmakusRequest) (*pb.GetDanmakusResponse, error) {
	h.logger.Info("GetDanmakus called", zap.Uint32("video_id", req.VideoId))

	// TODO: 从弹幕服务获取弹幕列表
	// 这里暂时返回空列表
	return &pb.GetDanmakusResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Danmakus:   []*pb.Danmaku{},
		Total:      0,
	}, nil
}
