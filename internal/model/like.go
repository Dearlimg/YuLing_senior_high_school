package model

import "time"

// ContentLike has one row per user and target; repeated PUT requests are idempotent.
type ContentLike struct {
	ID        uint64 `gorm:"primaryKey"`
	Kind      string `gorm:"type:varchar(16);not null;uniqueIndex:idx_like_target_user"`
	TargetID  uint64 `gorm:"not null;uniqueIndex:idx_like_target_user"`
	UserID    uint64 `gorm:"not null;uniqueIndex:idx_like_target_user"`
	CreatedAt time.Time
}

func (ContentLike) TableName() string { return "content_like" }
