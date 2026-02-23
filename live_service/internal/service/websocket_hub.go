package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"live_service/internal/config"
	"live_service/pkg/logger"
)

// ChatMessage 聊天消息结构
type ChatMessage struct {
	Type      string `json:"type"`      // message, gift, like, system, enter, leave
	UserID    string `json:"user_id"`   // 用户ID
	Username  string `json:"username"`  // 用户名
	Avatar    string `json:"avatar"`    // 头像URL
	Content   string `json:"content"`   // 消息内容
	RoomID    string `json:"room_id"`   // 房间ID
	Timestamp int64  `json:"timestamp"` // 时间戳
	SeqID     int64  `json:"seq_id"`    // 序列号

	// 礼物相关
	GiftID   int    `json:"gift_id,omitempty"`   // 礼物ID
	GiftName string `json:"gift_name,omitempty"` // 礼物名称
	Price    int    `json:"price,omitempty"`     // 礼物价格
	Count    int    `json:"count,omitempty"`     // 礼物数量
}

// Client WebSocket客户端
type Client struct {
	Hub      *Hub
	Conn     *websocket.Conn
	Send     chan []byte
	RoomID   string
	UserID   string
	Username string
	Avatar   string
}

// Room 房间
type Room struct {
	ID         string
	Hub        *Hub
	Clients    map[*Client]bool
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client
	stats      *RoomStats
	statsMux   sync.RWMutex
}

// RoomStats 房间统计
type RoomStats struct {
	MessageCount    int64     `json:"message_count"`
	ConnectionCount int       `json:"connection_count"`
	ClientCount     int64     `json:"client_count"`
	MaxClientCount  int64     `json:"max_client_count"`
	LastActiveTime  time.Time `json:"last_active_time"`
}

// Hub WebSocket Hub
type Hub struct {
	Config     *config.Config
	Logger     logger.Logger
	Rooms      map[string]*Room
	RoomsMux   sync.RWMutex
	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan *ChatMessage
	StopChan   chan struct{}
	wg         sync.WaitGroup

	// 消息序列号生成器
	seqMux sync.Mutex
	seqID  int64

	// RabbitMQ 生产者
	Producer *RabbitProducer

	// RabbitMQ 消费者
	Consumer *RabbitConsumer
}

// HubStats Hub统计
type HubStats struct {
	TotalConnections int64     `json:"total_connections"`
	TotalRooms       int       `json:"total_rooms"`
	TotalMessages    uint64    `json:"total_messages"`
	StartTime        time.Time `json:"start_time"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// NewHub 创建新的Hub
func NewHub(cfg *config.Config, logger logger.Logger, redisClient interface{}) *Hub {
	return &Hub{
		Config:     cfg,
		Logger:     logger,
		Rooms:      make(map[string]*Room),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan *ChatMessage, 256),
		StopChan:   make(chan struct{}),
	}
}

// buildAMQPURL 构建RabbitMQ连接URL
func buildAMQPURL(cfg *config.Config) string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d%s",
		cfg.RabbitMQ.Username,
		cfg.RabbitMQ.Password,
		cfg.RabbitMQ.Host,
		cfg.RabbitMQ.Port,
		cfg.RabbitMQ.VHost,
	)
}

// Start 启动Hub
func (h *Hub) Start() error {
	amqpURL := buildAMQPURL(h.Config)

	// 启动RabbitMQ生产者
	producer, err := NewRabbitProducer(h.Config, h.Logger, amqpURL)
	if err != nil {
		return fmt.Errorf("failed to start rabbitmq producer: %w", err)
	}
	h.Producer = producer

	// 启动RabbitMQ消费者
	consumer, err := NewRabbitConsumer(h.Config, h.Logger, amqpURL, h)
	if err != nil {
		return fmt.Errorf("failed to start rabbitmq consumer: %w", err)
	}
	h.Consumer = consumer

	// 启动Hub主循环
	h.wg.Add(1)
	go h.run()

	return nil
}

// Stop 停止Hub
func (h *Hub) Stop() {
	close(h.StopChan)
	h.wg.Wait()

	if h.Producer != nil {
		h.Producer.Close()
	}
	if h.Consumer != nil {
		h.Consumer.Close()
	}
}

// run Hub主循环
func (h *Hub) run() {
	defer h.wg.Done()

	for {
		select {
		case client := <-h.Register:
			h.registerClient(client)
		case client := <-h.Unregister:
			h.unregisterClient(client)
		case message := <-h.Broadcast:
			h.broadcastMessage(message)
		case <-h.StopChan:
			return
		}
	}
}

// registerClient 注册客户端
func (h *Hub) registerClient(client *Client) {
	room := h.getOrCreateRoom(client.RoomID)
	room.Clients[client] = true

	room.statsMux.Lock()
	room.stats.ConnectionCount = len(room.Clients)
	room.stats.LastActiveTime = time.Now()
	room.statsMux.Unlock()

	// 发送进入消息
	enterMsg := &ChatMessage{
		Type:      "enter",
		UserID:    client.UserID,
		Username:  client.Username,
		Avatar:    client.Avatar,
		Content:   fmt.Sprintf("%s 进入了直播间", client.Username),
		RoomID:    client.RoomID,
		Timestamp: time.Now().Unix(),
	}
	h.Broadcast <- enterMsg

	h.Logger.Info("Client registered", "user_id", client.UserID, "room_id", client.RoomID)
}

// unregisterClient 注销客户端
func (h *Hub) unregisterClient(client *Client) {
	room := h.getRoom(client.RoomID)
	if room == nil {
		return
	}

	if _, ok := room.Clients[client]; ok {
		delete(room.Clients, client)
		close(client.Send)

		room.statsMux.Lock()
		room.stats.ConnectionCount = len(room.Clients)
		room.statsMux.Unlock()

		// 如果房间为空，删除房间
		if len(room.Clients) == 0 {
			h.deleteRoom(client.RoomID)
		}

		// 发送离开消息
		leaveMsg := &ChatMessage{
			Type:      "leave",
			UserID:    client.UserID,
			Username:  client.Username,
			Avatar:    client.Avatar,
			Content:   fmt.Sprintf("%s 离开了直播间", client.Username),
			RoomID:    client.RoomID,
			Timestamp: time.Now().Unix(),
		}
		h.Broadcast <- leaveMsg

		h.Logger.Info("Client unregistered", "user_id", client.UserID, "room_id", client.RoomID)
	}
}

// broadcastMessage 广播消息
func (h *Hub) broadcastMessage(message *ChatMessage) {
	// 生成序列号
	h.seqMux.Lock()
	h.seqID++
	message.SeqID = h.seqID
	h.seqMux.Unlock()

	// 直接广播给房间内的所有客户端（单机模式）
	// 注意：如果是分布式部署，需要发送到RabbitMQ，由消费者广播
	h.broadcastToRoom(message, message.RoomID)

	// 发送到RabbitMQ（用于跨服务器广播，单机模式下可注释掉）
	// if h.Producer != nil {
	// 	h.Producer.Publish(message)
	// }
}

// broadcastToRoom 广播消息到指定房间
func (h *Hub) broadcastToRoom(message *ChatMessage, roomID string) {
	room := h.getRoom(roomID)
	if room != nil {
		data, _ := json.Marshal(message)
		room.doBroadcast(data)
	}
}

// getOrCreateRoom 获取或创建房间
func (h *Hub) getOrCreateRoom(roomID string) *Room {
	h.RoomsMux.Lock()
	defer h.RoomsMux.Unlock()

	if room, ok := h.Rooms[roomID]; ok {
		return room
	}

	room := &Room{
		ID:         roomID,
		Hub:        h,
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan []byte, 256),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		stats: &RoomStats{
			LastActiveTime: time.Now(),
		},
	}
	h.Rooms[roomID] = room

	// 启动房间协程
	go room.run()

	return room
}

// getRoom 获取房间
func (h *Hub) getRoom(roomID string) *Room {
	h.RoomsMux.RLock()
	defer h.RoomsMux.RUnlock()
	return h.Rooms[roomID]
}

// deleteRoom 删除房间
func (h *Hub) deleteRoom(roomID string) {
	h.RoomsMux.Lock()
	defer h.RoomsMux.Unlock()
	delete(h.Rooms, roomID)
}

// run 房间主循环
func (r *Room) run() {
	for {
		select {
		case client := <-r.Register:
			r.Clients[client] = true
		case client := <-r.Unregister:
			if _, ok := r.Clients[client]; ok {
				delete(r.Clients, client)
				close(client.Send)
			}
		case message := <-r.Broadcast:
			r.doBroadcast(message)
		}
	}
}

// doBroadcast 执行广播
func (r *Room) doBroadcast(message []byte) {
	for client := range r.Clients {
		select {
		case client.Send <- message:
		default:
			close(client.Send)
			delete(r.Clients, client)
		}
	}

	r.statsMux.Lock()
	r.stats.MessageCount++
	r.stats.LastActiveTime = time.Now()
	r.statsMux.Unlock()
}

// HandleWebSocket 处理WebSocket连接
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request, userID, roomID, username, avatar string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.Logger.Error("Failed to upgrade websocket", "error", err)
		return
	}

	client := &Client{
		Hub:      h,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		RoomID:   roomID,
		UserID:   userID,
		Username: username,
		Avatar:   avatar,
	}

	h.Register <- client

	// 启动读写协程
	go client.writePump()
	go client.readPump()
}

// readPump 读取客户端消息
func (c *Client) readPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.Hub.Logger.Error("WebSocket error", "error", err)
			}
			break
		}

		// 解析消息
		var msg ChatMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			c.Hub.Logger.Error("Failed to unmarshal message", "error", err)
			continue
		}

		msg.UserID = c.UserID
		msg.Username = c.Username
		msg.Avatar = c.Avatar
		msg.RoomID = c.RoomID
		msg.Timestamp = time.Now().Unix()

		// 广播消息
		c.Hub.Broadcast <- &msg
	}
}

// writePump 向客户端写入消息
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.Conn.WriteMessage(websocket.TextMessage, message)

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// GetRoomStats 获取房间统计
func (h *Hub) GetRoomStats(roomID string) *RoomStats {
	room := h.getRoom(roomID)
	if room == nil {
		return nil
	}

	room.statsMux.RLock()
	defer room.statsMux.RUnlock()

	return &RoomStats{
		MessageCount:    room.stats.MessageCount,
		ConnectionCount: len(room.Clients),
		LastActiveTime:  room.stats.LastActiveTime,
	}
}

// OnlineUser 在线用户信息
type OnlineUser struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// GetOnlineUsers 获取房间在线观众列表
func (h *Hub) GetOnlineUsers(roomID string) []*OnlineUser {
	room := h.getRoom(roomID)
	if room == nil {
		return []*OnlineUser{}
	}

	room.statsMux.RLock()
	defer room.statsMux.RUnlock()

	users := make([]*OnlineUser, 0, len(room.Clients))
	for client := range room.Clients {
		users = append(users, &OnlineUser{
			UserID:   client.UserID,
			Username: client.Username,
			Avatar:   client.Avatar,
		})
	}
	return users
}

// GetHubStats 获取Hub统计
func (h *Hub) GetHubStats() *HubStats {
	h.RoomsMux.RLock()
	defer h.RoomsMux.RUnlock()

	var totalConnections int
	for _, room := range h.Rooms {
		totalConnections += len(room.Clients)
	}

	return &HubStats{
		TotalConnections: int64(totalConnections),
		TotalRooms:       len(h.Rooms),
		TotalMessages:    uint64(h.seqID),
		StartTime:        time.Now(), // 应该记录实际启动时间
	}
}

// SendMessage 发送消息（HTTP接口用）
func (h *Hub) SendMessage(userID, username, avatar, roomID, content, msgType string) error {
	msg := &ChatMessage{
		Type:      msgType,
		UserID:    userID,
		Username:  username,
		Avatar:    avatar,
		Content:   content,
		RoomID:    roomID,
		Timestamp: time.Now().Unix(),
	}
	h.Broadcast <- msg
	return nil
}
