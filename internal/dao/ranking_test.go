package dao

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"YuLing_senior_high_school/internal/model"
)

func TestSortedSetRankingInvalidationAndFallback(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	base := NewMemoryStore()
	store := NewRankedStore(base, client, "test:")
	ctx := context.Background()
	for _, content := range []string{"校庆100%", "新帖"} {
		if err := store.CreatePost(ctx, &model.Post{Nickname: "作者", Content: content, Status: "active"}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := store.RankedPosts(ctx, "all")
	if err != nil || len(rows) != 2 || rows[0].ID != 2 {
		t.Fatalf("initial rank: %#v %v", rows, err)
	}
	// A second read must use the actual Sorted Set/Hash path with the same stable tie order.
	rows, err = store.RankedPosts(ctx, "all")
	if err != nil || rows[0].ID != 2 {
		t.Fatal("cached rank differs")
	}
	count, err := store.CreateReply(ctx, &model.Reply{PostID: 1, Nickname: "回复者", Content: "回复", Status: "active"})
	if err != nil || count != 1 {
		t.Fatal(err)
	}
	rows, err = store.RankedPosts(ctx, "all")
	if err != nil || rows[0].ID != 1 || rows[0].ReplyCount != 1 {
		t.Fatal("reply did not invalidate scores")
	}
	if err := store.Moderate(ctx, model.AuditLog{Kind: "reply", TargetID: 1, Action: "hidden"}); err != nil {
		t.Fatal(err)
	}
	rows, _ = store.RankedPosts(ctx, "all")
	if rows[0].ID != 2 {
		t.Fatal("hidden reply counted in rank")
	}
	base.mu.Lock()
	old := base.posts[2]
	old.CreatedAt = time.Now().Add(-8 * 24 * time.Hour)
	base.posts[2] = old
	base.mu.Unlock()
	week, _ := store.RankedPosts(ctx, "week")
	if len(week) != 1 || week[0].ID != 1 {
		t.Fatal("week includes old post")
	}
	if err := store.Moderate(ctx, model.AuditLog{Kind: "post", TargetID: 1, Action: "hidden"}); err != nil {
		t.Fatal(err)
	}
	rows, _ = store.RankedPosts(ctx, "all")
	if len(rows) != 1 || rows[0].ID != 2 {
		t.Fatal("hidden post still ranked")
	}
	server.FastForward(61 * time.Second)
	rows, err = store.RankedPosts(ctx, "all")
	if err != nil || len(rows) != 1 {
		t.Fatal("expiry failed to rebuild")
	}
	server.Close()
	rows, err = store.RankedPosts(ctx, "all")
	if err != nil || len(rows) != 1 {
		t.Fatal("redis outage did not fall back")
	}
}

func TestPostSearchEscapesWildcardsAndWhitelistsOrder(t *testing.T) {
	store, mock, queries := mockStore(t)
	pattern := "%100!%!_%"
	mock.ExpectQuery("SELECT count").WithArgs("active", pattern, pattern).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT").WithArgs("active", pattern, pattern, 10).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	_, _, err := store.ListPosts(context.Background(), ContentQuery{Keyword: "100%_", Sort: "hot", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains((*queries)[1], "reply_count DESC, post.id DESC") {
		t.Fatal("rank order missing")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
