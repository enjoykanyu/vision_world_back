package routes

import (
	"api_gateway/middleware"
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
		log.Println("StartLive error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start live"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"room_id":     resp.GetRoomId(),
			"stream_key":  resp.GetStreamKey(),
			"push_url":    resp.GetPushUrl(),
			"play_url":    resp.GetPlayUrl(),
			"flv_url":     resp.GetFlvUrl(),
			"webrtc_url":  resp.GetWebrtcUrl(),
			"is_new_room": resp.GetIsNewRoom(),
		},
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

// GetRoomInfo 获取直播间信息
func (h *LiveHandler) GetRoomInfo(c *gin.Context) {
	roomID := c.Param("id")
	if roomID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Room ID is required"})
		return
	}

	// 转换roomID为uint64
	id, err := strconv.ParseUint(roomID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	req := &pb.GetRoomInfoRequest{
		RoomId: id,
	}

	liveClient, err := h.getLiveClient()
	if err != nil {
		log.Printf("Failed to get live service client: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Live service temporarily unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := liveClient.GetRoomInfo(ctx, req)
	if err != nil {
		log.Printf("GetRoomInfo error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get room info"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"room": resp.GetRoom(),
		},
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

// SRSCallbackRequest SRS回调请求
type SRSCallbackRequest struct {
	Action   string `json:"action"`    // 动作: on_publish, on_unpublish, on_play, on_stop
	ClientID string `json:"client_id"` // 客户端ID
	IP       string `json:"ip"`        // 客户端IP
	VHost    string `json:"vhost"`     // 虚拟主机
	App      string `json:"app"`       // 应用名
	Stream   string `json:"stream"`    // 流名（即streamKey）
	Param    string `json:"param"`     // 参数
	PageURL  string `json:"pageUrl"`   // 页面URL
	SWFURL   string `json:"swfUrl"`    // SWF URL
	TCURL    string `json:"tcUrl"`     // TC URL
	URL      string `json:"url"`       // 完整URL
}

// SRSCallbackResponse SRS回调响应
type SRSCallbackResponse struct {
	Code int    `json:"code"` // 0=成功, 其他=失败
	Data string `json:"data"` // 可选数据
}

// SRSCallback 处理SRS回调
func (h *LiveHandler) SRSCallback(c *gin.Context) {
	var req SRSCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Failed to decode SRS callback request: %v", err)
		c.JSON(http.StatusOK, SRSCallbackResponse{Code: 1, Data: "invalid request"})
		return
	}

	log.Printf("Received SRS callback: action=%s, stream=%s, clientID=%s", req.Action, req.Stream, req.ClientID)

	// 根据动作类型处理
	switch req.Action {
	case "on_publish":
		// 开始推流 - 可以在这里更新直播状态为"直播中"
		log.Printf("Stream started publishing: %s", req.Stream)
	case "on_unpublish":
		// 停止推流 - 可以在这里更新直播状态为"已结束"
		log.Printf("Stream stopped publishing: %s", req.Stream)
	case "on_play":
		// 开始播放
		log.Printf("Stream started playing: %s", req.Stream)
	case "on_stop":
		// 停止播放
		log.Printf("Stream stopped playing: %s", req.Stream)
	default:
		log.Printf("Unknown SRS callback action: %s", req.Action)
	}

	c.JSON(http.StatusOK, SRSCallbackResponse{Code: 0})
}

// Close 关闭处理器
func (h *LiveHandler) Close() error {
	if h.discovery != nil {
		return h.discovery.Close()
	}
	return nil
}

func RegisterLiveRoutesWithHandler(router *gin.Engine, liveHandler *LiveHandler) {
	// 分片上传路由组 - 需要认证
	liveGroup := router.Group("/api/live")
	// SRS 回调路由（不需要认证，供SRS服务器调用）
	liveGroup.POST("/srs/callback", liveHandler.SRSCallback)
	liveGroup.Use(middleware.RequireAuthMiddleware())
	{
		liveGroup.POST("/start", liveHandler.StartLive)
		liveGroup.POST("/stop", liveHandler.StopLive)
		liveGroup.GET("/room/:id", liveHandler.GetRoomInfo)
		// api.GET("/live/stream/:id", r.liveHandler.GetLiveStream)
		liveGroup.GET("/list", liveHandler.GetLiveList)

	}

}
