package server

import (
	"audit_service/internal/config"
	"audit_service/internal/discovery"
	"audit_service/internal/handler"
	"audit_service/internal/model"
	"audit_service/internal/repository"
	"audit_service/internal/service"
	"audit_service/pkg/database"
	"audit_service/pkg/logger"
	pb "audit_service/proto/audit/v1"
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Server gRPC服务器
type Server struct {
	config        *config.Config
	logger        logger.Logger
	handler       *handler.AuditServiceHandler
	grpcServer    *grpc.Server
	gatewayServer *http.Server
	etcdDiscovery *discovery.EtcdDiscovery
}

// NewServer 创建gRPC服务器
func NewServer(cfg *config.Config, log logger.Logger) (*Server, error) {
	// 初始化数据库连接
	db, err := database.NewMySQLConnection(&cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 初始化模型
	if err := model.InitDB(db); err != nil {
		return nil, fmt.Errorf("failed to initialize models: %w", err)
	}

	// 初始化Redis连接
	redisClient, err := database.NewRedisConnection(&cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	// 创建仓库层
	repo := repository.NewAuditRepository(db)

	// 创建服务层
	svc := service.NewAuditService(cfg, log, repo, redisClient)

	// 创建处理器
	h := handler.NewAuditServiceHandler(cfg, log, svc, repo)

	// 创建etcd服务发现
	etcdDiscovery, err := discovery.NewEtcdDiscovery(cfg.Etcd.Endpoints)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd discovery: %w", err)
	}

	return &Server{
		config:        cfg,
		logger:        log,
		handler:       h,
		etcdDiscovery: etcdDiscovery,
	}, nil
}

// Start 启动服务器
func (s *Server) Start() error {
	// 启动gRPC服务器
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.Server.Port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// 创建gRPC服务器
	s.grpcServer = grpc.NewServer(
		grpc.UnaryInterceptor(s.unaryInterceptor),
		grpc.StreamInterceptor(s.streamInterceptor),
	)

	// 注册服务
	pb.RegisterAuditServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)

	// 注册到etcd
	serviceInfo := &discovery.ServiceInfo{
		Name:    s.config.Server.Name,
		Version: "1.0.0",
		Host:    s.config.Server.Host,
		Port:    s.config.Server.Port,
		Metadata: map[string]string{
			"protocol": "grpc",
			"health":   "healthy",
		},
	}

	if err := s.etcdDiscovery.Register(serviceInfo); err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	s.logger.Info("gRPC server starting",
		"host", s.config.Server.Host,
		"port", s.config.Server.Port,
	)

	// 启动HTTP网关（可选）
	if s.config.Server.EnableGateway {
		go s.startGateway()
	}

	// 启动健康检查服务
	go s.startHealthCheck()

	// 启动gRPC服务器
	if err := s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

// Stop 停止服务器
func (s *Server) Stop() error {
	s.logger.Info("Stopping gRPC server...")

	// 从etcd注销
	if err := s.etcdDiscovery.Deregister(s.config.Server.Name); err != nil {
		s.logger.Error("failed to deregister service", "error", err)
	}

	// 关闭etcd连接
	if err := s.etcdDiscovery.Close(); err != nil {
		s.logger.Error("failed to close etcd discovery", "error", err)
	}

	// 停止gRPC服务器
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	// 停止HTTP网关
	if s.gatewayServer != nil {
		ctx := context.Background()
		if err := s.gatewayServer.Shutdown(ctx); err != nil {
			s.logger.Error("failed to shutdown gateway server", "error", err)
		}
	}

	s.logger.Info("gRPC server stopped")
	return nil
}

// startGateway 启动HTTP网关
func (s *Server) startGateway() error {
	ctx := context.Background()
	mux := runtime.NewServeMux()

	// 注册gRPC服务到HTTP网关
	opts := []grpc.DialOption{grpc.WithInsecure()}
	endpoint := fmt.Sprintf("localhost:%d", s.config.Server.Port)

	if err := pb.RegisterAuditServiceHandlerFromEndpoint(ctx, mux, endpoint, opts); err != nil {
		return fmt.Errorf("failed to register gateway: %w", err)
	}

	gatewayAddr := fmt.Sprintf(":%d", s.config.Server.GatewayPort)
	s.gatewayServer = &http.Server{
		Addr:    gatewayAddr,
		Handler: mux,
	}

	s.logger.Info("HTTP gateway starting", "addr", gatewayAddr)
	if err := s.gatewayServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start gateway: %w", err)
	}

	return nil
}

// startHealthCheck 启动健康检查服务
func (s *Server) startHealthCheck() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/ready", s.readyHandler)

	healthAddr := fmt.Sprintf(":%d", s.config.Server.HealthPort)
	s.logger.Info("Health check service starting", "addr", healthAddr)

	if err := http.ListenAndServe(healthAddr, mux); err != nil {
		s.logger.Error("failed to start health check service", "error", err)
	}
}

// healthHandler 健康检查处理器
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	// 检查数据库连接
	if err := s.checkDatabaseHealth(); err != nil {
		s.logger.Error("database health check failed", "error", err)
		http.Error(w, "database unhealthy", http.StatusServiceUnavailable)
		return
	}

	// 检查Redis连接
	if err := s.checkRedisHealth(); err != nil {
		s.logger.Error("redis health check failed", "error", err)
		http.Error(w, "redis unhealthy", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("healthy"))
}

// readyHandler 就绪检查处理器
func (s *Server) readyHandler(w http.ResponseWriter, r *http.Request) {
	// 检查服务是否已启动
	if s.grpcServer == nil {
		http.Error(w, "service not ready", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ready"))
}

// checkDatabaseHealth 检查数据库健康状态
func (s *Server) checkDatabaseHealth() error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	return sqlDB.Ping()
}

// checkRedisHealth 检查Redis健康状态
func (s *Server) checkRedisHealth() error {
	redisClient, err := database.GetRedisClient()
	if err != nil {
		return err
	}

	return redisClient.Ping(context.Background()).Err()
}

// unaryInterceptor 一元拦截器
func (s *Server) unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// 记录请求日志
	s.logger.Info("gRPC request",
		"method", info.FullMethod,
		"request", req,
	)

	// 调用处理器
	resp, err := handler(ctx, req)

	// 记录响应日志
	if err != nil {
		s.logger.Error("gRPC request failed",
			"method", info.FullMethod,
			"error", err,
		)
	} else {
		s.logger.Info("gRPC request completed",
			"method", info.FullMethod,
			"response", resp,
		)
	}

	return resp, err
}

// streamInterceptor 流拦截器
func (s *Server) streamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	// 记录流请求日志
	s.logger.Info("gRPC stream request",
		"method", info.FullMethod,
	)

	// 调用处理器
	err := handler(srv, ss)

	// 记录流响应日志
	if err != nil {
		s.logger.Error("gRPC stream request failed",
			"method", info.FullMethod,
			"error", err,
		)
	} else {
		s.logger.Info("gRPC stream request completed",
			"method", info.FullMethod,
		)
	}

	return err
}
