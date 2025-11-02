package discovery

import (
	"fmt"
	"strings"

	"github.com/vision_world/video_service/internal/config"
)

// NewServiceDiscovery 根据配置创建服务发现实例
func NewServiceDiscovery(cfg *config.DiscoveryConfig, serviceName string) (ServiceDiscovery, error) {
	switch strings.ToLower(cfg.Type) {
	case "consul":
		// 解析consul地址，格式为 host:port
		parts := strings.Split(cfg.Address, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid consul address format, expected host:port, got %s", cfg.Address)
		}

		consulConfig := &ConsulConfig{
			Host:     parts[0],
			Port:     0, // 将在下面解析
			Interval: cfg.Interval,
			Timeout:  5, // 默认超时时间5秒
		}

		// 解析端口
		if _, err := fmt.Sscanf(parts[1], "%d", &consulConfig.Port); err != nil {
			return nil, fmt.Errorf("invalid consul port: %s", parts[1])
		}

		return NewConsulDiscovery(consulConfig)

	case "etcd":
		// etcd地址可能是单个地址或多个地址，用逗号分隔
		endpoints := strings.Split(cfg.Address, ",")
		for i, endpoint := range endpoints {
			endpoints[i] = strings.TrimSpace(endpoint)
		}

		etcdConfig := &EtcdConfig{
			Endpoints: endpoints,
			Timeout:   5, // 默认超时时间5秒
			TTL:       cfg.Interval,
		}

		return NewEtcdDiscovery(etcdConfig, serviceName)

	default:
		return nil, fmt.Errorf("unsupported discovery type: %s", cfg.Type)
	}
}
