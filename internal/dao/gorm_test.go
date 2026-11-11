package dao

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"YuLing_senior_high_school/internal/model"
)

func mockStore(t *testing.T) (*SQLStore, sqlmock.Sqlmock, *[]string) {
	t.Helper()
	queries := []string{}
	matcher := sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		queries = append(queries, actual)
		if !strings.HasPrefix(actual, expected) {
			return fmt.Errorf("expected %s, got %s", expected, actual)
		}
		return nil
	})
	connection, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: connection, SkipInitializeWithVersion: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewSQLStore(db), mock, &queries
}

func TestGORMSearchAndPagination(t *testing.T) {
	store, mock, queries := mockStore(t)
	pattern := "%西安%"
	mock.ExpectQuery("SELECT count").WithArgs("active", 2018, pattern, pattern, pattern, pattern, pattern, pattern).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(13))
	mock.ExpectQuery("SELECT").WithArgs("active", 2018, pattern, pattern, pattern, pattern, pattern, pattern, 12, 12).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "identity"}).AddRow(1, "测试", "alumni"))
	rows, total, err := store.ListAlumni(context.Background(), ListQuery{
		GraduationYear: 2018, Keyword: "西安", Page: 2, PageSize: 12,
	})
	if err != nil || len(rows) != 1 || total != 13 {
		t.Fatalf("%#v %d %v", rows, total, err)
	}
	query := (*queries)[len(*queries)-1]
	for _, fragment := range []string{"deleted_at IS NULL", "dream_university LIKE", "company LIKE", "updated_at DESC, id DESC", "LIMIT ? OFFSET ?"} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("missing %s in %s", fragment, query)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGORMPhotoListsDoNotLoadBlobOrSecret(t *testing.T) {
	store, mock, queries := mockStore(t)
	mock.ExpectQuery("SELECT count").WithArgs("active").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT").WithArgs("active", 12).
		WillReturnRows(sqlmock.NewRows([]string{"id", "caption", "status"}).AddRow(1, "校园", "active"))
	rows, _, err := store.ListPhotos(context.Background(), ContentQuery{Page: 1, PageSize: 12})
	if err != nil || len(rows) != 1 {
		t.Fatal(err)
	}
	query := (*queries)[len(*queries)-1]
	for _, forbidden := range []string{"`image`", "delete_code_hash", "create_key"} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("photo listing loaded %s", forbidden)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGORMReplyTransactionRejectsHiddenParent(t *testing.T) {
	store, mock, queries := mockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT").WithArgs(uint64(7), "active", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()
	_, err := store.CreateReply(context.Background(), &model.Reply{PostID: 7, Nickname: "校友", Content: "回复"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains((*queries)[0], "FOR UPDATE") {
		t.Fatal("parent must be locked before reply insert")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGORMCreateTranslatesDuplicateAndRollsBack(t *testing.T) {
	store, mock, _ := mockStore(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT").WillReturnError(&mysqlDriver.MySQLError{Number: 1062, Message: "duplicate"})
	mock.ExpectRollback()
	err := store.CreateAlumni(context.Background(), &model.Alumni{Name: "校友", GraduationYear: 2018})
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGORMUniversityReverseLookupExcludesDreams(t *testing.T) {
	store, mock, queries := mockStore(t)
	mock.ExpectQuery("SELECT count").WithArgs("active", "alumni", "北京大学").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT").WithArgs("active", "alumni", "北京大学", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	_, _, err := store.ListAlumni(context.Background(), ListQuery{University: "北京大学", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains((*queries)[1], "identity = ? AND university = ?") {
		t.Fatal("reverse lookup must match actual school and alumni identity")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
