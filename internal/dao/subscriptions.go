package dao

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"YuLing_senior_high_school/internal/model"
)

type FeedQuery struct {
	UserID   uint64
	AsOf     time.Time
	Page     int
	PageSize int
}

type Activity struct {
	Kind           string    `json:"kind"`
	TargetID       uint64    `json:"target_id"`
	Name           string    `json:"name"`
	Summary        string    `json:"summary"`
	GraduationYear int       `json:"graduation_year"`
	ClassName      string    `json:"class_name"`
	OccurredAt     time.Time `json:"occurred_at"`
	Unread         bool      `json:"unread"`
}

type FeedResult struct {
	Items  []Activity `json:"items"`
	Total  int64      `json:"total"`
	Unread int64      `json:"unread"`
	AsOf   time.Time  `json:"as_of"`
}

func (s *SQLStore) SaveSubscription(ctx context.Context, row *model.ClassSubscription, active bool) error {
	return recordError(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize a user's changes so retries and the subscription cap are deterministic.
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&user, row.UserID).Error; err != nil {
			return err
		}
		query := tx.Where(
			"user_id = ? AND graduation_year = ? AND class_name = ?",
			row.UserID,
			row.GraduationYear,
			row.ClassName,
		)
		var old model.ClassSubscription
		err := query.First(&old).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if !active {
			return tx.Model(&model.ClassSubscription{}).Where(
				"user_id = ? AND graduation_year = ? AND class_name = ?",
				row.UserID,
				row.GraduationYear,
				row.ClassName,
			).Update("active", false).Error
		}
		if err == nil && old.Active {
			return nil
		}
		var count int64
		err = tx.Model(&model.ClassSubscription{}).
			Where("user_id = ? AND active = ?", row.UserID, true).Count(&count).Error
		if err != nil {
			return err
		}
		if count >= 20 {
			return ErrSubscriptionLimit
		}
		row.Active, row.SinceAt, row.ReadAt = true, time.Now().UTC(), time.Now().UTC()
		if old.ID == 0 {
			return tx.Create(row).Error
		}
		return tx.Model(&old).Updates(map[string]any{
			"active": true, "since_at": row.SinceAt, "read_at": row.ReadAt, "updated_at": time.Now(),
		}).Error
	}))
}

func (s *SQLStore) Subscriptions(ctx context.Context, userID uint64) ([]model.ClassSubscription, error) {
	rows := []model.ClassSubscription{}
	err := s.db.WithContext(ctx).Where("user_id = ? AND active = ?", userID, true).
		Order("id DESC").Find(&rows).Error
	return rows, err
}

func (s *SQLStore) ReadActivities(ctx context.Context, userID uint64, through time.Time) error {
	return s.db.WithContext(ctx).Model(&model.ClassSubscription{}).
		Where(
			"user_id = ? AND active = ? AND read_at < ?",
			userID,
			true,
			through,
		).
		Update("read_at", through).Error
}

const activitySQL = `
SELECT 'alumni' AS kind, a.id AS target_id, a.name, a.message AS summary,
 a.graduation_year, a.class_name, a.updated_at AS occurred_at, (a.updated_at > s.read_at) AS unread
FROM alumni a JOIN class_subscription s ON s.graduation_year = a.graduation_year AND s.class_name = a.class_name
WHERE s.user_id = ? AND s.active = 1 AND a.status = 'active' AND a.deleted_at IS NULL
 AND a.updated_at >= s.since_at AND a.updated_at <= ?
UNION ALL
SELECT 'post' AS kind, p.id AS target_id, a.name, LEFT(p.content,80) AS summary,
 a.graduation_year, a.class_name, p.created_at AS occurred_at, (p.created_at > s.read_at) AS unread
FROM post p JOIN app_user u ON u.id = p.user_id JOIN alumni a ON a.id = u.alumni_id
 JOIN class_subscription s ON s.graduation_year = a.graduation_year AND s.class_name = a.class_name
WHERE s.user_id = ? AND s.active = 1 AND p.status = 'active' AND a.status = 'active' AND a.deleted_at IS NULL
 AND p.created_at >= s.since_at AND p.created_at <= ?
UNION ALL
SELECT 'photo' AS kind, p.id AS target_id, a.name, p.caption AS summary,
 a.graduation_year, a.class_name, p.created_at AS occurred_at, (p.created_at > s.read_at) AS unread
FROM photo p JOIN app_user u ON u.id = p.user_id JOIN alumni a ON a.id = u.alumni_id
 JOIN class_subscription s ON s.graduation_year = a.graduation_year AND s.class_name = a.class_name
WHERE s.user_id = ? AND s.active = 1 AND p.status = 'active' AND a.status = 'active' AND a.deleted_at IS NULL
 AND p.created_at >= s.since_at AND p.created_at <= ?`

func (s *SQLStore) ActivityFeed(ctx context.Context, q FeedQuery) (FeedResult, error) {
	result := FeedResult{Items: []Activity{}, AsOf: q.AsOf}
	subquery := s.db.WithContext(ctx).Raw(
		activitySQL,
		q.UserID,
		q.AsOf,
		q.UserID,
		q.AsOf,
		q.UserID,
		q.AsOf,
	)
	query := s.db.WithContext(ctx).Table("(?) AS activity", subquery)
	if err := query.Count(&result.Total).Error; err != nil {
		return result, err
	}
	if err := query.Select("COALESCE(SUM(unread),0)").Scan(&result.Unread).Error; err != nil {
		return result, err
	}
	err := query.Select("*").Order("occurred_at DESC, kind ASC, target_id DESC").
		Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Scan(&result.Items).Error
	return result, err
}
