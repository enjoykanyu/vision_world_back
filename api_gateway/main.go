package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	ginprometheus "github.com/zsais/go-gin-prometheus"

	"api_gateway/config"
	"api_gateway/router"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 创建Gin引擎
	engine := gin.New()
	engine.MaxMultipartMemory = 500 << 20 // 500 MB

	// 初始化Prometheus监控
	p := ginprometheus.NewPrometheus("vision_world_gateway")
	p.Use(engine)

	// 创建路由管理器（只处理路由转发）
	r, err := router.NewRouter(engine, cfg.Etcd.Endpoints)
	if err != nil {
		log.Fatalf("Failed to create router: %v", err)
	}
	defer r.Close()

	// 创建HTTP服务器
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: engine,
	}

	// 启动服务器
	go func() {
		log.Printf("Starting Vision World Gateway on port %d", cfg.Server.Port)
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
