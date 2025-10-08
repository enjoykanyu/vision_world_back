package model

import (
	"time"
)

// AuditStatus 审核状态
type AuditStatus string

const (
	AuditStatusPending     AuditStatus = "pending"      // 待审核
	AuditStatusApproved    AuditStatus = "approved"     // 已通过
	AuditStatusRejected    AuditStatus = "rejected"     // 已拒绝
	AuditStatusAutoPassed  AuditStatus = "auto_passed"  // 自动通过
	AuditStatusAutoBlocked AuditStatus = "auto_blocked" // 自动拦截
)

// ContentType 内容类型
type ContentType string

const (
	ContentTypeVideo ContentType = "video"
	ContentTypeImage ContentType = "image"
	ContentTypeText  ContentType = "text"
	ContentTypeAudio ContentType = "audio"
)

// AuditLevel 审核级别
type AuditLevel string

const (
	AuditLevelLow    AuditLevel = "low"
	AuditLevelMedium AuditLevel = "medium"
	AuditLevelHigh   AuditLevel = "high"
)

// AuditRecord 审核记录
type AuditRecord struct {
	ID              uint64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ContentID       string      `gorm:"index;not null" json:"content_id"`
	ContentType     ContentType `gorm:"index;not null;type:varchar(20)" json:"content_type"`
	ContentTitle    string      `gorm:"type:varchar(255)" json:"content_title"`
	ContentURL      string      `gorm:"type:text" json:"content_url"`
	ContentMetadata string      `gorm:"type:json" json:"content_metadata"`
	UploaderID      uint64      `gorm:"index;not null" json:"uploader_id"`
	UploaderName    string      `gorm:"type:varchar(100)" json:"uploader_name"`

	// 审核信息
	Status       AuditStatus `gorm:"index;not null;type:varchar(20)" json:"status"`
	Level        AuditLevel  `gorm:"index;not null;type:varchar(10)" json:"level"`
	Score        float64     `gorm:"type:decimal(5,4)" json:"score"`
	AIResult     string      `gorm:"type:json" json:"ai_result"`
	AIConfidence float64     `gorm:"type:decimal(5,4)" json:"ai_confidence"`

	// 审核详情
	Reason        string `gorm:"type:text" json:"reason"`
	Details       string `gorm:"type:text" json:"details"`
	Violations    string `gorm:"type:json" json:"violations"` // 违规类型列表
	Keywords      string `gorm:"type:text" json:"keywords"`
	SensitiveData string `gorm:"type:json" json:"sensitive_data"`

	// 审核人员
	ReviewerID   *uint64    `gorm:"index" json:"reviewer_id"`
	ReviewerName string     `gorm:"type:varchar(100)" json:"reviewer_name"`
	ReviewTime   *time.Time `json:"review_time"`

	// 第三方审核
	ThirdPartyResult   string     `gorm:"type:json" json:"third_party_result"`
	ThirdPartyStatus   string     `gorm:"type:varchar(20)" json:"third_party_status"`
	ThirdPartyResponse string     `gorm:"type:json" json:"third_party_response"`
	ThirdPartyTime     *time.Time `json:"third_party_time"`

	// 时间戳
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// 版本控制
	Version int `gorm:"default:1" json:"version"`

	// 软删除
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName 表名
func (AuditRecord) TableName() string {
	return "audit_records"
}

// AuditTemplate 审核模板
type AuditTemplate struct {
	ID          uint64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string      `gorm:"uniqueIndex;not null;type:varchar(100)" json:"name"`
	Description string      `gorm:"type:text" json:"description"`
	ContentType ContentType `gorm:"index;not null;type:varchar(20)" json:"content_type"`
	Level       AuditLevel  `gorm:"not null;type:varchar(10)" json:"level"`

	// 审核规则
	Rules       string  `gorm:"type:json" json:"rules"`
	Keywords    string  `gorm:"type:json" json:"keywords"`
	Violations  string  `gorm:"type:json" json:"violations"`
	Sensitivity float64 `gorm:"type:decimal(5,4)" json:"sensitivity"`

	// 第三方服务配置
	ThirdPartyConfig string `gorm:"type:json" json:"third_party_config"`

	// 状态
	IsActive bool `gorm:"default:true;index" json:"is_active"`

	// 时间戳
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// 操作者
	CreatedBy uint64 `gorm:"not null" json:"created_by"`
	UpdatedBy uint64 `gorm:"not null" json:"updated_by"`
}

// TableName 表名
func (AuditTemplate) TableName() string {
	return "audit_templates"
}

// AuditWhitelist 审核白名单
type AuditWhitelist struct {
	ID          uint64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ContentID   string      `gorm:"uniqueIndex;not null" json:"content_id"`
	ContentType ContentType `gorm:"index;not null;type:varchar(20)" json:"content_type"`
	UploaderID  uint64      `gorm:"index;not null" json:"uploader_id"`

	// 白名单信息
	Reason      string     `gorm:"type:text" json:"reason"`
	ExpiryDate  *time.Time `json:"expiry_date"`
	IsPermanent bool       `gorm:"default:false" json:"is_permanent"`

	// 时间戳
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	CreatedBy uint64    `gorm:"not null" json:"created_by"`
}

// TableName 表名
func (AuditWhitelist) TableName() string {
	return "audit_whitelists"
}

// AuditBlacklist 审核黑名单
type AuditBlacklist struct {
	ID          uint64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ContentID   string      `gorm:"index;not null" json:"content_id"`
	ContentType ContentType `gorm:"index;not null;type:varchar(20)" json:"content_type"`
	UploaderID  uint64      `gorm:"index;not null" json:"uploader_id"`

	// 黑名单信息
	Reason      string     `gorm:"type:text" json:"reason"`
	Violations  string     `gorm:"type:json" json:"violations"`
	ExpiryDate  *time.Time `json:"expiry_date"`
	IsPermanent bool       `gorm:"default:false" json:"is_permanent"`

	// 时间戳
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	CreatedBy uint64    `gorm:"not null" json:"created_by"`
}

// TableName 表名
func (AuditBlacklist) TableName() string {
	return "audit_blacklists"
}

// AuditStatistics 审核统计
type AuditStatistics struct {
	ID          uint64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Date        time.Time   `gorm:"index;not null" json:"date"`
	ContentType ContentType `gorm:"index;not null;type:varchar(20)" json:"content_type"`
	Level       AuditLevel  `gorm:"index;not null;type:varchar(10)" json:"level"`

	// 统计信息
	TotalCount       int64 `gorm:"default:0" json:"total_count"`
	PendingCount     int64 `gorm:"default:0" json:"pending_count"`
	ApprovedCount    int64 `gorm:"default:0" json:"approved_count"`
	RejectedCount    int64 `gorm:"default:0" json:"rejected_count"`
	AutoPassedCount  int64 `gorm:"default:0" json:"auto_passed_count"`
	AutoBlockedCount int64 `gorm:"default:0" json:"auto_blocked_count"`

	// 处理时间统计
	AvgReviewTime float64 `gorm:"type:decimal(10,2)" json:"avg_review_time"`
	MinReviewTime float64 `gorm:"type:decimal(10,2)" json:"min_review_time"`
	MaxReviewTime float64 `gorm:"type:decimal(10,2)" json:"max_review_time"`

	// AI审核统计
	AICount    int64   `gorm:"default:0" json:"ai_count"`
	AIAccuracy float64 `gorm:"type:decimal(5,4)" json:"ai_accuracy"`
	AIAvgScore float64 `gorm:"type:decimal(5,4)" json:"ai_avg_score"`

	// 第三方审核统计
	ThirdPartyCount    int64   `gorm:"default:0" json:"third_party_count"`
	ThirdPartyAccuracy float64 `gorm:"type:decimal(5,4)" json:"third_party_accuracy"`

	// 违规统计
	ViolationStats string `gorm:"type:json" json:"violation_stats"`

	// 时间戳
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 表名
func (AuditStatistics) TableName() string {
	return "audit_statistics"
}
