package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
	"github.com/vision_world/video_service/internal/config"
	"github.com/vision_world/video_service/pkg/logger"
	"go.uber.org/zap"
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

// RabbitMQClient RabbitMQ客户端
type RabbitMQClient struct {
	config  *config.Config
	logger  logger.Logger
	conn    *amqp091.Connection
	channel *amqp091.Channel
	queue   amqp091.Queue
}

// NewRabbitMQClient 创建RabbitMQ客户端
func NewRabbitMQClient(cfg *config.Config, logger logger.Logger) (*RabbitMQClient, error) {
	client := &RabbitMQClient{
		config: cfg,
		logger: logger,
	}

	if err := client.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	return client, nil
}

// connect 连接到RabbitMQ
func (c *RabbitMQClient) connect() error {
	// 构建连接URL
	url := fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		c.config.RabbitMQ.Username,
		c.config.RabbitMQ.Password,
		c.config.RabbitMQ.Host,
		c.config.RabbitMQ.Port,
		c.config.RabbitMQ.VHost,
	)

	conn, err := amqp091.Dial(url)
	if err != nil {
		return fmt.Errorf("failed to dial RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}

	// 声明队列
	queue, err := channel.QueueDeclare(
		c.config.RabbitMQ.QueueName, // 队列名称
		true,                        // 持久化
		false,                       // 不自动删除
		false,                       // 非排他
		false,                       // 不等待
		nil,                         // 参数
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	c.conn = conn
	c.channel = channel
	c.queue = queue
	c.logger.Info("Connected to RabbitMQ successfully",
		zap.String("queue", queue.Name),
		zap.Int("messages", queue.Messages),
		zap.Int("consumers", queue.Consumers))

	return nil
}

// PublishAuditMessage 发布审核消息
func (c *RabbitMQClient) PublishAuditMessage(ctx context.Context, message *AuditMessage) error {
	if message.MessageID == "" {
		message.MessageID = uuid.New().String()
	}
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}

	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = c.channel.PublishWithContext(
		ctx,
		"",           // exchange
		c.queue.Name, // routing key
		false,        // mandatory
		false,        // immediate
		amqp091.Publishing{
			ContentType: "application/json",
			Body:        body,
			Timestamp:   time.Now(),
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	c.logger.Info("Published audit message",
		zap.String("message_id", message.MessageID),
		zap.String("content_id", message.ContentID),
		zap.String("content_type", message.ContentType))

	return nil
}

// Close 关闭连接
func (c *RabbitMQClient) Close() error {
	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			c.logger.Error("Failed to close channel", zap.Error(err))
		}
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			c.logger.Error("Failed to close connection", zap.Error(err))
		}
	}

	c.logger.Info("RabbitMQ connection closed")
	return nil
}
