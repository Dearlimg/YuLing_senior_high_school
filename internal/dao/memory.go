package dao

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"YuLing_senior_high_school/internal/model"
)

// MemoryStore is an explicit, ephemeral demo/test store. Production never falls back to it.
type MemoryStore struct {
	organizers map[uint64]model.ClassOrganizer
	recoveries map[uint64]model.CardRecovery
	growth     []model.GrowthEvent
	likes      map[string]bool
	users      map[uint64]model.User
	sessions   map[string]memorySession
	mu         sync.Mutex
	alumni     map[uint64]model.Alumni
	photos     map[uint64]model.Photo
	posts      map[uint64]model.Post
	replies    map[uint64]model.Reply
	audits     []model.AuditLog
	keys       map[string]bool
	rates      map[string]rateWindow
}

type rateWindow struct {
	count   int
	expires time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		organizers: map[uint64]model.ClassOrganizer{}, recoveries: map[uint64]model.CardRecovery{},
		growth: []model.GrowthEvent{},
		likes:  map[string]bool{}, users: map[uint64]model.User{}, sessions: map[string]memorySession{},
		alumni: map[uint64]model.Alumni{}, photos: map[uint64]model.Photo{},
		posts: map[uint64]model.Post{}, replies: map[uint64]model.Reply{},
		audits: []model.AuditLog{}, keys: map[string]bool{}, rates: map[string]rateWindow{},
	}
}

func (s *MemoryStore) matching(q ListQuery) []model.Alumni {
	rows := []model.Alumni{}
	for _, row := range s.alumni {
		if row.Status != "active" || row.DeletedAt != nil {
			continue
		}
		if q.GraduationYear > 0 && row.GraduationYear != q.GraduationYear {
			continue
		}
		if q.City != "" && row.City != q.City {
			continue
		}
		if q.ClassName == "__unassigned__" && row.ClassName != "" {
			continue
		}
		if q.ClassName != "" && q.ClassName != "__unassigned__" && row.ClassName != q.ClassName {
			continue
		}
		if q.University != "" && (row.Identity != "alumni" || row.University != q.University) {
			continue
		}
		text := strings.Join([]string{row.Name, row.University, row.DreamUniversity, row.Major, row.City, row.Company}, "\n")
		if !strings.Contains(strings.ToLower(text), strings.ToLower(q.Keyword)) {
			continue
		}
		rows = append(rows, row)
	}
	slices.SortFunc(rows, func(a, b model.Alumni) int {
		if c := b.UpdatedAt.Compare(a.UpdatedAt); c != 0 {
			return c
		}
		return compareID(b.ID, a.ID)
	})
	return rows
}

func (s *MemoryStore) ListAlumni(_ context.Context, q ListQuery) ([]model.Alumni, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := s.matching(q)
	return pageSlice(rows, q.Page, q.PageSize), int64(len(rows)), nil
}

func (s *MemoryStore) FindAlumni(_ context.Context, id uint64) (*model.Alumni, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.alumni[id]
	if !ok || row.Status != "active" || row.DeletedAt != nil {
		return nil, ErrNotFound
	}
	return &row, nil
}

func (s *MemoryStore) useKey(kind string, key *string) error {
	if key == nil {
		return nil
	}
	kind += *key
	if s.keys[kind] {
		return ErrDuplicate
	}
	s.keys[kind] = true
	return nil
}

func (s *MemoryStore) CreateAlumni(_ context.Context, row *model.Alumni) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.useKey("alumni", row.CreateKey); err != nil {
		return err
	}
	row.ID = uint64(len(s.alumni) + 1)
	row.CreatedAt = time.Now()
	row.UpdatedAt = row.CreatedAt
	s.alumni[row.ID] = *row
	return nil
}

func (s *MemoryStore) UpdateAlumni(_ context.Context, row *model.Alumni) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.alumni[row.ID]
	if !ok || old.Status != "active" || old.EditCodeHash != row.EditCodeHash {
		return ErrNotFound
	}
	row.CreatedAt = old.CreatedAt
	row.UpdatedAt = time.Now()
	row.EditCodeHash = old.EditCodeHash
	row.CreateKey = old.CreateKey
	s.alumni[row.ID] = *row
	return nil
}

func (s *MemoryStore) DirectoryMeta(_ context.Context) (DirectoryMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := DirectoryMeta{Years: []int{}}
	years := map[int]bool{}
	for _, r := range s.matching(ListQuery{}) {
		meta.Total++
		years[r.GraduationYear] = true
	}
	for year := range years {
		meta.Years = append(meta.Years, year)
	}
	slices.SortFunc(meta.Years, func(a, b int) int { return b - a })
	return meta, nil
}

func (s *MemoryStore) Classes(_ context.Context, q ListQuery) ([]ClassGroup, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	groups := map[string]ClassGroup{}
	for _, r := range s.matching(q) {
		key := fmt.Sprintf("%d:%s", r.GraduationYear, r.ClassName)
		g := groups[key]
		g.GraduationYear = r.GraduationYear
		g.ClassName = r.ClassName
		g.Count++
		groups[key] = g
	}
	rows := []ClassGroup{}
	for _, g := range groups {
		rows = append(rows, g)
	}
	slices.SortFunc(rows, func(a, b ClassGroup) int {
		if a.GraduationYear != b.GraduationYear {
			return b.GraduationYear - a.GraduationYear
		}
		return strings.Compare(a.ClassName, b.ClassName)
	})
	return pageSlice(rows, q.Page, q.PageSize), int64(len(rows)), nil
}

func (s *MemoryStore) universities() []University {
	counts := map[string]int64{}
	for _, r := range s.matching(ListQuery{}) {
		if r.Identity == "alumni" && r.University != "" {
			counts[r.University]++
		}
	}
	rows := []University{}
	for name, count := range counts {
		rows = append(rows, University{Name: name, Count: count})
	}
	slices.SortFunc(rows, func(a, b University) int {
		if a.Count != b.Count {
			return int(b.Count - a.Count)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return rows
}

func (s *MemoryStore) Universities(_ context.Context) ([]University, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.universities(), nil
}

func (s *MemoryStore) Stats(_ context.Context) (Stats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats := Stats{Universities: int64(len(s.universities()))}
	cities := map[string]bool{}
	for _, r := range s.matching(ListQuery{}) {
		stats.Alumni++
		if r.City != "" {
			cities[r.City] = true
		}
	}
	stats.Cities = int64(len(cities))
	for _, r := range s.photos {
		if r.Status == "active" {
			stats.Photos++
		}
	}
	for _, r := range s.posts {
		if r.Status == "active" {
			stats.Posts++
		}
	}
	return stats, nil
}

func (s *MemoryStore) ListPhotos(_ context.Context, q ContentQuery) ([]model.Photo, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := []model.Photo{}
	for _, r := range s.photos {
		if r.Status == "active" && (q.Category == "" || r.Category == q.Category) {
			r.Image = nil
			r.DeleteCodeHash = ""
			r.LikeCount = s.likesFor(model.ContentLike{Kind: "photo", TargetID: r.ID}).Count
			r.ReplyCount = s.photoReplyCount(r.ID)
			rows = append(rows, r)
		}
	}
	slices.SortFunc(rows, func(a, b model.Photo) int { return compareID(b.ID, a.ID) })
	return pageSlice(rows, q.Page, q.PageSize), int64(len(rows)), nil
}

func (s *MemoryStore) FindPhoto(_ context.Context, id uint64) (*model.Photo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.photos[id]
	if !ok || r.Status != "active" {
		return nil, ErrNotFound
	}
	r.Image = slices.Clone(r.Image)
	return &r, nil
}

func (s *MemoryStore) CreatePhoto(_ context.Context, row *model.Photo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.useKey("photo", row.CreateKey); err != nil {
		return err
	}
	row.ID = uint64(len(s.photos) + 1)
	row.CreatedAt = time.Now()
	row.UpdatedAt = row.CreatedAt
	s.photos[row.ID] = *row
	return nil
}

func (s *MemoryStore) DeletePhoto(_ context.Context, id uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.photos[id]
	if !ok || r.Status != "active" {
		return ErrNotFound
	}
	r.Status = "deleted"
	s.photos[id] = r
	s.audits = append(s.audits, model.AuditLog{
		ID: uint64(len(s.audits) + 1), Kind: "photo", TargetID: id,
		Action: "deleted", Reason: "owner delete code", CreatedAt: time.Now(),
	})
	return nil
}

func (s *MemoryStore) replyCount(id uint64) int64 {
	var count int64
	for _, r := range s.replies {
		if r.PostID == id && r.Status == "active" {
			count++
		}
	}
	return count
}

func (s *MemoryStore) ListPosts(_ context.Context, q ContentQuery) ([]model.Post, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := []model.Post{}
	for _, r := range s.posts {
		if r.Status != "active" || (!q.Since.IsZero() && r.CreatedAt.Before(q.Since)) {
			continue
		}
		if !strings.Contains(strings.ToLower(r.Content+"\n"+r.Nickname), strings.ToLower(q.Keyword)) {
			continue
		}
		r.HasImage = len(r.Image) > 0
		r.Image = nil
		r.ReplyCount = s.replyCount(r.ID)
		r.LikeCount = s.likesFor(model.ContentLike{Kind: "post", TargetID: r.ID}).Count
		rows = append(rows, r)
	}
	slices.SortFunc(rows, func(a, b model.Post) int {
		if q.Sort == "hot" && a.ReplyCount != b.ReplyCount {
			return int(b.ReplyCount - a.ReplyCount)
		}
		return compareID(b.ID, a.ID)
	})
	return pageSlice(rows, q.Page, q.PageSize), int64(len(rows)), nil
}

func (s *MemoryStore) FindPost(_ context.Context, id uint64) (*model.Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.posts[id]
	if !ok || r.Status != "active" {
		return nil, ErrNotFound
	}
	r.Image = slices.Clone(r.Image)
	return &r, nil
}

func (s *MemoryStore) CreatePost(_ context.Context, row *model.Post) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.useKey("post", row.CreateKey); err != nil {
		return err
	}
	row.ID = uint64(len(s.posts) + 1)
	row.CreatedAt = time.Now()
	row.UpdatedAt = row.CreatedAt
	s.posts[row.ID] = *row
	return nil
}

func (s *MemoryStore) ListReplies(ctx context.Context, id uint64, q ContentQuery) ([]model.Reply, int64, error) {
	return s.listContentReplies(ctx, "post", id, q)
}
func (s *MemoryStore) ListPhotoReplies(ctx context.Context, id uint64, q ContentQuery) ([]model.Reply, int64, error) {
	return s.listContentReplies(ctx, "photo", id, q)
}
func (s *MemoryStore) listContentReplies(_ context.Context, kind string, id uint64, q ContentQuery) ([]model.Reply, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.contentActive(kind, id) {
		return nil, 0, ErrNotFound
	}
	rows := []model.Reply{}
	for _, r := range s.replies {
		matches := kind == "post" && r.PostID == id || kind == "photo" && r.PhotoID == id
		if !matches || r.Status != "active" {
			continue
		}
		if target, ok := s.replies[r.ParentID]; ok && target.Status == "active" {
			r.ParentNickname = target.Nickname
		}
		rows = append(rows, r)
	}
	slices.SortFunc(rows, func(a, b model.Reply) int { return compareID(a.ID, b.ID) })
	return pageSlice(rows, q.Page, q.PageSize), int64(len(rows)), nil
}
func (s *MemoryStore) CreateReply(_ context.Context, row *model.Reply) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kind, id := "post", row.PostID
	if row.PhotoID != 0 {
		kind, id = "photo", row.PhotoID
	}
	if !s.contentActive(kind, id) || (row.PostID != 0 && row.PhotoID != 0) {
		return 0, ErrNotFound
	}
	if row.ParentID != 0 {
		target, ok := s.replies[row.ParentID]
		if !ok || target.Status != "active" || target.PostID != row.PostID || target.PhotoID != row.PhotoID {
			return 0, ErrNotFound
		}
	}
	if err := s.useKey("reply", row.CreateKey); err != nil {
		return 0, err
	}
	row.ID = uint64(len(s.replies) + 1)
	row.CreatedAt = time.Now()
	row.UpdatedAt = row.CreatedAt
	s.replies[row.ID] = *row
	if kind == "photo" {
		return s.photoReplyCount(id), nil
	}
	return s.replyCount(id), nil
}
func (s *MemoryStore) photoReplyCount(id uint64) int64 {
	var count int64
	for _, r := range s.replies {
		if r.PhotoID == id && r.Status == "active" {
			count++
		}
	}
	return count
}

func (s *MemoryStore) Moderate(_ context.Context, audit model.AuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch audit.Kind {
	case "alumni":
		r, ok := s.alumni[audit.TargetID]
		if !ok {
			return ErrNotFound
		}
		r.Status = audit.Action
		s.alumni[r.ID] = r
	case "photo":
		r, ok := s.photos[audit.TargetID]
		if !ok || r.Status == "deleted" {
			return ErrNotFound
		}
		r.Status = audit.Action
		s.photos[r.ID] = r
	case "post":
		r, ok := s.posts[audit.TargetID]
		if !ok || r.Status == "deleted" {
			return ErrNotFound
		}
		r.Status = audit.Action
		s.posts[r.ID] = r
	case "reply":
		r, ok := s.replies[audit.TargetID]
		if !ok {
			return ErrNotFound
		}
		r.Status = audit.Action
		s.replies[r.ID] = r
	default:
		return ErrNotFound
	}
	audit.ID = uint64(len(s.audits) + 1)
	audit.CreatedAt = time.Now()
	s.audits = append(s.audits, audit)
	return nil
}

func (s *MemoryStore) AuditLogs(_ context.Context, q ContentQuery) ([]model.AuditLog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := slices.Clone(s.audits)
	slices.Reverse(rows)
	return pageSlice(rows, q.Page, q.PageSize), nil
}

func (s *MemoryStore) Allow(_ context.Context, key string, limit int, period time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, v := range s.rates {
		if now.After(v.expires) {
			delete(s.rates, k)
		}
	}
	window := s.rates[key]
	if now.After(window.expires) {
		window = rateWindow{expires: now.Add(period)}
	}
	window.count++
	s.rates[key] = window
	return window.count <= limit, nil
}

func pageSlice[T any](rows []T, page, size int) []T {
	start := min(max(0, (page-1)*size), len(rows))
	end := min(start+size, len(rows))
	out := make([]T, end-start)
	copy(out, rows[start:end])
	return out
}
func compareID(a, b uint64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func (s *MemoryStore) Inspect(_ context.Context, kind string, id uint64) (*ModerationItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := &ModerationItem{ID: id, Kind: kind}
	switch kind {
	case "alumni":
		row, ok := s.alumni[id]
		if !ok {
			return nil, ErrNotFound
		}
		item.Status, item.Summary, item.CreatedAt = row.Status, row.Name, row.CreatedAt
	case "photo":
		row, ok := s.photos[id]
		if !ok {
			return nil, ErrNotFound
		}
		item.Status, item.Summary, item.CreatedAt = row.Status, row.Caption, row.CreatedAt
	case "post":
		row, ok := s.posts[id]
		if !ok {
			return nil, ErrNotFound
		}
		item.Status, item.Summary, item.CreatedAt = row.Status, row.Content, row.CreatedAt
	case "reply":
		row, ok := s.replies[id]
		if !ok {
			return nil, ErrNotFound
		}
		item.Status, item.Summary, item.CreatedAt = row.Status, row.Content, row.CreatedAt
	default:
		return nil, ErrNotFound
	}
	return item, nil
}

func (s *MemoryStore) RankedPosts(ctx context.Context, period string) ([]model.Post, error) {
	rows, _, err := s.ListPosts(ctx, rankingQuery(period))
	return rows, err
}
