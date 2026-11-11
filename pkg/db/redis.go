package db

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"YuLing_senior_high_school/internal/conf"
)

// NewRedis 建立 Redis 客户端并完成一次探活。
//
// 注意：go-redis 的 Options 没有 KeyPrefix 字段（不像某些客户端），
// 因此键前缀不能在这里"自动生效"。KeyPrefix 由 conf 持有并传给 dao 层，
// 由 dao 在拼 key 时统一调用 keyOf()，见 internal/dao/cache.go。
// 这一点必须写清楚，否则很容易误以为前缀已经生效而直接裸写 key，
// 在共享实例上和其他业务撞键。
func NewRedis(cfg conf.Redis) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis 探活失败(%s db=%d): %w", cfg.Addr, cfg.DB, err)
	}
	return client, nil
}

// CloseRedis 关闭客户端连接池。
func CloseRedis(client *redis.Client) error {
	return client.Close()
}
