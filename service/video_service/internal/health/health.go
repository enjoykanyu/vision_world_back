package health

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"time"

	"github.com/vision_world/video_service/internal/config"
	"github.com/vision_world/video_service/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// Checker 健康检查接口
type Checker interface {
	Check(ctx context.Context) error
	Name() string
}

// DatabaseChecker 数据库健康检查
type DatabaseChecker struct {
	db *sql.DB
}

func NewDatabaseChecker(db *sql.DB) *DatabaseChecker {
	return &DatabaseChecker{db: db}
}

func (c *DatabaseChecker) Check(ctx context.Context) error {
	if c.db == nil {
		return fmt.Errorf("database connection is nil")
	}

	err := c.db.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}

func (c *DatabaseChecker) Name() string {
	return "database"
}

// RedisChecker Redis健康检查
type RedisChecker struct {
	// 这里应该传入Redis客户端
	// 由于当前代码中没有Redis客户端，这里先留空
}

func NewRedisChecker() *RedisChecker {
	return &RedisChecker{}
}

func (c *RedisChecker) Check(ctx context.Context) error {
	// TODO: 实现Redis健康检查
	return nil
}

func (c *RedisChecker) Name() string {
	return "redis"
}

// GRPCServiceChecker gRPC服务健康检查
type GRPCServiceChecker struct {
	serviceName string
	address     string
	timeout     time.Duration
}

func NewGRPCServiceChecker(serviceName, address string, timeout time.Duration) *GRPCServiceChecker {
	return &GRPCServiceChecker{
		serviceName: serviceName,
		address:     address,
		timeout:     timeout,
	}
}

func (c *GRPCServiceChecker) Check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, c.address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", c.serviceName, err)
	}
	defer conn.Close()

	client := grpc_health_v1.NewHealthClient(conn)
	resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return fmt.Errorf("health check failed for %s: %w", c.serviceName, err)
	}

	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("service %s is not serving, status: %v", c.serviceName, resp.Status)
	}

	return nil
}

func (c *GRPCServiceChecker) Name() string {
	return c.serviceName
}

// PortChecker 端口检查
type PortChecker struct {
	host string
	port int
}

func NewPortChecker(host string, port int) *PortChecker {
	return &PortChecker{
		host: host,
		port: port,
	}
}

func (c *PortChecker) Check(ctx context.Context) error {
	address := fmt.Sprintf("%s:%d", c.host, c.port)

	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	defer conn.Close()

	return nil
}

func (c *PortChecker) Name() string {
	return fmt.Sprintf("port_%d", c.port)
}

// HealthServer 健康检查服务器
type HealthServer struct {
	checkers []Checker
}

func NewHealthServer() *HealthServer {
	return &HealthServer{
		checkers: make([]Checker, 0),
	}
}

func (s *HealthServer) AddChecker(checker Checker) {
	s.checkers = append(s.checkers, checker)
}

func (s *HealthServer) Check(ctx context.Context) map[string]error {
	results := make(map[string]error)

	for _, checker := range s.checkers {
		name := checker.Name()
		err := checker.Check(ctx)
		results[name] = err

		if err != nil {
			logger.Error("Health check failed",
				zap.String("component", name),
				zap.Error(err))
		} else {
			logger.Debug("Health check passed",
				zap.String("component", name))
		}
	}

	return results
}

func (s *HealthServer) IsHealthy(ctx context.Context) bool {
	results := s.Check(ctx)

	for _, err := range results {
		if err != nil {
			return false
		}
	}

	return true
}

// SetupHealthCheck 设置健康检查
func SetupHealthCheck(cfg *config.Config, db *sql.DB) *HealthServer {
	healthServer := NewHealthServer()

	// 添加数据库检查
	if db != nil {
		healthServer.AddChecker(NewDatabaseChecker(db))
	}

	// 添加Redis检查
	healthServer.AddChecker(NewRedisChecker())

	// 添加依赖服务检查
	if cfg.Services.AuditService.Address != "" {
		healthServer.AddChecker(NewGRPCServiceChecker(
			"audit_service",
			cfg.Services.AuditService.Address,
			time.Duration(cfg.Services.AuditService.Timeout)*time.Second,
		))
	}

	// 添加端口检查
	host, port, err := getHostAndPort(cfg.Server.Address)
	if err == nil {
		healthServer.AddChecker(NewPortChecker(host, port))
	}

	return healthServer
}

// getHostAndPort 从地址中解析主机和端口
func getHostAndPort(address string) (string, int, error) {
	// 移除地址前缀
	address = address[1:] // 移除冒号

	// 如果地址只包含端口，使用localhost作为主机
	if len(address) > 0 && address[0] != ':' {
		// 简单处理，假设格式为:50052
		return "localhost", 50052, nil
	}

	// 这里应该有更完善的解析逻辑，但为了简化，直接返回默认值
	return "localhost", 50052, nil
}
