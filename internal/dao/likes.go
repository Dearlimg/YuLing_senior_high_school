package dao

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"YuLing_senior_high_school/internal/model"
)

type LikeState struct {
	Count int64 `json:"like_count"`
	Liked bool  `json:"liked"`
}

func (s *SQLStore) Likes(ctx context.Context, target model.ContentLike) (LikeState, error) {
	state := LikeState{}
	query := s.db.WithContext(ctx).Model(&model.ContentLike{}).
		Where("kind = ? AND target_id = ?", target.Kind, target.TargetID)
	if err := query.Count(&state.Count).Error; err != nil {
		return state, err
	}
	var mine int64
	err := query.Where("user_id = ?", target.UserID).Count(&mine).Error
	state.Liked = mine > 0
	return state, err
}

func (s *SQLStore) SetLike(ctx context.Context, target model.ContentLike, liked bool) (LikeState, error) {
	state := LikeState{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var parent model.Post
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").
			Where("id = ? AND status = ?", target.TargetID, "active").First(&parent).Error; err != nil {
			return recordError(err)
		}
		if liked {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&target).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Where("kind = ? AND target_id = ? AND user_id = ?", target.Kind, target.TargetID, target.UserID).
				Delete(&model.ContentLike{}).Error; err != nil {
				return err
			}
		}
		var err error
		state, err = (&SQLStore{db: tx}).Likes(ctx, target)
		return err
	})
	return state, err
}

func likeKey(target model.ContentLike) string {
	return fmt.Sprintf("%s:%d:%d", target.Kind, target.TargetID, target.UserID)
}

func (s *MemoryStore) likesFor(target model.ContentLike) LikeState {
	state := LikeState{Liked: s.likes[likeKey(target)]}
	for key := range s.likes {
		prefix := fmt.Sprintf("%s:%d:", target.Kind, target.TargetID)
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			state.Count++
		}
	}
	return state
}

func (s *MemoryStore) Likes(_ context.Context, target model.ContentLike) (LikeState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.likesFor(target), nil
}

func (s *MemoryStore) SetLike(_ context.Context, target model.ContentLike, liked bool) (LikeState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	parent, ok := s.posts[target.TargetID]
	if !ok || parent.Status != "active" {
		return LikeState{}, ErrNotFound
	}
	if liked {
		s.likes[likeKey(target)] = true
	} else {
		delete(s.likes, likeKey(target))
	}
	return s.likesFor(target), nil
}

func (s *CachedStore) SetLike(ctx context.Context, target model.ContentLike, liked bool) (LikeState, error) {
	state, err := s.Store.SetLike(ctx, target, liked)
	if err == nil {
		s.invalidate(ctx)
	}
	return state, err
}
