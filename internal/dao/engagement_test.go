package dao

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRecoveryResetAndAuditShareTransaction(t *testing.T) {
	store, mock, _ := mockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id", "edit_code_hash", "status"}).AddRow(9, "oldhash", "active"))
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id", "alumni_id", "status", "expires_at"}).AddRow(2, 9, "approved", time.Now().Add(time.Hour)))
	mock.ExpectExec("UPDATE `alumni`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `card_recovery`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `card_recovery`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `audit_log`").WillReturnError(errors.New("audit unavailable"))
	mock.ExpectRollback()
	err := store.CompleteRecovery(context.Background(), RecoveryCompletion{ID: 2, AlumniID: 9, ReceiptHash: "receipt-hash", NewEditHash: "new-hash"})
	if err == nil {
		t.Fatal("expected rollback")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
