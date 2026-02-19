package router

import (
	"log"

	"api_gateway/middleware"
	"api_gateway/pkg/minio"
	"api_gateway/routes"

	"github.com/gin-gonic/gin"
)

// Router 路由管理器
type Router struct {
	engine        *gin.Engine
	etcdEndpoints []string
	minioClient   *minio.Client
	bucketName    string

	// Handlers
	videoHandler *routes.VideoHandler
	userHandler  *routes.UserHandler
	liveHandler  *routes.LiveHandler
}

// NewRouter 创建路由管理器
func NewRouter(engine *gin.Engine, etcdEndpoints []string, minioClient *minio.Client, bucketName string) (*Router, error) {
	r := &Router{
		engine:        engine,
		etcdEndpoints: etcdEndpoints,
		minioClient:   minioClient,
		bucketName:    bucketName,
	}

	// 初始化所有 Handler
	if err := r.initHandlers(); err != nil {
		return nil, err
	}

	// 注册全局中间件
	r.registerMiddlewares()

	// 注册路由
	r.registerRoutes()

	return r, nil
}

// initHandlers 初始化所有 Handler
func (r *Router) initHandlers() error {
	var err error

	// 初始化用户服务 Handler
	r.userHandler, err = routes.NewUserHandler(r.etcdEndpoints)
	if err != nil {
		log.Printf("Failed to create user handler: %v", err)
		return err
	}

	// 初始化直播服务 Handler
	r.liveHandler, err = routes.NewLiveHandler(r.etcdEndpoints)
	if err != nil {
		log.Printf("Failed to create live handler: %v", err)
		return err
	}

	// 初始化视频服务 Handler
	r.videoHandler, err = routes.NewVideoHandler(r.etcdEndpoints, r.minioClient, r.bucketName)
	if err != nil {
		log.Printf("Failed to create video handler: %v", err)
		return err
	}

	return nil
}

// registerMiddlewares 注册全局中间件
func (r *Router) registerMiddlewares() {
	log.Println("进入新架构")
	// 初始化统一鉴权（基于 etcd 的 user-service 验证）
	middleware.InitAuth(r.etcdEndpoints)

	// 添加中间件（将鉴权放在业务路由前，但不使用全局鉴权）
	r.engine.Use(middleware.MetricsMiddleware())  // 自定义监控中间件
	r.engine.Use(middleware.LoggerMiddleware())   // 日志中间件
	r.engine.Use(middleware.RecoveryMiddleware()) // 恢复中间件
	r.engine.Use(middleware.CORSMiddleware())     // CORS中间件
}

// registerRoutes 注册所有路由
func (r *Router) registerRoutes() {
	// 健康检查
	r.engine.GET("/health", middleware.HealthCheck())
	r.engine.GET("/grafana/health", middleware.GrafanaHealthCheck())

	// API 路由组
	api := r.engine.Group("/api")
	{
		// 注册各模块路由
		r.registerUserRoutes(api)
		r.registerVideoRoutes(api)
		r.registerLiveRoutes(api)
		r.registerHomeRoutes(api)
	}
}

// registerUserRoutes 注册用户相关路由
func (r *Router) registerUserRoutes(api *gin.RouterGroup) {
	// 用户登录相关
	api.POST("/user/login/phone", r.userHandler.PhoneLogin)
	api.POST("/user/login/code", r.userHandler.CodeLogin)
	api.POST("/user/sms/send", r.userHandler.SendSmsCode)
	api.GET("/user/info/:id", r.userHandler.GetUserInfo)
	api.POST("/user/token/verify", r.userHandler.VerifyToken)

	// 认证相关（与前端API路径保持一致）
	api.POST("/auth/login", r.userHandler.CodeLogin)
	api.POST("/auth/logout", r.userHandler.Logout)
	api.POST("/auth/refresh", r.userHandler.RefreshToken)
	api.GET("/auth/userinfo", r.userHandler.GetUserInfo)
	api.POST("/auth/token/verify", r.userHandler.VerifyToken)

	// 用户信息更新
	api.POST("/user/profile/update", r.userHandler.UpdateUserInfo)

	// 头像上传
	api.POST("/upload/avatar", r.userHandler.UploadAvatar)
}

// registerVideoRoutes 注册视频相关路由
func (r *Router) registerVideoRoutes(api *gin.RouterGroup) {
	// 使用统一的视频路由注册函数
	routes.RegisterVideoRoutesWithHandler(r.engine, r.videoHandler)
}

// registerLiveRoutes 注册直播相关路由
func (r *Router) registerLiveRoutes(api *gin.RouterGroup) {
	api.POST("/live/start", r.liveHandler.StartLive)
	api.POST("/live/stop", r.liveHandler.StopLive)
	api.GET("/live/room/:id", r.liveHandler.GetRoomInfo)
	// api.GET("/live/stream/:id", r.liveHandler.GetLiveStream)
	api.GET("/live/list", r.liveHandler.GetLiveList)
}

// registerHomeRoutes 注册首页相关路由
func (r *Router) registerHomeRoutes(api *gin.RouterGroup) {
	api.GET("/home/recommended", r.videoHandler.GetRecommendedVideos)
	api.GET("/home/hot", r.videoHandler.GetHotVideos)
}

// Close 关闭所有 Handler 连接
func (r *Router) Close() {
	if r.userHandler != nil {
		r.userHandler.Close()
	}
	if r.liveHandler != nil {
		r.liveHandler.Close()
	}
	if r.videoHandler != nil {
		r.videoHandler.Close()
	}
}
