package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rabbitmq/amqp091-go"
	"github.com/vision_world/audit_service/internal/config"
	"github.com/vision_world/audit_service/internal/service"
	"github.com/vision_world/audit_service/pkg/logger"
)

// AuditMessage 审核消息结构
type AuditMessage struct {
	ContentID    string `json:"content_id"`
	ContentType  string `json:"content_type"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	Metadata     string `json:"metadata"`
	UploaderID   string `json:"uploader_id"`
	UploaderName string `json:"uploader_name"`
	CreatedAt    string `json:"created_at"`
}

// RabbitMQConsumer RabbitMQ消费者
type RabbitMQConsumer struct {
	conn         *amqp091.Connection
	channel      *amqp091.Channel
	queue        amqp091.Queue
	config       *config.Config
	logger       logger.Logger
	auditService service.AuditService
}

// NewRabbitMQConsumer 创建RabbitMQ消费者
func NewRabbitMQConsumer(cfg *config.Config, log logger.Logger, auditService service.AuditService) (*RabbitMQConsumer, error) {
	// 构建连接URL
	connectionURL := fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		cfg.RabbitMQ.Username,
		cfg.RabbitMQ.Password,
		cfg.RabbitMQ.Host,
		cfg.RabbitMQ.Port,
		cfg.RabbitMQ.VHost,
	)

	conn, err := amqp091.Dial(connectionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// 声明队列
	q, err := ch.QueueDeclare(
		cfg.RabbitMQ.QueueName, // 队列名称
		true,                   // 持久化
		false,                  // 不自动删除
		false,                  // 非排他性
		false,                  // 不等待
		nil,                    // 参数
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	return &RabbitMQConsumer{
		conn:         conn,
		channel:      ch,
		queue:        q,
		config:       cfg,
		logger:       log,
		auditService: auditService,
	}, nil
}

// StartConsuming 开始消费消息
func (c *RabbitMQConsumer) StartConsuming(ctx context.Context) error {
	msgs, err := c.channel.Consume(
		c.queue.Name, // 队列名称
		"",           // 消费者标签
		false,        // 不自动确认
		false,        // 非排他性
		false,        // 不等待
		false,        // 参数
		nil,          // 参数
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	c.logger.Info("Started consuming audit messages", "queue", c.queue.Name)

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.logger.Info("Stopping message consumer")
				return
			case msg, ok := <-msgs:
				if !ok {
					c.logger.Warn("Message channel closed")
					return
				}

				// 处理消息
				if err := c.processMessage(ctx, &msg); err != nil {
					c.logger.Error("Failed to process message", "error", err)
					// 处理失败，重新排队
					msg.Nack(false, true)
				} else {
					// 处理成功，确认消息
					msg.Ack(false)
				}
			}
		}
	}()

	return nil
}

// processMessage 处理单个消息
func (c *RabbitMQConsumer) processMessage(ctx context.Context, msg *amqp091.Delivery) error {
	var auditMsg AuditMessage
	if err := json.Unmarshal(msg.Body, &auditMsg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	c.logger.Info("Processing audit message",
		"content_id", auditMsg.ContentID,
		"content_type", auditMsg.ContentType,
		"title", auditMsg.Title)

	// 调用审核服务处理内容
	submitReq := &service.SubmitContentRequest{
		ContentID:       auditMsg.ContentID,
		ContentType:     auditMsg.ContentType,
		ContentTitle:    auditMsg.Title,
		ContentURL:      auditMsg.URL,
		ContentMetadata: auditMsg.Metadata,
		UploaderID:      auditMsg.UploaderID,
		UploaderName:    auditMsg.UploaderName,
	}

	result, err := c.auditService.SubmitContent(ctx, submitReq)
	if err != nil {
		return fmt.Errorf("audit service failed: %w", err)
	}

	c.logger.Info("Content audit completed",
		"content_id", auditMsg.ContentID,
		"audit_id", result.AuditID,
		"status", result.Status,
		"score", result.Score)

	return nil
}

// Close 关闭消费者连接
func (c *RabbitMQConsumer) Close() error {
	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			c.logger.Error("Failed to close channel", "error", err)
		}
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			c.logger.Error("Failed to close connection", "error", err)
		}
	}

	return nil
}
