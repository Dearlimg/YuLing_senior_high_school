package dao

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"YuLing_senior_high_school/internal/model"
)

func (s *SQLStore) ListPhotos(ctx context.Context, q ContentQuery) ([]model.Photo, int64, error) {
	tx := s.db.WithContext(ctx).Model(&model.Photo{}).Where("status = ?", "active")
	if q.Category != "" {
		tx = tx.Where("category = ?", q.Category)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []model.Photo{}
	err := tx.Select("photo.id, photo.caption, photo.uploader, photo.category, photo.created_at, " +
		"(SELECT COUNT(*) FROM content_like WHERE kind = 'photo' AND target_id = photo.id) AS like_count, " +
		"(SELECT COUNT(*) FROM reply WHERE reply.photo_id = photo.id " +
		"AND reply.status = 'active') AS reply_count").Order("id DESC").
		Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error
	return rows, total, err
}

func (s *SQLStore) FindPhoto(ctx context.Context, id uint64) (*model.Photo, error) {
	var row model.Photo
	err := s.db.WithContext(ctx).Where("id = ? AND status = ?", id, "active").First(&row).Error
	return &row, recordError(err)
}

func (s *SQLStore) CreatePhoto(ctx context.Context, row *model.Photo) error {
	return recordError(s.db.WithContext(ctx).Create(row).Error)
}

func (s *SQLStore) DeletePhoto(ctx context.Context, id uint64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Photo{}).Where("id = ? AND status = ?", id, "active").Update("status", "deleted")
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return tx.Create(&model.AuditLog{Kind: "photo", TargetID: id, Action: "deleted", Reason: "owner delete code"}).Error
	})
}

func (s *SQLStore) ListPosts(ctx context.Context, q ContentQuery) ([]model.Post, int64, error) {
	tx := s.db.WithContext(ctx).Model(&model.Post{}).Where("post.status = ?", "active")
	if q.Keyword != "" {
		keyword := "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(q.Keyword) + "%"
		tx = tx.Where("(post.content LIKE ? ESCAPE '!' OR post.nickname LIKE ? ESCAPE '!')", keyword, keyword)
	}
	if !q.Since.IsZero() {
		tx = tx.Where("post.created_at >= ?", q.Since)
	}
	order := "post.id DESC"
	if q.Sort == "hot" {
		order = "reply_count DESC, post.id DESC"
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []model.Post{}
	err := tx.Select("post.id, post.nickname, post.graduation_year, post.content, post.avatar_color, post.created_at, " +
		"(LENGTH(post.image) > 0) AS has_image, " +
		"(SELECT COUNT(*) FROM content_like WHERE kind = 'post' AND target_id = post.id) AS like_count, " +
		"(SELECT COUNT(*) FROM reply WHERE reply.post_id = post.id AND reply.status = 'active') AS reply_count").
		Order(order).Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error
	return rows, total, err
}

func (s *SQLStore) FindPost(ctx context.Context, id uint64) (*model.Post, error) {
	var row model.Post
	err := s.db.WithContext(ctx).Where("id = ? AND status = ?", id, "active").First(&row).Error
	return &row, recordError(err)
}

func (s *SQLStore) CreatePost(ctx context.Context, row *model.Post) error {
	return recordError(s.db.WithContext(ctx).Create(row).Error)
}

func (s *SQLStore) ListReplies(ctx context.Context, id uint64, q ContentQuery) ([]model.Reply, int64, error) {
	return s.listContentReplies(ctx, "post", id, q)
}

func (s *SQLStore) ListPhotoReplies(ctx context.Context, id uint64, q ContentQuery) ([]model.Reply, int64, error) {
	return s.listContentReplies(ctx, "photo", id, q)
}

func (s *SQLStore) listContentReplies(
	ctx context.Context,
	kind string,
	id uint64,
	q ContentQuery,
) ([]model.Reply, int64, error) {
	if contentEntity(kind) == nil {
		return nil, 0, ErrNotFound
	}
	tx := s.db.WithContext(ctx).Model(&model.Reply{}).
		Joins("JOIN "+kind+" ON "+kind+".id = reply."+kind+"_id AND "+kind+".status = ?", "active").
		Joins("LEFT JOIN reply AS target ON target.id = reply.parent_id AND target.status = ?", "active").
		Where("reply."+kind+"_id = ? AND reply.status = ?", id, "active")
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []model.Reply{}
	err := tx.Select("reply.id, reply.post_id, reply.photo_id, reply.parent_id, " +
		"target.nickname AS parent_nickname, reply.nickname, reply.content, reply.created_at").
		Order("reply.id ASC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error
	return rows, total, err
}

func (s *SQLStore) CreateReply(ctx context.Context, row *model.Reply) (int64, error) {
	kind, id := "post", row.PostID
	if row.PhotoID != 0 {
		kind, id = "photo", row.PhotoID
	}
	if id == 0 || (row.PhotoID != 0 && row.PostID != 0) {
		return 0, ErrNotFound
	}
	var count int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		parent := contentEntity(kind)
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").
			Where("id = ? AND status = ?", id, "active").First(parent).Error; err != nil {
			return recordError(err)
		}
		if row.ParentID != 0 {
			var target model.Reply
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").
				Where("id = ? AND "+kind+"_id = ? AND status = ?", row.ParentID, id, "active").First(&target).Error; err != nil {
				return recordError(err)
			}
		}
		if err := tx.Create(row).Error; err != nil {
			return recordError(err)
		}
		return tx.Model(&model.Reply{}).Where(kind+"_id = ? AND status = ?", id, "active").Count(&count).Error
	})
	return count, err
}

func (s *SQLStore) Moderate(ctx context.Context, audit model.AuditLog) error {
	entities := map[string]any{
		"alumni": &model.Alumni{}, "photo": &model.Photo{},
		"post": &model.Post{}, "reply": &model.Reply{},
	}
	entity, ok := entities[audit.Kind]
	if !ok {
		return ErrNotFound
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(entity).Where("id = ?", audit.TargetID)
		if audit.Kind == "photo" || audit.Kind == "post" {
			query = query.Where("status <> ?", "deleted")
		}
		// Lock the same parent row as likes and replies before changing visibility.
		var found struct{ ID uint64 }
		if err := query.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Take(&found).Error; err != nil {
			return recordError(err)
		}
		if err := query.Select("status", "updated_at").Update("status", audit.Action).Error; err != nil {
			return err
		}
		return tx.Create(&audit).Error
	})
}

func (s *SQLStore) AuditLogs(ctx context.Context, q ContentQuery) ([]model.AuditLog, error) {
	rows := []model.AuditLog{}
	err := s.db.WithContext(ctx).Order("id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error
	return rows, err
}

func (s *SQLStore) Inspect(ctx context.Context, kind string, id uint64) (*ModerationItem, error) {
	columns := map[string]string{"alumni": "name", "photo": "caption", "post": "content", "reply": "content"}
	column, ok := columns[kind]
	if !ok {
		return nil, ErrNotFound
	}
	row := ModerationItem{Kind: kind}
	// Both identifiers come from the fixed allowlist above, never user-supplied SQL.
	err := s.db.WithContext(ctx).Table(kind).Select("id, status, created_at, "+column+" AS summary").
		Where("id = ?", id).Take(&row).Error
	return &row, recordError(err)
}

func (s *SQLStore) RankedPosts(ctx context.Context, period string) ([]model.Post, error) {
	rows, _, err := s.ListPosts(ctx, rankingQuery(period))
	return rows, err
}
func rankingQuery(period string) ContentQuery {
	q := ContentQuery{Page: 1, PageSize: 100, Sort: "hot"}
	if period == "week" {
		q.Since = time.Now().Add(-7 * 24 * time.Hour)
	}
	return q
}
