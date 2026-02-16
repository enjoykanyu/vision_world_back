package routes

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"api_gateway/client"
	"api_gateway/discovery"
	pb "api_gateway/proto/proto_gen/live"

	"github.com/gin-gonic/gin"
)

// LiveHandler 直播处理器
type LiveHandler struct {
	liveClient     *client.LiveServiceClient
	discovery      *discovery.EtcdServiceDiscovery
	etcdEndpoints  []string
	serviceAddr    string
	mu             sync.RWMutex
	lastFailTime   time.Time
	circuitBreaker *CircuitBreaker
}

// NewLiveHandler 创建直播处理器
func NewLiveHandler(etcdEndpoints []string) (*LiveHandler, error) {
	// 创建服务发现客户端
	serviceDiscovery, err := discovery.NewEtcdServiceDiscovery(etcdEndpoints, "live-service")
	if err != nil {
		return nil, err
	}

	handler := &LiveHandler{
		etcdEndpoints:  etcdEndpoints,
		discovery:      serviceDiscovery,
		circuitBreaker: NewCircuitBreaker(),
	}

	// 监听服务变化
	serviceDiscovery.WatchService(handler.onServiceChange)

	return handler, nil
}

// onServiceChange 服务变化处理
func (h *LiveHandler) onServiceChange(serviceAddr string, isAdded bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if isAdded {
		if serviceAddr != h.serviceAddr {
			log.Printf("Live service address changed from %s to %s", h.serviceAddr, serviceAddr)
			h.serviceAddr = serviceAddr

			// 关闭旧连接
			if h.liveClient != nil {
				h.liveClient.Close()
				h.liveClient = nil
			}

			// 重置熔断器
			h.circuitBreaker.RecordSuccess()
		}
	} else {
		log.Printf("Live service instance removed: %s", serviceAddr)
		if serviceAddr == h.serviceAddr {
			h.serviceAddr = ""
			if h.liveClient != nil {
				h.liveClient.Close()
				h.liveClient = nil
			}
		}
	}
}

// getLiveClient 获取直播服务客户端（懒加载）
func (h *LiveHandler) getLiveClient() (*client.LiveServiceClient, error) {
	h.mu.RLock()
	if h.liveClient != nil && h.liveClient.IsConnected() {
		h.mu.RUnlock()
		return h.liveClient, nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	// 双重检查
	if h.liveClient != nil && h.liveClient.IsConnected() {
		return h.liveClient, nil
	}

	// 检查熔断器
	if !h.circuitBreaker.CanExecute() {
		return nil, fmt.Errorf("circuit breaker is open, please try again later")
	}

	// 检查服务地址
	if h.serviceAddr == "" {
		// 尝试发现服务
		serviceAddr, err := h.discovery.DiscoverService()
		if err != nil || serviceAddr == "" {
			h.circuitBreaker.RecordFailure()
			return nil, fmt.Errorf("live service not available: %v", err)
		}
		h.serviceAddr = serviceAddr
	}

	// 创建客户端
	liveClient, err := client.NewLiveServiceClient(h.serviceAddr)
	if err != nil {
		h.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to create live service client: %v", err)
	}

	h.liveClient = liveClient
	h.circuitBreaker.RecordSuccess()
	log.Printf("Successfully created live service client for %s", h.serviceAddr)
	return h.liveClient, nil
}

// StartLive 开始直播
func (h *LiveHandler) StartLive(c *gin.Context) {
	var req pb.StartLiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	liveClient, err := h.getLiveClient()
	if err != nil {
		log.Printf("Failed to get live service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Live service temporarily unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := liveClient.StartLive(ctx, &req)
	if err != nil {
		log.Printf("StartLive error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start live"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": resp,
	})
}

// StopLive 结束直播
func (h *LiveHandler) StopLive(c *gin.Context) {
	var req pb.StopLiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	liveClient, err := h.getLiveClient()
	if err != nil {
		log.Printf("Failed to get live service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Live service temporarily unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := liveClient.StopLive(ctx, &req)
	if err != nil {
		log.Printf("StopLive error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stop live"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": resp,
	})
}

// GetLiveStream 获取直播流信息
// func (h *LiveHandler) GetLiveStream(c *gin.Context) {
// 	streamID := c.Param("id")
// 	if streamID == "" {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Stream ID is required"})
// 		return
// 	}

// 	// 转换streamID为uint64
// 	id, err := strconv.ParseUint(streamID, 10, 64)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid stream ID"})
// 		return
// 	}

// 	req := &pb.GetLiveStreamRequest{
// 		StreamId: id,
// 	}

// 	liveClient, err := h.getLiveClient()
// 	if err != nil {
// 		log.Printf("Failed to get live service client: %v", err)
// 		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Live service temporarily unavailable"})
// 		return
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer cancel()

// 	resp, err := liveClient.GetLiveStream(ctx, req)
// 	if err != nil {
// 		log.Printf("GetLiveStream error: %v", err)
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get live stream"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"code": 0,
// 		"msg":  "success",
// 		"data": resp,
// 	})
// }

// GetLiveList 获取直播列表
func (h *LiveHandler) GetLiveList(c *gin.Context) {
	// 获取查询参数
	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("page_size", "10")
	//status := c.DefaultQuery("status", "")

	// 转换参数
	pageNum, _ := strconv.Atoi(page)
	pageSizeNum, _ := strconv.Atoi(pageSize)

	req := &pb.GetLiveListRequest{
		Page:     int32(pageNum),
		PageSize: int32(pageSizeNum),
		//Status:   status,
	}

	liveClient, err := h.getLiveClient()
	if err != nil {
		log.Printf("Failed to get live service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Live service temporarily unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := liveClient.GetLiveList(ctx, req)
	if err != nil {
		log.Printf("GetLiveList error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get live list"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": resp,
	})
}

// Close 关闭处理器
func (h *LiveHandler) Close() error {
	if h.discovery != nil {
		return h.discovery.Close()
	}
	return nil
}
