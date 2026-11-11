// Package observability carries safe structured logging fields across HTTP, logic and database layers.
package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

type loggerKey struct{}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

func Logger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// ErrorFields deliberately excludes err.Error(): driver errors can contain SQL parameters and credentials.
func ErrorFields(err error) []any {
	fields := []any{"error_type", fmt.Sprintf("%T", err)}
	var databaseError *mysqlDriver.MySQLError
	if errors.As(err, &databaseError) {
		fields = append(fields, "mysql_code", databaseError.Number)
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		fields = append(fields, "timeout", networkError.Timeout())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		fields = append(fields, "deadline_exceeded", true)
	}
	if errors.Is(err, context.Canceled) {
		fields = append(fields, "request_canceled", true)
	}
	return fields
}
