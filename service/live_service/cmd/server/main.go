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

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"vision_world_back/service/live_service/internal/config"
	"vision_world_back/service/live_service/internal/handler"
	"vision_world_back/service/live_service/internal/model"
	"vision_world_back/service/live_service/pkg/database"
	loggerPkg "vision_world_back/service/live_service/pkg/logger"
	pb "vision_world_back/service/live_service/proto/proto_gen"
)

var (
	Version    = "dev"
	BuildTime  = "unknown"
	CommitHash = "unknown"
)

func main() {
	fmt.Println("Starting live service...")

	// 1. 加载配置
	fmt.Println("Loading configuration...")
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	fmt.Printf("Configuration loaded successfully. Server port: %d, HTTP port: %d\n", cfg.Server.Port, cfg.Server.HTTPPort)

	// 2. 初始化日志
	fmt.Println("Initializing logger...")
	logger := loggerPkg.NewLogger(&loggerPkg.Config{
		Level:      cfg.Logger.Level,
		Format:     cfg.Logger.Format,
		OutputPath: cfg.Logger.OutputPath,
	})
	fmt.Println("Logger initialized successfully")
	logger.Info("Starting live service",
		"version", Version,
		"build_time", BuildTime,
		"commit_hash", CommitHash,
	)

	// 3. 初始化数据库连接
	db, err := database.NewMySQLConnection(database.MySQLConfig{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		Username:        cfg.Database.Username,
		Password:        cfg.Database.Password,
		Database:        cfg.Database.Database,
		Charset:         cfg.Database.Charset,
		TablePrefix:     cfg.Database.TablePrefix,
		LogLevel:        cfg.Database.LogLevel,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	})
	if err != nil {
		logger.Fatal("Failed to connect to database", "error", err)
	}
	defer func() {
		if sqlDB, err := db.DB(); err == nil && sqlDB != nil {
			sqlDB.Close()
		}
	}()
	logger.Info("Database connected successfully")

	// 设置模型数据库连接
	model.SetDB(db)
	logger.Info("Database models initialized successfully")

	// 4. 初始化Redis连接
	redisClient, err := database.NewRedisClient(database.RedisConfig{
		Host:         cfg.Redis.Host,
		Port:         cfg.Redis.Port,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		MaxRetries:   cfg.Redis.MaxRetries,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
	if err != nil {
		logger.Fatal("Failed to connect to redis", "error", err)
	}
	defer redisClient.Close()
	logger.Info("Redis connected successfully")

	// 5. 创建gRPC服务器
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(unaryInterceptor(logger)),
	)

	// 6. 注册健康检查服务
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("live_service", grpc_health_v1.HealthCheckResponse_SERVING)

	// 7. 注册直播服务
	liveHandler := handler.NewLiveServiceHandler(cfg, logger, db, redisClient)
	pb.RegisterLiveServiceServer(grpcServer, liveHandler)
	logger.Info("Live service registered")

	// 8. 注册反射服务（用于调试）
	if cfg.Server.Mode == "debug" {
		reflection.Register(grpcServer)
	}

	// 9. 启动gRPC服务器
	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		fmt.Printf("Starting gRPC server on %s\n", addr)
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			logger.Fatal("Failed to listen", "error", err)
		}

		logger.Info("gRPC server starting", "address", addr)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal("Failed to serve", "error", err)
		}
	}()

	// 11. 等待中断信号
	fmt.Println("Service started successfully. Press Ctrl+C to stop.")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down server...")

	// 12. 设置健康检查为不健康状态
	healthServer.SetServingStatus("live_service", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	// 13. 停止gRPC服务器
	grpcServer.GracefulStop()
	logger.Info("Server stopped gracefully")
}

// unaryInterceptor gRPC一元拦截器
func unaryInterceptor(log loggerPkg.Logger) grpc.UnaryServerInterceptor {
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

// startHTTPServer 启动HTTP服务器
func startHTTPServer(cfg *config.Config, logger loggerPkg.Logger) {
	if gin.Mode() == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// 中间件
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "healthy",
			"service":   "live_service",
			"timestamp": time.Now().Unix(),
		})
	})

	// 版本信息
	router.GET("/version", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"version":     Version,
			"build_time":  BuildTime,
			"commit_hash": CommitHash,
		})
	})

	// Prometheus指标
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.HTTPPort)
	logger.Info("HTTP server starting", "address", addr)

	if err := router.Run(addr); err != nil {
		logger.Error("HTTP server error", "error", err)
	}
}
