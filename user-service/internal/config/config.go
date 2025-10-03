package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config 全局配置
type Config struct {
	Server           ServerConfig           `mapstructure:"server"`
	GRPC             GRPCConfig             `mapstructure:"grpc"`
	Database         DatabaseConfig         `mapstructure:"database"`
	Redis            RedisConfig            `mapstructure:"redis"`
	JWT              JWTConfig              `mapstructure:"jwt"`
	SMS              SMSConfig              `mapstructure:"sms"`
	Log              LogConfig              `mapstructure:"log"`
	Metrics          MetricsConfig          `mapstructure:"metrics"`
	Tracing          TracingConfig          `mapstructure:"tracing"`
	RateLimit        RateLimitConfig        `mapstructure:"rate_limit"`
	CircuitBreaker   CircuitBreakerConfig   `mapstructure:"circuit_breaker"`
	Health           HealthConfig           `mapstructure:"health"`
	ExternalServices ExternalServicesConfig `mapstructure:"external_services"`
	Security         SecurityConfig         `mapstructure:"security"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Name string `mapstructure:"name"`
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

// GRPCConfig gRPC配置
type GRPCConfig struct {
	Port                  int   `mapstructure:"port"`
	MaxConnectionIdle     int64 `mapstructure:"max_connection_idle"`
	MaxConnectionAge      int64 `mapstructure:"max_connection_age"`
	MaxConnectionAgeGrace int64 `mapstructure:"max_connection_age_grace"`
	KeepaliveTime         int64 `mapstructure:"keepalive_time"`
	KeepaliveTimeout      int64 `mapstructure:"keepalive_timeout"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver          string `mapstructure:"driver"`
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Database        string `mapstructure:"database"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	Charset         string `mapstructure:"charset"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
	LogLevel        string `mapstructure:"log_level"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr         string `mapstructure:"addr"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
	MaxRetries   int    `mapstructure:"max_retries"`
	DialTimeout  int    `mapstructure:"dial_timeout"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret             string `mapstructure:"secret"`
	AccessTokenExpire  int64  `mapstructure:"access_token_expire"`
	RefreshTokenExpire int64  `mapstructure:"refresh_token_expire"`
	Issuer             string `mapstructure:"issuer"`
}

// SMSConfig 短信配置
type SMSConfig struct {
	ExpireTime    int64 `mapstructure:"expire_time"`
	MaxSendPerDay int   `mapstructure:"max_send_per_day"`
	CoolDownTime  int64 `mapstructure:"cool_down_time"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	FilePath   string `mapstructure:"file_path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

// MetricsConfig 监控配置
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    int    `mapstructure:"port"`
	Path    string `mapstructure:"path"`
}

// TracingConfig 链路追踪配置
type TracingConfig struct {
	Enabled        bool    `mapstructure:"enabled"`
	JaegerEndpoint string  `mapstructure:"jaeger_endpoint"`
	SampleRate     float64 `mapstructure:"sample_rate"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled           bool `mapstructure:"enabled"`
	RequestsPerSecond int  `mapstructure:"requests_per_second"`
	Burst             int  `mapstructure:"burst"`
}

// CircuitBreakerConfig 熔断配置
type CircuitBreakerConfig struct {
	Enabled          bool  `mapstructure:"enabled"`
	FailureThreshold int   `mapstructure:"failure_threshold"`
	SuccessThreshold int   `mapstructure:"success_threshold"`
	Timeout          int64 `mapstructure:"timeout"`
}

// HealthConfig 健康检查配置
type HealthConfig struct {
	Path    string `mapstructure:"path"`
	Timeout int    `mapstructure:"timeout"`
}

// ExternalServicesConfig 外部服务配置
type ExternalServicesConfig struct {
	APIGateway APIGatewayConfig `mapstructure:"api_gateway"`
	SMSService SMSServiceConfig `mapstructure:"sms_service"`
}

// APIGatewayConfig API网关配置
type APIGatewayConfig struct {
	Addr    string `mapstructure:"addr"`
	Timeout int    `mapstructure:"timeout"`
}

// SMSServiceConfig 短信服务配置
type SMSServiceConfig struct {
	Provider     string `mapstructure:"provider"`
	AccessKey    string `mapstructure:"access_key"`
	SecretKey    string `mapstructure:"secret_key"`
	SignName     string `mapstructure:"sign_name"`
	TemplateCode string `mapstructure:"template_code"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	BcryptCost               int   `mapstructure:"bcrypt_cost"`
	MaxLoginAttempts         int   `mapstructure:"max_login_attempts"`
	LockoutDuration          int64 `mapstructure:"lockout_duration"`
	PasswordMinLength        int   `mapstructure:"password_min_length"`
	PasswordRequireUppercase bool  `mapstructure:"password_require_uppercase"`
	PasswordRequireLowercase bool  `mapstructure:"password_require_lowercase"`
	PasswordRequireDigit     bool  `mapstructure:"password_require_digit"`
	PasswordRequireSpecial   bool  `mapstructure:"password_require_special"`
}

// GlobalConfig 全局配置实例
var GlobalConfig *Config

// Load 加载配置
func Load(configPath string) error {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// 设置环境变量前缀
	viper.SetEnvPrefix("USER_SERVICE")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	// 解析配置
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return fmt.Errorf("解析配置失败: %v", err)
	}

	GlobalConfig = &config
	return nil
}

// GetDSN 获取数据库连接字符串
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		c.Username, c.Password, c.Host, c.Port, c.Database, c.Charset)
}

// GetRedisAddr 获取Redis地址
func (c *RedisConfig) GetRedisAddr() string {
	return c.Addr
}

// GetServerAddr 获取服务器地址
func (c *ServerConfig) GetServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// GetGRPCAddr 获取gRPC地址
func (c *GRPCConfig) GetGRPCAddr() string {
	return fmt.Sprintf(":%d", c.Port)
}

// GetMetricsAddr 获取监控地址
func (c *MetricsConfig) GetMetricsAddr() string {
	return fmt.Sprintf(":%d", c.Port)
}
