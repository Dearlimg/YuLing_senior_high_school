// Package model defines persistent entities. Public responses use explicit DTOs.
package model

import "time"

// Alumni stores both graduates and current students; dream schools are kept separate.
type Alumni struct {
	ID       uint64 `gorm:"primaryKey;index:idx_feed,priority:3"`
	Name     string `gorm:"type:varchar(64);not null;index:idx_name"`
	Gender   int8
	Identity string `gorm:"type:varchar(16);not null;default:alumni"`
	// idx_feed 是列表页的覆盖索引：(deleted_at, graduation_year, id)。
	// 列表查询固定是 WHERE deleted_at IS NULL ORDER BY graduation_year DESC, id DESC，
	// 三列顺序必须和这个模式对齐，否则优化器会退回全表扫描 + filesort。
	// 压测实测：加索引前 type=ALL/rows=4981/Using filesort，
	// 加索引后 type=index/rows=20/Backward index scan。
	GraduationYear  int    `gorm:"not null;index:idx_year_class;index:idx_feed,priority:2"`
	ClassName       string `gorm:"type:varchar(32);not null;default:'';index:idx_year_class"`
	City            string `gorm:"type:varchar(64);not null;default:'';index:idx_city"`
	University      string `gorm:"type:varchar(100);not null;default:'';index:idx_university"`
	DreamUniversity string `gorm:"type:varchar(100);not null;default:''"`
	Major           string `gorm:"type:varchar(50);not null;default:''"`
	Education       string `gorm:"type:varchar(16);not null;default:''"`
	Industry        string `gorm:"type:varchar(64);not null;default:''"`
	Company         string `gorm:"type:varchar(128);not null;default:''"`
	Position        string `gorm:"type:varchar(64);not null;default:''"`
	Phone           string `gorm:"type:varchar(32);not null;default:''"`
	Wechat          string `gorm:"type:varchar(50);not null;default:''"`
	Email           string `gorm:"type:varchar(128);not null;default:''"`
	Remark          string `gorm:"type:varchar(255);not null;default:''"`
	Message         string `gorm:"type:varchar(120);not null;default:''"`
	AvatarColor     string `gorm:"type:varchar(8);not null;default:red"`
	ContactPublic   bool
	PhonePublic     bool
	WechatPublic    bool
	EditCodeHash    string  `gorm:"type:char(64);not null;default:''"`
	CreateKey       *string `gorm:"type:char(64);uniqueIndex:idx_alumni_create_key"`
	Status          string  `gorm:"type:varchar(16);not null;default:active"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	// idx_feed 的第一列。软删除条件几乎是所有查询的前缀，放最左。
	DeletedAt *time.Time `gorm:"index:idx_deleted_at;index:idx_feed,priority:1"`
}

func (Alumni) TableName() string { return "alumni" }
