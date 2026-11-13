package dao

import (
	"context"
	"errors"
	"time"

	"YuLing_senior_high_school/internal/model"
)

var (
	ErrNotFound          = errors.New("记录不存在")
	ErrDuplicate         = errors.New("重复提交")
	ErrSubscriptionLimit = errors.New("最多可订阅 20 个班级")
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
	Status   string
	Keyword  string
	Sort     string
	Since    time.Time
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
	SaveSubscription(context.Context, *model.ClassSubscription, bool) error
	Subscriptions(context.Context, uint64) ([]model.ClassSubscription, error)
	ActivityFeed(context.Context, FeedQuery) (FeedResult, error)
	ReadActivities(context.Context, uint64, time.Time) error
	AnnualReport(context.Context, ReportWindow) (AnnualReport, error)
	ApplyOrganizer(context.Context, *model.ClassOrganizer) error
	ListOrganizers(context.Context, OrganizerQuery) ([]model.ClassOrganizer, int64, error)
	ReviewOrganizer(context.Context, OrganizerReview) error
	CreateRecovery(context.Context, *model.CardRecovery) error
	FindRecovery(context.Context, uint64) (*model.CardRecovery, error)
	ListRecoveries(context.Context, ContentQuery) ([]model.CardRecovery, int64, error)
	ReviewRecovery(context.Context, RecoveryReview) error
	CompleteRecovery(context.Context, RecoveryCompletion) error
	ClaimAlumni(context.Context, *model.Alumni, *model.GrowthEvent) error
	RecordGrowth(context.Context, *model.GrowthEvent) error
	GrowthCounts(context.Context, GrowthQuery) ([]GrowthCount, error)
	ListUsers(context.Context, ContentQuery) ([]AdminUser, int64, error)
	SetUserAdmin(context.Context, uint64, bool, model.AuditLog) error
	ListModeration(context.Context, string, ContentQuery) ([]ModerationItem, int64, error)
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
	CohortUniversities(context.Context, int) ([]University, error)
	Stats(context.Context) (Stats, error)
	Likes(context.Context, model.ContentLike) (LikeState, error)
	SetLike(context.Context, model.ContentLike, bool) (LikeState, error)
	ListPhotos(context.Context, ContentQuery) ([]model.Photo, int64, error)
	FindPhoto(context.Context, uint64) (*model.Photo, error)
	CreatePhoto(context.Context, *model.Photo) error
	DeletePhoto(context.Context, uint64) error
	RankedPosts(context.Context, string) ([]model.Post, error)
	ListPosts(context.Context, ContentQuery) ([]model.Post, int64, error)
	FindPost(context.Context, uint64) (*model.Post, error)
	CreatePost(context.Context, *model.Post) error
	ListReplies(context.Context, uint64, ContentQuery) ([]model.Reply, int64, error)
	ListPhotoReplies(context.Context, uint64, ContentQuery) ([]model.Reply, int64, error)
	CreateReply(context.Context, *model.Reply) (int64, error)
	Moderate(context.Context, model.AuditLog) error
	Inspect(context.Context, string, uint64) (*ModerationItem, error)
	AuditLogs(context.Context, ContentQuery) ([]model.AuditLog, error)
}

type Limiter interface {
	Allow(context.Context, string, int, time.Duration) (bool, error)
}
