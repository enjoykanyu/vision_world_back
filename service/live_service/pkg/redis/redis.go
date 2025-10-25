package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

var client *redis.Client

// Init 初始化Redis连接
func Init(config map[string]interface{}) (*redis.Client, error) {
	addr, ok := config["addr"].(string)
	if !ok {
		addr = "localhost:6379"
	}

	password, _ := config["password"].(string)

	db := 0
	if dbNum, ok := config["db"].(int); ok {
		db = dbNum
	}

	client = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %v", err)
	}

	return client, nil
}

// GetClient 获取Redis客户端
func GetClient() *redis.Client {
	return client
}

// Close 关闭Redis连接
func (c *redis.Client) Close() error {
	return c.Close()
}
