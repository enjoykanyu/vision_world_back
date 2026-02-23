package main

import (
	"context"
	"fmt"
	"live_service/internal/discovery"
	"live_service/internal/model"
	"live_service/internal/service"
	live "live_service/proto/proto_gen/live"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"live_service/internal/config"
	"live_service/internal/handler"
	"live_service/pkg/database"
	"live_service/pkg/logger"
	// 使用审计服务客户端
)

func main() {
	// 1. 加载配置
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 打印WebSocket配置
	log.Printf("WebSocket config loaded: Enabled=%v, Port=%d", cfg.WebSocket.Enabled, cfg.WebSocket.Port)

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
	logger.Info("Starting live service", "version", "1.0.0")

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

	// 自动迁移数据库表结构
	logger.Info("Auto migrating database tables...")
	if err := db.AutoMigrate(
		&model.LiveRoom{},
		&model.LiveStream{},
		&model.LiveViewer{},
		&model.LiveGift{},
		&model.LiveChat{},
	); err != nil {
		logger.Fatal("Failed to auto migrate database", "error", err)
	}
	logger.Info("Database tables migrated successfully")

	// 4. 初始化Redis连接
	redisClient, err := database.NewRedisClient(cfg.Redis)
	if err != nil {
		logger.Fatal("Failed to connect to redis", "error", err)
	}
	logger.Info("Redis connected successfully")
	defer redisClient.Close()

	// 5. 初始化etcd服务注册
	etcdDiscovery, err := discovery.NewEtcdDiscovery(cfg.Etcd.Endpoints, "live-service")
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
	healthServer.SetServingStatus("live_service", grpc_health_v1.HealthCheckResponse_SERVING)

	// 8. 注册用户服务
	liveHandler := handler.NewLiveServiceHandler(cfg, logger, db, redisClient)

	live.RegisterLiveServiceServer(grpcServer, liveHandler)
	logger.Info("Live service registered")

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

	// 12. 启动WebSocket服务器（如果启用）
	logger.Info("WebSocket config", "enabled", cfg.WebSocket.Enabled, "port", cfg.WebSocket.Port)
	var wsHub *service.Hub
	if cfg.WebSocket.Enabled {
		wsHub = service.NewHub(cfg, logger, redisClient)

		// 启动Hub
		if err := wsHub.Start(); err != nil {
			logger.Fatal("Failed to start WebSocket hub", "error", err)
		}
		logger.Info("WebSocket Hub started")

		// 创建WebSocket handler
		wsHandler := handler.NewWebSocketHandler(wsHub, logger)

		// 设置HTTP路由
		http.HandleFunc("/ws/chat", func(w http.ResponseWriter, r *http.Request) {
			// 从查询参数获取用户信息
			userID := r.URL.Query().Get("user_id")
			roomID := r.URL.Query().Get("room_id")
			username := r.URL.Query().Get("username")
			avatar := r.URL.Query().Get("avatar")

			wsHub.HandleWebSocket(w, r, userID, roomID, username, avatar)
		})
		http.HandleFunc("/api/chat/stats", wsHandler.GetRoomStats)
		http.HandleFunc("/api/chat/hub/stats", wsHandler.GetHubStats)
		http.HandleFunc("/api/chat/online-users", wsHandler.GetOnlineUsers)
		http.HandleFunc("/api/chat/send", wsHandler.SendMessage)

		// 注册SRS回调处理器
		srsCallbackHandler := handler.NewSRSCallbackHandler(liveHandler.GetLiveService(), wsHub, logger)
		http.HandleFunc("/api/srs/callback", srsCallbackHandler.HandleCallback)
		logger.Info("SRS callback handler registered")

		// 启动HTTP服务器
		go func() {
			addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.WebSocket.Port)
			logger.Info("WebSocket HTTP server starting", "address", addr)
			if err := http.ListenAndServe(addr, nil); err != nil {
				logger.Fatal("Failed to start WebSocket HTTP server", "error", err)
			}
		}()
	}

	// 13. 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down server...")

	// 14. 设置健康检查为不健康状态
	healthServer.SetServingStatus("live_service", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	// 15. 停止WebSocket Hub
	if wsHub != nil {
		wsHub.Stop()
		logger.Info("WebSocket Hub stopped")
	}

	// 16. 停止gRPC服务器
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
