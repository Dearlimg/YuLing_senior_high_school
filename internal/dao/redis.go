package dao

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisLimiter struct {
	client *redis.Client
	prefix string
}

func NewRedisLimiter(client *redis.Client, prefix string) *RedisLimiter {
	return &RedisLimiter{client: client, prefix: prefix}
}

var limitScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
return count
`)

func (l *RedisLimiter) Allow(ctx context.Context, key string, limit int, period time.Duration) (bool, error) {
	count, err := limitScript.Run(ctx, l.client, []string{l.prefix + "rate:" + key}, period.Milliseconds()).Int64()
	return count <= int64(limit), err
}
