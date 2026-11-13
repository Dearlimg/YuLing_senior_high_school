package dao

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"YuLing_senior_high_school/internal/model"
)

type OrganizerQuery struct {
	GraduationYear int
	ClassName      string
	UserID         uint64
	Private        bool
	Page           int
	PageSize       int
}

type OrganizerReview struct {
	ID     uint64
	UserID uint64
	Status string
	Audit  model.AuditLog
}

type RecoveryReview struct {
	ID     uint64
	Status string
	Audit  model.AuditLog
}

type RecoveryCompletion struct {
	ID          uint64
	AlumniID    uint64
	ReceiptHash string
	NewEditHash string
	Audit       model.AuditLog
}

func (s *SQLStore) ApplyOrganizer(ctx context.Context, row *model.ClassOrganizer) error {
	return recordError(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var card model.Alumni
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status = ? AND deleted_at IS NULL", row.AlumniID, "active").First(&card).Error; err != nil {
			return err
		}
		if card.ClassName == "" {
			return ErrNotFound
		}
		row.GraduationYear, row.ClassName = card.GraduationYear, card.ClassName
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"graduation_year", "class_name", "introduction", "status", "updated_at",
			}),
		}).Create(row).Error
	}))
}

func (s *SQLStore) ListOrganizers(ctx context.Context, q OrganizerQuery) ([]model.ClassOrganizer, int64, error) {
	tx := s.db.WithContext(ctx).Model(&model.ClassOrganizer{}).
		Joins("JOIN alumni ON alumni.id = class_organizer.alumni_id")
	if !q.Private {
		tx = tx.Where("class_organizer.status = ? AND alumni.status = ? AND alumni.deleted_at IS NULL", "active", "active").
			Where("alumni.graduation_year = class_organizer.graduation_year AND alumni.class_name = class_organizer.class_name")
	}
	if q.GraduationYear != 0 {
		tx = tx.Where("class_organizer.graduation_year = ?", q.GraduationYear)
	}
	if q.ClassName != "" {
		tx = tx.Where("class_organizer.class_name = ?", q.ClassName)
	}
	if q.UserID != 0 {
		tx = tx.Where("class_organizer.user_id = ?", q.UserID)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []model.ClassOrganizer{}
	err := tx.Select("class_organizer.*, alumni.name AS name").Order("class_organizer.updated_at DESC, class_organizer.id DESC").
		Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Scan(&rows).Error
	return rows, total, err
}

func organizerPrevious(status string) []string {
	switch status {
	case "active", "rejected":
		return []string{"pending"}
	case "revoked":
		return []string{"active"}
	case "withdrawn":
		return []string{"pending", "active"}
	}
	return []string{}
}

func (s *SQLStore) ReviewOrganizer(ctx context.Context, req OrganizerReview) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&model.ClassOrganizer{}).Where("id = ? AND status IN ?", req.ID, organizerPrevious(req.Status))
		if req.UserID != 0 {
			query = query.Where("user_id = ?", req.UserID)
		}
		updated := query.Updates(map[string]any{"status": req.Status, "updated_at": time.Now()})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected == 0 {
			return ErrNotFound
		}
		return tx.Create(&req.Audit).Error
	})
}

func (s *SQLStore) CreateRecovery(ctx context.Context, row *model.CardRecovery) error {
	return recordError(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var card model.Alumni
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status = ? AND deleted_at IS NULL", row.AlumniID, "active").First(&card).Error; err != nil {
			return err
		}
		if card.EditCodeHash == "" {
			return ErrNotFound
		}
		return tx.Create(row).Error
	}))
}

func (s *SQLStore) FindRecovery(ctx context.Context, id uint64) (*model.CardRecovery, error) {
	var row model.CardRecovery
	err := s.db.WithContext(ctx).First(&row, id).Error
	return &row, recordError(err)
}

func (s *SQLStore) ListRecoveries(ctx context.Context, q ContentQuery) ([]model.CardRecovery, int64, error) {
	tx := s.db.WithContext(ctx).Model(&model.CardRecovery{}).
		Joins("JOIN alumni ON alumni.id = card_recovery.alumni_id")
	if q.Status != "" {
		tx = tx.Where("card_recovery.status = ?", q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []model.CardRecovery{}
	err := tx.Select("card_recovery.id, card_recovery.alumni_id, card_recovery.explanation, " +
		"card_recovery.status, card_recovery.expires_at, card_recovery.created_at, alumni.name AS name").
		Order("card_recovery.id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Scan(&rows).Error
	return rows, total, err
}

func (s *SQLStore) ReviewRecovery(ctx context.Context, req RecoveryReview) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updated := tx.Model(&model.CardRecovery{}).
			Where("id = ? AND status = ? AND expires_at > ?", req.ID, "pending", time.Now()).
			Updates(map[string]any{"status": req.Status, "updated_at": time.Now()})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected == 0 {
			return ErrNotFound
		}
		return tx.Create(&req.Audit).Error
	})
}

func (s *SQLStore) CompleteRecovery(ctx context.Context, req RecoveryCompletion) error {
	return recordError(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var card model.Alumni
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status = ? AND deleted_at IS NULL", req.AlumniID, "active").First(&card).Error; err != nil {
			return err
		}
		if card.EditCodeHash == "" {
			return ErrNotFound
		}
		var receipt model.CardRecovery
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"id = ? AND alumni_id = ? AND receipt_hash = ? AND status = ? AND expires_at > ?",
			req.ID,
			req.AlumniID,
			req.ReceiptHash,
			"approved",
			time.Now(),
		).First(&receipt).Error; err != nil {
			return err
		}
		if err := tx.Model(&card).UpdateColumn("edit_code_hash", req.NewEditHash).Error; err != nil {
			return err
		}
		if err := tx.Model(&receipt).Update("status", "used").Error; err != nil {
			return err
		}
		if err := tx.Model(&model.CardRecovery{}).
			Where("alumni_id = ? AND id <> ? AND status IN ?", card.ID, receipt.ID, []string{"pending", "approved"}).
			Update("status", "revoked").Error; err != nil {
			return err
		}
		return tx.Create(&req.Audit).Error
	}))
}
