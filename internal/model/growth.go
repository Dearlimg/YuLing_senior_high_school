package model

import "time"

// GrowthEvent stores only allowlisted attribution and a hashed anonymous browser ID.
type GrowthEvent struct {
	ID             uint64 `gorm:"primaryKey"`
	EventKey       string `gorm:"type:char(64);not null;uniqueIndex"`
	Event          string `gorm:"type:varchar(32);not null"`
	VisitorHash    string `gorm:"type:char(64);not null"`
	Source         string `gorm:"type:varchar(48);not null;default:''"`
	Campaign       string `gorm:"type:varchar(48);not null;default:''"`
	ShareID        string `gorm:"type:varchar(48);not null;default:''"`
	GraduationYear int
	ClassName      string `gorm:"type:varchar(32);not null;default:''"`
	AlumniID       uint64
	CreatedAt      time.Time `gorm:"index"`
}

func (GrowthEvent) TableName() string { return "growth_event" }
