package collaboration_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/tests/teststore"
	"agentroom/backend/internal/collaboration"
)

func TestHandlerCreatesAgentRunsAndCommitsTurnMessages(t *testing.T) {
	memory := &teststore.Store{}
	handler, err := collaboration.NewTurnHandler(memory, testRequest())
	if err != nil {
		t.Fatal(err)
	}
	firstStarted := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	if _, committed, err := handler.Handle(context.Background(), turnEvent(collaboration.EventSpeakerSelected, "turn_1", "agent_1", firstStarted)); err != nil || committed {
		t.Fatalf("select first turn: committed=%v err=%v", committed, err)
	}
	if _, committed, err := handler.Handle(context.Background(), turnEvent(collaboration.EventAgentTurnStarted, "turn_1", "agent_1", firstStarted)); err != nil || committed {
		t.Fatalf("start first turn: committed=%v err=%v", committed, err)
	}

	firstCompleted := firstStarted.Add(time.Second)
	completed := turnEvent(collaboration.EventAgentMessageCompleted, "turn_1", "agent_1", firstCompleted)
	completed.Message = &collaboration.AgentMessage{
		Content: "analysis complete",
		Model: collaboration.ModelAudit{
			ModelSelectionID: "model_sel_1", ProfileID: "profile_1", Source: "database", ModelName: "model-a",
		},
		KnowledgeSources: []collaboration.KnowledgeSource{{DocumentID: "doc_1", DocumentName: "Plan", Scope: "room"}},
		Artifacts: []collaboration.Artifact{{
			ID: "artifact_1", Type: "report", Title: "Analysis", FileName: "analysis.md",
			MIMEType: "text/markdown", Content: []byte("# Result"),
		}},
	}
	first, committed, err := handler.Handle(context.Background(), completed)
	if err != nil || !committed {
		t.Fatalf("commit first turn: committed=%v err=%v", committed, err)
	}

	if len(memory.AgentRuns) != 1 {
		t.Fatalf("expected one Agent run, got %#v", memory.AgentRuns)
	}
	run := memory.AgentRuns[0]
	if run.RoomID != "room_1" || run.AgentID != "agent_1" || run.TriggerMessageID != "human_1" ||
		run.CollaborationRunID != "collab_1" || run.TurnIndex != 1 || run.ParentMessageID != "human_1" ||
		run.Status != "succeeded" || run.ModelProfileID != "profile_1" || run.ModelSource != "database" || run.ModelName != "model-a" {
		t.Fatalf("unexpected first Agent run: %#v", run)
	}
	if first.AgentRunID != run.ID || first.SenderID != "agent_1" || first.SenderName != "Analyst" ||
		first.SenderType != model.SenderTypeAgent || first.Content != "analysis complete" || first.TurnIndex != 1 ||
		first.ParentMessageID != "human_1" || !first.CreatedAt.Equal(firstCompleted) {
		t.Fatalf("unexpected first message: %#v", first)
	}
	wantSources := []model.MessageKnowledgeSource{{DocumentID: "doc_1", DocumentName: "Plan", Scope: "room"}}
	if !reflect.DeepEqual(first.KnowledgeSources, wantSources) {
		t.Fatalf("unexpected knowledge sources: %#v", first.KnowledgeSources)
	}
	wantArtifacts := []model.MessageArtifact{{
		ID: "artifact_1", Type: "report", Title: "Analysis", FileName: "analysis.md", MIMEType: "text/markdown", Content: "# Result",
	}}
	if !reflect.DeepEqual(first.Artifacts, wantArtifacts) {
		t.Fatalf("unexpected artifacts: %#v", first.Artifacts)
	}

	secondStarted := firstCompleted.Add(time.Second)
	if _, _, err := handler.Handle(context.Background(), turnEvent(collaboration.EventSpeakerSelected, "turn_2", "agent_2", secondStarted)); err != nil {
		t.Fatal(err)
	}
	secondCompleted := turnEvent(collaboration.EventAgentMessageCompleted, "turn_2", "agent_2", secondStarted.Add(time.Second))
	secondCompleted.Message = &collaboration.AgentMessage{Content: "decision recorded"}
	second, committed, err := handler.Handle(context.Background(), secondCompleted)
	if err != nil || !committed {
		t.Fatalf("commit second turn: committed=%v err=%v", committed, err)
	}
	if memory.AgentRuns[1].TurnIndex != 2 || memory.AgentRuns[1].ParentMessageID != first.ID || second.ParentMessageID != first.ID {
		t.Fatalf("expected second turn to follow first message, run=%#v message=%#v", memory.AgentRuns[1], second)
	}
}

func TestHandlerCompletingOneTurnTwiceReturnsCommittedMessage(t *testing.T) {
	memory := &teststore.Store{}
	handler, err := collaboration.NewTurnHandler(memory, testRequest())
	if err != nil {
		t.Fatal(err)
	}
	selected := turnEvent(collaboration.EventSpeakerSelected, "turn_1", "agent_1", time.Now().UTC())
	if _, _, err := handler.Handle(context.Background(), selected); err != nil {
		t.Fatal(err)
	}
	completed := turnEvent(collaboration.EventAgentMessageCompleted, "turn_1", "agent_1", time.Now().UTC())
	completed.Message = &collaboration.AgentMessage{Content: "only once"}
	first, _, err := handler.Handle(context.Background(), completed)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := handler.Handle(context.Background(), completed)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || len(memory.RoomMessages["room_1"]) != 1 || len(memory.AgentRuns) != 1 {
		t.Fatalf("expected one idempotent commit, first=%#v second=%#v", first, second)
	}
}

func TestHandlerCommitFailureLeavesTurnRunningAndReturnsNoMessage(t *testing.T) {
	commitErr := errors.New("commit unavailable")
	memory := &teststore.Store{CommitAgentRunErr: commitErr}
	handler, err := collaboration.NewTurnHandler(memory, testRequest())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := handler.Handle(context.Background(), turnEvent(collaboration.EventSpeakerSelected, "turn_1", "agent_1", time.Now().UTC())); err != nil {
		t.Fatal(err)
	}
	completed := turnEvent(collaboration.EventAgentMessageCompleted, "turn_1", "agent_1", time.Now().UTC())
	completed.Message = &collaboration.AgentMessage{Content: "not persisted"}
	message, committed, err := handler.Handle(context.Background(), completed)
	if !errors.Is(err, commitErr) || committed || message.ID != "" {
		t.Fatalf("expected commit error without a message, message=%#v committed=%v err=%v", message, committed, err)
	}
	if memory.AgentRuns[0].Status != "failed" || memory.AgentRuns[0].Error != "Agent message commit failed" || len(memory.RoomMessages["room_1"]) != 0 {
		t.Fatalf("expected transaction rollback, run=%#v messages=%#v", memory.AgentRuns[0], memory.RoomMessages["room_1"])
	}
}

func TestHandlerRejectsCompletionWithoutCreatedTurn(t *testing.T) {
	handler, err := collaboration.NewTurnHandler(&teststore.Store{}, testRequest())
	if err != nil {
		t.Fatal(err)
	}
	completed := turnEvent(collaboration.EventAgentMessageCompleted, "turn_missing", "agent_1", time.Now().UTC())
	completed.Message = &collaboration.AgentMessage{Content: "invalid"}
	if _, _, err := handler.Handle(context.Background(), completed); !errors.Is(err, collaboration.ErrInvalidEvent) {
		t.Fatalf("expected invalid event error, got %v", err)
	}
}

func testRequest() collaboration.Request {
	return collaboration.Request{
		ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: "collab_1",
		Snapshot: collaboration.ConversationSnapshot{
			Room:    collaboration.RoomSnapshot{ID: "room_1"},
			Trigger: collaboration.MessageSnapshot{ID: "human_1"},
			Agents: []collaboration.AgentSnapshot{
				{ID: "agent_1", Name: "Analyst"},
				{ID: "agent_2", Name: "Recorder"},
			},
		},
	}
}

func turnEvent(kind collaboration.EventKind, turnID string, agentID string, occurredAt time.Time) collaboration.Event {
	return collaboration.Event{
		ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: "collab_1",
		Kind: kind, TurnID: turnID, AgentID: agentID, OccurredAt: occurredAt,
	}
}
