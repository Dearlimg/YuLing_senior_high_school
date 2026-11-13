package dao

import (
	"context"
	"time"

	"YuLing_senior_high_school/internal/model"
)

type ReportWindow struct {
	Since time.Time
	Until time.Time
}

type AnnualReport struct {
	Registrations int64        `json:"registrations"`
	Graduates     int64        `json:"graduates"`
	Students      int64        `json:"students"`
	Cities        int64        `json:"cities"`
	Classes       int64        `json:"classes"`
	Photos        int64        `json:"photos"`
	Posts         int64        `json:"posts"`
	Universities  []University `json:"universities" gorm:"-"`
	TopClasses    []ClassGroup `json:"top_classes" gorm:"-"`
}

func (s *SQLStore) AnnualReport(ctx context.Context, w ReportWindow) (AnnualReport, error) {
	result := AnnualReport{Universities: []University{}, TopClasses: []ClassGroup{}}
	query := s.alumniQuery(ctx, ListQuery{}).Where("created_at >= ? AND created_at < ?", w.Since, w.Until)
	err := query.Select("COUNT(*) AS registrations, COALESCE(SUM(identity = 'alumni'),0) AS graduates, " +
		"COALESCE(SUM(identity = 'student'),0) AS students, " +
		"COUNT(DISTINCT NULLIF(city,'')) AS cities, " +
		"COUNT(DISTINCT CASE WHEN class_name <> '' THEN CONCAT(graduation_year,':',class_name) END) AS classes").
		Scan(&result).Error
	if err != nil {
		return result, err
	}
	err = s.alumniQuery(ctx, ListQuery{}).Where("created_at >= ? AND created_at < ?", w.Since, w.Until).
		Where("identity = ? AND university <> ''", "alumni").Select("university AS name, COUNT(*) AS count").
		Group("university").Order("count DESC, university ASC").Scan(&result.Universities).Error
	if err != nil {
		return result, err
	}
	err = s.alumniQuery(ctx, ListQuery{}).Where("created_at >= ? AND created_at < ? AND class_name <> ''", w.Since, w.Until).
		Select("graduation_year, class_name, COUNT(*) AS count").Group("graduation_year, class_name").
		Order("count DESC, graduation_year DESC, class_name ASC").Limit(10).Scan(&result.TopClasses).Error
	if err != nil {
		return result, err
	}
	if err := s.db.WithContext(ctx).Model(&model.Photo{}).
		Where("status = ? AND created_at >= ? AND created_at < ?", "active", w.Since, w.Until).
		Count(&result.Photos).Error; err != nil {
		return result, err
	}
	err = s.db.WithContext(ctx).Model(&model.Post{}).
		Where("status = ? AND created_at >= ? AND created_at < ?", "active", w.Since, w.Until).Count(&result.Posts).Error
	return result, err
}
