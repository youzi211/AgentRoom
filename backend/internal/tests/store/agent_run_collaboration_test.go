package store_test

import (
	"context"
	"testing"

	"agentroom/backend/internal/store"
	"agentroom/backend/internal/tests/teststore"
)

func TestAgentRunCollaborationFieldsRemainOptionalAndRoundTrip(t *testing.T) {
	memory := &teststore.Store{}
	legacy := store.AgentRun{ID: "run_legacy", Status: "running"}
	collaborationTurn := store.AgentRun{
		ID:                 "run_turn_2",
		CollaborationRunID: "collaboration_1",
		TurnIndex:          2,
		ParentMessageID:    "message_1",
		Status:             "running",
	}

	if err := memory.CreateAgentRun(context.Background(), legacy); err != nil {
		t.Fatalf("create legacy Agent run: %v", err)
	}
	if err := memory.CreateAgentRun(context.Background(), collaborationTurn); err != nil {
		t.Fatalf("create collaboration Agent run: %v", err)
	}

	if memory.AgentRuns[0].CollaborationRunID != "" || memory.AgentRuns[0].TurnIndex != 0 || memory.AgentRuns[0].ParentMessageID != "" {
		t.Fatalf("expected legacy Agent run collaboration fields to remain empty, got %#v", memory.AgentRuns[0])
	}
	if got := memory.AgentRuns[1]; got.CollaborationRunID != "collaboration_1" || got.TurnIndex != 2 || got.ParentMessageID != "message_1" {
		t.Fatalf("expected collaboration Agent run fields to round-trip, got %#v", got)
	}
}
