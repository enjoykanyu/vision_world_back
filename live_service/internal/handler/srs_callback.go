package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"live_service/internal/service"
	"live_service/pkg/logger"
)

// SRSCallbackHandler SRS回调处理器
type SRSCallbackHandler struct {
	liveService service.LiveService
	hub         *service.Hub
	logger      logger.Logger
}

// NewSRSCallbackHandler 创建SRS回调处理器
func NewSRSCallbackHandler(liveService service.LiveService, hub *service.Hub, log logger.Logger) *SRSCallbackHandler {
	return &SRSCallbackHandler{
		liveService: liveService,
		hub:         hub,
		logger:      log,
	}
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

// HandleCallback 处理SRS回调
func (h *SRSCallbackHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SRSCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode SRS callback request", "error", err)
		respondWithJSON(w, SRSCallbackResponse{Code: 1, Data: "invalid request"})
		return
	}

	h.logger.Info("Received SRS callback",
		"action", req.Action,
		"stream", req.Stream,
		"clientID", req.ClientID)

	// 根据动作类型处理
	switch req.Action {
	case "on_publish":
		// 开始推流
		h.handleOnPublish(&req)
	case "on_unpublish":
		// 停止推流
		h.handleOnUnpublish(&req)
	case "on_play":
		// 开始播放
		h.handleOnPlay(&req)
	case "on_stop":
		// 停止播放
		h.handleOnStop(&req)
	default:
		h.logger.Warn("Unknown SRS callback action", "action", req.Action)
	}

	respondWithJSON(w, SRSCallbackResponse{Code: 0})
}

// handleOnPublish 处理开始推流回调
func (h *SRSCallbackHandler) handleOnPublish(req *SRSCallbackRequest) {
	h.logger.Info("Stream started publishing", "stream", req.Stream)

	// TODO: 根据streamKey查找直播流，更新状态为"直播中"
	// 可以调用 liveService.UpdateStreamStatus 方法
}

// handleOnUnpublish 处理停止推流回调
func (h *SRSCallbackHandler) handleOnUnpublish(req *SRSCallbackRequest) {
	h.logger.Info("Stream stopped publishing", "stream", req.Stream)

	// 1. 根据streamKey查找直播流
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 从streamKey获取直播流信息
	stream, err := h.liveService.GetLiveStreamByStreamKey(ctx, req.Stream)
	if err != nil {
		h.logger.Error("Failed to get live stream by stream key", "stream", req.Stream, "error", err)
		return
	}

	if stream == nil {
		h.logger.Warn("Live stream not found for stream key", "stream", req.Stream)
		return
	}

	// 2. 发送WebSocket下播通知给所有观众
	if h.hub != nil {
		roomIDStr := fmt.Sprintf("%d", stream.RoomID)
		streamEndMsg := &service.ChatMessage{
			Type:      "stream_end",
			UserID:    "system",
			Username:  "系统",
			Avatar:    "",
			Content:   "主播已下播",
			RoomID:    roomIDStr,
			Timestamp: time.Now().Unix(),
		}
		h.hub.Broadcast <- streamEndMsg
		h.logger.Info("Stream end notification sent via WebSocket", "roomID", stream.RoomID)
	}

	// 3. 更新直播流状态为已结束
	// 注意：这里不直接调用StopLive，因为StopLive需要userID验证
	// 实际业务中可以通过streamKey找到对应的userID然后调用StopLive
	h.logger.Info("Stream unpublished handled", "streamID", stream.ID, "roomID", stream.RoomID)
}

// handleOnPlay 处理开始播放回调
func (h *SRSCallbackHandler) handleOnPlay(req *SRSCallbackRequest) {
	h.logger.Debug("Stream started playing", "stream", req.Stream, "clientID", req.ClientID)

	// TODO: 更新观看人数统计
}

// handleOnStop 处理停止播放回调
func (h *SRSCallbackHandler) handleOnStop(req *SRSCallbackRequest) {
	h.logger.Debug("Stream stopped playing", "stream", req.Stream, "clientID", req.ClientID)

	// TODO: 更新观看人数统计
}

// respondWithJSON 返回JSON响应
func respondWithJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
