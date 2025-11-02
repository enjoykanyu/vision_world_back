package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Kafka     KafkaConfig     `mapstructure:"kafka"`
	RabbitMQ  RabbitMQConfig  `mapstructure:"rabbitmq"`
	Discovery DiscoveryConfig `mapstructure:"discovery"`
	Log       LogConfig       `mapstructure:"log"`
	Services  ServicesConfig  `mapstructure:"services"`
	MinIO     MinIOConfig     `mapstructure:"minio"`
}

type ServerConfig struct {
	Address     string `mapstructure:"address"`
	Name        string `mapstructure:"name"`
	Version     string `mapstructure:"version"`
	Environment string `mapstructure:"environment"`
}

// MinIOConfig MinIO配置
type MinIOConfig struct {
	Endpoint        string `mapstructure:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	UseSSL          bool   `mapstructure:"use_ssl"`
	BucketName      string `mapstructure:"bucket_name"`
	Location        string `mapstructure:"location"`
}

type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	Database     string `mapstructure:"database"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
	Topic   string   `mapstructure:"topic"`
}

type DiscoveryConfig struct {
	Type                           string       `mapstructure:"type"` // etcd, consul
	Address                        string       `mapstructure:"address"`
	Interval                       int          `mapstructure:"interval"`                          // 健康检查间隔(秒)
	Timeout                        int          `mapstructure:"timeout"`                           // 健康检查超时(秒)
	DeregisterCriticalServiceAfter string       `mapstructure:"deregister_critical_service_after"` // 服务不健康后多久注销
	Consul                         ConsulConfig `mapstructure:"consul"`
	Etcd                           EtcdConfig   `mapstructure:"etcd"`
}

type ConsulConfig struct {
	Datacenter string `mapstructure:"datacenter"`
	Token      string `mapstructure:"token"` // ACL令牌(如果需要)
}

type EtcdConfig struct {
	DialTimeout int    `mapstructure:"dial_timeout"` // 连接超时(秒)
	Username    string `mapstructure:"username"`     // 用户名(如果需要)
	Password    string `mapstructure:"password"`     // 密码(如果需要)
}

type LogConfig struct {
	Level string `mapstructure:"level"`
	File  string `mapstructure:"file"`
}

type ServicesConfig struct {
	AuditService ServiceConfig `mapstructure:"audit_service"`
}

type RabbitMQConfig struct {
	Host      string `mapstructure:"host"`
	Port      int    `mapstructure:"port"`
	Username  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
	VHost     string `mapstructure:"vhost"`
	QueueName string `mapstructure:"queue_name"`
}

type ServiceConfig struct {
	Name    string `mapstructure:"name"`
	Address string `mapstructure:"address"`
	Timeout int    `mapstructure:"timeout"`
}

func LoadConfig() (*Config, error) {
	v := viper.New()

	// 设置配置文件路径
	// 默认在当前目录和config目录下查找配置文件
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("../config")
	v.AddConfigPath("../../config")
	v.SetConfigName("video-service")
	v.SetConfigType("yaml")

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 绑定环境变量
	v.AutomaticEnv()
	v.SetEnvPrefix("VIDEO_SERVICE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 设置默认变量
	v.SetDefault("minio.endpoint", "localhost:9000")
	v.SetDefault("minio.access_key_id", "minioadmin")
	v.SetDefault("minio.secret_access_key", "minioadmin")
	v.SetDefault("minio.use_ssl", false)
	v.SetDefault("minio.bucket_name", "videos")
	v.SetDefault("minio.location", "us-east-1")

	// 设置服务发现默认值
	v.SetDefault("discovery.type", "etcd")
	v.SetDefault("discovery.address", "localhost:2379")
	v.SetDefault("discovery.interval", 10)
	v.SetDefault("discovery.timeout", 5)
	v.SetDefault("discovery.deregister_critical_service_after", "30s")

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}
