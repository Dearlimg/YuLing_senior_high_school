package dao

import (
	"context"
	"sync/atomic"
	"time"

	"YuLing_senior_high_school/pkg/observability"
)

// A shared outage can affect every request; bound warning volume without logging cache keys/queries.
func cacheWarning(ctx context.Context, last *atomic.Int64, operation string, err error) {
	now := time.Now().Unix()
	previous := last.Load()
	if now-previous < 5 || !last.CompareAndSwap(previous, now) {
		return
	}
	fields := []any{"event", "cache_error", "operation", operation, "fallback", "database"}
	observability.Logger(ctx).Warn("Redis 缓存不可用", append(fields, observability.ErrorFields(err)...)...)
}
