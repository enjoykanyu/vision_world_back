package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/visionworld/user-service/internal/config"
	"github.com/visionworld/user-service/internal/database"
	"github.com/visionworld/user-service/internal/service"
	"github.com/visionworld/user-service/pkg/jwt"
	"github.com/visionworld/user-service/pkg/logger"
	pb "github.com/visionworld/user-service/proto"
)

func main() {
	// 加载配置
	if err := config.Load("configs/config.yaml"); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	cfg := config.GlobalConfig

	// 初始化日志
	logger.InitLogger(&cfg.Log)
	defer logger.Sync()

	logger.Info("用户服务启动中...")

	// 初始化数据库
	if err := database.InitMySQL(&cfg.Database); err != nil {
		logger.Fatalf("初始化MySQL失败: %v", err)
	}
	defer database.CloseMySQL()

	// 初始化Redis
	if err := database.InitRedis(&cfg.Redis); err != nil {
		logger.Fatalf("初始化Redis失败: %v", err)
	}
	defer database.CloseRedis()

	// 自动迁移表结构
	if err := database.AutoMigrate(); err != nil {
		logger.Fatalf("自动迁移表结构失败: %v", err)
	}

	// 创建JWT管理器
	jwtManager := jwt.NewManager(cfg.JWT.Secret, cfg.JWT.AccessTokenExpire, cfg.JWT.RefreshTokenExpire)

	// 创建gRPC服务器
	grpcServer := grpc.NewServer()

	// 注册用户服务
	userService := service.NewUserService(database.GetMySQL(), database.GetRedis(), jwtManager, cfg)
	pb.RegisterUserServiceServer(grpcServer, userService)

	// 注册反射服务（用于调试）
	reflection.Register(grpcServer)

	// 监听端口
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPC.Port))
	if err != nil {
		logger.Fatalf("监听端口失败: %v", err)
	}

	// 启动服务
	go func() {
		logger.Infof("用户服务启动成功，监听端口: %d", cfg.GRPC.Port)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatalf("启动gRPC服务失败: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("用户服务关闭中...")
	grpcServer.GracefulStop()
	logger.Info("用户服务已关闭")
}
