package store_test

import (
	"testing"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

func TestCollaborationRunCarriesFrameworkNeutralAuditFields(t *testing.T) {
	now := time.Now().UTC()
	run := store.CollaborationRun{
		ID:            "collaboration_1",
		RoomID:        "room_1",
		RootMessageID: "message_1",
		Engine:        model.CollaborationEngineNative,
		EngineVersion: "native-v1",
		PolicyVersion: "v1",
		Status:        model.CollaborationRunStatusCreated,
		CreatedAt:     now,
	}
	if run.Engine != model.CollaborationEngineNative || run.RootMessageID != "message_1" || run.CreatedAt != now {
		t.Fatalf("unexpected collaboration run audit fields: %#v", run)
	}
}

func TestCollaborationRunStatusTransitionsAllowOneTerminalState(t *testing.T) {
	terminalStatuses := []string{
		model.CollaborationRunStatusSucceeded,
		model.CollaborationRunStatusStopped,
		model.CollaborationRunStatusFailed,
		model.CollaborationRunStatusCancelled,
		model.CollaborationRunStatusTimeout,
		model.CollaborationRunStatusInterrupted,
	}
	for _, status := range terminalStatuses {
		if !model.CanTransitionCollaborationRunStatus(model.CollaborationRunStatusRunning, status) {
			t.Fatalf("expected running -> %s transition to be valid", status)
		}
		if model.CanTransitionCollaborationRunStatus(status, model.CollaborationRunStatusFailed) {
			t.Fatalf("expected terminal status %s to reject a second terminal transition", status)
		}
	}
}

func TestCollaborationRunStartupReconciliationCanInterruptActiveStates(t *testing.T) {
	for _, status := range []string{model.CollaborationRunStatusCreated, model.CollaborationRunStatusRunning} {
		if !model.CanTransitionCollaborationRunStatus(status, model.CollaborationRunStatusInterrupted) {
			t.Fatalf("expected startup reconciliation to interrupt %s run", status)
		}
	}
	if model.CanTransitionCollaborationRunStatus(model.CollaborationRunStatusSucceeded, model.CollaborationRunStatusInterrupted) {
		t.Fatal("expected startup reconciliation not to overwrite a terminal run")
	}
}
