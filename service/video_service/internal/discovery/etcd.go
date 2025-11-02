package discovery

import (
	"context"
	"fmt"
	"log"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdDiscovery etcd服务注册与发现
type EtcdDiscovery struct {
	client      *clientv3.Client
	serviceName string
	leaseID     clientv3.LeaseID
}

// EtcdConfig etcd配置
type EtcdConfig struct {
	Endpoints []string `mapstructure:"endpoints"`
	Timeout   int      `mapstructure:"timeout"`
	TTL       int      `mapstructure:"ttl"`
}

// NewEtcdDiscovery 创建etcd服务发现实例
func NewEtcdDiscovery(config *EtcdConfig, serviceName string) (*EtcdDiscovery, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   config.Endpoints,
		DialTimeout: time.Duration(config.Timeout) * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Timeout)*time.Second)
	defer cancel()

	_, err = client.Status(ctx, config.Endpoints[0])
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}

	log.Printf("Successfully connected to etcd: %v", config.Endpoints)

	return &EtcdDiscovery{
		client:      client,
		serviceName: serviceName,
	}, nil
}

// Register 注册服务到etcd
func (d *EtcdDiscovery) Register(ctx context.Context, service *ServiceInfo) error {
	// 创建租约
	ttl := int64(10) // 默认TTL为10秒
	if service.Check != nil && service.Check.Interval != "" {
		if interval, err := time.ParseDuration(service.Check.Interval); err == nil {
			ttl = int64(interval.Seconds()) * 2 // TTL设置为间隔的2倍
		}
	}

	lease, err := d.client.Grant(context.Background(), ttl)
	if err != nil {
		return fmt.Errorf("failed to create lease: %w", err)
	}
	d.leaseID = lease.ID

	// 构造服务键
	serviceKey := fmt.Sprintf("/services/%s/%s", d.serviceName, service.ID)

	// 构造服务值
	serviceValue := fmt.Sprintf("%s:%d", service.Host, service.Port)

	// 注册服务
	_, err = d.client.Put(context.Background(), serviceKey, serviceValue, clientv3.WithLease(lease.ID))
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	// 续约
	ch, kaerr := d.client.KeepAlive(context.Background(), lease.ID)
	if kaerr != nil {
		return fmt.Errorf("failed to keep alive: %w", kaerr)
	}

	// 处理续约响应
	go func() {
		for ka := range ch {
			log.Printf("Service %s lease renewed: %d", d.serviceName, ka.ID)
		}
	}()

	log.Printf("Service %s registered successfully at %s:%d", d.serviceName, service.Host, service.Port)
	return nil
}

// Deregister 注销服务
func (d *EtcdDiscovery) Deregister(ctx context.Context, serviceID string) error {
	if d.leaseID != 0 {
		_, err := d.client.Revoke(context.Background(), d.leaseID)
		if err != nil {
			return fmt.Errorf("failed to revoke lease: %w", err)
		}
		log.Printf("Service %s deregistered successfully", d.serviceName)
	}
	return nil
}

// Discover 发现服务
func (d *EtcdDiscovery) Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	prefix := fmt.Sprintf("/services/%s/", serviceName)
	resp, err := d.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to discover services: %w", err)
	}

	instances := make([]*ServiceInstance, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		// 从键中提取服务ID
		key := string(kv.Key)
		id := key[len(prefix):]

		// 从值中解析主机和端口
		value := string(kv.Value)
		host := ""
		port := 0

		// 简单解析，假设值为 host:port 格式
		if n, err := fmt.Sscanf(value, "%s:%d", &host, &port); n == 2 && err == nil {
			instance := &ServiceInstance{
				ID:       id,
				Name:     serviceName,
				Host:     host,
				Port:     port,
				Status:   "passing", // etcd没有内置健康检查，假设为通过
				LastSeen: time.Now(),
			}
			instances = append(instances, instance)
		}
	}

	return instances, nil
}

// HealthCheck 健康检查
func (d *EtcdDiscovery) HealthCheck(ctx context.Context, serviceID string) error {
	// etcd没有内置的健康检查机制，这里只检查服务是否在etcd中注册
	prefix := fmt.Sprintf("/services/%s/%s", d.serviceName, serviceID)
	resp, err := d.client.Get(ctx, prefix)
	if err != nil {
		return fmt.Errorf("failed to check service health: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return fmt.Errorf("service %s not found", serviceID)
	}

	return nil
}

// Close 关闭etcd客户端
func (d *EtcdDiscovery) Close() error {
	if d.leaseID != 0 {
		_, err := d.client.Revoke(context.Background(), d.leaseID)
		if err != nil {
			log.Printf("Failed to revoke lease: %v", err)
		}
	}
	if d.client != nil {
		return d.client.Close()
	}
	return nil
}
