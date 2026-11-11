package dao

import (
	"context"

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
	err := tx.Omit("Image", "DeleteCodeHash", "CreateKey").Order("id DESC").
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
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []model.Post{}
	err := tx.Select("post.id, post.nickname, post.graduation_year, post.content, post.avatar_color, post.created_at, " +
		"(LENGTH(post.image) > 0) AS has_image, " +
		"(SELECT COUNT(*) FROM reply WHERE reply.post_id = post.id AND reply.status = 'active') AS reply_count").
		Order("post.id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error
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

func (s *SQLStore) ListReplies(ctx context.Context, postID uint64, q ContentQuery) ([]model.Reply, int64, error) {
	tx := s.db.WithContext(ctx).Model(&model.Reply{}).
		Joins("JOIN post ON post.id = reply.post_id AND post.status = ?", "active").
		Where("reply.post_id = ? AND reply.status = ?", postID, "active")
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []model.Reply{}
	err := tx.Select("reply.id, reply.post_id, reply.nickname, reply.content, reply.created_at").
		Order("reply.id ASC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error
	return rows, total, err
}

func (s *SQLStore) CreateReply(ctx context.Context, row *model.Reply) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Parent lock serializes moderation and replies.
		var parent model.Post
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").
			Where("id = ? AND status = ?", row.PostID, "active").First(&parent).Error; err != nil {
			return recordError(err)
		}
		if err := tx.Create(row).Error; err != nil {
			return recordError(err)
		}
		return tx.Model(&model.Reply{}).Where("post_id = ? AND status = ?", row.PostID, "active").Count(&count).Error
	})
	return count, err
}

func (s *SQLStore) Moderate(ctx context.Context, audit model.AuditLog) error {
	entities := map[string]any{"alumni": &model.Alumni{}, "photo": &model.Photo{}, "post": &model.Post{}, "reply": &model.Reply{}}
	entity, ok := entities[audit.Kind]
	if !ok {
		return ErrNotFound
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(entity).Where("id = ?", audit.TargetID)
		if audit.Kind == "photo" {
			query = query.Where("status <> ?", "deleted")
		}
		var found int64
		if err := query.Count(&found).Error; err != nil {
			return err
		}
		if found == 0 {
			return ErrNotFound
		}
		if err := query.Update("status", audit.Action).Error; err != nil {
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
