package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vision_world/audit_service/internal/config"
	"github.com/vision_world/audit_service/internal/model"
	"github.com/vision_world/audit_service/pkg/logger"
)

// AuditMessage 审核消息结构
type AuditMessage struct {
	MessageID    string    `json:"message_id"`
	ContentID    string    `json:"content_id"`
	ContentType  string    `json:"content_type"`
	Title        string    `json:"title"`
	URL          string    `json:"url"`
	Metadata     string    `json:"metadata"`
	UploaderID   string    `json:"uploader_id"`
	UploaderName string    `json:"uploader_name"`
	Timestamp    time.Time `json:"timestamp"`
}

// AuditMessageHandler 审核消息处理器
type AuditMessageHandler struct {
	config       *config.Config
	logger       logger.Logger
	auditService AuditService
	queueClient  *QueueClient
}

// NewAuditMessageHandler 创建审核消息处理器
func NewAuditMessageHandler(cfg *config.Config, log logger.Logger, auditService AuditService, queueClient *QueueClient) *AuditMessageHandler {
	return &AuditMessageHandler{
		config:       cfg,
		logger:       log,
		auditService: auditService,
		queueClient:  queueClient,
	}
}

// ProcessAuditMessage 处理审核消息
func (h *AuditMessageHandler) ProcessAuditMessage(ctx context.Context, message []byte) error {
	var auditMsg AuditMessage
	if err := json.Unmarshal(message, &auditMsg); err != nil {
		h.logger.Error("Failed to unmarshal audit message", "error", err)
		return fmt.Errorf("failed to unmarshal audit message: %w", err)
	}

	h.logger.Info("Processing audit message",
		"message_id", auditMsg.MessageID,
		"content_id", auditMsg.ContentID,
		"content_type", auditMsg.ContentType)

	// 提交内容审核
	req := &SubmitContentRequest{
		ContentID:       auditMsg.ContentID,
		ContentType:     auditMsg.ContentType,
		ContentTitle:    auditMsg.Title,
		ContentURL:      auditMsg.URL,
		ContentMetadata: auditMsg.Metadata,
		UploaderID:      auditMsg.UploaderID,
		UploaderName:    auditMsg.UploaderName,
	}

	resp, err := h.auditService.SubmitContent(ctx, req)
	if err != nil {
		h.logger.Error("Failed to submit content for audit",
			"error", err,
			"content_id", auditMsg.ContentID)
		return fmt.Errorf("failed to submit content for audit: %w", err)
	}

	h.logger.Info("Content audit submitted successfully",
		"content_id", auditMsg.ContentID,
		"audit_id", resp.AuditID,
		"status", resp.Status)

	return nil
}

// StartMessageConsumer 启动消息消费者
func (h *AuditMessageHandler) StartMessageConsumer(ctx context.Context) error {
	h.logger.Info("Starting audit message consumer", "queue", "audit_queue")

	return h.queueClient.StartConsumer(ctx, func(ctx context.Context, message model.AuditMessage) error {
		// 转换消息格式并处理
		msgBytes, err := json.Marshal(message)
		if err != nil {
			return err
		}
		return h.ProcessAuditMessage(ctx, msgBytes)
	})
}

// StopMessageConsumer 停止消息消费者
func (h *AuditMessageHandler) StopMessageConsumer(ctx context.Context) error {
	h.logger.Info("Stopping audit message consumer")
	return h.queueClient.Close()
}
