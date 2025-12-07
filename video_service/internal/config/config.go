package config

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Kafka     KafkaConfig     `mapstructure:"kafka"`
	RabbitMQ  RabbitMQConfig  `mapstructure:"rabbitmq"`
	Discovery DiscoveryConfig `mapstructure:"discovery"`
	Logger    LoggerConfig    `mapstructure:"logger"`
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

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	OutputPath string `mapstructure:"output_path"`
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
	// 直接返回默认配置，绕过配置文件读取
	return &Config{
		Server: ServerConfig{
			Address:     ":50052",
			Name:        "video-service",
			Version:     "1.0.0",
			Environment: "development",
		},
		Database: DatabaseConfig{
			Host:         "localhost",
			Port:         3306,
			Username:     "root",
			Password:     "901project",
			Database:     "videoworld",
			MaxOpenConns: 25,
			MaxIdleConns: 5,
		},
		Redis: RedisConfig{
			Host:         "localhost",
			Port:         6379,
			Password:     "",
			DB:           0,
			MaxRetries:   3,
			DialTimeout:  5,
			ReadTimeout:  3,
			WriteTimeout: 3,
		},
		Kafka: KafkaConfig{
			Brokers: []string{"localhost:9092"},
			Topic:   "video-events",
		},
		RabbitMQ: RabbitMQConfig{
			Host:      "localhost",
			Port:      5672,
			Username:  "guest",
			Password:  "guest",
			VHost:     "/",
			QueueName: "audit_queue",
		},
		Discovery: DiscoveryConfig{
			Type:                           "etcd",
			Address:                        "localhost:2379",
			Interval:                       10,
			Timeout:                        5,
			DeregisterCriticalServiceAfter: "30s",
			Consul: ConsulConfig{
				Datacenter: "dc1",
				Token:      "",
			},
			Etcd: EtcdConfig{
				DialTimeout: 5,
				Username:    "",
				Password:    "",
			},
		},
		Logger: LoggerConfig{
			Level:      "info",
			Format:     "json",
			OutputPath: "logs/video-service.log",
		},
		Services: ServicesConfig{
			AuditService: ServiceConfig{
				Name:    "audit-service",
				Address: "localhost:50053",
				Timeout: 5,
			},
		},
		MinIO: MinIOConfig{
			Endpoint:        "localhost:9000",
			AccessKeyID:     "minioadmin",
			SecretAccessKey: "minioadmin",
			UseSSL:          false,
			BucketName:      "videos",
			Location:        "us-east-1",
		},
	}, nil
}
