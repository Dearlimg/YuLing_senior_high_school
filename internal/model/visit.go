package model

import "time"

// VisitEvent 记录一次真实浏览器页面浏览（beacon 上报）。
// 隐私约束：不存明文 IP、UA、完整 referer；
// visitor_hash 来自前端匿名浏览器 ID，ip_hash 为 IP+UA 的 SHA-256。
// JSON 标签供管理看板接口直接序列化使用。
type VisitEvent struct {
	ID          uint64    `gorm:"primaryKey" json:"id"`
	EventKey    string    `gorm:"type:char(64);not null;uniqueIndex" json:"event_key"`
	VisitorHash string    `gorm:"type:char(64);not null;index" json:"visitor_hash"`
	IPHash      string    `gorm:"type:char(64);not null" json:"ip_hash"`
	Path        string    `gorm:"type:varchar(128);not null;index" json:"path"`
	Source      string    `gorm:"type:varchar(48);not null;default:'direct'" json:"source"`
	Device      string    `gorm:"type:varchar(16);not null;default:'desktop'" json:"device"`
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
}

func (VisitEvent) TableName() string { return "visit_event" }
