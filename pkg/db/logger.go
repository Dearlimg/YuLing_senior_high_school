package db

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"YuLing_senior_high_school/pkg/observability"
)

type queryLogger struct{ level logger.LogLevel }

func (l queryLogger) LogMode(level logger.LogLevel) logger.Interface {
	return queryLogger{level: level}
}
func (l queryLogger) Info(ctx context.Context, _ string, _ ...any) {
	if l.level >= logger.Info {
		observability.Logger(ctx).Debug("数据库提示", "event", "database_info")
	}
}
func (l queryLogger) Warn(ctx context.Context, _ string, _ ...any) {
	if l.level >= logger.Warn {
		observability.Logger(ctx).Warn("数据库警告", "event", "database_warning")
	}
}
func (l queryLogger) Error(ctx context.Context, _ string, _ ...any) {
	if l.level >= logger.Error {
		observability.Logger(ctx).Error("数据库错误", "event", "database_error")
	}
}

// ParamsFilter prevents GORM from interpolating contact data or password hashes into SQL text.
func (l queryLogger) ParamsFilter(_ context.Context, sql string, _ ...any) (string, []any) {
	return sql, nil
}

func (l queryLogger) Trace(ctx context.Context, start time.Time, query func() (string, int64), err error) {
	if l.level == logger.Silent {
		return
	}
	duration := time.Since(start)
	failed := err != nil && !errors.Is(err, gorm.ErrRecordNotFound)
	slow := duration >= 500*time.Millisecond
	if !failed && !slow {
		return
	}
	sql, rows := query()
	operation := "unknown"
	if words := strings.Fields(sql); len(words) > 0 {
		operation = strings.ToUpper(words[0])
	}
	fields := []any{"event", "database_query", "operation", operation,
		"query_hash", fmt.Sprintf("%x", sha256.Sum256([]byte(sql)))[:16], "duration_ms", duration.Milliseconds(), "rows", rows, "slow", slow}
	level := slog.LevelWarn
	if failed {
		level = slog.LevelError
		fields = append(fields, observability.ErrorFields(err)...)
	}
	// Raw statements, bound parameters and driver error messages are deliberately excluded.
	observability.Logger(ctx).Log(ctx, level, "数据库查询", fields...)
}
