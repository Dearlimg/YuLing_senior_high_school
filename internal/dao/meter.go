package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TrafficMeter 全量流量底账（含爬虫与非浏览器流量），与真实访客指标分离。
// 只按天计数，不落明细；键 72 小时过期。
type TrafficMeter interface {
	Track(ctx context.Context, ipHash string, now time.Time) error
	Today(ctx context.Context, now time.Time) (requests, ips int64, err error)
}

type RedisMeter struct {
	client *redis.Client
	prefix string
}

func NewRedisMeter(client *redis.Client, prefix string) *RedisMeter {
	return &RedisMeter{client: client, prefix: prefix + "visit:"}
}

func meterKeys(now time.Time) (string, string) {
	day := now.In(time.FixedZone("CST", 8*3600)).Format("20060102")
	return "req:" + day, "ip:" + day
}

var meterScript = redis.NewScript(`
redis.call('INCR', KEYS[1])
redis.call('SADD', KEYS[2], ARGV[1])
redis.call('EXPIRE', KEYS[1], 259200)
redis.call('EXPIRE', KEYS[2], 259200)
return 1
`)

func (m *RedisMeter) Track(ctx context.Context, ipHash string, now time.Time) error {
	reqKey, ipKey := meterKeys(now)
	return meterScript.Run(ctx, m.client, []string{m.prefix + reqKey, m.prefix + ipKey}, ipHash).Err()
}

func (m *RedisMeter) Today(ctx context.Context, now time.Time) (int64, int64, error) {
	reqKey, ipKey := meterKeys(now)
	requests, err := m.client.Get(ctx, m.prefix+reqKey).Int64()
	if err == redis.Nil {
		requests = 0
	} else if err != nil {
		return 0, 0, fmt.Errorf("读取底账请求计数: %w", err)
	}
	ips, err := m.client.SCard(ctx, m.prefix+ipKey).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("读取底账 IP 集合: %w", err)
	}
	return requests, ips, nil
}
