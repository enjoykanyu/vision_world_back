package client

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// ServiceDiscovery 服务发现接口
type ServiceDiscovery interface {
	DiscoverService(serviceName string) (string, error)
	WatchService(serviceName string, callback func(string, bool))
	Close() error
}

// EtcdServiceDiscovery etcd服务发现实现
type EtcdServiceDiscovery struct {
	client   *clientv3.Client
	mu       sync.RWMutex
	watchers map[string]context.CancelFunc
}

// NewEtcdServiceDiscovery 创建etcd服务发现实例
func NewEtcdServiceDiscovery(endpoints []string) (*EtcdServiceDiscovery, error) {
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

	return &EtcdServiceDiscovery{
		client:   client,
		watchers: make(map[string]context.CancelFunc),
	}, nil
}

// DiscoverService 发现服务实例
func (d *EtcdServiceDiscovery) DiscoverService(serviceName string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 构造服务键前缀
	keyPrefix := fmt.Sprintf("/services/%s/", serviceName)

	// 获取服务实例
	getResp, err := d.client.Get(ctx, keyPrefix, clientv3.WithPrefix())
	if err != nil {
		return "", fmt.Errorf("failed to get service instances: %w", err)
	}

	if len(getResp.Kvs) == 0 {
		return "", fmt.Errorf("no available instances for service: %s", serviceName)
	}

	// 简单负载均衡：返回第一个可用实例
	for _, kv := range getResp.Kvs {
		serviceAddr := string(kv.Value)
		if serviceAddr != "" {
			log.Printf("Discovered service %s at: %s", serviceName, serviceAddr)
			return serviceAddr, nil
		}
	}

	return "", fmt.Errorf("no valid service address found for: %s", serviceName)
}

// WatchService 监听服务变化
func (d *EtcdServiceDiscovery) WatchService(serviceName string, callback func(string, bool)) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 如果已经存在该服务的监听器，先停止它
	if cancel, exists := d.watchers[serviceName]; exists {
		cancel()
	}

	keyPrefix := fmt.Sprintf("/services/%s/", serviceName)
	ctx, cancel := context.WithCancel(context.Background())
	d.watchers[serviceName] = cancel

	watchChan := d.client.Watch(ctx, keyPrefix, clientv3.WithPrefix())

	go func() {
		for watchResp := range watchChan {
			for _, event := range watchResp.Events {
				serviceAddr := string(event.Kv.Value)
				switch event.Type {
				case mvccpb.PUT:
					log.Printf("Service %s instance added/updated: %s", serviceName, serviceAddr)
					callback(serviceAddr, true)
				case mvccpb.DELETE:
					log.Printf("Service %s instance removed: %s", serviceName, serviceAddr)
					callback(serviceAddr, false)
				}
			}
		}
	}()
}

// StopWatching 停止监听指定服务
func (d *EtcdServiceDiscovery) StopWatching(serviceName string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if cancel, exists := d.watchers[serviceName]; exists {
		cancel()
		delete(d.watchers, serviceName)
		log.Printf("Stopped watching service: %s", serviceName)
	}
}

// Close 关闭etcd客户端
func (d *EtcdServiceDiscovery) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 停止所有监听器
	for serviceName, cancel := range d.watchers {
		cancel()
		log.Printf("Stopped watching service: %s", serviceName)
	}
	d.watchers = make(map[string]context.CancelFunc)

	if d.client != nil {
		return d.client.Close()
	}
	return nil
}
