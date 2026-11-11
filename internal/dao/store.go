package dao

import (
	"context"
	"errors"
	"time"

	"YuLing_senior_high_school/internal/model"
)

var (
	ErrNotFound  = errors.New("记录不存在")
	ErrDuplicate = errors.New("重复提交")
)

type ListQuery struct {
	GraduationYear int
	City           string
	Keyword        string
	University     string
	ClassName      string
	Page           int
	PageSize       int
}

type DirectoryMeta struct {
	Total int64 `json:"total"`
	Years []int `json:"years"`
}

type ClassGroup struct {
	GraduationYear int    `json:"graduation_year"`
	ClassName      string `json:"class_name"`
	Count          int64  `json:"count"`
}

type University struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type Stats struct {
	Alumni       int64 `json:"alumni"`
	Universities int64 `json:"universities"`
	Cities       int64 `json:"cities"`
	Photos       int64 `json:"photos"`
	Posts        int64 `json:"posts"`
}

type ContentQuery struct {
	Page     int
	PageSize int
	Category string
}

type ModerationItem struct {
	ID        uint64    `json:"id"`
	Kind      string    `json:"kind"`
	Status    string    `json:"status"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"created_at"`
}

// Store keeps persistence details out of the business layer.
type Store interface {
	FindUser(context.Context, string) (*model.User, error)
	FindUserByID(context.Context, uint64) (*model.User, error)
	FindUserByAlumni(context.Context, uint64) (*model.User, error)
	RegisterUser(context.Context, *model.User, *model.Alumni) error
	ListAlumni(context.Context, ListQuery) ([]model.Alumni, int64, error)
	FindAlumni(context.Context, uint64) (*model.Alumni, error)
	CreateAlumni(context.Context, *model.Alumni) error
	UpdateAlumni(context.Context, *model.Alumni) error
	DirectoryMeta(context.Context) (DirectoryMeta, error)
	Classes(context.Context, ListQuery) ([]ClassGroup, int64, error)
	Universities(context.Context) ([]University, error)
	Stats(context.Context) (Stats, error)
	ListPhotos(context.Context, ContentQuery) ([]model.Photo, int64, error)
	FindPhoto(context.Context, uint64) (*model.Photo, error)
	CreatePhoto(context.Context, *model.Photo) error
	DeletePhoto(context.Context, uint64) error
	ListPosts(context.Context, ContentQuery) ([]model.Post, int64, error)
	FindPost(context.Context, uint64) (*model.Post, error)
	CreatePost(context.Context, *model.Post) error
	ListReplies(context.Context, uint64, ContentQuery) ([]model.Reply, int64, error)
	CreateReply(context.Context, *model.Reply) (int64, error)
	Moderate(context.Context, model.AuditLog) error
	Inspect(context.Context, string, uint64) (*ModerationItem, error)
	AuditLogs(context.Context, ContentQuery) ([]model.AuditLog, error)
}

type Limiter interface {
	Allow(context.Context, string, int, time.Duration) (bool, error)
}
