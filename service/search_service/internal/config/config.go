package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 全局配置
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Logger   LoggerConfig   `mapstructure:"logger"`
	Etcd     EtcdConfig     `mapstructure:"etcd"`
	Consul   ConsulConfig   `mapstructure:"consul"`
	Search   SearchConfig   `mapstructure:"search"`
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

// SearchConfig 搜索服务配置
type SearchConfig struct {
	Elasticsearch ElasticsearchConfig `mapstructure:"elasticsearch"`
	Search        SearchSettings      `mapstructure:"search"`
	Indexing      IndexingConfig      `mapstructure:"indexing"`
	Analyzer      AnalyzerConfig      `mapstructure:"analyzer"`
	SearchTypes   SearchTypesConfig   `mapstructure:"search_types"`
	Suggestions   SuggestionsConfig   `mapstructure:"suggestions"`
	Logging       LoggingConfig       `mapstructure:"logging"`
	Cache         CacheConfig         `mapstructure:"cache"`
}

// ElasticsearchConfig Elasticsearch配置
type ElasticsearchConfig struct {
	Enabled     bool          `mapstructure:"enabled"`
	Hosts       []string      `mapstructure:"hosts"`
	Username    string        `mapstructure:"username"`
	Password    string        `mapstructure:"password"`
	IndexPrefix string        `mapstructure:"index_prefix"`
	MaxRetries  int           `mapstructure:"max_retries"`
	Timeout     time.Duration `mapstructure:"request_timeout"`
}

// SearchSettings 搜索设置
type SearchSettings struct {
	DefaultPageSize   int           `mapstructure:"default_page_size"`
	MaxPageSize       int           `mapstructure:"max_page_size"`
	MaxQueryLength    int           `mapstructure:"max_query_length"`
	MinQueryLength    int           `mapstructure:"min_query_length"`
	SearchTimeout     time.Duration `mapstructure:"search_timeout"`
	EnableFuzzySearch bool          `mapstructure:"enable_fuzzy_search"`
	FuzzyThreshold    float64       `mapstructure:"fuzzy_threshold"`
}

// IndexingConfig 索引配置
type IndexingConfig struct {
	BatchSize         int           `mapstructure:"batch_size"`
	RefreshInterval   time.Duration `mapstructure:"refresh_interval"`
	MaxBulkSize       string        `mapstructure:"max_bulk_size"`
	ConcurrentWorkers int           `mapstructure:"concurrent_workers"`
	RetryAttempts     int           `mapstructure:"retry_attempts"`
}

// AnalyzerConfig 分词配置
type AnalyzerConfig struct {
	DefaultAnalyzer       string `mapstructure:"default_analyzer"`
	SearchAnalyzer        string `mapstructure:"search_analyzer"`
	Language              string `mapstructure:"language"`
	EnableSynonym         bool   `mapstructure:"enable_synonym"`
	SynonymDictionaryPath string `mapstructure:"synonym_dictionary_path"`
}

// SearchTypesConfig 搜索类型配置
type SearchTypesConfig struct {
	Video   VideoSearchConfig   `mapstructure:"video"`
	User    UserSearchConfig    `mapstructure:"user"`
	Content ContentSearchConfig `mapstructure:"content"`
}

// VideoSearchConfig 视频搜索配置
type VideoSearchConfig struct {
	Enabled          bool               `mapstructure:"enabled"`
	IndexName        string             `mapstructure:"index_name"`
	SearchableFields []string           `mapstructure:"searchable_fields"`
	BoostFields      map[string]float64 `mapstructure:"boost_fields"`
	FilterFields     []string           `mapstructure:"filter_fields"`
}

// UserSearchConfig 用户搜索配置
type UserSearchConfig struct {
	Enabled          bool               `mapstructure:"enabled"`
	IndexName        string             `mapstructure:"index_name"`
	SearchableFields []string           `mapstructure:"searchable_fields"`
	BoostFields      map[string]float64 `mapstructure:"boost_fields"`
	FilterFields     []string           `mapstructure:"filter_fields"`
}

// ContentSearchConfig 内容搜索配置
type ContentSearchConfig struct {
	Enabled          bool               `mapstructure:"enabled"`
	IndexName        string             `mapstructure:"index_name"`
	SearchableFields []string           `mapstructure:"searchable_fields"`
	BoostFields      map[string]float64 `mapstructure:"boost_fields"`
	FilterFields     []string           `mapstructure:"filter_fields"`
}

// SuggestionsConfig 推荐搜索配置
type SuggestionsConfig struct {
	Enabled              bool          `mapstructure:"enabled"`
	MaxSuggestions       int           `mapstructure:"max_suggestions"`
	MinPrefixLength      int           `mapstructure:"min_prefix_length"`
	CacheDuration        time.Duration `mapstructure:"cache_duration"`
	PopularSearchesLimit int           `mapstructure:"popular_searches_limit"`
}

// LoggingConfig 搜索日志配置
type LoggingConfig struct {
	Enabled            bool          `mapstructure:"enabled"`
	LogSlowQueries     bool          `mapstructure:"log_slow_queries"`
	SlowQueryThreshold time.Duration `mapstructure:"slow_query_threshold"`
	LogNoResults       bool          `mapstructure:"log_no_results"`
	AnalyticsEnabled   bool          `mapstructure:"analytics_enabled"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	TTL             time.Duration `mapstructure:"ttl"`
	MaxEntries      int           `mapstructure:"max_entries"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
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
		v.SetConfigName("search-service")
		v.SetConfigType("yaml")
	}

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 绑定环境变量
	v.AutomaticEnv()
	v.SetEnvPrefix("SEARCH_SERVICE")
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

	return nil
}

// GetDefaultConfigPath 获取默认配置文件路径
func GetDefaultConfigPath() string {
	// 尝试多个可能的配置文件路径
	paths := []string{
		"./config/search-service.yaml",
		"../config/search-service.yaml",
		"../../config/search-service.yaml",
		"./search-service.yaml",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			absPath, _ := filepath.Abs(path)
			return absPath
		}
	}

	return ""
}
