package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"YuLing_senior_high_school/internal/dao"
)

type reportWindowStore struct {
	dao.Store
	window dao.ReportWindow
}

func (s *reportWindowStore) AnnualReport(_ context.Context, window dao.ReportWindow) (dao.AnnualReport, error) {
	s.window = window
	return dao.AnnualReport{Universities: []dao.University{}, TopClasses: []dao.ClassGroup{}}, nil
}

func TestAnnualReportUsesShanghaiCalendarYear(t *testing.T) {
	store := &reportWindowStore{Store: dao.NewMemoryStore()}
	l := NewAlumniLogic(store)
	report, err := l.AnnualReport(context.Background(), 2025)
	if err != nil || report.Year != 2025 {
		t.Fatal(err)
	}
	if store.window.Since.Format(time.RFC3339) != "2024-12-31T16:00:00Z" ||
		store.window.Until.Format(time.RFC3339) != "2025-12-31T16:00:00Z" {
		t.Fatalf("wrong Shanghai year: %#v", store.window)
	}
	if _, err := l.AnnualReport(context.Background(), time.Now().Year()+1); !errors.Is(err, ErrInvalidParam) {
		t.Fatal("future annual report accepted")
	}
}

func TestSubscriptionsRequireActorExplicitChoiceAndValidClass(t *testing.T) {
	l := NewAlumniLogic(dao.NewMemoryStore())
	ctx := context.Background()
	yes := true
	req := SubscriptionRequest{GraduationYear: 2018, ClassName: "高三1班", Subscribed: &yes}
	if err := l.Subscribe(ctx, req); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("anonymous subscribe accepted")
	}
	owner := WithActor(ctx, &Actor{UserID: 1})
	if err := l.Subscribe(owner, req); err != nil {
		t.Fatal(err)
	}
	req.Subscribed = nil
	if err := l.Subscribe(owner, req); !errors.Is(err, ErrInvalidParam) {
		t.Fatal("implicit opt-in accepted")
	}
	req.Subscribed, req.ClassName = &yes, "__unassigned__"
	if err := l.Subscribe(owner, req); !errors.Is(err, ErrInvalidParam) {
		t.Fatal("placeholder class accepted")
	}
	if err := l.ReadUpdates(owner, time.Now().Add(time.Minute)); !errors.Is(err, ErrInvalidParam) {
		t.Fatal("future read checkpoint accepted")
	}
	other, err := l.Subscriptions(WithActor(ctx, &Actor{UserID: 2}))
	if err != nil || len(other) != 0 {
		t.Fatal("subscriptions leaked to another user")
	}
}
