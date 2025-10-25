package client

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ServiceDiscovery 服务发现接口
type ServiceDiscovery interface {
	DiscoverService(serviceName string) (string, error)
	WatchService(serviceName string, callback func([]string)) error
	Close() error
}

// AuditClientManager 审核服务客户端管理器
type AuditClientManager struct {
	discovery      ServiceDiscovery
	client         AuditServiceClient
	mu             sync.RWMutex
	serviceName    string
	circuitBreaker *CircuitBreaker
}

// NewAuditClientManager 创建审核服务客户端管理器
func NewAuditClientManager(etcdEndpoints []string) (*AuditClientManager, error) {
	// 创建etcd服务发现
	discovery, err := NewEtcdServiceDiscovery(etcdEndpoints)
	if err != nil {
		return nil, fmt.Errorf("failed to create service discovery: %w", err)
	}

	manager := &AuditClientManager{
		discovery:      discovery,
		serviceName:    "audit_service",
		circuitBreaker: NewCircuitBreaker(),
	}

	// 监听服务地址变化
	go func() {
		err := discovery.WatchService("audit_service", func(addresses []string) {
			log.Printf("Service addresses updated: %v", addresses)
			// 清空当前客户端，下次调用时会重新创建
			manager.mu.Lock()
			if manager.client != nil {
				manager.client.Close()
				manager.client = nil
			}
			manager.mu.Unlock()
		})
		if err != nil {
			log.Printf("Failed to watch service: %v", err)
		}
	}()

	return manager, nil
}

// getClient 获取客户端连接（懒加载，带双重检查锁定）
func (m *AuditClientManager) getClient() (AuditServiceClient, error) {
	// 第一次检查（读锁）
	m.mu.RLock()
	if m.client != nil && m.circuitBreaker.IsHealthy() {
		client := m.client
		m.mu.RUnlock()
		return client, nil
	}
	m.mu.RUnlock()

	// 第二次检查（写锁）
	m.mu.Lock()
	defer m.mu.Unlock()

	// 再次检查，防止并发情况下重复创建
	if m.client != nil && m.circuitBreaker.IsHealthy() {
		return m.client, nil
	}

	// 如果熔断器开启，拒绝请求
	if !m.circuitBreaker.IsHealthy() {
		return nil, fmt.Errorf("circuit breaker is open")
	}

	// 发现服务地址
	serviceAddr, err := m.discovery.DiscoverService(m.serviceName)
	if err != nil {
		m.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to discover service: %w", err)
	}

	// 创建新的客户端连接
	newClient, err := NewAuditServiceClient(serviceAddr)
	if err != nil {
		m.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to create audit service client: %w", err)
	}

	m.client = newClient
	m.circuitBreaker.RecordSuccess()
	return m.client, nil
}

// SubmitContent 提交内容审核
func (m *AuditClientManager) SubmitContent(ctx context.Context, req *SubmitContentRequest) (*SubmitContentResponse, error) {
	client, err := m.getClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	if !m.circuitBreaker.IsHealthy() {
		return nil, fmt.Errorf("circuit breaker is open")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.SubmitContent(ctx, req)
	if err != nil {
		m.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to submit content: %w", err)
	}

	m.circuitBreaker.RecordSuccess()
	return resp, nil
}

// GetAuditResult 获取审核结果
func (m *AuditClientManager) GetAuditResult(ctx context.Context, req *GetAuditResultRequest) (*GetAuditResultResponse, error) {
	client, err := m.getClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	if !m.circuitBreaker.IsHealthy() {
		return nil, fmt.Errorf("circuit breaker is open")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.GetAuditResult(ctx, req)
	if err != nil {
		m.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to get audit result: %w", err)
	}

	m.circuitBreaker.RecordSuccess()
	return resp, nil
}

// UpdateAuditStatus 更新审核状态
func (m *AuditClientManager) UpdateAuditStatus(ctx context.Context, req *UpdateAuditStatusRequest) (*UpdateAuditStatusResponse, error) {
	client, err := m.getClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	if !m.circuitBreaker.IsHealthy() {
		return nil, fmt.Errorf("circuit breaker is open")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.UpdateAuditStatus(ctx, req)
	if err != nil {
		m.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to update audit status: %w", err)
	}

	m.circuitBreaker.RecordSuccess()
	return resp, nil
}

// ListAuditRecords 获取审核记录列表
func (m *AuditClientManager) ListAuditRecords(ctx context.Context, req *ListAuditRecordsRequest) (*ListAuditRecordsResponse, error) {
	client, err := m.getClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	if !m.circuitBreaker.IsHealthy() {
		return nil, fmt.Errorf("circuit breaker is open")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.ListAuditRecords(ctx, req)
	if err != nil {
		m.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to list audit records: %w", err)
	}

	m.circuitBreaker.RecordSuccess()
	return resp, nil
}

// AddToWhitelist 添加到白名单
func (m *AuditClientManager) AddToWhitelist(ctx context.Context, req *AddToWhitelistRequest) (*AddToWhitelistResponse, error) {
	client, err := m.getClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	if !m.circuitBreaker.IsHealthy() {
		return nil, fmt.Errorf("circuit breaker is open")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.AddToWhitelist(ctx, req)
	if err != nil {
		m.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to add to whitelist: %w", err)
	}

	m.circuitBreaker.RecordSuccess()
	return resp, nil
}

// RemoveFromWhitelist 从白名单移除
func (m *AuditClientManager) RemoveFromWhitelist(ctx context.Context, req *RemoveFromWhitelistRequest) (*RemoveFromWhitelistResponse, error) {
	client, err := m.getClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	if !m.circuitBreaker.IsHealthy() {
		return nil, fmt.Errorf("circuit breaker is open")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.RemoveFromWhitelist(ctx, req)
	if err != nil {
		m.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to remove from whitelist: %w", err)
	}

	m.circuitBreaker.RecordSuccess()
	return resp, nil
}

// AddToBlacklist 添加到黑名单
func (m *AuditClientManager) AddToBlacklist(ctx context.Context, req *AddToBlacklistRequest) (*AddToBlacklistResponse, error) {
	client, err := m.getClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	if !m.circuitBreaker.IsHealthy() {
		return nil, fmt.Errorf("circuit breaker is open")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.AddToBlacklist(ctx, req)
	if err != nil {
		m.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to add to blacklist: %w", err)
	}

	m.circuitBreaker.RecordSuccess()
	return resp, nil
}

// RemoveFromBlacklist 从黑名单移除
func (m *AuditClientManager) RemoveFromBlacklist(ctx context.Context, req *RemoveFromBlacklistRequest) (*RemoveFromBlacklistResponse, error) {
	client, err := m.getClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	if !m.circuitBreaker.IsHealthy() {
		return nil, fmt.Errorf("circuit breaker is open")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.RemoveFromBlacklist(ctx, req)
	if err != nil {
		m.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to remove from blacklist: %w", err)
	}

	m.circuitBreaker.RecordSuccess()
	return resp, nil
}

// Close 关闭管理器
func (m *AuditClientManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client != nil {
		m.client.Close()
		m.client = nil
	}

	if m.discovery != nil {
		return m.discovery.Close()
	}
	return nil
}
