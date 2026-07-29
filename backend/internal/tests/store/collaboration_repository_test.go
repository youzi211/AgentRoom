package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
	"agentroom/backend/internal/tests/teststore"
)

func TestCollaborationRunLifecycleAndIdempotentTerminalCommit(t *testing.T) {
	memory := &teststore.Store{}
	ctx := context.Background()
	createdAt := time.Now().UTC()
	startedAt := createdAt.Add(time.Second)
	completedAt := startedAt.Add(time.Second)
	if err := memory.CreateCollaborationRun(ctx, store.CollaborationRun{
		ID: "collaboration_1", RoomID: "room_1", RootMessageID: "message_1",
		Engine: model.CollaborationEngineNative, CreatedAt: createdAt,
	}); err != nil {
		t.Fatalf("create collaboration run: %v", err)
	}
	if err := memory.StartCollaborationRun(ctx, "collaboration_1", "native-v1", startedAt); err != nil {
		t.Fatalf("start collaboration run: %v", err)
	}
	terminal := store.FinishCollaborationRunInput{
		RunID: "collaboration_1", Status: model.CollaborationRunStatusSucceeded,
		StopReason: model.CollaborationStopReasonCompleted, TurnCount: 2, CompletedAt: completedAt,
	}
	if err := memory.FinishCollaborationRun(ctx, terminal); err != nil {
		t.Fatalf("finish collaboration run: %v", err)
	}
	if err := memory.FinishCollaborationRun(ctx, terminal); err != nil {
		t.Fatalf("repeat same terminal commit should be idempotent: %v", err)
	}

	conflict := terminal
	conflict.Status = model.CollaborationRunStatusFailed
	conflict.StopReason = model.CollaborationStopReasonEngineFailure
	if err := memory.FinishCollaborationRun(ctx, conflict); !errors.Is(err, store.ErrCollaborationRunFinished) {
		t.Fatalf("expected conflicting terminal commit to fail, got %v", err)
	}
	got := memory.CollaborationRuns[0]
	if got.Status != model.CollaborationRunStatusSucceeded || got.TurnCount != 2 || got.EngineVersion != "native-v1" || got.CompletedAt == nil {
		t.Fatalf("unexpected committed collaboration run: %#v", got)
	}
}

func TestCollaborationRunSupportsFailureCancellationAndStartupReconciliation(t *testing.T) {
	tests := []struct {
		status string
		reason string
	}{
		{model.CollaborationRunStatusFailed, model.CollaborationStopReasonEngineFailure},
		{model.CollaborationRunStatusCancelled, model.CollaborationStopReasonCancelled},
	}
	for _, test := range tests {
		t.Run(test.status, func(t *testing.T) {
			memory := &teststore.Store{}
			now := time.Now().UTC()
			if err := memory.CreateCollaborationRun(context.Background(), store.CollaborationRun{ID: "run", Engine: model.CollaborationEngineNative, CreatedAt: now}); err != nil {
				t.Fatal(err)
			}
			if err := memory.FinishCollaborationRun(context.Background(), store.FinishCollaborationRunInput{RunID: "run", Status: test.status, StopReason: test.reason, CompletedAt: now}); err != nil {
				t.Fatalf("finish %s collaboration run: %v", test.status, err)
			}
		})
	}

	memory := &teststore.Store{CollaborationRuns: []store.CollaborationRun{
		{ID: "created", Status: model.CollaborationRunStatusCreated},
		{ID: "running", Status: model.CollaborationRunStatusRunning},
		{ID: "done", Status: model.CollaborationRunStatusSucceeded},
	}}
	completedAt := time.Now().UTC()
	count, err := memory.ReconcileActiveCollaborationRuns(context.Background(), completedAt)
	if err != nil || count != 2 {
		t.Fatalf("reconcile active collaboration runs: count=%d err=%v", count, err)
	}
	if memory.CollaborationRuns[0].Status != model.CollaborationRunStatusInterrupted || memory.CollaborationRuns[1].Status != model.CollaborationRunStatusInterrupted || memory.CollaborationRuns[2].Status != model.CollaborationRunStatusSucceeded {
		t.Fatalf("unexpected reconciliation result: %#v", memory.CollaborationRuns)
	}
}
