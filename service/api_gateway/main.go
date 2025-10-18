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

	"api_gateway/middleware"
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
	// 初始化配置
	cfg := &Config{
		Etcd: EtcdConfig{
			Endpoints: []string{"localhost:2379"},
		},
	}

	// 创建Gin引擎
	router := gin.New()

	// 初始化Prometheus监控
	p := ginprometheus.NewPrometheus("vision_world_gateway")
	p.Use(router)

	// 添加中间件
	router.Use(middleware.MetricsMiddleware())  // 自定义监控中间件
	router.Use(middleware.LoggerMiddleware())   // 日志中间件
	router.Use(middleware.RecoveryMiddleware()) // 恢复中间件
	router.Use(middleware.CORSMiddleware())     // CORS中间件

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

	// 注册用户相关路由
	router.POST("/api/user/login/phone", userHandler.PhoneLogin)
	router.POST("/api/user/login/code", userHandler.CodeLogin)
	router.POST("/api/user/sms/send", userHandler.SendSmsCode)
	router.GET("/api/user/info/:id", userHandler.GetUserInfo)

	// 新增Token相关路由
	router.POST("/api/user/token/verify", userHandler.VerifyToken)
	router.POST("/api/user/token/refresh", userHandler.RefreshToken)

	// 注册直播相关路由
	router.POST("/api/live/start", liveHandler.StartLive)
	router.POST("/api/live/stop", liveHandler.StopLive)
	router.GET("/api/live/stream/:id", liveHandler.GetLiveStream)
	router.GET("/api/live/list", liveHandler.GetLiveList)

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
