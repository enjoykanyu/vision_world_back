package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/vision_world/audit_service/internal/config"
	"github.com/vision_world/audit_service/internal/service"
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

// RabbitMQConsumer RabbitMQ消费者
type RabbitMQConsumer struct {
	conn         *amqp091.Connection
	channel      *amqp091.Channel
	queue        amqp091.Queue
	config       *config.Config
	logger       logger.Logger
	auditService service.AuditService
	maxRetries   int
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
		maxRetries:   3, // 默认重试3次
	}, nil
}

// StartConsuming 开始消费消息
func (c *RabbitMQConsumer) StartConsuming(ctx context.Context) error {
	// 设置QoS，每次只处理一条消息
	err := c.channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

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

	c.logger.Info("Started consuming audit messages",
		"queue", c.queue.Name,
		"messages", c.queue.Messages,
		"consumers", c.queue.Consumers)

	// 记录处理统计
	var processedCount int64
	var errorCount int64

	// 定期报告处理状态
	reportTicker := time.NewTicker(30 * time.Second)
	defer reportTicker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.logger.Info("Stopping message consumer")
				c.logger.Info("Final consumer statistics",
					"processed", processedCount,
					"errors", errorCount,
					"success_rate", float64(processedCount-errorCount)/float64(processedCount)*100)
				return
			case msg, ok := <-msgs:
				if !ok {
					c.logger.Warn("Message channel closed")
					return
				}

				startTime := time.Now()
				// 处理消息
				err := c.processMessageWithRetry(ctx, &msg)
				if err != nil {
					errorCount++
					duration := time.Since(startTime)
					c.logger.Error("Failed to process message after retries",
						"error", err,
						"processing_time", duration,
						"message_id", getMessageID(&msg),
						"processed_count", processedCount,
						"error_count", errorCount)

					// 根据错误类型决定是否拒绝消息
					if shouldRejectMessage(err) {
						if err := msg.Nack(false, false); err != nil {
							c.logger.Error("Failed to nack message",
								"error", err,
								"message_id", getMessageID(&msg))
						}
					} else {
						// 对于可重试的错误，重新入队
						if err := msg.Nack(false, true); err != nil {
							c.logger.Error("Failed to requeue message",
								"error", err,
								"message_id", getMessageID(&msg))
						}
					}
				} else {
					processedCount++
					duration := time.Since(startTime)
					c.logger.Info("Message processed successfully",
						"processing_time", duration,
						"message_id", getMessageID(&msg),
						"processed_count", processedCount,
						"error_count", errorCount)

					// 确认消息
					if err := msg.Ack(false); err != nil {
						c.logger.Error("Failed to ack message",
							"error", err,
							"message_id", getMessageID(&msg))
					}
				}
			case <-reportTicker.C:
				// 定期报告处理统计
				if processedCount > 0 {
					c.logger.Info("Consumer statistics",
						"processed", processedCount,
						"errors", errorCount,
						"success_rate", float64(processedCount-errorCount)/float64(processedCount)*100)
				}
			}
		}
	}()

	return nil
}

// getMessageID 从消息中提取消息ID，用于日志记录
func getMessageID(msg *amqp091.Delivery) string {
	var auditMsg AuditMessage
	if err := json.Unmarshal(msg.Body, &auditMsg); err != nil {
		return "unknown"
	}
	return auditMsg.MessageID
}

// shouldRejectMessage 判断是否应该拒绝消息（不再重试）
func shouldRejectMessage(err error) bool {
	// 对于解析错误、认证错误等不可恢复的错误，应该拒绝消息
	errorStr := err.Error()
	return strings.Contains(errorStr, "invalid") ||
		strings.Contains(errorStr, "unmarshal") ||
		strings.Contains(errorStr, "authentication") ||
		strings.Contains(errorStr, "authorization")
}

// processMessageWithRetry 处理单个消息，包含重试逻辑
func (c *RabbitMQConsumer) processMessageWithRetry(ctx context.Context, msg *amqp091.Delivery) error {
	// 解析消息
	var auditMsg AuditMessage
	if err := json.Unmarshal(msg.Body, &auditMsg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// 记录开始处理时间
	startTime := time.Now()

	// 重试处理消息
	var lastErr error
	// 使用配置的重试次数
	retryAttempts := c.config.Audit.Queue.MaxRetryCount
	if retryAttempts <= 0 {
		retryAttempts = 3 // 默认重试3次
	}

	for attempt := 1; attempt <= retryAttempts; attempt++ {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 处理消息
		err := c.processMessage(ctx, auditMsg)
		if err == nil {
			duration := time.Since(startTime)
			c.logger.Info("Message processed successfully",
				"message_id", auditMsg.MessageID,
				"content_id", auditMsg.ContentID,
				"attempt", attempt,
				"processing_time", duration)
			return nil
		}

		lastErr = err
		c.logger.Warn("Message processing attempt failed",
			"message_id", auditMsg.MessageID,
			"content_id", auditMsg.ContentID,
			"attempt", attempt,
			"max_retries", retryAttempts,
			"error", err)

		// 如果不是最后一次尝试，等待一段时间后重试
		if attempt < retryAttempts {
			// 计算重试延迟，使用指数退避策略
			backoffTime := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			// 添加随机抖动，避免重试风暴
			backoffTime += time.Duration(rand.Int63n(int64(backoffTime / 4)))
			c.logger.Info("Retrying message processing",
				"message_id", auditMsg.MessageID,
				"content_id", auditMsg.ContentID,
				"attempt", attempt+1,
				"delay", backoffTime)
			select {
			case <-time.After(backoffTime):
				// 继续重试
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	duration := time.Since(startTime)
	c.logger.Error("Message processing failed after all retries",
		"message_id", auditMsg.MessageID,
		"content_id", auditMsg.ContentID,
		"total_attempts", retryAttempts,
		"processing_time", duration,
		"last_error", lastErr)

	return fmt.Errorf("failed to process message after %d attempts: %w", retryAttempts, lastErr)
}

// processMessage 处理单个消息
func (c *RabbitMQConsumer) processMessage(ctx context.Context, auditMsg AuditMessage) error {
	c.logger.Info("Processing audit message",
		"message_id", auditMsg.MessageID,
		"content_id", auditMsg.ContentID,
		"content_type", auditMsg.ContentType,
		"title", auditMsg.Title,
		"timestamp", auditMsg.Timestamp)

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
		"message_id", auditMsg.MessageID,
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
