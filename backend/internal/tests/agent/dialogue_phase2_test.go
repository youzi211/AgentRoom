package agent_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
	"agentroom/backend/internal/tests/teststore"
)

func TestGuidedDialogueMentionedAgentsReplyFirst(t *testing.T) {
	llmClient := &sequenceLLM{
		responses: []string{
			"@Architect I think we should validate the rollout order.",
			"I reviewed the current plan and agree with the ordering.",
			"Here is the architecture follow-up.",
		},
	}
	store := &dialogueStore{}
	runner := agent.NewRunner(llmClient, store)
	room := newDialogueRuntimeRoom(model.DialoguePolicy{
		Mode:                      model.DialogueModeGuided,
		MaxAutonomousTurns:        4,
		MaxTurnsPerAgent:          2,
		AllowSelfFollowup:         false,
		AllowAgentToAgentMentions: true,
		ResponseStrategy:          model.DialogueResponseStrategyMentionedFirst,
	}, []model.Agent{
		testAgent("alpha", "Analyst"),
		testAgent("beta", "Builder"),
		testAgent("architect", "Architect"),
	})

	trigger := room.newHumanMessage("Alice", "@Builder @Analyst please discuss the rollout sequence.")
	room.AppendMessage(trigger)

	runner.HandleHumanMessage(context.Background(), room, trigger)

	got := room.agentMessages()
	if len(got) != 3 {
		t.Fatalf("expected 3 guided dialogue replies, got %#v", got)
	}
	if got[0].SenderID != "beta" || got[1].SenderID != "alpha" || got[2].SenderID != "architect" {
		t.Fatalf("expected Builder, Analyst, then Architect replies, got %#v", got)
	}
	if got[0].DialogueRunID == "" {
		t.Fatal("expected guided dialogue replies to include a dialogue run id")
	}
	if got[0].DialogueRunID != got[1].DialogueRunID || got[1].DialogueRunID != got[2].DialogueRunID {
		t.Fatalf("expected all guided dialogue replies to share one dialogue run id, got %#v", got)
	}
	if got[0].TurnIndex != 1 || got[1].TurnIndex != 2 || got[2].TurnIndex != 3 {
		t.Fatalf("expected turn indexes 1,2,3; got %#v", got)
	}
	if got[0].ParentMessageID != trigger.ID || got[1].ParentMessageID != got[0].ID || got[2].ParentMessageID != got[1].ID {
		t.Fatalf("expected parent message chain trigger->1->2, got %#v", got)
	}
	if len(store.dialogueRuns) != 1 {
		t.Fatalf("expected one dialogue run record, got %#v", store.dialogueRuns)
	}
	if store.dialogueRuns[0].Status != model.DialogueRunStatusSucceeded {
		t.Fatalf("expected successful dialogue run, got %#v", store.dialogueRuns[0])
	}
	if store.dialogueRuns[0].TurnCount != 3 {
		t.Fatalf("expected dialogue run turn count 3, got %#v", store.dialogueRuns[0])
	}
}

func TestGuidedDialogueFollowsNormalizedGeneratedMentions(t *testing.T) {
	llmClient := &sequenceLLM{
		responses: []string{
			"请 ＠Reviewer 和 @ Architect 一起补充评审意见。",
			"Reviewer follow-up.",
			"Architect follow-up.",
		},
	}
	store := &dialogueStore{}
	runner := agent.NewRunner(llmClient, store)
	room := newDialogueRuntimeRoom(model.DialoguePolicy{
		Mode:                      model.DialogueModeGuided,
		MaxAutonomousTurns:        3,
		MaxTurnsPerAgent:          1,
		AllowSelfFollowup:         false,
		AllowAgentToAgentMentions: true,
		ResponseStrategy:          model.DialogueResponseStrategyMentionedFirst,
	}, []model.Agent{
		testAgent("author", "Author"),
		testAgent("reviewer", "Reviewer"),
		testAgent("architect", "Architect"),
	})

	trigger := room.newHumanMessage("Alice", "@Author please start the review.")
	room.AppendMessage(trigger)

	runner.HandleHumanMessage(context.Background(), room, trigger)

	got := room.agentMessages()
	if len(got) != 3 {
		t.Fatalf("expected author handoff to produce 3 replies, got %#v", got)
	}
	if got[0].SenderID != "author" || got[1].SenderID != "reviewer" || got[2].SenderID != "architect" {
		t.Fatalf("expected Author, Reviewer, then Architect replies, got %#v", got)
	}
}

func TestGuidedDialogueRespectsMaxAutonomousTurns(t *testing.T) {
	llmClient := &sequenceLLM{
		responses: []string{
			"@Reviewer First pass from Product.",
			"@Product Follow-up from Reviewer.",
			"@Reviewer This third autonomous turn should never be emitted.",
		},
	}
	store := &dialogueStore{}
	runner := agent.NewRunner(llmClient, store)
	room := newDialogueRuntimeRoom(model.DialoguePolicy{
		Mode:                      model.DialogueModeGuided,
		MaxAutonomousTurns:        2,
		MaxTurnsPerAgent:          3,
		AllowSelfFollowup:         false,
		AllowAgentToAgentMentions: true,
		ResponseStrategy:          model.DialogueResponseStrategyMentionedFirst,
	}, []model.Agent{
		testAgent("product", "Product"),
		testAgent("reviewer", "Reviewer"),
	})

	trigger := room.newHumanMessage("Alice", "@Product start the review.")
	room.AppendMessage(trigger)

	runner.HandleHumanMessage(context.Background(), room, trigger)

	if got := room.agentMessages(); len(got) != 2 {
		t.Fatalf("expected 2 guided dialogue replies, got %#v", got)
	}
	if llmClient.calls != 2 {
		t.Fatalf("expected LLM to stop after 2 turns, got %d calls", llmClient.calls)
	}
	if len(store.dialogueRuns) != 1 || store.dialogueRuns[0].TurnCount != 2 {
		t.Fatalf("expected dialogue run to record 2 turns, got %#v", store.dialogueRuns)
	}
}

func TestGuidedDialogueDisallowsSelfFollowupWhenDisabled(t *testing.T) {
	llmClient := &sequenceLLM{
		responses: []string{
			"@Solo I have one more thing to add.",
		},
	}
	store := &dialogueStore{}
	runner := agent.NewRunner(llmClient, store)
	room := newDialogueRuntimeRoom(model.DialoguePolicy{
		Mode:                      model.DialogueModeGuided,
		MaxAutonomousTurns:        3,
		MaxTurnsPerAgent:          3,
		AllowSelfFollowup:         false,
		AllowAgentToAgentMentions: true,
		ResponseStrategy:          model.DialogueResponseStrategyMentionedFirst,
	}, []model.Agent{
		testAgent("solo", "Solo"),
	})

	trigger := room.newHumanMessage("Alice", "@Solo please review this alone.")
	room.AppendMessage(trigger)

	runner.HandleHumanMessage(context.Background(), room, trigger)

	if got := room.agentMessages(); len(got) != 1 {
		t.Fatalf("expected only one agent reply when self-followup is disabled, got %#v", got)
	}
	if llmClient.calls != 1 {
		t.Fatalf("expected only one LLM call, got %d", llmClient.calls)
	}
}

func TestGuidedDialogueSuppressesDuplicateGeneratedContent(t *testing.T) {
	llmClient := &sequenceLLM{
		responses: []string{
			"@Reviewer Shared duplicate content.",
			"Shared duplicate content.",
		},
	}
	store := &dialogueStore{}
	runner := agent.NewRunner(llmClient, store)
	room := newDialogueRuntimeRoom(model.DialoguePolicy{
		Mode:                      model.DialogueModeGuided,
		MaxAutonomousTurns:        4,
		MaxTurnsPerAgent:          2,
		AllowSelfFollowup:         false,
		AllowAgentToAgentMentions: true,
		ResponseStrategy:          model.DialogueResponseStrategyMentionedFirst,
	}, []model.Agent{
		testAgent("author", "Author"),
		testAgent("reviewer", "Reviewer"),
	})

	trigger := room.newHumanMessage("Alice", "@Author kick this off.")
	room.AppendMessage(trigger)

	runner.HandleHumanMessage(context.Background(), room, trigger)

	if got := room.agentMessages(); len(got) != 1 {
		t.Fatalf("expected duplicate second reply to be suppressed, got %#v", got)
	}
	if len(store.dialogueRuns) != 1 || store.dialogueRuns[0].Status != model.DialogueRunStatusStoppedDuplicate {
		t.Fatalf("expected duplicate stop status, got %#v", store.dialogueRuns)
	}
}

func TestGuidedDialogueStopsCleanlyOnProviderError(t *testing.T) {
	llmClient := &sequenceLLM{
		responses: []string{
			"@Reviewer First reply before failure.",
		},
		errors: []error{
			nil,
			errors.New("provider unavailable"),
		},
	}
	store := &dialogueStore{}
	runner := agent.NewRunner(llmClient, store)
	room := newDialogueRuntimeRoom(model.DialoguePolicy{
		Mode:                      model.DialogueModeGuided,
		MaxAutonomousTurns:        4,
		MaxTurnsPerAgent:          2,
		AllowSelfFollowup:         false,
		AllowAgentToAgentMentions: true,
		ResponseStrategy:          model.DialogueResponseStrategyMentionedFirst,
	}, []model.Agent{
		testAgent("author", "Author"),
		testAgent("reviewer", "Reviewer"),
	})

	trigger := room.newHumanMessage("Alice", "@Author please coordinate with Reviewer.")
	room.AppendMessage(trigger)

	runner.HandleHumanMessage(context.Background(), room, trigger)

	got := room.Messages()
	if len(got) != 3 {
		t.Fatalf("expected human, one agent, and one system failure message, got %#v", got)
	}
	last := got[len(got)-1]
	if last.SenderType != model.SenderTypeSystem || !strings.Contains(last.Content, "failed to respond") {
		t.Fatalf("expected trailing system failure message, got %#v", last)
	}
	if len(store.dialogueRuns) != 1 || store.dialogueRuns[0].Status != model.DialogueRunStatusFailed {
		t.Fatalf("expected failed dialogue run, got %#v", store.dialogueRuns)
	}
	if store.dialogueRuns[0].TurnCount != 1 {
		t.Fatalf("expected dialogue run to record the single completed turn before failure, got %#v", store.dialogueRuns[0])
	}
}

type sequenceLLM struct {
	responses []string
	errors    []error
	calls     int
	requests  [][]llm.ChatMessage
}

func (s *sequenceLLM) Complete(_ context.Context, messages []llm.ChatMessage) (string, error) {
	idx := s.calls
	s.calls++
	copied := make([]llm.ChatMessage, len(messages))
	copy(copied, messages)
	s.requests = append(s.requests, copied)

	if idx < len(s.errors) && s.errors[idx] != nil {
		return "", s.errors[idx]
	}
	if idx >= len(s.responses) {
		return "", fmt.Errorf("unexpected llm call %d", idx+1)
	}
	return s.responses[idx], nil
}

type dialogueRuntimeRoom struct {
	meta         model.RoomMeta
	participants []model.Participant
	agents       []model.Agent
	messages     []model.Message
}

func newDialogueRuntimeRoom(policy model.DialoguePolicy, agents []model.Agent) *dialogueRuntimeRoom {
	return &dialogueRuntimeRoom{
		meta: model.RoomMeta{
			ID:             "room_1",
			Name:           "Planning",
			CreatedAt:      time.Now().UTC(),
			DialoguePolicy: policy.WithDefaults(),
		},
		agents:   append([]model.Agent(nil), agents...),
		messages: make([]model.Message, 0),
	}
}

func (r *dialogueRuntimeRoom) Info() model.RoomMeta { return r.meta }

func (r *dialogueRuntimeRoom) Participants() []model.Participant {
	result := make([]model.Participant, len(r.participants))
	copy(result, r.participants)
	return result
}

func (r *dialogueRuntimeRoom) Agents() []model.Agent {
	public := make([]model.Agent, 0, len(r.agents))
	for _, candidate := range r.agents {
		public = append(public, candidate.Public())
	}
	return public
}

func (r *dialogueRuntimeRoom) AgentsWithPrompts() []model.Agent {
	result := make([]model.Agent, len(r.agents))
	copy(result, r.agents)
	return result
}

func (r *dialogueRuntimeRoom) RecentMessages(limit int) []model.Message {
	if limit <= 0 || len(r.messages) <= limit {
		result := make([]model.Message, len(r.messages))
		copy(result, r.messages)
		return result
	}
	start := len(r.messages) - limit
	result := make([]model.Message, len(r.messages[start:]))
	copy(result, r.messages[start:])
	return result
}

func (r *dialogueRuntimeRoom) NewSystemMessage(content string) model.Message {
	return model.Message{
		ID:         fmt.Sprintf("msg_system_%d", len(r.messages)+1),
		RoomID:     r.meta.ID,
		SenderID:   "system",
		SenderName: "System",
		SenderType: model.SenderTypeSystem,
		Content:    content,
		CreatedAt:  time.Now().UTC(),
	}
}

func (r *dialogueRuntimeRoom) NewAgentMessage(agent model.Agent, content string) model.Message {
	return model.Message{
		ID:         fmt.Sprintf("msg_agent_%d", len(r.messages)+1),
		RoomID:     r.meta.ID,
		SenderID:   agent.ID,
		SenderName: agent.Name,
		SenderType: model.SenderTypeAgent,
		Content:    content,
		CreatedAt:  time.Now().UTC(),
	}
}

func (r *dialogueRuntimeRoom) AppendMessage(message model.Message) {
	r.messages = append(r.messages, message)
}

func (r *dialogueRuntimeRoom) Broadcast(model.Message) {}

func (r *dialogueRuntimeRoom) Messages() []model.Message {
	result := make([]model.Message, len(r.messages))
	copy(result, r.messages)
	return result
}

func (r *dialogueRuntimeRoom) agentMessages() []model.Message {
	result := make([]model.Message, 0)
	for _, message := range r.messages {
		if message.SenderType == model.SenderTypeAgent {
			result = append(result, message)
		}
	}
	return result
}

func (r *dialogueRuntimeRoom) newHumanMessage(name string, content string) model.Message {
	return model.Message{
		ID:         fmt.Sprintf("msg_human_%d", len(r.messages)+1),
		RoomID:     r.meta.ID,
		SenderID:   "participant_1",
		SenderName: name,
		SenderType: model.SenderTypeHuman,
		Content:    content,
		CreatedAt:  time.Now().UTC(),
	}
}

type dialogueStore struct {
	teststore.Store
	dialogueRuns []store.DialogueRun
}

func (s *dialogueStore) AddMessage(_ context.Context, message model.Message) (model.Message, error) {
	return message, nil
}

func (s *dialogueStore) CreateAgentRun(context.Context, store.AgentRun) error { return nil }

func (s *dialogueStore) FinishAgentRun(context.Context, string, string, string, time.Time) error {
	return nil
}

func (s *dialogueStore) CreateDialogueRun(_ context.Context, run store.DialogueRun) error {
	s.dialogueRuns = append(s.dialogueRuns, run)
	return nil
}

func (s *dialogueStore) FinishDialogueRun(_ context.Context, runID string, status string, turnCount int, completedAt time.Time) error {
	for i := range s.dialogueRuns {
		if s.dialogueRuns[i].ID != runID {
			continue
		}
		s.dialogueRuns[i].Status = status
		s.dialogueRuns[i].TurnCount = turnCount
		s.dialogueRuns[i].CompletedAt = &completedAt
		return nil
	}
	return fmt.Errorf("dialogue run %s not found", runID)
}

func testAgent(id string, name string) model.Agent {
	return model.Agent{
		ID:           id,
		Name:         name,
		Mention:      "@" + name,
		Role:         name + " role",
		SystemPrompt: "You are " + name + ".",
		Enabled:      true,
	}
}
