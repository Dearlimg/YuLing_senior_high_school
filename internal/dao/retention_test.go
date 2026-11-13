package dao

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"YuLing_senior_high_school/internal/model"
)

func TestSubscriptionFeedIsolationReadCheckpointAndCancellation(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	sub := &model.ClassSubscription{UserID: 1, GraduationYear: 2018, ClassName: "高三1班"}
	if err := s.SaveSubscription(ctx, sub, true); err != nil {
		t.Fatal(err)
	}
	since := time.Now().UTC().Add(-10 * time.Minute)
	stored := s.subscriptions[sub.ID]
	stored.SinceAt, stored.ReadAt = since, since
	s.subscriptions[sub.ID] = stored
	sub2 := &model.ClassSubscription{UserID: 1, GraduationYear: 2018, ClassName: "高三1班"}
	if err := s.SaveSubscription(ctx, sub2, true); err != nil {
		t.Fatal(err)
	}
	if len(s.subscriptions) != 1 || !s.subscriptions[1].SinceAt.Equal(since) {
		t.Fatal("idempotent subscribe reset the cursor")
	}
	// Explicit times make before/after and the refresh/read race deterministic.
	old, first, later := since.Add(-time.Minute), since.Add(time.Minute), since.Add(2*time.Minute)
	s.alumni[1] = model.Alumni{ID: 1, Name: "同窗", GraduationYear: 2018, ClassName: "高三1班",
		Status: "active", UpdatedAt: first}
	s.alumni[2] = model.Alumni{ID: 2, Name: "订阅前", GraduationYear: 2018, ClassName: "高三1班",
		Status: "active", UpdatedAt: old}
	s.alumni[3] = model.Alumni{ID: 3, Name: "已隐藏", GraduationYear: 2018, ClassName: "高三1班",
		Status: "hidden", UpdatedAt: first}
	s.alumni[4] = model.Alumni{ID: 4, Name: "其他班", GraduationYear: 2018, ClassName: "高三2班",
		Status: "active", UpdatedAt: first}
	s.users[1] = model.User{ID: 1, AlumniID: 1}
	s.users[2] = model.User{ID: 2, AlumniID: 3}
	s.posts[1] = model.Post{ID: 1, UserID: 1, Content: "新近况", Status: "active", CreatedAt: later}
	s.posts[2] = model.Post{ID: 2, UserID: 2, Content: "隐藏卡片的帖子", Status: "active", CreatedAt: first}
	s.posts[3] = model.Post{ID: 3, UserID: 1, Content: "隐藏帖子", Status: "hidden", CreatedAt: first}
	s.photos[1] = model.Photo{ID: 1, UserID: 1, Caption: "毕业照", Status: "active", CreatedAt: first}
	q := FeedQuery{UserID: 1, AsOf: first, Page: 1, PageSize: 20}
	feed, err := s.ActivityFeed(ctx, q)
	if err != nil || feed.Total != 2 || feed.Unread != 2 {
		t.Fatalf("initial feed: %#v %v", feed, err)
	}
	if err := s.ReadActivities(ctx, 2, first); err != nil {
		t.Fatal(err)
	}
	feed, _ = s.ActivityFeed(ctx, q)
	if feed.Unread != 2 {
		t.Fatal("another user changed read state")
	}
	if err := s.ReadActivities(ctx, 1, feed.AsOf); err != nil {
		t.Fatal(err)
	}
	q.AsOf = later
	feed, _ = s.ActivityFeed(ctx, q)
	if feed.Total != 3 || feed.Unread != 1 || feed.Items[0].Kind != "post" {
		t.Fatalf("new arrivals were consumed by stale read checkpoint: %#v", feed)
	}
	q.UserID = 2
	private, _ := s.ActivityFeed(ctx, q)
	if private.Total != 0 {
		t.Fatal("another user received the subscriptions")
	}
	q.UserID = 1
	if err := s.SaveSubscription(ctx, sub2, false); err != nil {
		t.Fatal(err)
	}
	feed, _ = s.ActivityFeed(ctx, q)
	if feed.Total != 0 {
		t.Fatal("cancelled subscription still delivered activity")
	}
	// Re-subscription starts now, so old entries do not become new notifications.
	if err := s.SaveSubscription(ctx, sub2, true); err != nil {
		t.Fatal(err)
	}
	if !sub2.SinceAt.After(since) {
		t.Fatal("re-subscribe retained previous start time")
	}
}

func TestSubscriptionCapAndAnnualReportVisibility(t *testing.T) {
	ctx, s := context.Background(), NewMemoryStore()
	for i := range 20 {
		err := s.SaveSubscription(ctx, &model.ClassSubscription{
			UserID: 1, GraduationYear: 2018, ClassName: fmt.Sprintf("高三%d班", i+1),
		}, true)
		if err != nil {
			t.Fatal(err)
		}
	}
	err := s.SaveSubscription(ctx, &model.ClassSubscription{
		UserID: 1, GraduationYear: 2018, ClassName: "额外班级",
	}, true)
	if !errors.Is(err, ErrSubscriptionLimit) {
		t.Fatalf("subscription cap: %v", err)
	}
	start := time.Now().UTC().Truncate(time.Hour)
	w := ReportWindow{Since: start, Until: start.Add(time.Hour)}
	for id := uint64(1); id <= 6; id++ {
		s.alumni[id] = model.Alumni{ID: id, Identity: "alumni", GraduationYear: 2018, ClassName: "高三1班",
			Status: "active", CreatedAt: start, University: "西安交通大学", City: "西安"}
	}
	row := s.alumni[2]
	row.Identity, row.University, row.DreamUniversity = "student", "", "理想大学不能计入"
	s.alumni[2] = row
	row = s.alumni[3]
	row.CreatedAt = start.Add(-time.Nanosecond)
	s.alumni[3] = row
	row = s.alumni[4]
	row.CreatedAt = w.Until
	s.alumni[4] = row
	row = s.alumni[5]
	row.Status = "hidden"
	s.alumni[5] = row
	row = s.alumni[6]
	row.DeletedAt = &start
	s.alumni[6] = row
	s.photos[1] = model.Photo{ID: 1, Status: "active", CreatedAt: start}
	s.photos[2] = model.Photo{ID: 2, Status: "hidden", CreatedAt: start}
	s.posts[1] = model.Post{ID: 1, Status: "active", CreatedAt: w.Until}
	report, err := s.AnnualReport(ctx, w)
	if err != nil || report.Registrations != 2 || report.Graduates != 1 || report.Students != 1 {
		t.Fatalf("report population: %#v %v", report, err)
	}
	if report.Cities != 1 || report.Classes != 1 || report.Photos != 1 || report.Posts != 0 {
		t.Fatalf("report bounds/visibility: %#v", report)
	}
	if len(report.Universities) != 1 || report.Universities[0].Count != 1 {
		t.Fatal("student dream university entered actual destinations")
	}
}

func TestSQLFeedHasScopedPublicProjectionAndIndependentQueries(t *testing.T) {
	store, mock, queries := mockStore(t)
	now := time.Now().UTC()
	for _, column := range []string{"count", "unread"} {
		mock.ExpectQuery("SELECT").WithArgs(7, now, 7, now, 7, now).
			WillReturnRows(sqlmock.NewRows([]string{column}).AddRow(1))
	}
	mock.ExpectQuery("SELECT").WithArgs(7, now, 7, now, 7, now, 20).
		WillReturnRows(sqlmock.NewRows([]string{"kind", "target_id", "name", "occurred_at", "unread"}).
			AddRow("alumni", 9, "同窗", now, true))
	feed, err := store.ActivityFeed(context.Background(), FeedQuery{UserID: 7, AsOf: now, Page: 1, PageSize: 20})
	if err != nil || feed.Total != 1 || feed.Unread != 1 || len(feed.Items) != 1 {
		t.Fatalf("SQL feed: %#v %v", feed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	for _, query := range *queries {
		if strings.Count(query, "s.user_id = ?") != 3 || strings.Count(query, "a.deleted_at IS NULL") != 3 {
			t.Fatal("feed lost user or public-state filters")
		}
		for _, secret := range []string{"phone", "wechat", "edit_code", "password", "p.image"} {
			if strings.Contains(query, secret) {
				t.Fatalf("feed queried private or binary field: %s", secret)
			}
		}
	}
}

func TestSQLSubscriptionCapSerializesByUser(t *testing.T) {
	store, mock, queries := mockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT").WithArgs(7, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectQuery("SELECT").WithArgs(7, 2018, "高三1班", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT").WithArgs(7, true).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(20))
	mock.ExpectRollback()
	err := store.SaveSubscription(context.Background(), &model.ClassSubscription{
		UserID: 7, GraduationYear: 2018, ClassName: "高三1班",
	}, true)
	if !errors.Is(err, ErrSubscriptionLimit) {
		t.Fatalf("SQL cap: %v", err)
	}
	if len(*queries) == 0 || !strings.Contains((*queries)[0], "FOR UPDATE") {
		t.Fatal("subscription limit is vulnerable to concurrent writes")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLAnnualReportUsesPublicRecordsWithinWindow(t *testing.T) {
	store, mock, queries := mockStore(t)
	now := time.Now().UTC()
	w := ReportWindow{Since: now.Add(-time.Hour), Until: now}
	mock.ExpectQuery("SELECT").WithArgs("active", w.Since, w.Until).
		WillReturnRows(sqlmock.NewRows([]string{"registrations", "graduates", "students", "cities", "classes"}).
			AddRow(2, 1, 1, 1, 1))
	mock.ExpectQuery("SELECT").WithArgs("active", w.Since, w.Until, "alumni").
		WillReturnRows(sqlmock.NewRows([]string{"name", "count"}).AddRow("西安交通大学", 1))
	mock.ExpectQuery("SELECT").WithArgs("active", w.Since, w.Until, 10).
		WillReturnRows(sqlmock.NewRows([]string{"graduation_year", "class_name", "count"}).AddRow(2018, "高三1班", 2))
	for range 2 {
		mock.ExpectQuery("SELECT").WithArgs("active", w.Since, w.Until).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	}
	report, err := store.AnnualReport(context.Background(), w)
	if err != nil || report.Registrations != 2 || len(report.Universities) != 1 || len(report.TopClasses) != 1 {
		t.Fatalf("SQL report: %#v %v", report, err)
	}
	if report.Photos != 1 || report.Posts != 1 {
		t.Fatal("SQL content counts missing")
	}
	for _, query := range *queries {
		if !strings.Contains(query, "created_at >= ? AND created_at < ?") {
			t.Fatal("SQL report lost half-open year window")
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
