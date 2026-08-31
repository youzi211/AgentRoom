package collaborationrun_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"agentroom/backend/internal/collaboration"
	"agentroom/backend/internal/collaborationevent"
	"agentroom/backend/internal/collaborationgrpc"
	"agentroom/backend/internal/collaborationrun"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
	"agentroom/backend/internal/tests/teststore"
)

func TestLifecycleMapsAllRemoteTerminalStates(t *testing.T) {
	tests := []struct {
		name       string
		kind       collaboration.EventKind
		reason     collaboration.StopReason
		failure    *collaboration.Failure
		wantStatus string
		wantReason string
	}{
		{name: "success", kind: collaboration.EventCompleted, reason: collaboration.StopReasonCompleted, wantStatus: model.CollaborationRunStatusSucceeded, wantReason: model.CollaborationStopReasonCompleted},
		{name: "stopped", kind: collaboration.EventStopped, reason: collaboration.StopReasonMaxTurns, wantStatus: model.CollaborationRunStatusStopped, wantReason: model.CollaborationStopReasonMaxTurns},
		{name: "failed", kind: collaboration.EventFailed, reason: collaboration.StopReasonEngineFailure, failure: &collaboration.Failure{Code: collaboration.ErrorModelTimeout, Message: "provider secret"}, wantStatus: model.CollaborationRunStatusFailed, wantReason: model.CollaborationStopReasonEngineFailure},
		{name: "cancelled", kind: collaboration.EventCancelled, reason: collaboration.StopReasonCancelled, wantStatus: model.CollaborationRunStatusCancelled, wantReason: model.CollaborationStopReasonCancelled},
		{name: "timeout", kind: collaboration.EventCancelled, reason: collaboration.StopReasonDeadlineExceeded, wantStatus: model.CollaborationRunStatusTimeout, wantReason: model.CollaborationStopReasonDeadlineExceeded},
		{name: "interrupted", kind: collaboration.EventCancelled, reason: collaboration.StopReasonInterrupted, wantStatus: model.CollaborationRunStatusInterrupted, wantReason: model.CollaborationStopReasonInterrupted},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			memory, lifecycle := startedLifecycle(t)
			completed := lifecycleEvent(collaboration.EventAgentMessageCompleted, time.Now().UTC())
			completed.TurnID = "turn_1"
			if err := lifecycle.Handle(context.Background(), completed); err != nil {
				t.Fatal(err)
			}
			terminal := lifecycleEvent(test.kind, time.Now().UTC())
			terminal.Terminal = &collaboration.Terminal{TurnCount: 1, Reason: test.reason, Failure: test.failure}
			if err := lifecycle.Handle(context.Background(), terminal); err != nil {
				t.Fatal(err)
			}
			run := memory.CollaborationRuns[0]
			if run.Status != test.wantStatus || run.StopReason != test.wantReason || run.TurnCount != 1 || run.CompletedAt == nil {
				t.Fatalf("unexpected terminal run: %#v", run)
			}
			if test.failure != nil && (run.Error == "" || run.Error == test.failure.Message) {
				t.Fatalf("expected classified failure without raw message, got %q", run.Error)
			}
		})
	}
}

func TestLifecycleConvergesExecutionErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus string
		wantReason string
	}{
		{name: "cancelled", err: context.Canceled, wantStatus: model.CollaborationRunStatusCancelled, wantReason: model.CollaborationStopReasonCancelled},
		{name: "timeout", err: context.DeadlineExceeded, wantStatus: model.CollaborationRunStatusTimeout, wantReason: model.CollaborationStopReasonDeadlineExceeded},
		{name: "protocol", err: fmt.Errorf("validate: %w", collaborationevent.ErrProtocol), wantStatus: model.CollaborationRunStatusFailed, wantReason: model.CollaborationStopReasonProtocolError},
		{name: "transport protocol", err: collaborationgrpc.ErrProtocol, wantStatus: model.CollaborationRunStatusFailed, wantReason: model.CollaborationStopReasonProtocolError},
		{name: "interrupted", err: collaborationgrpc.ErrUnavailable, wantStatus: model.CollaborationRunStatusInterrupted, wantReason: model.CollaborationStopReasonInterrupted},
		{name: "engine failure", err: errors.New("provider secret"), wantStatus: model.CollaborationRunStatusFailed, wantReason: model.CollaborationStopReasonEngineFailure},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			memory, lifecycle := startedLifecycle(t)
			cancelledCtx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := lifecycle.Converge(cancelledCtx, test.err); err != nil {
				t.Fatal(err)
			}
			run := memory.CollaborationRuns[0]
			if run.Status != test.wantStatus || run.StopReason != test.wantReason || run.CompletedAt == nil {
				t.Fatalf("unexpected converged run: %#v", run)
			}
			if run.Error == "provider secret" {
				t.Fatal("raw execution error entered the collaboration audit")
			}
		})
	}
}

func TestLifecycleTerminalCommitAndConvergenceAreIdempotent(t *testing.T) {
	memory, lifecycle := startedLifecycle(t)
	terminal := lifecycleEvent(collaboration.EventCompleted, time.Now().UTC())
	terminal.Terminal = &collaboration.Terminal{Reason: collaboration.StopReasonCompleted}
	if err := lifecycle.Handle(context.Background(), terminal); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.Handle(context.Background(), terminal); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.Converge(context.Background(), errors.New("late stream error")); err != nil {
		t.Fatal(err)
	}
	if run := memory.CollaborationRuns[0]; run.Status != model.CollaborationRunStatusSucceeded || run.StopReason != model.CollaborationStopReasonCompleted {
		t.Fatalf("terminal state changed after convergence: %#v", run)
	}
}

func startedLifecycle(t *testing.T) (*teststore.Store, *collaborationrun.Lifecycle) {
	t.Helper()
	memory := &teststore.Store{}
	now := time.Now().UTC()
	if err := memory.CreateCollaborationRun(context.Background(), store.CollaborationRun{
		ID: "collab_1", RoomID: "room_1", RootMessageID: "human_1", Engine: model.CollaborationEngineNative, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	lifecycle, err := collaborationrun.New(memory, collaboration.Request{CollaborationRunID: "collab_1"}, "native-v1")
	if err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.Handle(context.Background(), lifecycleEvent(collaboration.EventAccepted, now.Add(time.Second))); err != nil {
		t.Fatal(err)
	}
	run := memory.CollaborationRuns[0]
	if run.Status != model.CollaborationRunStatusRunning || run.EngineVersion != "native-v1" || run.StartedAt == nil {
		t.Fatalf("unexpected started run: %#v", run)
	}
	return memory, lifecycle
}

func lifecycleEvent(kind collaboration.EventKind, occurredAt time.Time) collaboration.Event {
	return collaboration.Event{CollaborationRunID: "collab_1", Kind: kind, OccurredAt: occurredAt}
}
