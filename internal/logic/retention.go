package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/model"
)

type SubscriptionRequest struct {
	GraduationYear int    `json:"graduation_year"`
	ClassName      string `json:"class_name"`
	Subscribed     *bool  `json:"subscribed"`
}

func (l *AlumniLogic) Subscribe(ctx context.Context, req SubscriptionRequest) error {
	actor, err := CurrentActor(ctx)
	if err != nil {
		return err
	}
	if req.Subscribed == nil {
		return invalid("请明确选择订阅或取消订阅")
	}
	req.ClassName = strings.TrimSpace(req.ClassName)
	if err := validateClass(req.GraduationYear, req.ClassName); err != nil {
		return err
	}
	err = l.store.SaveSubscription(ctx, &model.ClassSubscription{
		UserID: actor.UserID, GraduationYear: req.GraduationYear, ClassName: req.ClassName,
	}, *req.Subscribed)
	if errors.Is(err, dao.ErrSubscriptionLimit) {
		return invalid("最多可订阅 20 个班级，请先取消其他订阅")
	}
	return businessError(err)
}

func (l *AlumniLogic) Subscriptions(ctx context.Context) ([]model.ClassSubscription, error) {
	actor, err := CurrentActor(ctx)
	if err != nil {
		return nil, err
	}
	return l.store.Subscriptions(ctx, actor.UserID)
}

func (l *AlumniLogic) Updates(ctx context.Context, req ContentRequest) (dao.FeedResult, error) {
	actor, err := CurrentActor(ctx)
	if err != nil {
		return dao.FeedResult{}, err
	}
	page, size := pagination(req.Page, req.PageSize)
	return l.store.ActivityFeed(ctx, dao.FeedQuery{
		UserID: actor.UserID, AsOf: time.Now().UTC(), Page: page, PageSize: size,
	})
}

func (l *AlumniLogic) ReadUpdates(ctx context.Context, through time.Time) error {
	actor, err := CurrentActor(ctx)
	if err != nil {
		return err
	}
	if through.IsZero() || through.After(time.Now()) {
		return invalid("已读时间不正确，请刷新动态后重试")
	}
	return l.store.ReadActivities(ctx, actor.UserID, through.UTC())
}

type AnnualReportResult struct {
	dao.AnnualReport
	Year int       `json:"year"`
	AsOf time.Time `json:"as_of"`
}

func (l *AlumniLogic) AnnualReport(ctx context.Context, year int) (*AnnualReportResult, error) {
	zone := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Now().In(zone)
	if year == 0 {
		year = now.Year()
	}
	if year < 1903 || year > now.Year() {
		return nil, invalid("报告年份需在 1903 年至今年之间")
	}
	since := time.Date(
		year,
		1,
		1,
		0,
		0,
		0,
		0,
		zone,
	)
	until := since.AddDate(1, 0, 0)
	if until.After(now) {
		until = now
	}
	data, err := l.store.AnnualReport(ctx, dao.ReportWindow{Since: since.UTC(), Until: until.UTC()})
	if err != nil {
		return nil, err
	}
	return &AnnualReportResult{AnnualReport: data, Year: year, AsOf: until.UTC()}, nil
}
