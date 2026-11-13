package dao

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"YuLing_senior_high_school/internal/model"
)

type GrowthQuery struct {
	Since          time.Time
	Source         string
	Campaign       string
	GraduationYear int
	ClassName      string
}

type GrowthCount struct {
	Event   string `json:"event"`
	UV      int64  `json:"uv"`
	ShareUV int64  `json:"share_uv"`
}

// ClaimAlumni commits the card and authoritative success event together.
func (s *SQLStore) ClaimAlumni(ctx context.Context, row *model.Alumni, event *model.GrowthEvent) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		event.AlumniID = row.ID
		return tx.Create(event).Error
	})
	return recordError(err)
}

func (s *SQLStore) RecordGrowth(ctx context.Context, event *model.GrowthEvent) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(event).Error
}

func (s *SQLStore) GrowthCounts(ctx context.Context, q GrowthQuery) ([]GrowthCount, error) {
	tx := s.db.WithContext(ctx).Model(&model.GrowthEvent{}).Where("created_at >= ?", q.Since)
	if q.Source != "" {
		tx = tx.Where("source = ?", q.Source)
	}
	if q.Campaign != "" {
		tx = tx.Where("campaign = ?", q.Campaign)
	}
	if q.GraduationYear != 0 {
		tx = tx.Where("graduation_year = ?", q.GraduationYear)
	}
	if q.ClassName != "" {
		tx = tx.Where("class_name = ?", q.ClassName)
	}
	// Union buckets count a browser once even if it copies a link and saves a poster.
	bucket := "CASE WHEN event IN ('share_click','poster_save') THEN 'share' ELSE event END"
	rows := []GrowthCount{}
	err := tx.Select(bucket + " AS event, COUNT(DISTINCT visitor_hash) AS uv, " +
		"COUNT(DISTINCT CASE WHEN share_id <> '' THEN visitor_hash END) AS share_uv").Group(bucket).Scan(&rows).Error
	return rows, err
}

func (s *MemoryStore) ClaimAlumni(_ context.Context, row *model.Alumni, event *model.GrowthEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.useKey("alumni", row.CreateKey); err != nil {
		return err
	}
	row.ID = uint64(len(s.alumni) + 1)
	row.CreatedAt = time.Now()
	row.UpdatedAt = row.CreatedAt
	s.alumni[row.ID] = *row
	event.AlumniID = row.ID
	s.growth = append(s.growth, *event)
	return nil
}

func (s *MemoryStore) RecordGrowth(_ context.Context, event *model.GrowthEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, old := range s.growth {
		if old.EventKey == event.EventKey {
			return nil
		}
	}
	s.growth = append(s.growth, *event)
	return nil
}

func (s *MemoryStore) GrowthCounts(_ context.Context, q GrowthQuery) ([]GrowthCount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	visitors := map[string]map[string]bool{}
	shared := map[string]map[string]bool{}
	for _, row := range s.growth {
		if row.CreatedAt.Before(q.Since) || (q.Source != "" && row.Source != q.Source) {
			continue
		}
		if q.Campaign != "" && row.Campaign != q.Campaign {
			continue
		}
		if q.GraduationYear != 0 && row.GraduationYear != q.GraduationYear {
			continue
		}
		if q.ClassName != "" && row.ClassName != q.ClassName {
			continue
		}
		event := row.Event
		if event == "share_click" || event == "poster_save" {
			event = "share"
		}
		if visitors[event] == nil {
			visitors[event] = map[string]bool{}
		}
		visitors[event][row.VisitorHash] = true
		if row.ShareID != "" {
			if shared[event] == nil {
				shared[event] = map[string]bool{}
			}
			shared[event][row.VisitorHash] = true
		}
	}
	rows := []GrowthCount{}
	for event, ids := range visitors {
		rows = append(rows, GrowthCount{Event: event, UV: int64(len(ids)), ShareUV: int64(len(shared[event]))})
	}
	return rows, nil
}

func (s *CachedStore) ClaimAlumni(ctx context.Context, row *model.Alumni, event *model.GrowthEvent) error {
	if err := s.Store.ClaimAlumni(ctx, row, event); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}
