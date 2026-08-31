package mysql

import (
	"context"
	"errors"
	"testing"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
	"github.com/DATA-DOG/go-sqlmock"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestFinishCollaborationRunCommitsOneTerminalState(t *testing.T) {
	repository, mock := newMockCollaborationRepository(t)
	expectLockedCollaborationRun(mock, model.CollaborationRunStatusRunning, "", 0, "")
	mock.ExpectExec("UPDATE .*collaboration_runs.* SET .* WHERE id = \\?").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repository.FinishCollaborationRun(context.Background(), store.FinishCollaborationRunInput{
		RunID: "collaboration_1", Status: model.CollaborationRunStatusSucceeded,
		StopReason: model.CollaborationStopReasonCompleted, TurnCount: 2, CompletedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("finish collaboration run: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFinishCollaborationRunIsIdempotentForSameTerminalState(t *testing.T) {
	repository, mock := newMockCollaborationRepository(t)
	expectLockedCollaborationRun(mock, model.CollaborationRunStatusSucceeded, model.CollaborationStopReasonCompleted, 2, "")
	mock.ExpectCommit()

	err := repository.FinishCollaborationRun(context.Background(), store.FinishCollaborationRunInput{
		RunID: "collaboration_1", Status: model.CollaborationRunStatusSucceeded,
		StopReason: model.CollaborationStopReasonCompleted, TurnCount: 2, CompletedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("repeat terminal state: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFinishCollaborationRunRollsBackConflictingTerminalState(t *testing.T) {
	repository, mock := newMockCollaborationRepository(t)
	expectLockedCollaborationRun(mock, model.CollaborationRunStatusSucceeded, model.CollaborationStopReasonCompleted, 2, "")
	mock.ExpectRollback()

	err := repository.FinishCollaborationRun(context.Background(), store.FinishCollaborationRunInput{
		RunID: "collaboration_1", Status: model.CollaborationRunStatusFailed,
		StopReason: model.CollaborationStopReasonEngineFailure, TurnCount: 2, CompletedAt: time.Now().UTC(),
	})
	if !errors.Is(err, store.ErrCollaborationRunFinished) {
		t.Fatalf("expected conflicting terminal error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFinishCollaborationRunRollsBackUpdateFailure(t *testing.T) {
	repository, mock := newMockCollaborationRepository(t)
	expectLockedCollaborationRun(mock, model.CollaborationRunStatusRunning, "", 0, "")
	mock.ExpectExec("UPDATE .*collaboration_runs.* SET .* WHERE id = \\?").WillReturnError(errors.New("write failed"))
	mock.ExpectRollback()

	err := repository.FinishCollaborationRun(context.Background(), store.FinishCollaborationRunInput{
		RunID: "collaboration_1", Status: model.CollaborationRunStatusCancelled,
		StopReason: model.CollaborationStopReasonCancelled, CompletedAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected update failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func newMockCollaborationRepository(t *testing.T) (*MySQLStore, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &MySQLStore{db: db}, mock
}

func expectLockedCollaborationRun(mock sqlmock.Sqlmock, status string, reason string, turnCount int, errText string) {
	mock.ExpectBegin()
	errorValue := any(nil)
	if errText != "" {
		errorValue = errText
	}
	rows := sqlmock.NewRows([]string{"id", "status", "stop_reason", "turn_count", "error"}).
		AddRow("collaboration_1", status, reason, turnCount, errorValue)
	mock.ExpectQuery("SELECT \\* FROM .*collaboration_runs.* WHERE id = \\? ORDER BY .* LIMIT \\? FOR UPDATE").
		WithArgs("collaboration_1", 1).
		WillReturnRows(rows)
}
