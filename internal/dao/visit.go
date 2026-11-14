package dao

import (
	"context"
	"time"

	"gorm.io/gorm/clause"

	"YuLing_senior_high_school/internal/model"
)

// VisitDayCount 某自然日（东八区）的页面浏览与独立访客。
type VisitDayCount struct {
	Day string `json:"day" gorm:"column:day"`
	PV  int64  `json:"pv" gorm:"column:pv"`
	UV  int64  `json:"uv" gorm:"column:uv"`
}

// VisitPathCount 页面维度的浏览量聚合。
type VisitPathCount struct {
	Path string `json:"path" gorm:"column:path"`
	PV   int64  `json:"pv" gorm:"column:pv"`
	UV   int64  `json:"uv" gorm:"column:uv"`
}

// VisitWindow 单时间窗口的 PV / UV / 独立 IP 三口径。
type VisitWindow struct {
	PV  int64 `json:"pv" gorm:"column:pv"`
	UV  int64 `json:"uv" gorm:"column:uv"`
	IPs int64 `json:"ips" gorm:"column:ips"`
}

// VisitOverview 看板所需的全部聚合数据。
// Days 覆盖近 14 天，Pages 为近 7 天 Top 页面，Recent 为最近 30 条脱敏明细。
type VisitOverview struct {
	Today     VisitWindow        `json:"today"`
	Yesterday VisitWindow        `json:"yesterday"`
	Total     VisitWindow        `json:"total"`
	Days      []VisitDayCount    `json:"days"`
	Pages     []VisitPathCount   `json:"pages"`
	Recent    []model.VisitEvent `json:"recent"`
}

func (s *SQLStore) RecordVisit(ctx context.Context, event *model.VisitEvent) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(event).Error
}

// cstDayStart 返回 now 所在东八区自然日的起点（UTC 表示）。
// 与 growth 按 UTC 天分桶不同，访客报表面向国内运营，按北京时间切日。
func cstDayStart(now time.Time) time.Time {
	cst := time.FixedZone("CST", 8*3600)
	y, m, d := now.In(cst).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, cst).UTC()
}

func (s *SQLStore) visitWindow(ctx context.Context, since, until *time.Time) (VisitWindow, error) {
	var w VisitWindow
	q := s.db.WithContext(ctx).Model(&model.VisitEvent{})
	if since != nil {
		q = q.Where("created_at >= ?", *since)
	}
	if until != nil {
		q = q.Where("created_at < ?", *until)
	}
	err := q.Select("COUNT(*) AS pv, COUNT(DISTINCT visitor_hash) AS uv, COUNT(DISTINCT ip_hash) AS ips").Scan(&w).Error
	return w, err
}

func (s *SQLStore) VisitOverview(ctx context.Context, now time.Time) (*VisitOverview, error) {
	start := cstDayStart(now)
	yesterday := start.Add(-24 * time.Hour)
	out := &VisitOverview{Days: []VisitDayCount{}, Pages: []VisitPathCount{}, Recent: []model.VisitEvent{}}
	var err error
	if out.Today, err = s.visitWindow(ctx, &start, nil); err != nil {
		return nil, err
	}
	if out.Yesterday, err = s.visitWindow(ctx, &yesterday, &start); err != nil {
		return nil, err
	}
	if out.Total, err = s.visitWindow(ctx, nil, nil); err != nil {
		return nil, err
	}
	trendSince := start.Add(-13 * 24 * time.Hour)
	// DSN loc=Local 下 created_at 以东八区字符串入库，DATE() 即北京时间自然日；
	// CAST 保证 parseTime 不把 DATE 列转回时间戳。
	if err := s.db.WithContext(ctx).Model(&model.VisitEvent{}).
		Select("CAST(DATE(created_at) AS CHAR) AS day, COUNT(*) AS pv, COUNT(DISTINCT visitor_hash) AS uv").
		Where("created_at >= ?", trendSince).Group("day").Order("day").Scan(&out.Days).Error; err != nil {
		return nil, err
	}
	pagesSince := start.Add(-6 * 24 * time.Hour)
	if err := s.db.WithContext(ctx).Model(&model.VisitEvent{}).
		Select("path, COUNT(*) AS pv, COUNT(DISTINCT visitor_hash) AS uv").
		Where("created_at >= ?", pagesSince).Group("path").Order("pv DESC, path").Limit(10).Scan(&out.Pages).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(&model.VisitEvent{}).Order("id DESC").Limit(30).Find(&out.Recent).Error; err != nil {
		return nil, err
	}
	return out, nil
}
