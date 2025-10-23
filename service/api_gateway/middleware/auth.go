package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"api_gateway/client"
	"api_gateway/discovery"
	pb "api_gateway/proto/proto_gen/proto"
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
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && strings.EqualFold(parts[0], "Bearer")) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be Bearer {token}"})
			c.Abort()
			return
		}
		token := parts[1]

		// 调用用户服务远程校验 Token
		userID, expire, err := verifyTokenWithUserService(c.Request.Context(), token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
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
func inAuthWhitelist(path, uri string) bool {
	whitelistPrefixes := []string{
		"/health",
		"/grafana/health",
		"/api/auth/login",
		"/api/auth/refresh",
		"/api/user/sms/send",
	}
	p := path
	if p == "" {
		p = uri
	}
	for _, pre := range whitelistPrefixes {
		if strings.HasPrefix(p, pre) {
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
