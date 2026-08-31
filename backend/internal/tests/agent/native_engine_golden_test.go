package agent_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

type nativeGoldenSuite struct {
	Cases []nativeGoldenCase `json:"cases"`
}

type nativeGoldenCase struct {
	Name                     string                  `json:"name"`
	Trigger                  string                  `json:"trigger"`
	Agents                   []nativeGoldenAgent     `json:"agents"`
	InitialCandidateAgentIDs []string                `json:"initial_candidate_agent_ids"`
	Policy                   nativeGoldenPolicy      `json:"policy"`
	Responses                []nativeGoldenResponse  `json:"responses"`
	Expected                 nativeGoldenExpectation `json:"expected"`
}

type nativeGoldenAgent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type nativeGoldenPolicy struct {
	MaxTurns         int `json:"max_turns"`
	MaxTurnsPerAgent int `json:"max_turns_per_agent"`
}

type nativeGoldenModel struct {
	ProfileID string `json:"profile_id"`
	Source    string `json:"source"`
	ModelName string `json:"model_name"`
}

type nativeGoldenResponse struct {
	AgentID             string            `json:"agent_id"`
	Content             string            `json:"content"`
	ActivityText        string            `json:"activity_text"`
	ArtifactID          string            `json:"artifact_id"`
	KnowledgeDocumentID string            `json:"knowledge_document_id"`
	Model               nativeGoldenModel `json:"model"`
}

type nativeGoldenExpectation struct {
	SpeakerIDs []string              `json:"speaker_ids"`
	Messages   []nativeGoldenMessage `json:"messages"`
	Terminal   nativeGoldenTerminal  `json:"terminal"`
}

type nativeGoldenMessage struct {
	AgentID             string            `json:"agent_id"`
	Content             string            `json:"content"`
	ArtifactID          string            `json:"artifact_id"`
	KnowledgeDocumentID string            `json:"knowledge_document_id"`
	Model               nativeGoldenModel `json:"model"`
}

type nativeGoldenTerminal struct {
	Kind      string `json:"kind"`
	Reason    string `json:"reason"`
	TurnCount int    `json:"turn_count"`
}

func TestLegacyGoRunnerMatchesNativeEngineGoldenScenarios(t *testing.T) {
	payload, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "proto", "collaboration_runtime", "v1", "testdata", "native_engine_golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var suite nativeGoldenSuite
	if err := json.Unmarshal(payload, &suite); err != nil {
		t.Fatal(err)
	}

	for _, scenario := range suite.Cases {
		scenario := scenario
		t.Run(scenario.Name, func(t *testing.T) {
			responses := &nativeGoldenRuntime{t: t, responses: scenario.Responses}
			backingStore := &dialogueStore{}
			runner := agent.NewRunner(nil, backingStore).WithRuntimeRegistry(agent.NewRuntimeRegistry(responses))
			agents := make([]model.Agent, 0, len(scenario.Agents))
			for _, item := range scenario.Agents {
				agents = append(agents, model.Agent{
					ID: item.ID, Name: item.Name, Mention: "@" + item.Name,
					Role: item.Name + " role", Runtime: model.AgentRuntimeDeepAgent,
					SystemPrompt: "You are " + item.Name + ".", Enabled: true,
				})
			}
			room := newDialogueRuntimeRoom(model.DialoguePolicy{
				Mode: model.DialogueModeGuided, MaxAutonomousTurns: scenario.Policy.MaxTurns,
				MaxTurnsPerAgent:          scenario.Policy.MaxTurnsPerAgent,
				AllowAgentToAgentMentions: true,
				ResponseStrategy:          model.DialogueResponseStrategyMentionedFirst,
			}, agents)
			trigger := room.newHumanMessage("Alice", scenario.Trigger)
			room.AppendMessage(trigger)

			runner.HandleHumanMessage(context.Background(), room, trigger)

			if !equalStrings(responses.speakers, scenario.Expected.SpeakerIDs) {
				t.Fatalf("speaker order mismatch: got %v want %v", responses.speakers, scenario.Expected.SpeakerIDs)
			}
			assertGoldenMessages(t, room.agentMessages(), backingStore.AgentRuns, scenario.Expected.Messages)
			if len(backingStore.dialogueRuns) != 1 {
				t.Fatalf("expected one dialogue run, got %#v", backingStore.dialogueRuns)
			}
			kind, reason := legacyTerminal(backingStore.dialogueRuns[0].Status)
			if kind != scenario.Expected.Terminal.Kind || reason != scenario.Expected.Terminal.Reason || backingStore.dialogueRuns[0].TurnCount != scenario.Expected.Terminal.TurnCount {
				t.Fatalf("terminal mismatch: got %s/%s/%d want %#v", kind, reason, backingStore.dialogueRuns[0].TurnCount, scenario.Expected.Terminal)
			}
			for _, response := range scenario.Responses {
				if response.ActivityText == "" {
					continue
				}
				for _, message := range room.agentMessages() {
					if message.Content == response.ActivityText {
						t.Fatalf("runtime activity became a chat message: %q", response.ActivityText)
					}
				}
			}
		})
	}
}

type nativeGoldenRuntime struct {
	t         *testing.T
	responses []nativeGoldenResponse
	calls     int
	speakers  []string
}

func (r *nativeGoldenRuntime) Name() string { return model.AgentRuntimeDeepAgent }

func (r *nativeGoldenRuntime) Respond(ctx context.Context, request agent.AgentRuntimeRequest, observers ...agent.AgentEventObserver) (agent.AgentRuntimeResponse, error) {
	r.t.Helper()
	if r.calls >= len(r.responses) {
		r.t.Fatalf("unexpected response call %d", r.calls+1)
	}
	response := r.responses[r.calls]
	r.calls++
	r.speakers = append(r.speakers, request.Agent.ID)
	if request.Agent.ID != response.AgentID {
		r.t.Fatalf("response agent mismatch: got %s want %s", request.Agent.ID, response.AgentID)
	}
	if response.ActivityText != "" && len(observers) > 0 {
		observers[0].ObserveAgentEvent(ctx, agent.AgentRuntimeEvent{
			RunID: request.RunID, Kind: "output_delta", OccurredAt: time.Now().UTC(),
		})
	}
	result := agent.AgentRuntimeResponse{
		Content: response.Content,
		Metadata: map[string]string{
			"model_profile_id": response.Model.ProfileID,
			"model_source":     response.Model.Source,
			"model_name":       response.Model.ModelName,
		},
	}
	if response.ArtifactID != "" {
		result.Artifacts = []agent.AgentRuntimeArtifact{{
			ID: response.ArtifactID, Type: "markdown_report", Title: "Report",
			FileName: response.ArtifactID + ".md", MIMEType: "text/markdown",
			Content: "# " + response.ArtifactID,
		}}
	}
	if response.KnowledgeDocumentID != "" {
		result.KnowledgeSources = []model.MessageKnowledgeSource{{
			DocumentID:   response.KnowledgeDocumentID,
			DocumentName: response.KnowledgeDocumentID + ".md",
			Scope:        "room",
		}}
	}
	return result, nil
}

func assertGoldenMessages(t *testing.T, messages []model.Message, runs []store.AgentRun, expected []nativeGoldenMessage) {
	t.Helper()
	if len(messages) != len(expected) {
		t.Fatalf("message count mismatch: got %#v want %#v", messages, expected)
	}
	for index, want := range expected {
		got := messages[index]
		if got.SenderID != want.AgentID || got.Content != want.Content {
			t.Fatalf("message %d mismatch: got %#v want %#v", index, got, want)
		}
		if firstMessageArtifactID(got) != want.ArtifactID || firstKnowledgeDocumentID(got) != want.KnowledgeDocumentID {
			t.Fatalf("message %d attachment audit mismatch: got %#v want %#v", index, got, want)
		}
		run, ok := agentRunByID(runs, got.AgentRunID)
		if !ok || run.ModelProfileID != want.Model.ProfileID || run.ModelSource != want.Model.Source || run.ModelName != want.Model.ModelName {
			t.Fatalf("message %d model audit mismatch: got %#v want %#v", index, run, want.Model)
		}
	}
}

func legacyTerminal(status string) (string, string) {
	switch status {
	case model.DialogueRunStatusSucceeded:
		return "completed", "completed"
	case model.DialogueRunStatusStoppedDuplicate:
		return "stopped", "duplicate_output"
	case model.DialogueRunStatusStoppedEmpty:
		return "stopped", "empty_output"
	case model.DialogueRunStatusStoppedLimit:
		return "stopped", "max_turns"
	default:
		return "failed", "engine_failure"
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func firstMessageArtifactID(message model.Message) string {
	if len(message.Artifacts) == 0 {
		return ""
	}
	return message.Artifacts[0].ID
}

func firstKnowledgeDocumentID(message model.Message) string {
	if len(message.KnowledgeSources) == 0 {
		return ""
	}
	return message.KnowledgeSources[0].DocumentID
}

func agentRunByID(runs []store.AgentRun, id string) (store.AgentRun, bool) {
	for _, run := range runs {
		if run.ID == id {
			return run, true
		}
	}
	return store.AgentRun{}, false
}
