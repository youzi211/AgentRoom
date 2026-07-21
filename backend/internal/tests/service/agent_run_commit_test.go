package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
	"agentroom/backend/internal/tests/teststore"
)

func TestCommitAgentRunSuccessIsIdempotentAndLinksOneMessage(t *testing.T) {
	memory := &teststore.Store{}
	run := store.AgentRun{ID: "run_1", RoomID: "room_1", Status: "running", StartedAt: time.Now().UTC()}
	if err := memory.CreateAgentRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	completedAt := time.Now().UTC()
	input := store.CommitAgentRunSuccessInput{
		RunID: "run_1", CompletedAt: completedAt,
		Message:        model.Message{ID: "message_1", RoomID: "room_1", Content: "done", CreatedAt: completedAt},
		ModelProfileID: "profile_1", ModelSource: "database", ModelName: "test-model",
	}
	first, err := memory.CommitAgentRunSuccess(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	input.Message.ID = "message_duplicate"
	second, err := memory.CommitAgentRunSuccess(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.AgentRunID != "run_1" {
		t.Fatalf("expected the existing committed message, got %#v and %#v", first, second)
	}
	if len(memory.RoomMessages["room_1"]) != 1 {
		t.Fatalf("expected exactly one final message, got %#v", memory.RoomMessages["room_1"])
	}
	if memory.AgentRuns[0].Status != "succeeded" || memory.AgentRuns[0].ModelName != "test-model" {
		t.Fatalf("expected terminal run and model audit, got %#v", memory.AgentRuns[0])
	}
}

func TestAgentRunCompletionAndCancellationRaceHasOneWinner(t *testing.T) {
	memory := &teststore.Store{}
	_ = memory.CreateAgentRun(context.Background(), store.AgentRun{ID: "run_race", RoomID: "room_1", Status: "running", StartedAt: time.Now().UTC()})
	now := time.Now().UTC()
	start := make(chan struct{})
	errorsSeen := make(chan error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		_, err := memory.CommitAgentRunSuccess(context.Background(), store.CommitAgentRunSuccessInput{
			RunID: "run_race", CompletedAt: now,
			Message: model.Message{ID: "message_race", RoomID: "room_1", CreatedAt: now},
		})
		errorsSeen <- err
	}()
	go func() {
		defer wait.Done()
		<-start
		errorsSeen <- memory.FinishAgentRun(context.Background(), "run_race", "canceled", "cancelled", now)
	}()
	close(start)
	wait.Wait()
	close(errorsSeen)

	successes := 0
	alreadyFinished := 0
	for err := range errorsSeen {
		if err == nil {
			successes++
		} else if errors.Is(err, store.ErrAgentRunAlreadyFinished) {
			alreadyFinished++
		} else {
			t.Fatalf("unexpected race error: %v", err)
		}
	}
	if successes != 1 || alreadyFinished != 1 {
		t.Fatalf("expected one terminal winner, got successes=%d alreadyFinished=%d", successes, alreadyFinished)
	}
	if status := memory.AgentRuns[0].Status; status != "succeeded" && status != "canceled" {
		t.Fatalf("unexpected terminal status %q", status)
	}
}

func TestCommitAgentRunSuccessFailureRollsBackMessageAndRun(t *testing.T) {
	memory := &teststore.Store{CommitAgentRunErr: errors.New("commit unavailable")}
	_ = memory.CreateAgentRun(context.Background(), store.AgentRun{ID: "run_rollback", RoomID: "room_1", Status: "running"})
	_, err := memory.CommitAgentRunSuccess(context.Background(), store.CommitAgentRunSuccessInput{
		RunID:       "run_rollback",
		Message:     model.Message{ID: "message_rollback", RoomID: "room_1"},
		CompletedAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected commit failure")
	}
	if memory.AgentRuns[0].Status != "running" || len(memory.RoomMessages["room_1"]) != 0 {
		t.Fatalf("expected atomic rollback, got run=%#v messages=%#v", memory.AgentRuns[0], memory.RoomMessages["room_1"])
	}
}

func TestReconcileActiveAgentRunsOnlyInterruptsRunningRows(t *testing.T) {
	memory := &teststore.Store{AgentRuns: []store.AgentRun{
		{ID: "running", Status: "running"},
		{ID: "succeeded", Status: "succeeded"},
	}}
	count, err := memory.ReconcileActiveAgentRuns(context.Background(), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || memory.AgentRuns[0].Status != "interrupted" || memory.AgentRuns[1].Status != "succeeded" {
		t.Fatalf("unexpected reconciliation: %#v", memory.AgentRuns)
	}
}

func TestLegacyMessageWithoutAgentRunReferenceRemainsValid(t *testing.T) {
	legacy := model.Message{ID: "legacy", RoomID: "room_1", Content: "old message"}
	if legacy.AgentRunID != "" {
		t.Fatalf("legacy message must default to no Agent Run reference: %#v", legacy)
	}
}
