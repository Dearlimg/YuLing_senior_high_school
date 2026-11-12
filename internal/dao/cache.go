package dao

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"

	"YuLing_senior_high_school/internal/model"
)

const cacheTimeout = 200 * time.Millisecond

// CachedStore caches public query projections. Identity, owner data and image bytes always hit MySQL.
type CachedStore struct {
	Store
	client      *redis.Client
	prefix      string
	loads       singleflight.Group
	lastWarning atomic.Int64
}

func NewCachedStore(store Store, client *redis.Client, prefix string) *CachedStore {
	return &CachedStore{Store: store, client: client, prefix: prefix + "query:v1:"}
}

var generationScript = redis.NewScript(`
local version = redis.call('GET', KEYS[1])
if version then return version end
redis.call('SET', KEYS[1], ARGV[1])
return ARGV[1]
`)

func cacheGeneration(ctx context.Context, client *redis.Client, key string) (string, error) {
	return generationScript.Run(ctx, client, []string{key}, rand.Text()).Text()
}

func bumpGeneration(ctx context.Context, client *redis.Client, key string) error {
	return client.Set(ctx, key, rand.Text(), 0).Err()
}

type cachedPage[T any] struct {
	Rows  []T
	Total int64
}

func cached[T any](ctx context.Context, s *CachedStore, kind string, query any, load func() (T, error)) (T, error) {
	cacheCtx, cancel := context.WithTimeout(ctx, cacheTimeout)
	version, err := cacheGeneration(cacheCtx, s.client, s.prefix+"version")
	cancel()
	if err != nil {
		cacheWarning(ctx, &s.lastWarning, "query.version", err)
		return load()
	}
	encoded, _ := json.Marshal(query)
	key := fmt.Sprintf("%s%s:%s:%x", s.prefix, version, kind, sha256.Sum256(encoded))
	read := func() ([]byte, error) {
		readCtx, readCancel := context.WithTimeout(ctx, cacheTimeout)
		defer readCancel()
		data, err := s.client.Get(readCtx, key).Bytes()
		if err != nil && err != redis.Nil {
			cacheWarning(ctx, &s.lastWarning, "query.read", err)
		}
		return data, err
	}
	if data, readErr := read(); readErr == nil {
		var value T
		if json.Unmarshal(data, &value) == nil {
			return value, nil
		}
	}
	// Coalesce a burst of identical misses per process. Redis TTL jitter spreads cross-key expirations.
	result, err, _ := s.loads.Do(key, func() (any, error) {
		if data, readErr := read(); readErr == nil {
			var value T
			if json.Unmarshal(data, &value) == nil {
				return data, nil
			}
		}
		value, loadErr := load()
		if loadErr != nil {
			return nil, loadErr
		}
		data, encodeErr := json.Marshal(value)
		if encodeErr != nil {
			return nil, encodeErr
		}
		// Bound individual values and never cache errors. Old-generation fills are unreachable after a write.
		if len(data) <= 512<<10 {
			writeCtx, writeCancel := context.WithTimeout(ctx, cacheTimeout)
			defer writeCancel()
			ttl := time.Duration(60+int(sha256.Sum256([]byte(key))[0])%21) * time.Second
			if err := s.client.Set(writeCtx, key, data, ttl).Err(); err != nil {
				cacheWarning(ctx, &s.lastWarning, "query.fill", err)
			}
		}
		return data, nil
	})
	var value T
	if err != nil {
		return value, err
	}
	err = json.Unmarshal(result.([]byte), &value)
	return value, err
}

func (s *CachedStore) invalidate(ctx context.Context) {
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cacheTimeout)
	defer cancel()
	if err := bumpGeneration(writeCtx, s.client, s.prefix+"version"); err != nil {
		cacheWarning(ctx, &s.lastWarning, "query.invalidate", err)
	}
}

func publicCard(row model.Alumni) model.Alumni {
	return model.Alumni{ID: row.ID, Name: row.Name, Identity: row.Identity, GraduationYear: row.GraduationYear,
		ClassName: row.ClassName, City: row.City, University: row.University, DreamUniversity: row.DreamUniversity,
		Major: row.Major, Education: row.Education, Company: row.Company, Position: row.Position, Message: row.Message,
		AvatarColor: row.AvatarColor, ContactPublic: row.ContactPublic}
}

func (s *CachedStore) ListAlumni(ctx context.Context, q ListQuery) ([]model.Alumni, int64, error) {
	page, err := cached(ctx, s, "alumni", q, func() (cachedPage[model.Alumni], error) {
		rows, total, err := s.Store.ListAlumni(ctx, q)
		for i := range rows {
			rows[i] = publicCard(rows[i])
		}
		return cachedPage[model.Alumni]{Rows: rows, Total: total}, err
	})
	return page.Rows, page.Total, err
}

func (s *CachedStore) DirectoryMeta(ctx context.Context) (DirectoryMeta, error) {
	return cached(ctx, s, "meta", nil, func() (DirectoryMeta, error) { return s.Store.DirectoryMeta(ctx) })
}

func (s *CachedStore) Classes(ctx context.Context, q ListQuery) ([]ClassGroup, int64, error) {
	page, err := cached(ctx, s, "classes", q, func() (cachedPage[ClassGroup], error) {
		rows, total, err := s.Store.Classes(ctx, q)
		return cachedPage[ClassGroup]{Rows: rows, Total: total}, err
	})
	return page.Rows, page.Total, err
}

func (s *CachedStore) Universities(ctx context.Context) ([]University, error) {
	return cached(ctx, s, "universities", nil, func() ([]University, error) { return s.Store.Universities(ctx) })
}

func (s *CachedStore) Stats(ctx context.Context) (Stats, error) {
	return cached(ctx, s, "stats", nil, func() (Stats, error) { return s.Store.Stats(ctx) })
}

func (s *CachedStore) ListPhotos(ctx context.Context, q ContentQuery) ([]model.Photo, int64, error) {
	page, err := cached(ctx, s, "photos", q, func() (cachedPage[model.Photo], error) {
		rows, total, err := s.Store.ListPhotos(ctx, q)
		for i, row := range rows {
			rows[i] = model.Photo{LikeCount: row.LikeCount, ReplyCount: row.ReplyCount, ID: row.ID, Caption: row.Caption, Uploader: row.Uploader, Category: row.Category, CreatedAt: row.CreatedAt}
		}
		return cachedPage[model.Photo]{Rows: rows, Total: total}, err
	})
	return page.Rows, page.Total, err
}

func (s *CachedStore) ListPosts(ctx context.Context, q ContentQuery) ([]model.Post, int64, error) {
	page, err := cached(ctx, s, "posts", q, func() (cachedPage[model.Post], error) {
		rows, total, err := s.Store.ListPosts(ctx, q)
		for i, row := range rows {
			rows[i] = model.Post{ID: row.ID, Nickname: row.Nickname, Content: row.Content, GraduationYear: row.GraduationYear,
				AvatarColor: row.AvatarColor, LikeCount: row.LikeCount, ReplyCount: row.ReplyCount, HasImage: row.HasImage, CreatedAt: row.CreatedAt}
		}
		return cachedPage[model.Post]{Rows: rows, Total: total}, err
	})
	return page.Rows, page.Total, err
}

func (s *CachedStore) ListReplies(ctx context.Context, id uint64, q ContentQuery) ([]model.Reply, int64, error) {
	return s.listContentReplies(ctx, "post", id, q)
}
func (s *CachedStore) ListPhotoReplies(ctx context.Context, id uint64, q ContentQuery) ([]model.Reply, int64, error) {
	return s.listContentReplies(ctx, "photo", id, q)
}
func (s *CachedStore) listContentReplies(ctx context.Context, kind string, id uint64, q ContentQuery) ([]model.Reply, int64, error) {
	page, err := cached(ctx, s, kind+"-replies", struct {
		ID    uint64
		Query ContentQuery
	}{ID: id, Query: q}, func() (cachedPage[model.Reply], error) {
		list := s.Store.ListReplies
		if kind == "photo" {
			list = s.Store.ListPhotoReplies
		}
		rows, total, err := list(ctx, id, q)
		for i, row := range rows {
			rows[i] = model.Reply{ParentID: row.ParentID, ParentNickname: row.ParentNickname, ID: row.ID, PostID: row.PostID, PhotoID: row.PhotoID, Nickname: row.Nickname, Content: row.Content, CreatedAt: row.CreatedAt}
		}
		return cachedPage[model.Reply]{Rows: rows, Total: total}, err
	})
	return page.Rows, page.Total, err
}

func (s *CachedStore) RegisterUser(ctx context.Context, user *model.User, card *model.Alumni) error {
	if err := s.Store.RegisterUser(ctx, user, card); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}
func (s *CachedStore) CreateAlumni(ctx context.Context, row *model.Alumni) error {
	if err := s.Store.CreateAlumni(ctx, row); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}
func (s *CachedStore) UpdateAlumni(ctx context.Context, row *model.Alumni) error {
	if err := s.Store.UpdateAlumni(ctx, row); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}
func (s *CachedStore) CreatePhoto(ctx context.Context, row *model.Photo) error {
	if err := s.Store.CreatePhoto(ctx, row); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}
func (s *CachedStore) DeletePhoto(ctx context.Context, id uint64) error {
	if err := s.Store.DeletePhoto(ctx, id); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}
func (s *CachedStore) CreatePost(ctx context.Context, row *model.Post) error {
	if err := s.Store.CreatePost(ctx, row); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}
func (s *CachedStore) CreateReply(ctx context.Context, row *model.Reply) (int64, error) {
	count, err := s.Store.CreateReply(ctx, row)
	if err == nil {
		s.invalidate(ctx)
	}
	return count, err
}
func (s *CachedStore) Moderate(ctx context.Context, audit model.AuditLog) error {
	if err := s.Store.Moderate(ctx, audit); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}
