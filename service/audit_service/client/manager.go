package client

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// AuditClientManager 审核服务客户端管理器
type AuditClientManager struct {
	discovery      ServiceDiscovery
	client         AuditServiceClient // 使用接口类型
	mu             sync.RWMutex
	serviceName    string
	circuitBreaker *CircuitBreaker
}

// NewAuditClientManager 创建审核服务客户端管理器
func NewAuditClientManager(etcdEndpoints []string) (*AuditClientManager, error) {
	// 创建服务发现客户端
	discovery, err := NewEtcdServiceDiscovery(etcdEndpoints)
	if err != nil {
		return nil, fmt.Errorf("failed to create service discovery: %w", err)
	}

	manager := &AuditClientManager{
		etcdEndpoints:  etcdEndpoints,
		discovery:      discovery,
		serviceName:    "audit-service",
		circuitBreaker: NewCircuitBreaker(),
	}

	// 监听服务变化
	discovery.WatchService(manager.serviceName, manager.onServiceChange)

	return manager, nil
}

// onServiceChange 服务变化处理
func (m *AuditClientManager) onServiceChange(serviceAddr string, isAdded bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if isAdded {
		if serviceAddr != m.serviceAddr {
			log.Printf("Audit service address changed from %s to %s", m.serviceAddr, serviceAddr)
			m.serviceAddr = serviceAddr

			// 关闭旧连接
			if m.client != nil {
				m.client.Close()
				m.client = nil
			}

			// 重置熔断器
			m.circuitBreaker.RecordSuccess()
		}
	} else {
		log.Printf("Audit service instance removed: %s", serviceAddr)
		if serviceAddr == m.serviceAddr {
			m.serviceAddr = ""
			if m.client != nil {
				m.client.Close()
				m.client = nil
			}
		}
	}
}

// getClient 获取审核服务客户端（懒加载，带熔断检查）
func (m *AuditClientManager) getClient() (*AuditServiceClient, error) {
	m.mu.RLock()
	if m.client != nil && m.client.IsConnected() {
		m.mu.RUnlock()
		return m.client, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查
	if m.client != nil && m.client.IsConnected() {
		return m.client, nil
	}

	// 检查熔断器
	if !m.circuitBreaker.CanExecute() {
		return nil, fmt.Errorf("circuit breaker is open, please try again later")
	}

	// 检查服务地址
	if m.serviceAddr == "" {
		// 尝试发现服务
		serviceAddr, err := m.discovery.DiscoverService(m.serviceName)
		if err != nil || serviceAddr == "" {
			m.circuitBreaker.RecordFailure()
			return nil, fmt.Errorf("audit service not available: %v", err)
		}
		m.serviceAddr = serviceAddr
	}

	// 创建客户端
	client, err := NewAuditServiceClient(m.serviceAddr)
	if err != nil {
		m.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to create audit service client: %v", err)
	}

	m.client = client
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
