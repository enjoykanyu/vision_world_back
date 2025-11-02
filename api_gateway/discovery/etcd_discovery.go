package discovery

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdServiceDiscovery etcd服务发现
type EtcdServiceDiscovery struct {
	client      *clientv3.Client
	serviceName string
}

// NewEtcdServiceDiscovery 创建etcd服务发现实例
func NewEtcdServiceDiscovery(endpoints []string, serviceName string) (*EtcdServiceDiscovery, error) {
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
		client:      client,
		serviceName: serviceName,
	}, nil
}

// DiscoverService 发现服务实例
func (d *EtcdServiceDiscovery) DiscoverService() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 构造服务键前缀
	keyPrefix := fmt.Sprintf("/services/%s/", d.serviceName)

	// 获取服务实例
	getResp, err := d.client.Get(ctx, keyPrefix, clientv3.WithPrefix())
	if err != nil {
		return "", fmt.Errorf("failed to get service instances: %w", err)
	}

	if len(getResp.Kvs) == 0 {
		return "", fmt.Errorf("no available instances for service: %s", d.serviceName)
	}

	// 简单负载均衡：返回第一个可用实例
	for _, kv := range getResp.Kvs {
		serviceAddr := string(kv.Value)
		if serviceAddr != "" {
			log.Printf("Discovered service %s at: %s", d.serviceName, serviceAddr)
			return serviceAddr, nil
		}
	}

	return "", fmt.Errorf("no valid service address found for: %s", d.serviceName)
}

// WatchService 监听服务变化
func (d *EtcdServiceDiscovery) WatchService(callback func(string, bool)) {
	keyPrefix := fmt.Sprintf("/services/%s/", d.serviceName)

	watchChan := d.client.Watch(context.Background(), keyPrefix, clientv3.WithPrefix())

	go func() {
		for watchResp := range watchChan {
			for _, event := range watchResp.Events {
				serviceAddr := string(event.Kv.Value)
				switch event.Type {
				case clientv3.EventTypePut:
					log.Printf("Service %s instance added/updated: %s", d.serviceName, serviceAddr)
					callback(serviceAddr, true)
				case clientv3.EventTypeDelete:
					// 从key中提取服务地址
					key := string(event.Kv.Key)
					parts := strings.Split(key, "/")
					if len(parts) > 0 {
						addr := parts[len(parts)-1]
						log.Printf("Service %s instance removed: %s", d.serviceName, addr)
						callback(addr, false)
					}
				}
			}
		}
	}()
}

// Close 关闭etcd客户端
func (d *EtcdServiceDiscovery) Close() error {
	if d.client != nil {
		return d.client.Close()
	}
	return nil
}
