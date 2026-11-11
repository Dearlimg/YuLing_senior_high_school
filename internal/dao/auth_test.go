package dao

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"

	"YuLing_senior_high_school/internal/model"
)

func TestRedisSessionTTLAndRevocation(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	sessions := NewRedisSessions(client, "test:")
	ctx := context.Background()
	hash := strings.Repeat("a", 64)
	if err := sessions.SaveSession(ctx, hash, 42, time.Hour); err != nil {
		t.Fatal(err)
	}
	if id, err := sessions.SessionUser(ctx, hash); err != nil || id != 42 {
		t.Fatalf("lookup %d: %v", id, err)
	}
	if server.TTL("test:session:"+hash) != time.Hour {
		t.Fatal("session expiry missing")
	}
	server.FastForward(time.Hour)
	if _, err := sessions.SessionUser(ctx, hash); !errors.Is(err, ErrNotFound) {
		t.Fatal("expired redis token accepted")
	}
	_ = sessions.SaveSession(ctx, hash, 42, time.Hour)
	if err := sessions.DeleteSession(ctx, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.SessionUser(ctx, hash); !errors.Is(err, ErrNotFound) {
		t.Fatal("revoked redis token accepted")
	}
}

func TestSQLRegistrationRollsBackCardWhenAccountConflicts(t *testing.T) {
	store, mock, _ := mockStore(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `alumni`").WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectExec("UPDATE `alumni`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `app_user`").WillReturnError(&mysqlDriver.MySQLError{Number: 1062, Message: "duplicate"})
	mock.ExpectRollback()
	err := store.RegisterUser(context.Background(), &model.User{Account: "taken", PasswordHash: "hash"},
		&model.Alumni{Name: "校友", Status: "active", GraduationYear: 2018})
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("expected account conflict: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLLegacyClaimLocksCardAndRevokesOldEditCode(t *testing.T) {
	store, mock, queries := mockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT").WithArgs(uint64(42), "active", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "edit_code_hash"}).AddRow(42, "old-hash"))
	mock.ExpectExec("UPDATE `alumni`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `app_user`").WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectCommit()
	user := &model.User{Account: "claim_user", PasswordHash: "hash"}
	if err := store.RegisterUser(context.Background(), user, &model.Alumni{ID: 42}); err != nil {
		t.Fatal(err)
	}
	if user.AlumniID != 42 || !strings.Contains((*queries)[0], "FOR UPDATE") {
		t.Fatal("claim must bind the locked card")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
