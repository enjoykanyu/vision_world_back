package discovery

import (
	"context"
	"fmt"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdDiscovery etcd服务发现
type EtcdDiscovery struct {
	client      *clientv3.Client
	lease       clientv3.LeaseID
	serviceName string
}

// NewEtcdDiscovery 创建etcd服务发现
func NewEtcdDiscovery(endpoints []string, serviceName string) (*EtcdDiscovery, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}

	return &EtcdDiscovery{
		client:      client,
		serviceName: serviceName,
	}, nil
}

// Register 注册服务
func (d *EtcdDiscovery) Register(addr string, ttl int64) error {
	ctx := context.Background()

	// 创建租约
	resp, err := d.client.Grant(ctx, ttl)
	if err != nil {
		return fmt.Errorf("failed to create lease: %w", err)
	}
	d.lease = resp.ID

	// 注册服务
	key := fmt.Sprintf("/services/%s/%s", d.serviceName, addr)
	value := addr

	_, err = d.client.Put(ctx, key, value, clientv3.WithLease(d.lease))
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	// 启动心跳
	ch, err := d.client.KeepAlive(ctx, d.lease)
	if err != nil {
		return fmt.Errorf("failed to keep alive: %w", err)
	}

	// 处理心跳响应
	go func() {
		for {
			select {
			case ka := <-ch:
				if ka == nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// Deregister 注销服务
func (d *EtcdDiscovery) Deregister(addr string) error {
	ctx := context.Background()
	key := fmt.Sprintf("/services/%s/%s", d.serviceName, addr)

	_, err := d.client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to deregister service: %w", err)
	}

	return nil
}

// Discover 发现服务
func (d *EtcdDiscovery) Discover(serviceName string) ([]string, error) {
	ctx := context.Background()
	prefix := fmt.Sprintf("/services/%s/", serviceName)

	resp, err := d.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to discover services: %w", err)
	}

	var services []string
	for _, kv := range resp.Kvs {
		services = append(services, string(kv.Value))
	}

	return services, nil
}

// Watch 监听服务变化
func (d *EtcdDiscovery) Watch(serviceName string, callback func([]string)) error {
	ctx := context.Background()
	prefix := fmt.Sprintf("/services/%s/", serviceName)

	watchChan := d.client.Watch(ctx, prefix, clientv3.WithPrefix())

	go func() {
		for watchResp := range watchChan {
			var services []string
			for _, event := range watchResp.Events {
				if event.Type == mvccpb.PUT {
					services = append(services, string(event.Kv.Value))
				}
			}
			if len(services) > 0 {
				callback(services)
			}
		}
	}()

	return nil
}

// Close 关闭连接
func (d *EtcdDiscovery) Close() error {
	if d.lease != 0 {
		ctx := context.Background()
		d.client.Revoke(ctx, d.lease)
	}
	return d.client.Close()
}
