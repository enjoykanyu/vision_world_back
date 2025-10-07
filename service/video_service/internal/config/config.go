package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Kafka     KafkaConfig     `mapstructure:"kafka"`
	Discovery DiscoveryConfig `mapstructure:"discovery"`
	Log       LogConfig       `mapstructure:"log"`
}

type ServerConfig struct {
	Address     string `mapstructure:"address"`
	Name        string `mapstructure:"name"`
	Version     string `mapstructure:"version"`
	Environment string `mapstructure:"environment"`
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
	Type     string `mapstructure:"type"` // etcd, consul
	Address  string `mapstructure:"address"`
	Interval int    `mapstructure:"interval"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
	File  string `mapstructure:"file"`
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("video-service")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/video-service/")
	viper.AddConfigPath("$HOME/.video-service")
	viper.AddConfigPath("./config/")
	viper.AddConfigPath(".")

	// 设置默认值
	viper.SetDefault("server.address", ":50052")
	viper.SetDefault("server.name", "video-service")
	viper.SetDefault("server.version", "1.0.0")
	viper.SetDefault("server.environment", "development")

	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 3306)
	viper.SetDefault("database.username", "root")
	viper.SetDefault("database.password", "901project")
	viper.SetDefault("database.database", "videoworld")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)

	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)

	viper.SetDefault("discovery.type", "etcd")
	viper.SetDefault("discovery.address", "localhost:2379")
	viper.SetDefault("discovery.interval", 10)

	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.file", "logs/video-service.log")

	// 读取环境变量
	viper.AutomaticEnv()

	var config Config
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// 配置文件未找到，使用默认值
			if err := viper.Unmarshal(&config); err != nil {
				return nil, fmt.Errorf("failed to unmarshal default config: %w", err)
			}
			return &config, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}
