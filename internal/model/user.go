package model

import "time"

// User owns exactly one alumni card. Credentials never appear in public DTOs.
type User struct {
	ID           uint64 `gorm:"primaryKey"`
	Account      string `gorm:"type:varchar(32);not null;uniqueIndex"`
	PasswordHash string `gorm:"type:varchar(100);not null"`
	AlumniID     uint64 `gorm:"not null;uniqueIndex"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (User) TableName() string { return "app_user" }
