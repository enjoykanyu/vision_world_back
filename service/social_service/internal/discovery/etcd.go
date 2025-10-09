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

// NewEtcdDiscovery 创建etcd服务发现实例
func NewEtcdDiscovery(endpoints []string, serviceName string) (*EtcdDiscovery, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = client.Status(ctx, endpoints[0])
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}

	log.Printf("Successfully connected to etcd: %v", endpoints)

	return &EtcdDiscovery{
		client:      client,
		serviceName: serviceName,
	}, nil
}

// Register 注册服务到etcd
func (d *EtcdDiscovery) Register(serviceAddr string, ttl int64) error {
	// 创建租约
	lease, err := d.client.Grant(context.Background(), ttl)
	if err != nil {
		return fmt.Errorf("failed to create lease: %w", err)
	}
	d.leaseID = lease.ID

	// 构造服务键
	serviceKey := fmt.Sprintf("/services/%s/%s", d.serviceName, serviceAddr)

	// 注册服务
	_, err = d.client.Put(context.Background(), serviceKey, serviceAddr, clientv3.WithLease(lease.ID))
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

	log.Printf("Service %s registered successfully at %s", d.serviceName, serviceAddr)
	return nil
}

// Deregister 注销服务
func (d *EtcdDiscovery) Deregister() error {
	if d.leaseID != 0 {
		_, err := d.client.Revoke(context.Background(), d.leaseID)
		if err != nil {
			return fmt.Errorf("failed to revoke lease: %w", err)
		}
		log.Printf("Service %s deregistered successfully", d.serviceName)
	}
	return nil
}

// Close 关闭etcd客户端
func (d *EtcdDiscovery) Close() error {
	if err := d.Deregister(); err != nil {
		log.Printf("Failed to deregister service: %v", err)
	}
	if d.client != nil {
		return d.client.Close()
	}
	return nil
}
