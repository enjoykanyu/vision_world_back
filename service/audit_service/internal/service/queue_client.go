package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/vision_world/audit_service/internal/config"
	"github.com/vision_world/audit_service/internal/model"
	"go.uber.org/zap"
)

// QueueClient RabbitMQ客户端
type QueueClient struct {
	config  *config.Config
	logger  *zap.Logger
	conn    *amqp091.Connection
	channel *amqp091.Channel
	queue   amqp091.Queue
}

// NewQueueClient 创建队列客户端
func NewQueueClient(cfg *config.Config, logger *zap.Logger) (*QueueClient, error) {
	// 构建RabbitMQ连接URL
	rabbitmqURL := fmt.Sprintf("amqp://%s:%s@%s:%d%s",
		cfg.RabbitMQ.Username,
		cfg.RabbitMQ.Password,
		cfg.RabbitMQ.Host,
		cfg.RabbitMQ.Port,
		cfg.RabbitMQ.VHost,
	)

	conn, err := amqp091.Dial(rabbitmqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// 声明队列
	queue, err := channel.QueueDeclare(
		cfg.RabbitMQ.QueueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	logger.Info("RabbitMQ queue client initialized",
		zap.String("queue_name", queue.Name),
		zap.Int("messages", queue.Messages),
		zap.Int("consumers", queue.Consumers))

	return &QueueClient{
		config:  cfg,
		logger:  logger,
		conn:    conn,
		channel: channel,
		queue:   queue,
	}, nil
}

// PublishAuditMessage 发布审核消息
func (c *QueueClient) PublishAuditMessage(ctx context.Context, message model.AuditMessage) error {
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

	c.logger.Info("Audit message published to queue",
		zap.String("content_id", message.ContentID),
		zap.String("content_type", message.ContentType))

	return nil
}

// StartConsumer 启动消息消费者
func (c *QueueClient) StartConsumer(ctx context.Context, handler func(context.Context, model.AuditMessage) error) error {
	msgs, err := c.channel.Consume(
		c.queue.Name, // queue
		"",           // consumer
		false,        // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	c.logger.Info("RabbitMQ consumer started",
		zap.String("queue_name", c.queue.Name))

	// 启动goroutine处理消息
	go func() {
		for {
			select {
			case <-ctx.Done():
				c.logger.Info("Consumer context cancelled, stopping...")
				return
			case msg, ok := <-msgs:
				if !ok {
					c.logger.Info("Message channel closed, stopping consumer...")
					return
				}

				// 处理消息
				if err := c.processMessage(ctx, msg, handler); err != nil {
					c.logger.Error("Failed to process message",
						zap.Error(err),
						zap.ByteString("body", msg.Body))
				}

				// 手动确认消息
				msg.Ack(false)
			}
		}
	}()

	return nil
}

// processMessage 处理单个消息
func (c *QueueClient) processMessage(ctx context.Context, msg amqp091.Delivery, handler func(context.Context, model.AuditMessage) error) error {
	var message model.AuditMessage
	if err := json.Unmarshal(msg.Body, &message); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	c.logger.Info("Processing audit message",
		zap.String("content_id", message.ContentID),
		zap.String("content_type", message.ContentType),
		zap.Int("retry_count", message.RetryCount))

	// 调用处理函数
	if err := handler(ctx, message); err != nil {
		return fmt.Errorf("handler failed: %w", err)
	}

	return nil
}

// republishMessage 重新发布消息
func (c *QueueClient) republishMessage(ctx context.Context, message model.AuditMessage) error {
	// 添加延迟重试
	time.Sleep(time.Duration(message.RetryCount) * time.Second)

	return c.PublishAuditMessage(ctx, message)
}

// Close 关闭连接
func (c *QueueClient) Close() error {
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

	c.logger.Info("RabbitMQ queue client closed")
	return nil
}
