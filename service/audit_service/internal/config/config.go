package config

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config 全局配置
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Logger   LoggerConfig   `mapstructure:"logger"`
	Etcd     EtcdConfig     `mapstructure:"etcd"`
	Consul   ConsulConfig   `mapstructure:"consul"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Audit    AuditConfig    `mapstructure:"audit"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Mode         string        `mapstructure:"mode"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	Database        string `mapstructure:"database"`
	Charset         string `mapstructure:"charset"`
	TablePrefix     string `mapstructure:"table_prefix"`
	LogLevel        string `mapstructure:"log_level"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	MaxRetries   int    `mapstructure:"max_retries"`
	DialTimeout  int    `mapstructure:"dial_timeout"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	OutputPath string `mapstructure:"output_path"`
}

// EtcdConfig etcd配置
type EtcdConfig struct {
	Endpoints   []string `mapstructure:"endpoints"`
	DialTimeout int      `mapstructure:"dial_timeout"`
	Username    string   `mapstructure:"username"`
	Password    string   `mapstructure:"password"`
}

// ConsulConfig Consul配置
type ConsulConfig struct {
	Host      string `mapstructure:"host"`
	Port      int    `mapstructure:"port"`
	ServiceID string `mapstructure:"service_id"`
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret            string        `mapstructure:"secret"`
	RefreshSecret     string        `mapstructure:"refresh_secret"`
	TokenExpiration   time.Duration `mapstructure:"token_expiration"`
	RefreshExpiration time.Duration `mapstructure:"refresh_expiration"`
}

// AuditConfig 审核服务配置
type AuditConfig struct {
	Strategies   AuditStrategies    `mapstructure:"strategies"`
	ThirdParty   ThirdPartyConfig   `mapstructure:"third_party"`
	Queue        QueueConfig        `mapstructure:"queue"`
	Notification NotificationConfig `mapstructure:"notification"`
}

// AuditStrategies 审核策略配置
type AuditStrategies struct {
	Content AuditStrategy `mapstructure:"content"`
	Image   AuditStrategy `mapstructure:"image"`
	Video   AuditStrategy `mapstructure:"video"`
}

// AuditStrategy 单个审核策略配置
type AuditStrategy struct {
	Enabled               bool          `mapstructure:"enabled"`
	SensitivityLevel      string        `mapstructure:"sensitivity_level"`
	AutoBlockThreshold    float64       `mapstructure:"auto_block_threshold"`
	AllowAiReview         bool          `mapstructure:"allow_ai_review"`
	ManualReviewThreshold float64       `mapstructure:"manual_review_threshold"`
	FrameSampleRate       int           `mapstructure:"frame_sample_rate"`
	AiReviewTimeout       time.Duration `mapstructure:"ai_review_timeout"`
}

// ThirdPartyConfig 第三方审核服务配置
type ThirdPartyConfig struct {
	TextReviewAPI  string `mapstructure:"text_review_api"`
	ImageReviewAPI string `mapstructure:"image_review_api"`
	VideoReviewAPI string `mapstructure:"video_review_api"`
	APIKey         string `mapstructure:"api_key"`
	SecretKey      string `mapstructure:"secret_key"`
}

// QueueConfig 审核队列配置
type QueueConfig struct {
	MaxRetryCount int           `mapstructure:"max_retry_count"`
	RetryInterval time.Duration `mapstructure:"retry_interval"`
	BatchSize     int           `mapstructure:"batch_size"`
	WorkerCount   int           `mapstructure:"worker_count"`
}

// NotificationConfig 审核结果通知配置
type NotificationConfig struct {
	WebhookURL      string   `mapstructure:"webhook_url"`
	EmailEnabled    bool     `mapstructure:"email_enabled"`
	EmailRecipients []string `mapstructure:"email_recipients"`
}

// LoadConfig 加载配置
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// 设置配置文件路径
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// 默认在当前目录和config目录下查找配置文件
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("../config")
		v.AddConfigPath("../../config")
		v.SetConfigName("audit-service")
		v.SetConfigType("yaml")
	}

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 绑定环境变量
	v.AutomaticEnv()
	v.SetEnvPrefix("AUDIT_SERVICE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}

	if c.Database.Port <= 0 || c.Database.Port > 65535 {
		return fmt.Errorf("invalid database port: %d", c.Database.Port)
	}

	if c.Database.Database == "" {
		return fmt.Errorf("database name is required")
	}

	if c.Redis.Host == "" {
		return fmt.Errorf("redis host is required")
	}

	if c.Redis.Port <= 0 || c.Redis.Port > 65535 {
		return fmt.Errorf("invalid redis port: %d", c.Redis.Port)
	}

	if len(c.Etcd.Endpoints) == 0 {
		return fmt.Errorf("etcd endpoints are required")
	}

	if c.JWT.Secret == "" {
		return fmt.Errorf("jwt secret is required")
	}

	if c.JWT.TokenExpiration <= 0 {
		return fmt.Errorf("jwt token expiration must be positive")
	}

	return nil
}

// GetDefaultConfigPath 获取默认配置文件路径
func GetDefaultConfigPath() string {
	// 尝试多个可能的配置文件路径
	paths := []string{
		"./config/audit-service.yaml",
		"../config/audit-service.yaml",
		"../../config/audit-service.yaml",
		"./audit-service.yaml",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			absPath, _ := filepath.Abs(path)
			return absPath
		}
	}

	return ""
}
