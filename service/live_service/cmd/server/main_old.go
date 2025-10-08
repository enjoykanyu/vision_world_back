package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"gorm.io/gorm"

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

// Config 服务配置
type Config struct {
	ServerPort  int
	GRPCPort    int
	MySQLConfig database.MySQLConfig
	RedisConfig database.RedisConfig
	LogLevel    string
	Environment string
}

// 转换为内部config类型
func (c *Config) ToInternalConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Host: "0.0.0.0",
			Port: c.ServerPort,
			Mode: "release",
		},
		Database: config.DatabaseConfig{
			Host:         c.MySQLConfig.Host,
			Port:         c.MySQLConfig.Port,
			Username:     c.MySQLConfig.Username,
			Password:     c.MySQLConfig.Password,
			Database:     c.MySQLConfig.Database,
			MaxIdleConns: c.MySQLConfig.MaxIdleConns,
			MaxOpenConns: c.MySQLConfig.MaxOpenConns,
		},
		Redis: config.RedisConfig{
			Host:     c.RedisConfig.Host,
			Port:     c.RedisConfig.Port,
			Password: c.RedisConfig.Password,
			DB:       c.RedisConfig.DB,
		},
		Logger: config.LoggerConfig{
			Level:  c.LogLevel,
			Format: "console",
		},
	}
}

// 默认配置
func getDefaultConfig() *Config {
	return &Config{
		ServerPort:  8080,
		GRPCPort:    8081,
		LogLevel:    "info",
		Environment: "development",
		MySQLConfig: database.MySQLConfig{
			Host:         getEnv("MYSQL_HOST", "localhost"),
			Port:         getEnvAsInt("MYSQL_PORT", 3306),
			Username:     getEnv("MYSQL_USER", "root"),
			Password:     getEnv("MYSQL_PASSWORD", "password"),
			Database:     getEnv("MYSQL_DATABASE", "live_service"),
			MaxOpenConns: getEnvAsInt("MYSQL_MAX_OPEN_CONNS", 100),
			MaxIdleConns: getEnvAsInt("MYSQL_MAX_IDLE_CONNS", 10),
		},
		RedisConfig: database.RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvAsInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
	}
}

// 获取环境变量
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// 获取环境变量（整数）
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}

// 初始化日志
func initLogger(level string) loggerPkg.Logger {
	logLevel := loggerPkg.InfoLevel
	switch level {
	case "debug":
		logLevel = loggerPkg.DebugLevel
	case "info":
		logLevel = loggerPkg.InfoLevel
	case "warn":
		logLevel = loggerPkg.WarnLevel
	case "error":
		logLevel = loggerPkg.ErrorLevel
	}

	logConfig := &loggerPkg.Config{
		Level:      logLevel,
		Format:     "console",
		OutputPath: "",
	}

	return loggerPkg.NewLogger(logConfig)
}

// 初始化数据库
func initDatabase(config database.MySQLConfig, log loggerPkg.Logger) (*gorm.DB, error) {
	db, err := database.NewMySQLConnection(config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Info("Database connected successfully", "host", config.Host, "port", config.Port, "database", config.Database)
	return db, nil
}

// 初始化Redis
func initRedis(config database.RedisConfig, log loggerPkg.Logger) (*redis.Client, error) {
	redisClient, err := database.NewRedisClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Info("Redis connected successfully", "host", config.Host, "port", config.Port, "db", config.DB)
	return redisClient, nil
}

// 创建HTTP服务器
func createHTTPServer(log loggerPkg.Logger) *gin.Engine {
	if getEnv("GIN_MODE", "debug") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// 中间件
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":      "healthy",
			"version":     Version,
			"build_time":  BuildTime,
			"commit_hash": CommitHash,
			"timestamp":   time.Now().Unix(),
		})
	})

	// 版本信息
	router.GET("/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"version":     Version,
			"build_time":  BuildTime,
			"commit_hash": CommitHash,
		})
	})

	// Prometheus指标
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API路由组
	api := router.Group("/api/v1")
	{
		// 直播相关接口
		api.GET("/lives", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "Live list endpoint"})
		})

		api.GET("/lives/:id", func(c *gin.Context) {
			id := c.Param("id")
			c.JSON(http.StatusOK, gin.H{"message": "Get live by ID", "id": id})
		})
	}

	return router
}

// 创建gRPC服务器
func createGRPCServer(db *gorm.DB, redisClient *redis.Client, log loggerPkg.Logger, config *Config) *grpc.Server {
	// 初始化处理器
	liveHandler := handler.NewLiveServiceHandler(config.ToInternalConfig(), log, db, redisClient)

	// 创建gRPC服务器
	grpcServer := grpc.NewServer()

	// 注册服务
	pb.RegisterLiveServiceServer(grpcServer, liveHandler)

	// 注册健康检查服务
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("live_service", grpc_health_v1.HealthCheckResponse_SERVING)

	// 注册反射服务（开发环境）
	if getEnv("ENVIRONMENT", "development") == "development" {
		reflection.Register(grpcServer)
	}

	return grpcServer
}

// 启动HTTP服务器
func startHTTPServer(server *gin.Engine, port int, log loggerPkg.Logger) error {
	addr := fmt.Sprintf(":%d", port)
	log.Info("Starting HTTP server", "address", addr)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: server,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", "error", err)
		}
	}()

	return nil
}

// 启动gRPC服务器
func startGRPCServer(server *grpc.Server, port int, log loggerPkg.Logger) error {
	addr := fmt.Sprintf(":%d", port)
	log.Info("Starting gRPC server", "address", addr)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	go func() {
		if err := server.Serve(lis); err != nil {
			log.Error("gRPC server error", "error", err)
		}
	}()

	return nil
}

func main() {
	// 解析命令行参数
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.Parse()

	if showVersion {
		fmt.Printf("Live Service\n")
		fmt.Printf("Version: %s\n", Version)
		fmt.Printf("Build Time: %s\n", BuildTime)
		fmt.Printf("Commit Hash: %s\n", CommitHash)
		return
	}

	// 加载配置
	config := getDefaultConfig()

	// 初始化日志
	log := initLogger(config.LogLevel)
	log.Info("Starting Live Service",
		"version", Version,
		"build_time", BuildTime,
		"commit_hash", CommitHash,
		"environment", config.Environment,
	)

	// 初始化数据库
	db, err := initDatabase(config.MySQLConfig, log)
	if err != nil {
		log.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}

	// 设置全局数据库连接
	model.SetDB(db)

	// 初始化Redis
	redisClient, err := initRedis(config.RedisConfig, log)
	if err != nil {
		log.Error("Failed to initialize Redis", "error", err)
		os.Exit(1)
	}

	// 创建HTTP服务器
	httpServer := createHTTPServer(log)

	// 创建gRPC服务器
	grpcServer := createGRPCServer(db, redisClient, log, config)

	// 启动HTTP服务器
	if err := startHTTPServer(httpServer, config.ServerPort, log); err != nil {
		log.Error("Failed to start HTTP server", "error", err)
		os.Exit(1)
	}

	// 启动gRPC服务器
	if err := startGRPCServer(grpcServer, config.GRPCPort, log); err != nil {
		log.Error("Failed to start gRPC server", "error", err)
		os.Exit(1)
	}

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down servers...")

	// 优雅关闭HTTP服务器
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	httpServerInstance := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.ServerPort),
		Handler: httpServer,
	}

	if err := httpServerInstance.Shutdown(ctx); err != nil {
		log.Error("HTTP server shutdown error", "error", err)
	}

	// 优雅关闭gRPC服务器
	grpcServer.GracefulStop()

	log.Info("Servers stopped gracefully")
}
