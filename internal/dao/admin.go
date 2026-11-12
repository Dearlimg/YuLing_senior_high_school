package dao

import (
	"context"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"YuLing_senior_high_school/internal/model"
)

// AdminUser is an explicit projection, never a credential-bearing User model.
type AdminUser struct {
	ID        uint64    `json:"id"`
	Account   string    `json:"account"`
	AlumniID  uint64    `json:"alumni_id"`
	Name      string    `json:"name"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *SQLStore) ListUsers(ctx context.Context, q ContentQuery) ([]AdminUser, int64, error) {
	tx := s.db.WithContext(ctx).Model(&model.User{}).Joins("LEFT JOIN alumni ON alumni.id = app_user.alumni_id")
	if q.Keyword != "" {
		keyword := "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(q.Keyword) + "%"
		tx = tx.Where("(app_user.account LIKE ? ESCAPE '!' OR alumni.name LIKE ? ESCAPE '!')", keyword, keyword)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []AdminUser{}
	err := tx.Select("app_user.id, app_user.account, app_user.alumni_id, " +
		"app_user.is_admin, app_user.created_at, alumni.name").
		Order("app_user.id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Scan(&rows).Error
	return rows, total, err
}

func (s *SQLStore) SetUserAdmin(ctx context.Context, id uint64, admin bool, audit model.AuditLog) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&user, id).Error; err != nil {
			return recordError(err)
		}
		if err := tx.Model(&user).Update("is_admin", admin).Error; err != nil {
			return err
		}
		return tx.Create(&audit).Error
	})
}

func moderationColumn(kind string) string {
	return map[string]string{"alumni": "name", "photo": "caption", "post": "content", "reply": "content"}[kind]
}

func (s *SQLStore) ListModeration(ctx context.Context, kind string, q ContentQuery) ([]ModerationItem, int64, error) {
	column := moderationColumn(kind)
	if column == "" {
		return nil, 0, ErrNotFound
	}
	tx := s.db.WithContext(ctx).Table(kind)
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.Keyword != "" {
		keyword := "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(q.Keyword) + "%"
		tx = tx.Where(column+" LIKE ? ESCAPE '!'", keyword)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []ModerationItem{}
	err := tx.Select("id, status, created_at, " + column + " AS summary").Order("id DESC").
		Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Scan(&rows).Error
	for i := range rows {
		rows[i].Kind = kind
	}
	return rows, total, err
}

func (s *MemoryStore) ListUsers(_ context.Context, q ContentQuery) ([]AdminUser, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := []AdminUser{}
	for _, u := range s.users {
		name := s.alumni[u.AlumniID].Name
		if !strings.Contains(strings.ToLower(u.Account+"\n"+name), strings.ToLower(q.Keyword)) {
			continue
		}
		rows = append(rows, AdminUser{
			ID: u.ID, Account: u.Account, AlumniID: u.AlumniID,
			Name: name, IsAdmin: u.IsAdmin, CreatedAt: u.CreatedAt,
		})
	}
	slices.SortFunc(rows, func(a, b AdminUser) int { return compareID(b.ID, a.ID) })
	return pageSlice(rows, q.Page, q.PageSize), int64(len(rows)), nil
}

func (s *MemoryStore) SetUserAdmin(_ context.Context, id uint64, admin bool, audit model.AuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[id]
	if !ok {
		return ErrNotFound
	}
	user.IsAdmin = admin
	user.UpdatedAt = time.Now()
	s.users[id] = user
	audit.ID = uint64(len(s.audits) + 1)
	audit.CreatedAt = time.Now()
	s.audits = append(s.audits, audit)
	return nil
}

func (s *MemoryStore) ListModeration(_ context.Context, kind string, q ContentQuery) ([]ModerationItem, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := []ModerationItem{}
	add := func(id uint64, status, summary string, created time.Time) {
		if q.Status != "" && q.Status != status {
			return
		}
		if !strings.Contains(strings.ToLower(summary), strings.ToLower(q.Keyword)) {
			return
		}
		rows = append(rows, ModerationItem{ID: id, Kind: kind, Status: status, Summary: summary, CreatedAt: created})
	}
	switch kind {
	case "photo":
		for _, r := range s.photos {
			add(r.ID, r.Status, r.Caption, r.CreatedAt)
		}
	case "post":
		for _, r := range s.posts {
			add(r.ID, r.Status, r.Content, r.CreatedAt)
		}
	case "reply":
		for _, r := range s.replies {
			add(r.ID, r.Status, r.Content, r.CreatedAt)
		}
	case "alumni":
		for _, r := range s.alumni {
			add(r.ID, r.Status, r.Name, r.CreatedAt)
		}
	default:
		return nil, 0, ErrNotFound
	}
	slices.SortFunc(rows, func(a, b ModerationItem) int { return compareID(b.ID, a.ID) })
	return pageSlice(rows, q.Page, q.PageSize), int64(len(rows)), nil
}
