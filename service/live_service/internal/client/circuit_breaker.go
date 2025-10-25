package client

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	failCount    int
	lastFailTime time.Time
	isOpen       bool
	mutex        sync.Mutex
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		lastFailTime: time.Now(),
	}
}

// CanExecute 检查是否可以执行请求
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if cb.isOpen {
		// 熔断器开启，检查是否过了冷却时间（30秒）
		if time.Since(cb.lastFailTime) > 30*time.Second {
			cb.isOpen = false
			cb.failCount = 0
			return true
		}
		return false
	}
	return true
}

// RecordSuccess 记录成功
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.failCount = 0
	cb.isOpen = false
}

// RecordFailure 记录失败
func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.failCount++
	cb.lastFailTime = time.Now()

	// 连续失败3次开启熔断器
	if cb.failCount >= 3 {
		cb.isOpen = true
		log.Printf("Circuit breaker opened due to %d consecutive failures", cb.failCount)
	}
}

// GetState 获取熔断器状态
func (cb *CircuitBreaker) GetState() string {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if cb.isOpen {
		return "open"
	} else if cb.failCount > 0 {
		return fmt.Sprintf("half-open (failures: %d)", cb.failCount)
	}
	return "closed"
}
