package db

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm/logger"

	"YuLing_senior_high_school/pkg/observability"
)

func TestSlowAndFailedQueriesAreCorrelatedAndRedacted(t *testing.T) {
	var output bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&output, nil)).With("request_id", "db-trace")
	ctx := observability.WithLogger(context.Background(), log)
	l := queryLogger{level: logger.Warn}
	l.Trace(ctx, time.Now().Add(-time.Second), func() (string, int64) {
		return "INSERT INTO app_user VALUES ('private-password-hash', 'private-email')", 1
	}, &mysqlDriver.MySQLError{Number: 1062, Message: "private-password-hash"})
	text := output.String()
	for _, secret := range []string{"private-password-hash", "private-email", "INSERT INTO"} {
		if strings.Contains(text, secret) {
			t.Fatal("database logs exposed SQL or values")
		}
	}
	for _, field := range []string{`"request_id":"db-trace"`, `"mysql_code":1062`, `"slow":true`, `"operation":"INSERT"`, `"query_hash":`} {
		if !strings.Contains(text, field) {
			t.Fatalf("missing %s", field)
		}
	}
	sql, params := l.ParamsFilter(ctx, "SELECT * FROM alumni WHERE phone = ?", "private-phone")
	if sql != "SELECT * FROM alumni WHERE phone = ?" || len(params) != 0 {
		t.Fatal("SQL interpolation enabled")
	}
}
