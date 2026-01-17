package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/vision_world/video_service/internal/config"
	"github.com/vision_world/video_service/internal/handler"
	"github.com/vision_world/video_service/internal/model"

	//"github.com/vision_world/video_service/proto/proto_gen"

	//"github.com/vision_world/video_service/internal/health"
	"github.com/vision_world/video_service/pkg/database"
	"github.com/vision_world/video_service/pkg/logger"
	pb "github.com/vision_world/video_service/proto/proto_gen/video"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	// 初始化配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 调试输出数据库配置
	log.Printf("Database config - Host: %s, Port: %d, Username: %s, Database: %s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.Username, cfg.Database.Database)

	// 初始化日志
	logger, err := logger.NewLogger(logger.Config{
		Level:      cfg.Logger.Level,
		Format:     cfg.Logger.Format,
		OutputPath: cfg.Logger.OutputPath,
	})
	// 初始化数据库连接
	db, err := database.NewMySQLConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	logger.Info("Database connected successfully")
	defer func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}()

	// 自动迁移数据库表结构
	modelDB := model.NewDB(db)
	if err := modelDB.InitTables(); err != nil {
		log.Fatalf("Failed to migrate database tables: %v", err)
	}
	logger.Info("Database tables migrated successfully")

	redisClient, err := database.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Printf("FATAL ERROR: Failed to connect to redis: %v\n", err)
		logger.Fatal("Failed to connect to redis", zap.Error(err))
	}
	log.Println("Redis connected successfully")
	defer redisClient.Close()
	// 创建gRPC服务器，增加最大消息大小限制以支持大文件上传
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(100*1024*1024), // 设置最大接收消息大小为100MB
		grpc.MaxSendMsgSize(100*1024*1024), // 设置最大发送消息大小为100MB
	)
	log.Println("gRPC server created")

	// 注册健康检查服务
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// 创建视频处理器
	log.Println("Creating video handler...")
	videoHandler, err := handler.NewVideoHandler(cfg, logger, db, redisClient)
	log.Println("Video handler created")
	if err != nil {
		log.Printf("ERROR: Failed to create video handler: %v\n", err)
		logger.Fatal("Failed to create video handler", zap.Error(err))
	}
	log.Println("Video handler created successfully")

	// 注册视频服务
	pb.RegisterVideoServiceServer(grpcServer, videoHandler)

	// 监听端口
	lis, err := net.Listen("tcp", cfg.Server.Address)
	if err != nil {
		log.Printf("FATAL ERROR: Failed to listen on %s: %v\n", cfg.Server.Address, err)
		logger.Fatal("Failed to listen", zap.String("address", cfg.Server.Address), zap.Error(err))
	}
	log.Println("Listening on", zap.String("address", cfg.Server.Address))

	// 启动服务发现注册
	logger.Info("Registering service...")
	if err := videoHandler.RegisterService(); err != nil {
		logger.Fatal("Failed to register service", zap.Error(err))
	}
	logger.Info("Service registered successfully")

	// 优雅关闭
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		logger.Info("Shutting down server...")
		healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		videoHandler.Close()
		grpcServer.GracefulStop()
	}()

	logger.Info("Video service starting", zap.String("address", cfg.Server.Address))
	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatal("Failed to serve", zap.Error(err))
	}
}
