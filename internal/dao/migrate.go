package dao

import (
	"context"

	"YuLing_senior_high_school/internal/model"
)

// Migrate is an explicit operator action. Back up MySQL before schema synchronization.
func (s *SQLStore) Migrate(ctx context.Context) error {
	return s.db.WithContext(ctx).Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").
		AutoMigrate(
			&model.Alumni{},
			&model.User{},
			&model.Photo{},
			&model.Post{},
			&model.Reply{},
			&model.AuditLog{},
			&model.ContentLike{},
		)
}

// CheckSchema prevents serving the new application against the old skeleton schema.
func (s *SQLStore) CheckSchema(ctx context.Context) error {
	checks := []struct {
		entity  any
		columns string
	}{
		{entity: &model.User{}, columns: "id, account, password_hash, alumni_id, is_admin"},
		{entity: &model.Alumni{}, columns: "id, identity, university, dream_university, message, edit_code_hash, status"},
		{entity: &model.Photo{}, columns: "id, user_id, status, delete_code_hash"},
		{entity: &model.Post{}, columns: "id, user_id, status"},
		{entity: &model.Reply{}, columns: "id, user_id, post_id, photo_id, parent_id, status"},
		{entity: &model.ContentLike{}, columns: "id, kind, target_id, user_id"},
		{entity: &model.AuditLog{}, columns: "id, kind, target_id, actor_user_id"},
	}
	for _, check := range checks {
		rows := []map[string]any{}
		if err := s.db.WithContext(ctx).Model(check.entity).Select(check.columns).Limit(1).Find(&rows).Error; err != nil {
			return err
		}
	}
	return nil
}
