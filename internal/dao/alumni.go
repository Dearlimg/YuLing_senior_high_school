// Package dao owns all GORM and Redis data access.
package dao

import (
	"context"
	"errors"
	"strings"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"YuLing_senior_high_school/internal/model"
)

type SQLStore struct{ db *gorm.DB }

func NewSQLStore(db *gorm.DB) *SQLStore { return &SQLStore{db: db} }

func (s *SQLStore) alumniQuery(ctx context.Context, q ListQuery) *gorm.DB {
	tx := s.db.WithContext(ctx).Model(&model.Alumni{}).Where("deleted_at IS NULL AND status = ?", "active")
	if q.GraduationYear > 0 {
		tx = tx.Where("graduation_year = ?", q.GraduationYear)
	}
	if q.City != "" {
		tx = tx.Where("city = ?", q.City)
	}
	if q.ClassName == "__unassigned__" {
		tx = tx.Where("class_name = ?", "")
	} else if q.ClassName != "" {
		tx = tx.Where("class_name = ?", q.ClassName)
	}
	if q.University != "" {
		tx = tx.Where("identity = ? AND university = ?", "alumni", q.University)
	}
	if q.Keyword != "" {
		keyword := "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(q.Keyword) + "%"
		tx = tx.Where(
			"(name LIKE ? ESCAPE '!' OR university LIKE ? ESCAPE '!' OR dream_university LIKE ? ESCAPE '!' "+
				"OR major LIKE ? ESCAPE '!' OR city LIKE ? ESCAPE '!' OR company LIKE ? ESCAPE '!')",
			keyword, keyword, keyword, keyword, keyword, keyword,
		)
	}
	return tx
}

func (s *SQLStore) ListAlumni(ctx context.Context, q ListQuery) ([]model.Alumni, int64, error) {
	tx := s.alumniQuery(ctx, q)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []model.Alumni{}
	err := tx.Order("updated_at DESC, id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error
	return rows, total, err
}

func (s *SQLStore) FindAlumni(ctx context.Context, id uint64) (*model.Alumni, error) {
	var row model.Alumni
	err := s.alumniQuery(ctx, ListQuery{}).Where("id = ?", id).First(&row).Error
	return &row, recordError(err)
}

func (s *SQLStore) CreateAlumni(ctx context.Context, row *model.Alumni) error {
	return recordError(s.db.WithContext(ctx).Create(row).Error)
}

func (s *SQLStore) UpdateAlumni(ctx context.Context, row *model.Alumni) error {
	// Explicit fields save false/empty values while protecting management columns.
	tx := s.db.WithContext(ctx).Model(row).Where("status = ? AND deleted_at IS NULL", "active").
		Select("Name", "Identity", "GraduationYear", "ClassName", "City", "University", "DreamUniversity",
			"Major", "Education", "Company", "Position", "Phone", "Wechat", "Email", "Message",
			"AvatarColor", "ContactPublic", "UpdatedAt").Updates(row)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) DirectoryMeta(ctx context.Context) (DirectoryMeta, error) {
	meta := DirectoryMeta{Years: []int{}}
	if err := s.alumniQuery(ctx, ListQuery{}).Count(&meta.Total).Error; err != nil {
		return meta, err
	}
	err := s.alumniQuery(ctx, ListQuery{}).Distinct("graduation_year").Order("graduation_year DESC").
		Pluck("graduation_year", &meta.Years).Error
	return meta, err
}

func (s *SQLStore) Classes(ctx context.Context, q ListQuery) ([]ClassGroup, int64, error) {
	groups := s.alumniQuery(ctx, q).Select("graduation_year, class_name, COUNT(*) AS count").Group("graduation_year, class_name")
	var total int64
	if err := s.db.WithContext(ctx).Table("(?) AS groups_count", groups).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []ClassGroup{}
	err := groups.Order("graduation_year DESC, class_name ASC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Scan(&rows).Error
	return rows, total, err
}

func (s *SQLStore) Universities(ctx context.Context) ([]University, error) {
	rows := []University{}
	err := s.alumniQuery(ctx, ListQuery{}).Where("identity = ? AND university <> ''", "alumni").
		Select("university AS name, COUNT(*) AS count").Group("university").Order("count DESC, university ASC").Scan(&rows).Error
	return rows, err
}

func (s *SQLStore) Stats(ctx context.Context) (Stats, error) {
	stats := Stats{}
	if err := s.alumniQuery(ctx, ListQuery{}).Count(&stats.Alumni).Error; err != nil {
		return stats, err
	}
	if err := s.alumniQuery(ctx, ListQuery{}).Where("city <> ''").Distinct("city").Count(&stats.Cities).Error; err != nil {
		return stats, err
	}
	if err := s.alumniQuery(ctx, ListQuery{}).Where("identity = ? AND university <> ''", "alumni").
		Distinct("university").Count(&stats.Universities).Error; err != nil {
		return stats, err
	}
	if err := s.db.WithContext(ctx).Model(&model.Photo{}).Where("status = ?", "active").Count(&stats.Photos).Error; err != nil {
		return stats, err
	}
	err := s.db.WithContext(ctx).Model(&model.Post{}).Where("status = ?", "active").Count(&stats.Posts).Error
	return stats, err
}

func recordError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	var duplicate *mysqlDriver.MySQLError
	if errors.As(err, &duplicate) && duplicate.Number == 1062 {
		return ErrDuplicate
	}
	return err
}
