package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vision_world/video_service/internal/config"
	"github.com/vision_world/video_service/internal/handler"
	"github.com/vision_world/video_service/internal/health"
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
	logger.InitLogger(cfg.Log.Level, cfg.Log.File)

	// 初始化数据库连接
	db, err := database.InitDatabase(cfg.Database)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer db.Close()

	// 创建gRPC服务器
	grpcServer := grpc.NewServer()

	// 注册健康检查服务
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// 设置自定义健康检查
	customHealthServer := health.SetupHealthCheck(cfg, db)

	// 启动健康检查后台任务
	go func() {
		ticker := time.NewTicker(30 * time.Second) // 每30秒检查一次
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				healthy := customHealthServer.IsHealthy(ctx)
				cancel()

				if healthy {
					healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
				} else {
					healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
				}
			}
		}
	}()

	// 创建视频处理器
	videoHandler, err := handler.NewVideoHandler(cfg)
	if err != nil {
		logger.Fatal("Failed to create video handler", zap.Error(err))
	}

	// 注册视频服务
	pb.RegisterVideoServiceServer(grpcServer, videoHandler)

	// 监听端口
	lis, err := net.Listen("tcp", cfg.Server.Address)
	if err != nil {
		logger.Fatal("Failed to listen", zap.String("address", cfg.Server.Address), zap.Error(err))
	}

	// 启动服务发现注册
	if err := videoHandler.RegisterService(); err != nil {
		logger.Fatal("Failed to register service", zap.Error(err))
	}

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
