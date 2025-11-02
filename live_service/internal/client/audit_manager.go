package client

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	pb "live_service/proto/proto_gen/audit"
)

// AuditClientManager 审计服务客户端管理器
type AuditClientManager struct {
	discovery      *EtcdServiceDiscovery
	client         *AuditServiceClient
	etcdEndpoints  []string
	serviceAddr    string
	mu             sync.RWMutex
	lastFailTime   time.Time
	circuitBreaker *CircuitBreaker
}

// NewAuditClientManager 创建审计服务客户端管理器
func NewAuditClientManager(etcdEndpoints []string) (*AuditClientManager, error) {
	// 创建服务发现客户端
	serviceDiscovery, err := NewEtcdServiceDiscovery(etcdEndpoints, "audit-service")
	if err != nil {
		return nil, err
	}

	manager := &AuditClientManager{
		etcdEndpoints:  etcdEndpoints,
		discovery:      serviceDiscovery,
		circuitBreaker: NewCircuitBreaker(),
	}

	// 监听服务变化
	serviceDiscovery.WatchService(manager.onServiceChange)

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

// getClient 获取审计服务客户端（懒加载）
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
		return nil, fmt.Errorf("circuit breaker is open, please try again later (state: %s)", m.circuitBreaker.GetState())
	}

	// 检查服务地址
	if m.serviceAddr == "" {
		// 尝试发现服务
		serviceAddr, err := m.discovery.DiscoverService()
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
	log.Printf("Successfully created audit service client for %s", m.serviceAddr)
	return m.client, nil
}

// SubmitContent 提交内容审核 - 使用pb_gen类型
func (m *AuditClientManager) SubmitContent(ctx context.Context, req interface{}) (interface{}, error) {
	client, err := m.getClient()
	if err != nil {
		return nil, err
	}

	// 类型转换 - 期望传入的是*pb.SubmitContentRequest
	submitReq, ok := req.(*pb.SubmitContentRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type: expected *pb.SubmitContentRequest")
	}

	// 直接调用pb_gen生成的函数
	return client.SubmitContent(ctx, submitReq)
}

// GetAuditResult 获取审核结果 - 使用pb_gen类型
func (m *AuditClientManager) GetAuditResult(ctx context.Context, req *pb.GetAuditResultRequest) (*pb.GetAuditResultResponse, error) {
	client, err := m.getClient()
	if err != nil {
		return nil, err
	}
	// 直接调用pb_gen生成的函数
	return client.GetAuditResult(ctx, req)
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
		m.discovery.Close()
	}

	return nil
}
