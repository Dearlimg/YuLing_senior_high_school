package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"YuLing_senior_high_school/internal/model"
)

// RankedStore uses a Sorted Set for scores and a Hash for the corresponding public projections.
// MySQL remains authoritative; cache errors always fall back to the underlying store.
type RankedStore struct {
	Store
	client *redis.Client
	prefix string
}

func NewRankedStore(store Store, client *redis.Client, prefix string) *RankedStore {
	return &RankedStore{Store: store, client: client, prefix: prefix + "ranking:v1:"}
}

var readRanking = redis.NewScript(`
local count = redis.call('HGET', KEYS[2], '__count')
if not count or redis.call('ZCARD', KEYS[1]) ~= tonumber(count) then return nil end
local result = {'ready'}
for _, id in ipairs(redis.call('ZREVRANGE', KEYS[1], 0, 9)) do
  local row = redis.call('HGET', KEYS[2], id)
  if not row then return nil end
  table.insert(result, row)
end
return result
`)

var saveRanking = redis.NewScript(`
redis.call('DEL', KEYS[1], KEYS[2])
redis.call('HSET', KEYS[2], '__count', (#ARGV - 1) / 3)
for i = 2, #ARGV, 3 do
  redis.call('ZADD', KEYS[1], ARGV[i], ARGV[i+1])
  redis.call('HSET', KEYS[2], ARGV[i+1], ARGV[i+2])
end
redis.call('EXPIRE', KEYS[1], ARGV[1])
redis.call('EXPIRE', KEYS[2], ARGV[1])
return 1
`)

func (s *RankedStore) RankedPosts(ctx context.Context, period string) ([]model.Post, error) {
	cacheCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	version, err := cacheGeneration(cacheCtx, s.client, s.prefix+"version")

	key := s.prefix + version + ":" + period
	keys := []string{key, key + ":rows"}
	if err == nil {
		values, readErr := readRanking.Run(cacheCtx, s.client, keys).StringSlice()
		if readErr == nil && len(values) > 0 {
			rows := make([]model.Post, 0, len(values)-1)
			valid := true
			for _, value := range values[1:] {
				var row model.Post
				if json.Unmarshal([]byte(value), &row) != nil {
					valid = false
					break
				}
				rows = append(rows, row)
			}
			if valid {
				return rows, nil
			}
		}
	}
	rows, loadErr := s.Store.RankedPosts(ctx, period)
	if loadErr != nil {
		return nil, loadErr
	}
	if err == nil {
		args := []any{60}
		for _, row := range rows {
			// Projection only: never place image bytes or idempotency/management secrets in this cache.
			row.Image, row.CreateKey = nil, nil
			encoded, encodeErr := json.Marshal(row)
			if encodeErr != nil {
				break
			}
			args = append(args, row.ReplyCount, fmt.Sprintf("%020d", row.ID), string(encoded))
		}
		writeCtx, writeCancel := context.WithTimeout(ctx, 200*time.Millisecond)
		defer writeCancel()
		_ = saveRanking.Run(writeCtx, s.client, keys, args...).Err()
	}
	return rows[:min(len(rows), 10)], nil
}

func (s *RankedStore) invalidateRanking(ctx context.Context) {
	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cacheTimeout)
	defer cancel()
	// Generations prevent an in-flight pre-write read from repopulating the current leaderboard.
	_ = bumpGeneration(cacheCtx, s.client, s.prefix+"version")
}

func (s *RankedStore) CreatePost(ctx context.Context, row *model.Post) error {
	if err := s.Store.CreatePost(ctx, row); err != nil {
		return err
	}
	s.invalidateRanking(ctx)
	return nil
}

func (s *RankedStore) CreateReply(ctx context.Context, row *model.Reply) (int64, error) {
	count, err := s.Store.CreateReply(ctx, row)
	if err == nil {
		s.invalidateRanking(ctx)
	}
	return count, err
}

func (s *RankedStore) Moderate(ctx context.Context, audit model.AuditLog) error {
	if err := s.Store.Moderate(ctx, audit); err != nil {
		return err
	}
	if audit.Kind == "post" || audit.Kind == "reply" {
		s.invalidateRanking(ctx)
	}
	return nil
}
