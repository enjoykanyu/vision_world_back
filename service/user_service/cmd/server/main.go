package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
	"user_service/pkg/logger"

	"user_service/internal/config"
	"user_service/internal/handler"
	"user_service/internal/model"
	"user_service/pkg/database"

	//"user_service/pkg/logger"
	"user_service/proto/proto_gen"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	//ctx := context.Background()

	// 1. 加载配置
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. 初始化日志
	log, err := logger.NewLogger(logger.Config{
		Level:      cfg.Logger.Level,
		Format:     cfg.Logger.Format,
		OutputPath: cfg.Logger.OutputPath,
	})
	if err != nil {
		log.Fatal("Failed to initialize logger: %v", err)
	}
	log.Info("Starting user service", "version", "1.0.0")

	// 3. 初始化数据库连接
	db, err := database.NewMySQLConnection(cfg.Database)
	if err != nil {
		log.Fatal("Failed to connect to database", "error", err)
	}
	log.Info("Database connected successfully")
	defer func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}()

	// 设置模型数据库连接
	model.SetDB(db)
	log.Info("Database models initialized successfully")

	// 4. 初始化Redis连接
	redisClient, err := database.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Fatal("Failed to connect to redis", "error", err)
	}
	log.Info("Redis connected successfully")
	defer redisClient.Close()

	// 5. 创建gRPC服务器
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(unaryInterceptor(log)),
	)

	// 6. 注册健康检查服务
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("user_service", grpc_health_v1.HealthCheckResponse_SERVING)

	// 7. 注册用户服务
	userHandler := handler.NewUserServiceHandler(cfg, log, db, redisClient)
	proto_gen.RegisterUserServiceServer(grpcServer, userHandler)
	log.Info("User service registered")

	// 8. 注册反射服务（用于调试）
	reflection.Register(grpcServer)

	// 9. 启动gRPC服务器
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port))
	if err != nil {
		log.Fatal("Failed to listen", "error", err)
	}

	// 10. 启动服务器（异步）
	go func() {
		log.Info("Starting gRPC server", "address", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port))
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal("Failed to serve", "error", err)
		}
	}()

	// 11. 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down server...")

	// 12. 优雅关闭
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 13. 设置健康检查为不健康状态
	healthServer.SetServingStatus("user_service", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	// 14. 停止gRPC服务器
	grpcServer.GracefulStop()
	log.Info("Server stopped gracefully")
}

// unaryInterceptor gRPC一元拦截器
func unaryInterceptor(log logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		log.Info("gRPC request started",
			"method", info.FullMethod,
			"request", req,
		)

		// 调用实际的处理函数
		resp, err := handler(ctx, req)

		duration := time.Since(start)

		if err != nil {
			log.Error("gRPC request failed",
				"method", info.FullMethod,
				"error", err,
				"duration", duration,
			)
		} else {
			log.Info("gRPC request completed",
				"method", info.FullMethod,
				"duration", duration,
			)
		}

		return resp, err
	}
}
