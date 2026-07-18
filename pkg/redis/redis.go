package redis

import (
	"github.com/redis/go-redis/v9"

	"bs-net-monitor/internal/conf"
)

// SpacePrefix 本服务在 Redis 中的键命名空间前缀，所有键必须经 WrapKey 拼装。
const SpacePrefix = "bs-net-monitor:"

var client *redis.Client

// Init 根据配置初始化 Redis 客户端（普通的单点 Redis 客户端）。
func Init(cfg *conf.RedisConfig) {
	client = redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
}

// GetClient 返回已初始化的 Redis 客户端。
func GetClient() *redis.Client {
	return client
}

// WrapKey 给原始键添加服务专属前缀。
func WrapKey(key string) string {
	return SpacePrefix + key
}
