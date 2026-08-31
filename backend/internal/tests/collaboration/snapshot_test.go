package collaboration_test

import (
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/collaboration"
)

func TestBuildCreatesOrderedAuthoritativeSnapshot(t *testing.T) {
	input := validInput()
	request, err := collaboration.Build(input)
	if err != nil {
		t.Fatal(err)
	}

	if request.ProtocolVersion != collaboration.ProtocolVersion || request.Engine != collaboration.EngineNative {
		t.Fatalf("unexpected request envelope: %#v", request)
	}
	if request.Snapshot.Room.Status != model.RoomStatusActive {
		t.Fatalf("expected default active room status, got %q", request.Snapshot.Room.Status)
	}
	if len(request.Snapshot.Agents) != 2 || request.Snapshot.Agents[0].ID != "agent_1" || request.Snapshot.Agents[1].ID != "agent_2" {
		t.Fatalf("Agent order changed: %#v", request.Snapshot.Agents)
	}
	if request.Snapshot.Agents[0].Runtime != model.AgentRuntimeLLM || request.Snapshot.Agents[0].ModelReferenceID != "model_1" {
		t.Fatalf("unexpected Agent snapshot: %#v", request.Snapshot.Agents[0])
	}
	if len(request.Snapshot.ModelReferences) != 1 {
		t.Fatalf("shared model reference was not deduplicated: %#v", request.Snapshot.ModelReferences)
	}
	if got := request.Snapshot.InitialCandidateAgentIDs; len(got) != 2 || got[0] != "agent_2" || got[1] != "agent_1" {
		t.Fatalf("candidate order or deduplication changed: %#v", got)
	}
	if request.Snapshot.Policy.Version != model.CollaborationPolicyVersion || !request.Snapshot.Policy.StopOnEmptyOutput || !request.Snapshot.Policy.StopOnRepeatedOutput {
		t.Fatalf("unexpected policy snapshot: %#v", request.Snapshot.Policy)
	}
	if request.Snapshot.Trigger.SenderType != collaboration.SenderHuman || request.Snapshot.Transcript[0].SenderType != collaboration.SenderAgent {
		t.Fatal("message sender types were not mapped")
	}
	if request.Snapshot.Transcript[0].CollaborationRunID != "" {
		t.Fatal("legacy dialogue run ID must not be relabeled as a collaboration run ID")
	}
	if request.Checkpoint == nil || string(request.Checkpoint.Payload) != "state" {
		t.Fatalf("checkpoint was not copied: %#v", request.Checkpoint)
	}
}

func TestBuildDetachesSnapshotFromMutableInputs(t *testing.T) {
	input := validInput()
	request, err := collaboration.Build(input)
	if err != nil {
		t.Fatal(err)
	}

	input.Agents[0].Agent.Name = "changed"
	input.Agents[0].ToolNames[0] = "changed"
	input.Agents[0].ModelReference.ModelName = "changed"
	input.Transcript[0].Content = "changed"
	input.KnowledgeChunks[0].Content = "changed"
	input.InitialCandidateAgentIDs[0] = "changed"
	input.Checkpoint.Payload[0] = 'X'

	if request.Snapshot.Agents[0].Name != "Architect" || request.Snapshot.Agents[0].ToolNames[0] != "search" {
		t.Fatalf("Agent snapshot changed with input: %#v", request.Snapshot.Agents[0])
	}
	if request.Snapshot.ModelReferences[0].ModelName != "test-model" {
		t.Fatalf("model reference changed with input: %#v", request.Snapshot.ModelReferences[0])
	}
	if request.Snapshot.Transcript[0].Content != "Earlier" || request.Snapshot.KnowledgeChunks[0].Content != "Knowledge" {
		t.Fatal("transcript or knowledge snapshot changed with input")
	}
	if request.Snapshot.InitialCandidateAgentIDs[0] != "agent_2" || string(request.Checkpoint.Payload) != "state" {
		t.Fatal("candidate or checkpoint snapshot changed with input")
	}
}

func TestBuildRejectsInvalidAuthorityInputs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*collaboration.Input)
	}{
		{name: "no eligible Agents", mutate: func(input *collaboration.Input) { input.Agents = nil }},
		{name: "disabled Agent", mutate: func(input *collaboration.Input) { input.Agents[0].Agent.Enabled = false }},
		{name: "duplicate Agent", mutate: func(input *collaboration.Input) { input.Agents[1].Agent.ID = "agent_1" }},
		{name: "missing model metadata", mutate: func(input *collaboration.Input) { input.Agents[0].ModelReference.Protocol = "" }},
		{name: "candidate outside snapshot", mutate: func(input *collaboration.Input) { input.InitialCandidateAgentIDs = []string{"agent_3"} }},
		{name: "non-human trigger", mutate: func(input *collaboration.Input) { input.Trigger.SenderType = model.SenderTypeAgent }},
		{name: "negative transcript turn", mutate: func(input *collaboration.Input) { input.Transcript[0].TurnIndex = -1 }},
		{name: "invalid policy", mutate: func(input *collaboration.Input) { input.Policy.MaxTurns = -1 }},
		{name: "invalid limits", mutate: func(input *collaboration.Input) { input.Limits.MaxToolSteps = 0 }},
		{name: "checkpoint Engine mismatch", mutate: func(input *collaboration.Input) { input.Checkpoint.Engine = collaboration.EngineAutoGen }},
		{name: "checkpoint digest mismatch", mutate: func(input *collaboration.Input) { input.Checkpoint.SHA256 = "wrong" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validInput()
			test.mutate(&input)
			if _, err := collaboration.Build(input); err == nil {
				t.Fatal("expected snapshot build to fail")
			}
		})
	}
}

func validInput() collaboration.Input {
	payload := []byte("state")
	reference := collaboration.ModelReference{
		ID: "model_1", ProfileID: "profile_1", Source: "database",
		Protocol: model.ModelProtocolOpenAIChatCompletions, ModelName: "test-model", RuntimeScope: model.ModelRuntimeGo,
	}
	return collaboration.Input{
		CollaborationRunID: "collab_1", TraceID: "trace_1",
		Room: model.RoomMeta{ID: "room_1", Name: "Planning"},
		Agents: []collaboration.AgentBinding{
			{Agent: model.Agent{ID: "agent_1", Name: "Architect", Runtime: "", Enabled: true}, ToolNames: []string{"search"}, ModelReference: reference},
			{Agent: model.Agent{ID: "agent_2", Name: "Reviewer", Runtime: model.AgentRuntimeLLM, Enabled: true}, ModelReference: reference},
		},
		Trigger: model.Message{
			ID: "message_1", SenderID: "user_1", SenderName: "Alice", SenderType: model.SenderTypeHuman,
			Content: "Plan this", CreatedAt: time.Now().UTC(),
		},
		Transcript: []model.Message{{
			ID: "message_0", SenderID: "agent_2", SenderName: "Reviewer", SenderType: model.SenderTypeAgent,
			Content: "Earlier", CreatedAt: time.Now().UTC(), DialogueRunID: "legacy_dialogue_1", TurnIndex: 1,
		}},
		KnowledgeChunks: []model.KnowledgeChunk{{
			ID: "chunk_1", DocumentID: "doc_1", DocumentName: "Plan", Scope: model.KnowledgeScopeRoom,
			ScopeID: "room_1", ChunkIndex: 0, Content: "Knowledge",
		}},
		Policy: model.CollaborationPolicy{
			Engine: model.CollaborationEngineNative, TriggerMode: model.CollaborationTriggerMentionOnly,
			MaxTurns: 3, MaxTurnsPerAgent: 2, AllowAgentHandoff: true, CooldownMS: 5,
		},
		Limits: collaboration.ExecutionLimits{
			Timeout: time.Second, MaxOutputBytes: 1024, MaxArtifactBytes: 1024, MaxToolSteps: 4,
			MaxRequestBytes: 4096, MaxEventBytes: 4096, MaxCheckpointBytes: 1024,
		},
		InitialCandidateAgentIDs: []string{"agent_2", "agent_1", "agent_2"},
		Checkpoint: &collaboration.Checkpoint{
			Engine: collaboration.EngineNative, EngineVersion: "native-v1", FormatVersion: "1",
			SHA256: fmt.Sprintf("%x", sha256.Sum256(payload)), SizeBytes: uint64(len(payload)), Payload: payload,
		},
	}
}
