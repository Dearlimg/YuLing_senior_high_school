package dao

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"YuLing_senior_high_school/internal/model"
)

type countedStore struct {
	Store
	reads   atomic.Int64
	loaded  chan struct{}
	release chan struct{}
}

func (s *countedStore) ListAlumni(ctx context.Context, q ListQuery) ([]model.Alumni, int64, error) {
	n := s.reads.Add(1)
	rows, total, err := s.Store.ListAlumni(ctx, q)
	if n == 1 && s.loaded != nil {
		close(s.loaded)
		<-s.release
	}
	return rows, total, err
}

func cacheFixture(t *testing.T) (*CachedStore, *countedStore, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1, ContextTimeoutEnabled: true})
	t.Cleanup(func() { _ = client.Close() })
	base := &countedStore{Store: NewMemoryStore()}
	return NewCachedStore(base, client, "test:"), base, server
}

func TestCachedPhotoInteractionsAndReplyTargets(t *testing.T) {
	store, _, _ := cacheFixture(t)
	ctx := context.Background()
	photo := &model.Photo{Caption: "校园", Status: "active"}
	if err := store.CreatePhoto(ctx, photo); err != nil {
		t.Fatal(err)
	}
	query := ContentQuery{Page: 1, PageSize: 20}
	_, _, _ = store.ListPhotos(ctx, query)
	_, _, _ = store.ListPhotoReplies(ctx, photo.ID, query)
	_, err := store.CreateReply(ctx, &model.Reply{PhotoID: photo.ID, Nickname: "甲", Content: "评论", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.CreateReply(ctx, &model.Reply{PhotoID: photo.ID, ParentID: 1, Nickname: "乙", Content: "回复", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SetLike(ctx, model.ContentLike{Kind: "photo", TargetID: photo.ID, UserID: 1}, true)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		photos, _, _ := store.ListPhotos(ctx, query)
		rows, total, err := store.ListPhotoReplies(ctx, photo.ID, query)
		if err != nil || total != 2 || rows[1].ParentNickname != "甲" || photos[0].LikeCount != 1 || photos[0].ReplyCount != 2 {
			t.Fatalf("stale photo interactions: %+v %+v %v", photos, rows, err)
		}
	}
}

func TestQueryCachePrivacyInvalidationAndFailureFallback(t *testing.T) {
	store, base, server := cacheFixture(t)
	ctx := context.Background()
	card := &model.Alumni{Name: "校友", GraduationYear: 2018, Status: "active", Phone: "private-phone", Wechat: "private-wechat", Email: "private-email", EditCodeHash: "private-hash"}
	if err := store.CreateAlumni(ctx, card); err != nil {
		t.Fatal(err)
	}
	q := ListQuery{Page: 1, PageSize: 10}
	for range 2 {
		rows, total, err := store.ListAlumni(ctx, q)
		if err != nil || total != 1 || len(rows) != 1 {
			t.Fatal("cache read failed")
		}
		if rows[0].Phone != "" || rows[0].EditCodeHash != "" {
			t.Fatal("private fields cached")
		}
	}
	if base.reads.Load() != 1 {
		t.Fatal("repeat query missed cache")
	}
	for _, key := range server.Keys() {
		value, _ := server.Get(key)
		for _, secret := range []string{"private-phone", "private-wechat", "private-email", "private-hash"} {
			if strings.Contains(value, secret) {
				t.Fatal("redis contains private card field")
			}
		}
	}
	owner, err := store.FindAlumni(ctx, card.ID)
	if err != nil || owner.Phone != "private-phone" {
		t.Fatal("owner data must bypass public cache")
	}
	card.City = "西安"
	if err := store.UpdateAlumni(ctx, card); err != nil {
		t.Fatal(err)
	}
	rows, _, _ := store.ListAlumni(ctx, q)
	if rows[0].City != "西安" || base.reads.Load() != 2 {
		t.Fatal("write did not invalidate")
	}
	_, _, _ = store.ListAlumni(ctx, ListQuery{Page: 1, PageSize: 10, City: "北京"})
	if base.reads.Load() != 3 {
		t.Fatal("query parameters share a cache key")
	}
	server.FastForward(81 * time.Second)
	_, _, _ = store.ListAlumni(ctx, q)
	if base.reads.Load() != 4 {
		t.Fatal("expired query was not reloaded")
	}
	// Evicting a generation key must not resurrect old entries.
	server.Del("test:query:v1:version")
	_, _, _ = store.ListAlumni(ctx, q)
	if base.reads.Load() != 5 {
		t.Fatal("generation reuse after eviction")
	}
	server.Close()
	_, total, err := store.ListAlumni(ctx, q)
	if err != nil || total != 1 || base.reads.Load() != 6 {
		t.Fatal("cache outage prevented database fallback")
	}
}

func TestConcurrentOldReadCannotRefillNewGeneration(t *testing.T) {
	store, base, _ := cacheFixture(t)
	ctx := context.Background()
	card := &model.Alumni{Name: "之前", Status: "active", GraduationYear: 2018}
	if err := store.CreateAlumni(ctx, card); err != nil {
		t.Fatal(err)
	}
	base.loaded, base.release = make(chan struct{}), make(chan struct{})
	q := ListQuery{Page: 1, PageSize: 10}
	done := make(chan struct{})
	go func() { defer close(done); _, _, _ = store.ListAlumni(ctx, q) }()
	<-base.loaded
	card.Name = "之后"
	if err := store.UpdateAlumni(ctx, card); err != nil {
		t.Fatal(err)
	}
	close(base.release)
	<-done
	rows, _, err := store.ListAlumni(ctx, q)
	if err != nil || rows[0].Name != "之后" {
		t.Fatal("stale in-flight data filled current cache")
	}
}

func TestConcurrentMissesUseOneDatabaseRead(t *testing.T) {
	store, base, _ := cacheFixture(t)
	base.loaded, base.release = make(chan struct{}), make(chan struct{})
	var group sync.WaitGroup
	for range 16 {
		group.Go(func() {
			_, _, err := store.ListAlumni(context.Background(), ListQuery{Page: 1, PageSize: 10})
			if err != nil {
				t.Error(err)
			}
		})
	}
	<-base.loaded
	close(base.release)
	group.Wait()
	if base.reads.Load() != 1 {
		t.Fatalf("got %d database loads", base.reads.Load())
	}
}
