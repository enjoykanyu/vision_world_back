package routes

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"api_gateway/proto/proto_gen/xiaov"
)

// XiaovHandler 小V助手Handler
type XiaovHandler struct {
	client xiaov.XiaovServiceClient
	conn   *grpc.ClientConn
}

// NewXiaovHandler 创建小V助手Handler
func NewXiaovHandler(etcdEndpoints []string) (*XiaovHandler, error) {
	// 直接连接到video_agent服务（端口500902）
	// 实际生产环境应该通过etcd服务发现
	target := "localhost:50090"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		log.Printf("Failed to connect to xiaov service: %v", err)
		// 返回nil错误，让服务可以继续启动，只是小V功能不可用
		return &XiaovHandler{
			client: nil,
			conn:   nil,
		}, nil
	}

	client := xiaov.NewXiaovServiceClient(conn)
	log.Println("Connected to xiaov service successfully")

	return &XiaovHandler{
		client: client,
		conn:   conn,
	}, nil
}

// Close 关闭连接
func (h *XiaovHandler) Close() {
	if h.conn != nil {
		h.conn.Close()
	}
}

// ChatRequest HTTP聊天请求
type ChatRequest struct {
	UserID    string `json:"user_id" binding:"required"`
	Message   string `json:"message" binding:"required"`
	SessionID string `json:"session_id,omitempty"`
}

// ChatResponse HTTP聊天响应
type ChatResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Reply     string `json:"reply"`
	SessionID string `json:"session_id"`
	Intent    string `json:"intent"`
	Timestamp int64  `json:"timestamp"`
}

// Chat 发送消息并获取回复
func (h *XiaovHandler) Chat(c *gin.Context) {
	if h.client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    -1,
			"message": "小V助手服务暂时不可用",
		})
		return
	}

	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1800*time.Second)
	defer cancel()

	grpcReq := &xiaov.ChatRequest{
		UserId:    req.UserID,
		Message:   req.Message,
		SessionId: req.SessionID,
	}

	resp, err := h.client.Chat(ctx, grpcReq)
	if err != nil {
		log.Printf("Chat failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"message": "聊天请求失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ChatResponse{
		Code:      int(resp.Code),
		Message:   resp.Message,
		Reply:     resp.Reply,
		SessionID: resp.SessionId,
		Intent:    resp.Intent,
		Timestamp: resp.Timestamp,
	})
}

// GetSessionHistory 获取会话历史
func (h *XiaovHandler) GetSessionHistory(c *gin.Context) {
	if h.client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    -1,
			"message": "小V助手服务暂时不可用",
		})
		return
	}

	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"message": "缺少session_id参数",
		})
		return
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &xiaov.GetSessionHistoryRequest{
		SessionId: sessionID,
		Limit:     int32(limit),
	}

	resp, err := h.client.GetSessionHistory(ctx, req)
	if err != nil {
		log.Printf("GetSessionHistory failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"message": "获取会话历史失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":     resp.Code,
		"message":  resp.Message,
		"messages": resp.Messages,
		"total":    resp.Total,
	})
}

// ClearSession 清空会话
func (h *XiaovHandler) ClearSession(c *gin.Context) {
	if h.client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    -1,
			"message": "小V助手服务暂时不可用",
		})
		return
	}

	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	grpcReq := &xiaov.ClearSessionRequest{
		SessionId: req.SessionID,
	}

	resp, err := h.client.ClearSession(ctx, grpcReq)
	if err != nil {
		log.Printf("ClearSession failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"message": "清空会话失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    resp.Code,
		"message": resp.Message,
		"cleared": resp.Cleared,
	})
}

// ChatStream SSE流式聊天
func (h *XiaovHandler) ChatStream(c *gin.Context) {
	if h.client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    -1,
			"message": "小V助手服务暂时不可用",
		})
		return
	}

	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	// 设置SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	ctx, cancel := context.WithTimeout(context.Background(), 1800*time.Second)
	defer cancel()

	grpcReq := &xiaov.ChatRequest{
		UserId:    req.UserID,
		Message:   req.Message,
		SessionId: req.SessionID,
	}

	stream, err := h.client.ChatStream(ctx, grpcReq)
	if err != nil {
		c.SSEvent("error", gin.H{"message": "流式请求失败: " + err.Error()})
		return
	}

	// 发送初始事件
	c.SSEvent("start", gin.H{"message": "开始流式响应"})
	c.Writer.Flush()

	for {
		resp, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				c.SSEvent("done", gin.H{"message": "流式响应完成"})
			} else {
				c.SSEvent("error", gin.H{"message": err.Error()})
			}
			return
		}

		// 根据响应类型发送不同的事件
		switch payload := resp.Payload.(type) {
		case *xiaov.ChatStreamResponse_Content:
			c.SSEvent("content", gin.H{
				"content":    payload.Content.Content,
				"session_id": payload.Content.SessionId,
				"intent":     payload.Content.Intent,
			})
		case *xiaov.ChatStreamResponse_Done:
			c.SSEvent("done", gin.H{
				"session_id": payload.Done.SessionId,
				"intent":     payload.Done.Intent,
				"timestamp":  payload.Done.Timestamp,
			})
			return
		case *xiaov.ChatStreamResponse_Error:
			c.SSEvent("error", gin.H{
				"code":    payload.Error.Code,
				"message": payload.Error.Message,
			})
			return
		}
		c.Writer.Flush()
	}
}
