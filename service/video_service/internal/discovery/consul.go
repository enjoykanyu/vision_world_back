package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/consul/api"
)

// ServiceDiscovery 服务发现接口
type ServiceDiscovery interface {
	Register(ctx context.Context, service *ServiceInfo) error
	Deregister(ctx context.Context, serviceID string) error
	Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
	HealthCheck(ctx context.Context, serviceID string) error
	Close() error
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	ID    string            `json:"id"`
	Name  string            `json:"name"`
	Host  string            `json:"host"`
	Port  int               `json:"port"`
	Tags  []string          `json:"tags"`
	Meta  map[string]string `json:"meta"`
	Check *HealthCheck      `json:"check"`
}

// HealthCheck 健康检查配置
type HealthCheck struct {
	GRPC                           string `json:"grpc"`
	HTTP                           string `json:"http"`
	TCP                            string `json:"tcp"`
	Interval                       string `json:"interval"`
	Timeout                        string `json:"timeout"`
	DeregisterCriticalServiceAfter string `json:"deregister_critical_service_after"`
}

// ServiceInstance 服务实例
type ServiceInstance struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Host     string            `json:"host"`
	Port     int               `json:"port"`
	Tags     []string          `json:"tags"`
	Meta     map[string]string `json:"meta"`
	Status   string            `json:"status"`
	LastSeen time.Time         `json:"last_seen"`
}

// ConsulDiscovery Consul服务发现实现
type ConsulDiscovery struct {
	client *api.Client
	config *ConsulConfig
}

// ConsulConfig Consul配置
type ConsulConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Interval int    `mapstructure:"interval"`
	Timeout  int    `mapstructure:"timeout"`
}

// NewConsulDiscovery 创建Consul服务发现
func NewConsulDiscovery(config *ConsulConfig) (*ConsulDiscovery, error) {
	consulConfig := api.DefaultConfig()
	consulConfig.Address = fmt.Sprintf("%s:%d", config.Host, config.Port)

	client, err := api.NewClient(consulConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}

	return &ConsulDiscovery{
		client: client,
		config: config,
	}, nil
}

// Register 注册服务
func (d *ConsulDiscovery) Register(ctx context.Context, service *ServiceInfo) error {
	registration := &api.AgentServiceRegistration{
		ID:      service.ID,
		Name:    service.Name,
		Tags:    service.Tags,
		Meta:    service.Meta,
		Port:    service.Port,
		Address: service.Host,
	}

	// 配置健康检查
	if service.Check != nil {
		check := &api.AgentServiceCheck{}

		if service.Check.GRPC != "" {
			check.GRPC = service.Check.GRPC
			check.GRPCUseTLS = false
		} else if service.Check.HTTP != "" {
			check.HTTP = service.Check.HTTP
		} else if service.Check.TCP != "" {
			check.TCP = service.Check.TCP
		}

		if service.Check.Interval != "" {
			interval, err := time.ParseDuration(service.Check.Interval)
			if err != nil {
				return fmt.Errorf("invalid check interval: %w", err)
			}
			check.Interval = interval.String()
		}

		if service.Check.Timeout != "" {
			timeout, err := time.ParseDuration(service.Check.Timeout)
			if err != nil {
				return fmt.Errorf("invalid check timeout: %w", err)
			}
			check.Timeout = timeout.String()
		}

		if service.Check.DeregisterCriticalServiceAfter != "" {
			deregisterAfter, err := time.ParseDuration(service.Check.DeregisterCriticalServiceAfter)
			if err != nil {
				return fmt.Errorf("invalid deregister after: %w", err)
			}
			check.DeregisterCriticalServiceAfter = deregisterAfter.String()
		}

		registration.Check = check
	}

	if err := d.client.Agent().ServiceRegister(registration); err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	return nil
}

// Deregister 注销服务
func (d *ConsulDiscovery) Deregister(ctx context.Context, serviceID string) error {
	if err := d.client.Agent().ServiceDeregister(serviceID); err != nil {
		return fmt.Errorf("failed to deregister service: %w", err)
	}
	return nil
}

// Discover 发现服务
func (d *ConsulDiscovery) Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	services, _, err := d.client.Health().Service(serviceName, "", true, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to discover service: %w", err)
	}

	instances := make([]*ServiceInstance, 0, len(services))
	for _, service := range services {
		instance := &ServiceInstance{
			ID:       service.Service.ID,
			Name:     service.Service.Service,
			Host:     service.Service.Address,
			Port:     service.Service.Port,
			Tags:     service.Service.Tags,
			Meta:     service.Service.Meta,
			Status:   service.Checks.AggregatedStatus(),
			LastSeen: time.Now(),
		}
		instances = append(instances, instance)
	}

	return instances, nil
}

// HealthCheck 健康检查
func (d *ConsulDiscovery) HealthCheck(ctx context.Context, serviceID string) error {
	checks, _, err := d.client.Health().Checks(serviceID, nil)
	if err != nil {
		return fmt.Errorf("failed to get health checks: %w", err)
	}

	for _, check := range checks {
		if check.Status != api.HealthPassing {
			return fmt.Errorf("service health check failed: %s", check.Status)
		}
	}

	return nil
}

// Close 关闭连接
func (d *ConsulDiscovery) Close() error {
	// Consul client 不需要显式关闭
	return nil
}
