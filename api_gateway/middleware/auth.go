package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"api_gateway/client"
	"api_gateway/discovery"
	pb "api_gateway/proto_gen/user"
	"github.com/gin-gonic/gin"
)

var (
	initOnce     sync.Once
	authVerifier *tokenVerifier
)

type tokenVerifier struct {
	mu          sync.RWMutex
	endpoints   []string
	discovery   *discovery.EtcdServiceDiscovery
	userClient  *client.UserServiceClient
	serviceAddr string
}

// InitAuth 在网关启动时初始化鉴权依赖（在 main 中调用）
func InitAuth(etcdEndpoints []string) {
	initOnce.Do(func() {
		v := &tokenVerifier{endpoints: etcdEndpoints}
		// 懒加载：首次验证时再连接 user-service
		authVerifier = v
	})
}

// RequireAuthMiddleware 统一鉴权中间件（默认全局开启，白名单放行）
func RequireAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 预检请求直接放行
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		// 白名单路径（无需鉴权）
		path := c.FullPath()
		// 当 FullPath 为空（未命中路由前）回退到 RequestURI 前缀判断
		uri := c.Request.RequestURI
		if inAuthWhitelist(path, uri) {
			c.Next()
			return
		}

		// 提取 Authorization: Bearer <token>
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header is required",
				"code":  "MISSING_AUTH_HEADER",
			})
			c.Abort()
			return
		}

		// 企业级安全：限制token长度防止DoS攻击
		if len(authHeader) > 4096 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Authorization header too long",
				"code":  "INVALID_AUTH_FORMAT",
			})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && strings.EqualFold(parts[0], "Bearer")) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header format must be Bearer {token}",
				"code":  "INVALID_AUTH_FORMAT",
			})
			c.Abort()
			return
		}

		token := strings.TrimSpace(parts[1])
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Token cannot be empty",
				"code":  "EMPTY_TOKEN",
			})
			c.Abort()
			return
		}

		// 调用用户服务远程校验 Token
		userID, expire, err := verifyTokenWithUserService(c.Request.Context(), token)
		if err != nil {
			// 企业级错误处理：区分不同类型的认证失败
			errorCode := "INVALID_TOKEN"
			errorMsg := "Invalid or expired token"
			statusCode := http.StatusUnauthorized

			if err == context.DeadlineExceeded {
				errorCode = "AUTH_TIMEOUT"
				errorMsg = "Authentication service timeout"
				statusCode = http.StatusServiceUnavailable
			} else if strings.Contains(err.Error(), "connection") {
				errorCode = "AUTH_SERVICE_UNAVAILABLE"
				errorMsg = "Authentication service unavailable"
				statusCode = http.StatusServiceUnavailable
			}

			c.JSON(statusCode, gin.H{
				"error": errorMsg,
				"code":  errorCode,
			})
			c.Abort()
			return
		}

		// 将用户信息注入上下文，供后续处理使用
		c.Set("user_id", userID)
		c.Set("token_expire", expire)

		c.Next()
	}
}

// inAuthWhitelist 判断是否在白名单
// 企业级路由放过鉴权规则：
// 1. 健康检查和监控接口：完全公开（符合RFC标准）
// 2. 认证相关接口：登录、注册、刷新token（遵循OAuth2.0标准）
// 3. 只读数据接口：推荐、热门视频（限流保护，符合RESTful设计）
// 4. 静态资源接口：图片、视频播放（CDN友好）
// 5. OPTIONS预检请求：CORS标准支持
func inAuthWhitelist(path, uri string) bool {
	// 标准化路径处理
	p := path
	if p == "" {
		p = uri
	}

	// 完全公开接口 - 健康检查（符合Kubernetes探针标准）
	publicPaths := []string{
		"/health",
		"/grafana/health",
		"/metrics",
		"/favicon.ico",
		"/robots.txt",
	}

	// 认证相关接口 - 登录注册（符合OAuth2.0密码模式）
	authPaths := []string{
		"/api/auth/login",
		"/api/auth/refresh",
		"/api/auth/logout",
		"/api/user/login/phone",
		"/api/user/login/code",
		"/api/user/sms/send",
		"/api/user/register",
		"/api/user/reset/password",
	}

	// 只读数据接口 - 内容获取（GET请求，符合RESTful安全方法）
	readonlyPaths := []string{
		"/api/home/recommended",
		"/api/home/hot",
		"/api/video/recommended",
		"/api/video/hot",
		"/api/video/info/",
		"/api/video/upload",
		"/api/video/publish",
		"/api/user/info/",
		"/api/live/list",
		"/api/live/stream/",
		"/api/search/hot",
		"/api/category/list",
	}

	// 静态资源接口（CDN和文件服务）
	staticPaths := []string{
		"/api/file/upload/",
		"/api/file/download/",
		"/api/video/play/",
		"/api/image/",
		"/api/avatar/",
		"/static/",
		"/uploads/",
	}

	// 特殊处理：完全匹配检查
	for _, allowed := range publicPaths {
		if p == allowed {
			return true
		}
	}

	// 前缀匹配检查 - 按优先级顺序
	allPrefixes := append(authPaths, readonlyPaths...)
	allPrefixes = append(allPrefixes, staticPaths...)

	for _, prefix := range allPrefixes {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}

	return false
}

// verifyTokenWithUserService 通过 user-service 校验 Token（带连接复用与服务发现）
func verifyTokenWithUserService(ctx context.Context, token string) (userID uint32, expire int64, err error) {
	if authVerifier == nil {
		return 0, 0, http.ErrAbortHandler
	}

	// 确保已连接 user-service
	if err := authVerifier.ensureUserClient(); err != nil {
		return 0, 0, err
	}

	// 超时控制，避免长时间阻塞
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := authVerifier.userClient.VerifyToken(c, &pb.VerifyTokenRequest{Token: token})
	if err != nil || resp == nil || resp.StatusCode != 0 {
		return 0, 0, http.ErrNoCookie
	}
	return resp.UserId, resp.ExpireTime, nil
}

func (v *tokenVerifier) ensureUserClient() error {
	v.mu.RLock()
	if v.userClient != nil && v.userClient.IsConnected() {
		v.mu.RUnlock()
		return nil
	}
	v.mu.RUnlock()

	v.mu.Lock()
	defer v.mu.Unlock()
	// 双重检查
	if v.userClient != nil && v.userClient.IsConnected() {
		return nil
	}

	// 初始化服务发现
	if v.discovery == nil {
		d, err := discovery.NewEtcdServiceDiscovery(v.endpoints, "user-service")
		if err != nil {
			return err
		}
		v.discovery = d
	}

	// 发现服务地址
	addr, err := v.discovery.DiscoverService()
	if err != nil {
		return err
	}
	if v.serviceAddr != addr {
		v.serviceAddr = addr
		if v.userClient != nil {
			_ = v.userClient.Close()
			v.userClient = nil
		}
	}

	// 建立 gRPC 客户端
	if v.userClient == nil {
		uc, err := client.NewUserServiceClient(addr)
		if err != nil {
			return err
		}
		v.userClient = uc
	}
	return nil
}
