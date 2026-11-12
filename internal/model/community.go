package model

import "time"

// Photo persists compressed image bytes alongside their access state.
type Photo struct {
	LikeCount      int64   `gorm:"->;-:migration"`
	ReplyCount     int64   `gorm:"->;-:migration"`
	UserID         uint64  `gorm:"not null;default:0;index"`
	ID             uint64  `gorm:"primaryKey"`
	Caption        string  `gorm:"type:varchar(80);not null"`
	Uploader       string  `gorm:"type:varchar(30);not null"`
	Category       string  `gorm:"type:varchar(20);not null;index"`
	Image          []byte  `gorm:"type:mediumblob"`
	MIME           string  `gorm:"type:varchar(32);not null"`
	DeleteCodeHash string  `gorm:"type:char(64);not null"`
	CreateKey      *string `gorm:"type:char(64);uniqueIndex:idx_photo_create_key"`
	Status         string  `gorm:"type:varchar(16);not null;default:active;index"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (Photo) TableName() string { return "photo" }

type Post struct {
	UserID         uint64 `gorm:"not null;default:0;index"`
	ID             uint64 `gorm:"primaryKey"`
	Nickname       string `gorm:"type:varchar(64);not null"`
	GraduationYear int
	Content        string  `gorm:"type:varchar(500);not null"`
	Image          []byte  `gorm:"type:mediumblob"`
	MIME           string  `gorm:"type:varchar(32);not null;default:''"`
	AvatarColor    string  `gorm:"type:varchar(8);not null;default:red"`
	CreateKey      *string `gorm:"type:char(64);uniqueIndex:idx_post_create_key"`
	Status         string  `gorm:"type:varchar(16);not null;default:active;index"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	LikeCount      int64 `gorm:"->;-:migration"`
	ReplyCount     int64 `gorm:"->;-:migration"`
	HasImage       bool  `gorm:"->;-:migration"`
}

func (Post) TableName() string { return "post" }

type Reply struct {
	PhotoID        uint64  `gorm:"not null;default:0;index"`
	UserID         uint64  `gorm:"not null;default:0;index"`
	ID             uint64  `gorm:"primaryKey"`
	PostID         uint64  `gorm:"not null;index"`
	ParentID       uint64  `gorm:"not null;default:0;index"`
	ParentNickname string  `gorm:"->;-:migration"`
	Nickname       string  `gorm:"type:varchar(64);not null"`
	Content        string  `gorm:"type:varchar(500);not null"`
	CreateKey      *string `gorm:"type:char(64);uniqueIndex:idx_reply_create_key"`
	Status         string  `gorm:"type:varchar(16);not null;default:active"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (Reply) TableName() string { return "reply" }

type AuditLog struct {
	ID        uint64 `gorm:"primaryKey"`
	Kind      string `gorm:"type:varchar(16);not null"`
	TargetID  uint64 `gorm:"not null;index"`
	Action    string `gorm:"type:varchar(16);not null"`
	Reason    string `gorm:"type:varchar(200);not null"`
	CreatedAt time.Time
}

func (AuditLog) TableName() string { return "audit_log" }
