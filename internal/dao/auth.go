package dao

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"YuLing_senior_high_school/internal/model"
)

type SessionStore interface {
	SaveSession(context.Context, string, uint64, time.Duration) error
	SessionUser(context.Context, string) (uint64, error)
	DeleteSession(context.Context, string) error
}

func (s *SQLStore) FindUser(ctx context.Context, account string) (*model.User, error) {
	var row model.User
	err := s.db.WithContext(ctx).Where("account = ?", account).First(&row).Error
	return &row, recordError(err)
}

func (s *SQLStore) FindUserByID(ctx context.Context, id uint64) (*model.User, error) {
	var row model.User
	err := s.db.WithContext(ctx).First(&row, id).Error
	return &row, recordError(err)
}

func (s *SQLStore) FindUserByAlumni(ctx context.Context, id uint64) (*model.User, error) {
	var row model.User
	err := s.db.WithContext(ctx).Where("alumni_id = ?", id).First(&row).Error
	return &row, recordError(err)
}

// RegisterUser atomically creates a card and account, or claims an unowned legacy card.
func (s *SQLStore) RegisterUser(ctx context.Context, user *model.User, card *model.Alumni) error {
	return recordError(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if card.ID == 0 {
			if err := tx.Create(card).Error; err != nil {
				return err
			}
		} else {
			var existing model.Alumni
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND status = ? AND deleted_at IS NULL", card.ID, "active").First(&existing).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.Alumni{}).Where("id = ?", card.ID).Update("edit_code_hash", "").Error; err != nil {
			return err
		}
		user.AlumniID = card.ID
		return tx.Create(user).Error
	}))
}

type RedisSessions struct {
	client *redis.Client
	prefix string
}

func NewRedisSessions(client *redis.Client, prefix string) *RedisSessions {
	return &RedisSessions{client: client, prefix: prefix + "session:"}
}

func (s *RedisSessions) SaveSession(ctx context.Context, hash string, id uint64, ttl time.Duration) error {
	return s.client.Set(ctx, s.prefix+hash, id, ttl).Err()
}

func (s *RedisSessions) SessionUser(ctx context.Context, hash string) (uint64, error) {
	id, err := s.client.Get(ctx, s.prefix+hash).Uint64()
	if err == redis.Nil {
		return 0, ErrNotFound
	}
	return id, err
}

func (s *RedisSessions) DeleteSession(ctx context.Context, hash string) error {
	return s.client.Del(ctx, s.prefix+hash).Err()
}
