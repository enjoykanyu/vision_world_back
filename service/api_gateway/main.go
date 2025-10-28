package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	ginprometheus "github.com/zsais/go-gin-prometheus"

	"api_gateway/config"
	"api_gateway/middleware"
	"api_gateway/pkg/minio"
	"api_gateway/pkg/redis"
	"api_gateway/routes"
)

// EtcdConfig etcd配置
type EtcdConfig struct {
	Endpoints []string `mapstructure:"endpoints"`
}

// Config 应用配置
type Config struct {
	Etcd EtcdConfig `mapstructure:"etcd"`
}

func main() {
	// 加载配置
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Redis客户端初始化
	redisConfig := redis.Config{
		Host:     "localhost",
		Port:     6379,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: 10,
	}
	redisClient, err := redis.NewClient(redisConfig)
	if err != nil {
		log.Fatalf("Failed to initialize Redis client: %v", err)
	}
	defer redisClient.Close()

	// 初始化MinIO客户端
	minioClient, err := minio.NewClient(minio.Config{
		Endpoint:        cfg.MinIO.Endpoint,
		AccessKeyID:     cfg.MinIO.AccessKeyID,
		SecretAccessKey: cfg.MinIO.SecretAccessKey,
		UseSSL:          cfg.MinIO.UseSSL,
		BucketName:      cfg.MinIO.BucketName,
		Location:        cfg.MinIO.Location,
	})
	if err != nil {
		log.Fatalf("Failed to create MinIO client: %v", err)
	}
	defer minioClient.Close()

	// 创建Gin引擎
	router := gin.New()

	// 初始化Prometheus监控
	p := ginprometheus.NewPrometheus("vision_world_gateway")
	p.Use(router)

	// 初始化统一鉴权（基于 etcd 的 user-service 验证）
	middleware.InitAuth(cfg.Etcd.Endpoints)

	// 添加中间件（将鉴权放在业务路由前，全局生效）
	router.Use(middleware.MetricsMiddleware())     // 自定义监控中间件
	router.Use(middleware.LoggerMiddleware())      // 日志中间件
	router.Use(middleware.RecoveryMiddleware())    // 恢复中间件
	router.Use(middleware.CORSMiddleware())        // CORS中间件
	router.Use(middleware.RequireAuthMiddleware()) // 统一鉴权中间件

	// 健康检查路由
	router.GET("/health", middleware.HealthCheck())

	// Grafana健康检查路由
	router.GET("/grafana/health", middleware.GrafanaHealthCheck())

	// 注册用户服务路由
	userHandler, err := routes.NewUserHandler(cfg.Etcd.Endpoints)
	if err != nil {
		log.Fatalf("Failed to connect to user service: %v", err)
	}
	defer userHandler.Close()

	// 注册直播服务路由
	liveHandler, err := routes.NewLiveHandler(cfg.Etcd.Endpoints)
	if err != nil {
		log.Fatalf("Failed to connect to live service: %v", err)
	}
	defer liveHandler.Close()

	// 注册视频服务路由
	videoHandler, err := routes.NewVideoHandler(cfg.Etcd.Endpoints)
	if err != nil {
		log.Fatalf("Failed to connect to video service: %v", err)
	}
	defer videoHandler.Close()

	// 注册视频相关路由
	routes.RegisterVideoRoutesWithHandler(router, videoHandler)

	// 注册视频上传路由（全局鉴权中间件已生效，无需重复添加）
	router.POST("/api/video/upload", videoHandler.HandleVideoUpload)

	// 注册用户相关路由
	router.POST("/api/user/login/phone", userHandler.PhoneLogin)
	router.POST("/api/user/login/code", userHandler.CodeLogin)
	router.POST("/api/user/sms/send", userHandler.SendSmsCode)
	router.GET("/api/user/info/:id", userHandler.GetUserInfo)

	// 添加认证相关路由，与前端API路径保持一致
	router.POST("/api/auth/login", userHandler.CodeLogin) // 使用验证码登录接口
	router.POST("/api/auth/logout", userHandler.Logout)
	router.POST("/api/auth/refresh", userHandler.RefreshToken)
	router.GET("/api/auth/userinfo", userHandler.GetUserInfo)

	// 注册直播相关路由
	router.POST("/api/live/start", liveHandler.StartLive)
	router.POST("/api/live/stop", liveHandler.StopLive)
	router.GET("/api/live/stream/:id", liveHandler.GetLiveStream)
	router.GET("/api/live/list", liveHandler.GetLiveList)

	// 注册首页相关路由
	router.GET("/api/home/recommended", videoHandler.GetRecommendedVideos)
	router.GET("/api/home/hot", videoHandler.GetHotVideos)

	// 直接启动Gin服务器
	log.Printf("Starting Vision World Gateway on port %s", ":8080")

	// 创建HTTP服务器
	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// 启动服务器
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// 等待中断信号以优雅地关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// 设置5秒的超时时间
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 关闭服务器
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting")
}
