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
	Live     LiveConfig     `mapstructure:"live"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	HTTPPort     int           `mapstructure:"http_port"`
	Mode         string        `mapstructure:"mode"`
	EnableHTTP   bool          `mapstructure:"enable_http"`
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

// LiveConfig 直播服务配置
type LiveConfig struct {
	RTMP        RTMPConfig        `mapstructure:"rtmp"`
	WebRTC      WebRTCConfig      `mapstructure:"webrtc"`
	Stream      StreamConfig      `mapstructure:"stream"`
	Recording   RecordingConfig   `mapstructure:"recording"`
	Transcoding TranscodingConfig `mapstructure:"transcoding"`
	Limits      LimitsConfig      `mapstructure:"limits"`
	CDN         CDNConfig         `mapstructure:"cdn"`
}

// RTMPConfig RTMP配置
type RTMPConfig struct {
	Host        string        `mapstructure:"host"`
	Port        int           `mapstructure:"port"`
	ChunkSize   int           `mapstructure:"chunk_size"`
	IdleTimeout time.Duration `mapstructure:"idle_timeout"`
}

// WebRTCConfig WebRTC配置
type WebRTCConfig struct {
	Enabled     bool               `mapstructure:"enabled"`
	StunServers []string           `mapstructure:"stun_servers"`
	TurnServers []TurnServerConfig `mapstructure:"turn_servers"`
}

// TurnServerConfig TURN服务器配置
type TurnServerConfig struct {
	URL      string `mapstructure:"url"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// StreamConfig 直播流配置
type StreamConfig struct {
	MaxBitrate       int    `mapstructure:"max_bitrate"`
	MaxResolution    string `mapstructure:"max_resolution"`
	KeyframeInterval int    `mapstructure:"keyframe_interval"`
	BufferSize       int    `mapstructure:"buffer_size"`
}

// RecordingConfig 录制配置
type RecordingConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	StoragePath     string `mapstructure:"storage_path"`
	Format          string `mapstructure:"format"`
	SegmentDuration int    `mapstructure:"segment_duration"`
	MaxFileSize     int64  `mapstructure:"max_file_size"`
}

// TranscodingConfig 转码配置
type TranscodingConfig struct {
	Enabled  bool               `mapstructure:"enabled"`
	Profiles []TranscodeProfile `mapstructure:"profiles"`
}

// TranscodeProfile 转码配置
type TranscodeProfile struct {
	Name       string `mapstructure:"name"`
	Resolution string `mapstructure:"resolution"`
	Bitrate    int    `mapstructure:"bitrate"`
	Framerate  int    `mapstructure:"framerate"`
}

// LimitsConfig 限制配置
type LimitsConfig struct {
	MaxConcurrentStreams int `mapstructure:"max_concurrent_streams"`
	MaxViewersPerStream  int `mapstructure:"max_viewers_per_stream"`
	MaxStreamDuration    int `mapstructure:"max_stream_duration"`
	BanDuration          int `mapstructure:"ban_duration"`
}

// CDNConfig CDN配置
type CDNConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	BaseURL string   `mapstructure:"base_url"`
	Regions []string `mapstructure:"regions"`
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
		v.SetConfigName("live-service")
		v.SetConfigType("yaml")
	}

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 绑定环境变量
	v.AutomaticEnv()
	v.SetEnvPrefix("LIVE_SERVICE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Server.HTTPPort <= 0 || c.Server.HTTPPort > 65535 {
		return fmt.Errorf("invalid http port: %d", c.Server.HTTPPort)
	}

	if c.Server.Port == c.Server.HTTPPort {
		return fmt.Errorf("gRPC port and HTTP port cannot be the same")
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

	return nil
}

// GetDefaultConfigPath 获取默认配置文件路径
func GetDefaultConfigPath() string {
	// 尝试多个可能的配置文件路径
	paths := []string{
		"./config/live-service.yaml",
		"../config/live-service.yaml",
		"../../config/live-service.yaml",
		"./live-service.yaml",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			absPath, _ := filepath.Abs(path)
			return absPath
		}
	}

	return ""
}
