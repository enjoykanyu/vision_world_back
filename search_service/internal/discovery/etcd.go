package discovery

import (
	"context"
	"fmt"
	"log"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdDiscovery etcd服务发现
type EtcdDiscovery struct {
	client      *clientv3.Client
	serviceName string
	leaseID     clientv3.LeaseID
}

// NewEtcdDiscovery 创建新的etcd服务发现实例
func NewEtcdDiscovery(endpoints []string, serviceName string) (*EtcdDiscovery, error) {
	// 创建etcd客户端
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}

	return &EtcdDiscovery{
		client:      cli,
		serviceName: serviceName,
	}, nil
}

// Register 注册服务到etcd
func (e *EtcdDiscovery) Register(serviceAddr string, ttl int64) error {
	// 创建租约
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	leaseResp, err := e.client.Grant(ctx, ttl)
	if err != nil {
		return fmt.Errorf("failed to create lease: %w", err)
	}

	e.leaseID = leaseResp.ID

	// 注册服务
	key := fmt.Sprintf("/services/%s/%s", e.serviceName, serviceAddr)
	_, err = e.client.Put(ctx, key, serviceAddr, clientv3.WithLease(leaseResp.ID))
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	// 启动心跳保持
	go e.keepAlive(ttl)

	log.Printf("Service registered to etcd: %s -> %s", key, serviceAddr)
	return nil
}

// keepAlive 保持与etcd的心跳
func (e *EtcdDiscovery) keepAlive(ttl int64) {
	ticker := time.NewTicker(time.Duration(ttl-1) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := e.client.KeepAliveOnce(ctx, e.leaseID)
			cancel()

			if err != nil {
				log.Printf("Failed to keep alive: %v", err)
				// 如果心跳失败，尝试重新注册
				// 这里可以添加重试逻辑
			}
		}
	}
}

// Close 关闭etcd连接
func (e *EtcdDiscovery) Close() error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}
