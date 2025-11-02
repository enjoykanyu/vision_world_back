package model

import "time"

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
	RetryCount   int       `json:"retry_count"`
}
