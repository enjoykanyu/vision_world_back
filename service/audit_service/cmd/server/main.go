package main

import (
	"audit_service/internal/config"
	"audit_service/internal/discovery"
	"audit_service/internal/handler"
	"audit_service/internal/model"
	"audit_service/internal/repository"
	"audit_service/internal/service"
	"audit_service/pkg/database"
	"audit_service/pkg/logger"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	auditv1 "audit_service/proto_gen/audit/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	// 1. 加载配置
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 打印配置信息，用于调试
	log.Printf("Logger config: Level=%s, Format=%s, OutputPath=%s", cfg.Logger.Level, cfg.Logger.Format, cfg.Logger.OutputPath)

	// 2. 初始化日志
	log.Printf("Attempting to initialize logger with output path: %s", cfg.Logger.OutputPath)
	logger, err := logger.NewLogger(logger.Config{
		Level:      cfg.Logger.Level,
		Format:     cfg.Logger.Format,
		OutputPath: cfg.Logger.OutputPath,
	})
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	log.Printf("Logger initialized successfully")
	logger.Info("Starting audit service", "version", "1.0.0")

	// 3. 初始化数据库连接
	log.Printf("Attempting to connect to database")
	log.Printf("Database config: Host=%s, Port=%d, Username=%s, Database=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.Username, cfg.Database.Database)
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

	// 设置模型数据库连接
	model.SetDB(db)
	logger.Info("Database models initialized successfully")

	// 4. 初始化Redis连接
	redisClient, err := database.NewRedisClient(cfg.Redis)
	if err != nil {
		logger.Fatal("Failed to connect to redis", "error", err)
	}
	logger.Info("Redis connected successfully")
	defer redisClient.Close()

	// 5. 初始化etcd服务注册
	etcdDiscovery, err := discovery.NewEtcdDiscovery(cfg.Etcd.Endpoints, "audit-service")
	if err != nil {
		logger.Fatal("Failed to connect to etcd", "error", err)
	}
	defer etcdDiscovery.Close()

	// 6. 创建gRPC服务器
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(unaryInterceptor(logger)),
	)

	// 7. 注册健康检查服务
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("audit_service", grpc_health_v1.HealthCheckResponse_SERVING)

	// 8. 注册审核服务
	// 创建repository
	auditRepo := repository.NewAuditRepository(db)
	// 创建service
	auditService := service.NewAuditService(cfg, logger, auditRepo)
	// 创建handler
	auditHandler := handler.NewAuditServiceHandler(auditService, logger)
	auditv1.RegisterAuditServiceServer(grpcServer, auditHandler)
	logger.Info("Audit service registered")

	// 9. 注册反射服务（用于调试）
	reflection.Register(grpcServer)

	// 10. 启动gRPC服务器
	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatal("Failed to listen", "error", err)
		}

		logger.Info("gRPC server starting", "address", addr)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal("Failed to serve", "error", err)
		}
	}()

	// 11. 注册服务到etcd
	serviceAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	if err := etcdDiscovery.Register(serviceAddr, 10); err != nil {
		logger.Fatal("Failed to register service to etcd", "error", err)
	}
	logger.Info("Service registered to etcd", "address", serviceAddr)

	// 12. 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down server...")

	// 13. 设置健康检查为不健康状态
	healthServer.SetServingStatus("audit_service", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	// 14. 停止gRPC服务器
	grpcServer.GracefulStop()
	logger.Info("Server stopped gracefully")
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
