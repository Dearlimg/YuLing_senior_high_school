package model

import "time"

// ClassOrganizer is a volunteer application; approval does not confer alumni verification.
type ClassOrganizer struct {
	ID             uint64 `gorm:"primaryKey"`
	UserID         uint64 `gorm:"not null;uniqueIndex"`
	AlumniID       uint64 `gorm:"not null;index"`
	GraduationYear int    `gorm:"index:idx_organizer_class"`
	ClassName      string `gorm:"type:varchar(32);not null;index:idx_organizer_class"`
	Introduction   string `gorm:"type:varchar(120);not null"`
	Status         string `gorm:"type:varchar(16);not null;index"`
	Name           string `gorm:"->;-:migration"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (ClassOrganizer) TableName() string { return "class_organizer" }

// CardRecovery is a private, manually reviewed request with an expiring bearer receipt.
type CardRecovery struct {
	ID          uint64 `gorm:"primaryKey"`
	AlumniID    uint64 `gorm:"not null;index"`
	ReceiptHash string `gorm:"type:char(64);not null;uniqueIndex"`
	Explanation string `gorm:"type:varchar(200);not null"`
	Status      string `gorm:"type:varchar(16);not null;index"`
	Name        string `gorm:"->;-:migration"`
	ExpiresAt   time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (CardRecovery) TableName() string { return "card_recovery" }
