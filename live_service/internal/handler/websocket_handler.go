package handler

import (
	"net/http"
	"strconv"

	"live_service/internal/service"
	"live_service/pkg/logger"
)

// WebSocketHandler WebSocket处理器
type WebSocketHandler struct {
	hub    *service.Hub
	logger logger.Logger
}

// NewWebSocketHandler 创建WebSocket处理器
func NewWebSocketHandler(hub *service.Hub, log logger.Logger) *WebSocketHandler {
	return &WebSocketHandler{
		hub:    hub,
		logger: log,
	}
}

// enableCORS 启用跨域支持
func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	w.Header().Set("Access-Control-Max-Age", "86400")
}

// handleCORSOptions 处理 CORS 预检请求
func handleCORSOptions(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == "OPTIONS" {
		enableCORS(w)
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}

// HandleChat WebSocket聊天连接处理
// 接口地址: ws://localhost:8088/ws/chat?room_id=xxx&user_id=xxx&username=xxx
func (h *WebSocketHandler) HandleChat(w http.ResponseWriter, r *http.Request) {
	// 获取请求参数
	roomID := r.URL.Query().Get("room_id")
	userID := r.URL.Query().Get("user_id")
	username := r.URL.Query().Get("username")
	avatar := r.URL.Query().Get("avatar")

	// 参数验证
	if roomID == "" || userID == "" {
		http.Error(w, "Missing required parameters: room_id, user_id", http.StatusBadRequest)
		return
	}

	if username == "" {
		username = "用户" + userID
	}

	h.logger.Info("WebSocket connection request",
		"roomID", roomID,
		"userID", userID,
		"username", username,
		"remoteAddr", r.RemoteAddr)

	// 升级为WebSocket连接
	h.hub.HandleWebSocket(w, r, userID, roomID, username, avatar)
}

// GetRoomStats 获取房间统计信息
// 接口地址: GET /api/chat/stats?room_id=xxx
func (h *WebSocketHandler) GetRoomStats(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room_id")
	if roomID == "" {
		http.Error(w, "Missing room_id parameter", http.StatusBadRequest)
		return
	}

	stats := h.hub.GetRoomStats(roomID)
	if stats == nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	// 返回JSON格式统计信息
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{
		"room_id": "` + roomID + `",
		"online_count": ` + strconv.Itoa(stats.ConnectionCount) + `,
		"max_online_count": ` + strconv.FormatInt(stats.MaxClientCount, 10) + `,
		"message_count": ` + strconv.FormatInt(stats.MessageCount, 10) + `,
		"last_active_time": "` + stats.LastActiveTime.Format("2006-01-02 15:04:05") + `"
	}`))
}

// GetHubStats 获取Hub全局统计信息
// 接口地址: GET /api/chat/hub/stats
func (h *WebSocketHandler) GetHubStats(w http.ResponseWriter, r *http.Request) {
	stats := h.hub.GetHubStats()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{
		"total_connections": ` + strconv.FormatInt(stats.TotalConnections, 10) + `,
		"total_rooms": ` + strconv.Itoa(int(stats.TotalRooms)) + `,
		"total_messages": ` + strconv.FormatUint(stats.TotalMessages, 10) + `,
		"start_time": "` + stats.StartTime.Format("2006-01-02 15:04:05") + `"
	}`))
}

// GetOnlineUsers 获取房间在线观众列表
// 接口地址: GET /api/chat/online-users?room_id=xxx
func (h *WebSocketHandler) GetOnlineUsers(w http.ResponseWriter, r *http.Request) {
	// 处理 CORS 预检请求
	if handleCORSOptions(w, r) {
		return
	}

	// 启用 CORS
	enableCORS(w)

	roomID := r.URL.Query().Get("room_id")
	if roomID == "" {
		http.Error(w, "Missing room_id parameter", http.StatusBadRequest)
		return
	}

	users := h.hub.GetOnlineUsers(roomID)

	// 构建JSON响应
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"code": 0, "data": ` + h.formatOnlineUsers(users) + `}`))
}

// formatOnlineUsers 格式化在线用户列表为JSON
func (h *WebSocketHandler) formatOnlineUsers(users []*service.OnlineUser) string {
	if len(users) == 0 {
		return "[]"
	}

	result := "["
	for i, user := range users {
		if i > 0 {
			result += ","
		}
		result += `{"user_id":"` + user.UserID + `","username":"` + user.Username + `","avatar":"` + user.Avatar + `"}`
	}
	result += "]"
	return result
}

// SendMessage 发送消息（HTTP接口，用于测试或系统消息）
// 接口地址: POST /api/chat/send
func (h *WebSocketHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析请求体
	r.ParseForm()
	roomID := r.FormValue("room_id")
	userID := r.FormValue("user_id")
	username := r.FormValue("username")
	avatar := r.FormValue("avatar")
	content := r.FormValue("content")
	msgType := r.FormValue("type")

	if roomID == "" || content == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	if msgType == "" {
		msgType = "message"
	}

	// 发送消息
	if err := h.hub.SendMessage(userID, username, avatar, roomID, content, msgType); err != nil {
		h.logger.Error("Failed to send message", "error", err)
		http.Error(w, "Failed to send message", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"code": 0, "message": "发送成功"}`))
}
