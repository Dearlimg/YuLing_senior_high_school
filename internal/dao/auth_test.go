package dao

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
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
