// Package model 定义持久化实体。
// 这一层只描述"数据长什么样"，不包含任何业务规则，也不依赖 gin / http。
package model

import "time"

// Alumni 校友档案。
//
// 字段设计对应真实诉求：一个高中校友录要回答的核心问题——"这个人是谁、
// 哪一届哪一班、现在在哪做什么、怎么联系"。
//
// 隐私相关字段（Phone/Email）标记了 remark，后续接入可见性控制时
// 应以字段级白名单输出，而不是整行 JSON 直出。
type Alumni struct {
	ID uint64 `gorm:"column:id;primaryKey;autoIncrement"`

	Name           string `gorm:"column:name;type:varchar(64);not null;index:idx_name"`
	Gender         int8   `gorm:"column:gender;not null;default:0"` // 0 未知 1 男 2 女
	GraduationYear int    `gorm:"column:graduation_year;not null;index:idx_year_class"` // 毕业年份，如 2015
	ClassName      string `gorm:"column:class_name;type:varchar(32);not null;default:'';index:idx_year_class"` // 班级，如 高三(2)班

	City     string `gorm:"column:city;type:varchar(64);not null;default:'';index:idx_city"`
	Industry string `gorm:"column:industry;type:varchar(64);not null;default:''"`
	Company  string `gorm:"column:company;type:varchar(128);not null;default:''"`
	Position string `gorm:"column:position;type:varchar(64);not null;default:''"`

	// 以下两项属于个人信息，接口层必须按可见性策略决定是否输出原始值。
	Phone string `gorm:"column:phone;type:varchar(32);not null;default:''"`
	Email string `gorm:"column:email;type:varchar(128);not null;default:''"`

	Remark string `gorm:"column:remark;type:varchar(255);not null;default:''"`

	CreatedAt time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time  `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt *time.Time `gorm:"column:deleted_at;index:idx_deleted_at"`
}

// TableName 显式指定表名，避免依赖 GORM 的复数化规则。
// 表名一旦被 ORM 隐式决定，后续换 ORM 或写裸 SQL 时的对不上就是灾难。
func (Alumni) TableName() string { return "alumni" }
