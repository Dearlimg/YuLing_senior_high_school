package dao

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"YuLing_senior_high_school/internal/model"
)

func TestClaimRollsBackWhenSuccessEventFails(t *testing.T) {
	store, mock, _ := mockStore(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `alumni`").WillReturnResult(sqlmock.NewResult(17, 1))
	mock.ExpectExec("INSERT INTO `growth_event`").WillReturnError(errors.New("write failed"))
	mock.ExpectRollback()
	err := store.ClaimAlumni(
		context.Background(),
		&model.Alumni{Name: "校友"},
		&model.GrowthEvent{Event: "registration_success"},
	)
	if err == nil {
		t.Fatal("expected transaction failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGrowthSQLDistinctUVAndParameterizedFilters(t *testing.T) {
	store, mock, queries := mockStore(t)
	since := time.Now().Add(-24 * time.Hour)
	mock.ExpectQuery("SELECT CASE").WithArgs(since, "class_invite", "reunion", 2018, "高三1班").
		WillReturnRows(sqlmock.NewRows([]string{"event", "uv", "share_uv"}).AddRow("share", 2, 2))
	rows, err := store.GrowthCounts(context.Background(), GrowthQuery{
		Since: since, Source: "class_invite", Campaign: "reunion", GraduationYear: 2018, ClassName: "高三1班",
	})
	if err != nil || len(rows) != 1 || rows[0].UV != 2 {
		t.Fatalf("%#v %v", rows, err)
	}
	query := (*queries)[0]
	if !strings.Contains(query, "COUNT(DISTINCT visitor_hash)") || !strings.Contains(query, "GROUP BY CASE") {
		t.Fatal(query)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClaimInvalidatesClassAndDirectoryCaches(t *testing.T) {
	store, _, _ := cacheFixture(t)
	ctx := context.Background()
	query := ListQuery{GraduationYear: 2018, ClassName: "高三1班", Page: 1, PageSize: 12}
	_, _, _ = store.ListAlumni(ctx, query)
	_, _, _ = store.Classes(ctx, query)
	_, _ = store.DirectoryMeta(ctx)
	card := &model.Alumni{Name: "报到", GraduationYear: 2018, ClassName: "高三1班", Status: "active"}
	if err := store.ClaimAlumni(ctx, card, &model.GrowthEvent{Event: "registration_success"}); err != nil {
		t.Fatal(err)
	}
	_, total, err := store.ListAlumni(ctx, query)
	if err != nil || total != 1 {
		t.Fatalf("stale members: %d %v", total, err)
	}
	groups, total, err := store.Classes(ctx, query)
	if err != nil || total != 1 || groups[0].Count != 1 {
		t.Fatalf("stale classes: %#v %v", groups, err)
	}
	meta, err := store.DirectoryMeta(ctx)
	if err != nil || meta.Total != 1 {
		t.Fatalf("stale meta: %#v %v", meta, err)
	}
}
