package agent_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/tests/teststore"
)

func TestRunnerPromptIncludesStructuredMeetingContext(t *testing.T) {
	llmClient := &recordingLLM{}
	knowledge := &recordingKnowledgeProvider{
		chunks: []model.KnowledgeChunk{
			{ID: "room_chunk", Scope: model.KnowledgeScopeRoom, ScopeID: "room_1", Content: "Room launch date is July."},
			{ID: "agent_chunk", Scope: model.KnowledgeScopeAgent, ScopeID: "builder", Content: "Builder should highlight rollback risk."},
		},
	}
	runner := agent.NewRunner(llmClient, &teststore.Store{}).WithKnowledge(knowledge)
	now := time.Now().UTC()

	room := &runtimeRoom{
		meta: model.RoomMeta{
			ID:             "room_1",
			Name:           "Planning",
			CreatedAt:      now,
			DialoguePolicy: model.DefaultDialoguePolicy(),
		},
		participants: []model.Participant{
			{ID: "human_1", Name: "Alice", JoinedAt: now.Add(-2 * time.Minute)},
			{ID: "human_2", Name: "Bob", JoinedAt: now.Add(-1 * time.Minute)},
		},
		agents: []model.Agent{
			{
				ID:           "builder",
				Name:         "Builder",
				Mention:      "@Builder",
				Role:         "Backend Engineer",
				Description:  "Turns requirements into backend changes.",
				SystemPrompt: "You are a pragmatic builder.",
				Enabled:      true,
			},
			{
				ID:           "reviewer",
				Name:         "Reviewer",
				Mention:      "@Reviewer",
				Role:         "QA Engineer",
				Description:  "Finds edge cases and regressions.",
				SystemPrompt: "You are a careful reviewer.",
				Enabled:      true,
			},
		},
		messages: []model.Message{
			{
				ID:         "msg_system_1",
				RoomID:     "room_1",
				SenderID:   "system",
				SenderName: "System",
				SenderType: model.SenderTypeSystem,
				Content:    "System should stay hidden",
				CreatedAt:  now.Add(-3 * time.Minute),
			},
			{
				ID:         "msg_agent_1",
				RoomID:     "room_1",
				SenderID:   "reviewer",
				SenderName: "Reviewer",
				SenderType: model.SenderTypeAgent,
				Content:    "We still need a rollback checklist.",
				CreatedAt:  now.Add(-2 * time.Minute),
			},
			{
				ID:         "msg_human_1",
				RoomID:     "room_1",
				SenderID:   "human_1",
				SenderName: "Alice",
				SenderType: model.SenderTypeHuman,
				Content:    "@Builder please walk through the launch risks.",
				CreatedAt:  now.Add(-1 * time.Minute),
			},
		},
	}

	trigger := room.messages[len(room.messages)-1]
	runner.HandleHumanMessage(context.Background(), room, trigger)

	if len(llmClient.messages) != 2 {
		t.Fatalf("expected system and user chat messages, got %#v", llmClient.messages)
	}

	systemPrompt := llmClient.messages[0].Content
	userPrompt := llmClient.messages[1].Content

	assertContainsAll(t, systemPrompt,
		"You are a pragmatic builder.",
		"exactly one visible room message",
		"Do not impersonate other roles",
	)

	assertContainsAll(t, userPrompt,
		"Room: Planning",
		"Dialogue mode: mention_fanout",
		"Online human participants:",
		"Alice",
		"Bob",
		"Room agents:",
		"Builder (@Builder)",
		"Reviewer (@Reviewer)",
		"Trigger sender: Alice (human)",
		"Latest visible speaker: Alice (human)",
		"Room launch date is July.",
		"Builder should highlight rollback risk.",
		"We still need a rollback checklist.",
		"@Builder please walk through the launch risks.",
	)

	if strings.Contains(userPrompt, "System should stay hidden") {
		t.Fatalf("expected system messages to be excluded from transcript, got %q", userPrompt)
	}

	if len(knowledge.queries) != 1 || knowledge.queries[0] != trigger.Content {
		t.Fatalf("expected knowledge query to use the human trigger content, got %#v", knowledge.queries)
	}
}

func TestRunnerPromptLabelsKnowledgeSources(t *testing.T) {
	llmClient := &recordingLLM{}
	knowledge := &recordingKnowledgeProvider{
		chunks: []model.KnowledgeChunk{
			{
				ID:           "room_chunk",
				DocumentID:   "doc_room",
				DocumentName: "roadmap.md",
				Scope:        model.KnowledgeScopeRoom,
				ScopeID:      "room_1",
				ChunkIndex:   0,
				Content:      "Room launch date is July.",
			},
			{
				ID:           "agent_chunk",
				DocumentID:   "doc_agent",
				DocumentName: "qa-playbook.md",
				Scope:        model.KnowledgeScopeAgent,
				ScopeID:      "builder",
				ChunkIndex:   1,
				Content:      "Builder should highlight rollback risk.",
			},
		},
	}
	runner := agent.NewRunner(llmClient, &teststore.Store{}).WithKnowledge(knowledge)
	now := time.Now().UTC()

	room := &runtimeRoom{
		meta: model.RoomMeta{
			ID:             "room_1",
			Name:           "Planning",
			CreatedAt:      now,
			DialoguePolicy: model.DefaultDialoguePolicy(),
		},
		participants: []model.Participant{{ID: "human_1", Name: "Alice", JoinedAt: now}},
		agents: []model.Agent{
			{
				ID:           "builder",
				Name:         "Builder",
				Mention:      "@Builder",
				Role:         "Backend Engineer",
				Description:  "Turns requirements into backend changes.",
				SystemPrompt: "You are a pragmatic builder.",
				Enabled:      true,
			},
		},
		messages: []model.Message{
			{
				ID:         "msg_human_1",
				RoomID:     "room_1",
				SenderID:   "human_1",
				SenderName: "Alice",
				SenderType: model.SenderTypeHuman,
				Content:    "@Builder please summarize the launch risks.",
				CreatedAt:  now,
			},
		},
	}

	runner.HandleHumanMessage(context.Background(), room, room.messages[0])

	if len(llmClient.messages) != 2 {
		t.Fatalf("expected system and user chat messages, got %#v", llmClient.messages)
	}
	userPrompt := llmClient.messages[1].Content
	assertContainsAll(t, userPrompt,
		"[room: roadmap.md #1]",
		"Room launch date is July.",
		"[agent: qa-playbook.md #2]",
		"Builder should highlight rollback risk.",
	)
}

func TestGuidedDialoguePromptTracksImmediateAndRootTriggers(t *testing.T) {
	llmClient := &sequenceLLM{
		responses: []string{
			"@Reviewer Please pressure-test the rollout order.",
			"I would stage the rollout and keep rollback ready.",
		},
	}
	knowledge := &recordingKnowledgeProvider{}
	store := &dialogueStore{}
	runner := agent.NewRunner(llmClient, store).WithKnowledge(knowledge)
	now := time.Now().UTC()

	room := newDialogueRuntimeRoom(model.DialoguePolicy{
		Mode:                      model.DialogueModeGuided,
		MaxAutonomousTurns:        3,
		MaxTurnsPerAgent:          2,
		AllowSelfFollowup:         false,
		AllowAgentToAgentMentions: true,
		ResponseStrategy:          model.DialogueResponseStrategyMentionedFirst,
	}, []model.Agent{
		testAgent("builder", "Builder"),
		testAgent("reviewer", "Reviewer"),
		testAgent("architect", "Architect"),
	})
	room.participants = []model.Participant{
		{ID: "human_1", Name: "Alice", JoinedAt: now.Add(-2 * time.Minute)},
		{ID: "human_2", Name: "Bob", JoinedAt: now.Add(-1 * time.Minute)},
	}
	room.messages = []model.Message{
		{
			ID:         "msg_system_1",
			RoomID:     room.meta.ID,
			SenderID:   "system",
			SenderName: "System",
			SenderType: model.SenderTypeSystem,
			Content:    "System breadcrumb that should not leak",
			CreatedAt:  now.Add(-3 * time.Minute),
		},
	}

	trigger := room.newHumanMessage("Alice", "@Builder please work through the rollout risks with @Reviewer.")
	room.AppendMessage(trigger)

	runner.HandleHumanMessage(context.Background(), room, trigger)

	if len(llmClient.requests) != 2 {
		t.Fatalf("expected two LLM calls, got %d", len(llmClient.requests))
	}

	secondCall := llmClient.requests[1]
	if len(secondCall) != 2 {
		t.Fatalf("expected guided prompt to use system and user messages, got %#v", secondCall)
	}

	systemPrompt := secondCall[0].Content
	userPrompt := secondCall[1].Content

	assertContainsAll(t, systemPrompt,
		"You are Reviewer.",
		"exactly one visible room message",
	)

	assertContainsAll(t, userPrompt,
		"Dialogue mode: guided_dialogue",
		"Current speaker: Reviewer",
		"Autonomous turn: 2/3",
		"Trigger sender: Builder (agent)",
		"Trigger content:",
		"@Reviewer Please pressure-test the rollout order.",
		"Root human trigger sender: Alice (human)",
		"Root human trigger content:",
		"@Builder please work through the rollout risks with @Reviewer.",
		"Eligible peers for follow-up: @Architect",
		"Stop conditions: stop when there are no eligible peers, when turn limits are reached, or when the next reply would be empty or duplicate prior dialogue.",
		"Latest visible speaker: Builder (agent)",
		"Online human participants:",
		"Alice",
		"Bob",
	)

	if strings.Contains(userPrompt, "System breadcrumb that should not leak") {
		t.Fatalf("expected system messages to be excluded from guided transcript, got %q", userPrompt)
	}

	if len(knowledge.queries) != 2 {
		t.Fatalf("expected one knowledge query per guided turn, got %#v", knowledge.queries)
	}
	if knowledge.queries[0] != trigger.Content {
		t.Fatalf("expected first guided turn to use human trigger content, got %#v", knowledge.queries)
	}
	if knowledge.queries[1] != "@Reviewer Please pressure-test the rollout order." {
		t.Fatalf("expected follow-up guided turn to use immediate parent trigger content, got %#v", knowledge.queries)
	}
}

func TestRunnerPromptKeepsOutputContractWithoutRoleTemplate(t *testing.T) {
	llmClient := &recordingLLM{}
	runner := agent.NewRunner(llmClient, &teststore.Store{})
	now := time.Now().UTC()

	room := &runtimeRoom{
		meta: model.RoomMeta{
			ID:             "room_1",
			Name:           "Planning",
			CreatedAt:      now,
			DialoguePolicy: model.DefaultDialoguePolicy(),
		},
		participants: []model.Participant{
			{ID: "human_1", Name: "Alice", JoinedAt: now},
		},
		agents: []model.Agent{
			{
				ID:           "builder",
				Name:         "Builder",
				Mention:      "@Builder",
				Role:         "Backend Engineer",
				Description:  "Turns requirements into backend changes.",
				SystemPrompt: "",
				Enabled:      true,
			},
		},
		messages: []model.Message{
			{
				ID:         "msg_human_1",
				RoomID:     "room_1",
				SenderID:   "human_1",
				SenderName: "Alice",
				SenderType: model.SenderTypeHuman,
				Content:    "@Builder please summarize the rollout risks.",
				CreatedAt:  now,
			},
		},
	}

	runner.HandleHumanMessage(context.Background(), room, room.messages[0])

	if len(llmClient.messages) != 2 {
		t.Fatalf("expected system and user chat messages, got %#v", llmClient.messages)
	}

	systemPrompt := llmClient.messages[0].Content
	userPrompt := llmClient.messages[1].Content

	assertContainsAll(t, systemPrompt,
		"You are participating in an AgentRoom meeting.",
		"Reply with exactly one visible room message.",
	)
	if strings.Contains(systemPrompt, "Agent role template:") {
		t.Fatalf("expected blank role template to avoid extra system template text, got %q", systemPrompt)
	}
	assertContainsAll(t, userPrompt,
		"Output contract:",
		"Reply with one concise room-visible message.",
	)
}

func TestRunnerPromptFallsBackToTriggerSpeakerWhenTranscriptHasNoVisibleMessages(t *testing.T) {
	llmClient := &recordingLLM{}
	runner := agent.NewRunner(llmClient, &teststore.Store{})
	now := time.Now().UTC()

	room := &runtimeRoom{
		meta: model.RoomMeta{
			ID:             "room_1",
			Name:           "Planning",
			CreatedAt:      now,
			DialoguePolicy: model.DefaultDialoguePolicy(),
		},
		participants: []model.Participant{
			{ID: "human_1", Name: "Alice", JoinedAt: now},
		},
		agents: []model.Agent{
			{
				ID:           "builder",
				Name:         "Builder",
				Mention:      "@Builder",
				Role:         "Backend Engineer",
				Description:  "Turns requirements into backend changes.",
				SystemPrompt: "You are a pragmatic builder.",
				Enabled:      true,
			},
		},
		messages: []model.Message{
			{
				ID:         "msg_system_1",
				RoomID:     "room_1",
				SenderID:   "system",
				SenderName: "System",
				SenderType: model.SenderTypeSystem,
				Content:    "System breadcrumb that should stay hidden",
				CreatedAt:  now.Add(-1 * time.Minute),
			},
		},
	}

	trigger := model.Message{
		ID:         "msg_human_1",
		RoomID:     "room_1",
		SenderID:   "human_1",
		SenderName: "Alice",
		SenderType: model.SenderTypeHuman,
		Content:    "@Builder please summarize the rollout risks.",
		CreatedAt:  now,
	}

	runner.HandleHumanMessage(context.Background(), room, trigger)

	if len(llmClient.messages) != 2 {
		t.Fatalf("expected system and user chat messages, got %#v", llmClient.messages)
	}

	userPrompt := llmClient.messages[1].Content
	assertContainsAll(t, userPrompt,
		"Latest visible speaker: Alice (human)",
		"Visible room transcript:\n- none",
	)
	if strings.Contains(userPrompt, "System breadcrumb that should stay hidden") {
		t.Fatalf("expected system-only transcript to remain hidden, got %q", userPrompt)
	}
}

func TestMentionFanoutPromptMarksAgentTriggeredFollowup(t *testing.T) {
	llmClient := &sequenceLLM{
		responses: []string{
			"@Reviewer please pressure-test the rollback steps.",
			"Reviewer follow-up.",
		},
	}
	knowledge := &recordingKnowledgeProvider{}
	store := &dialogueStore{}
	runner := agent.NewRunner(llmClient, store).WithKnowledge(knowledge)
	now := time.Now().UTC()

	room := newDialogueRuntimeRoom(model.DialoguePolicy{
		Mode:                      model.DialogueModeMentionFanout,
		MaxAutonomousTurns:        3,
		MaxTurnsPerAgent:          1,
		AllowSelfFollowup:         false,
		AllowAgentToAgentMentions: true,
		ResponseStrategy:          model.DialogueResponseStrategyMentionedFirst,
	}, []model.Agent{
		testAgent("builder", "Builder"),
		testAgent("reviewer", "Reviewer"),
	})
	room.participants = []model.Participant{
		{ID: "human_1", Name: "Alice", JoinedAt: now},
	}

	trigger := room.newHumanMessage("Alice", "@Builder please start the rollback review.")
	room.AppendMessage(trigger)

	runner.HandleHumanMessage(context.Background(), room, trigger)

	if len(llmClient.requests) != 2 {
		t.Fatalf("expected two LLM calls after agent handoff in mention_fanout mode, got %d", len(llmClient.requests))
	}

	secondCall := llmClient.requests[1]
	if len(secondCall) != 2 {
		t.Fatalf("expected system and user prompt messages for follow-up turn, got %#v", secondCall)
	}

	userPrompt := secondCall[1].Content
	assertContainsAll(t, userPrompt,
		"Dialogue mode: mention_fanout",
		"Trigger sender: Builder (agent)",
		"Latest visible speaker: Builder (agent)",
		"Current explicit @mention trigger was sent by another agent.",
	)

	if len(knowledge.queries) != 2 {
		t.Fatalf("expected one knowledge query per mention_fanout turn, got %#v", knowledge.queries)
	}
	if knowledge.queries[1] != "@Reviewer please pressure-test the rollback steps." {
		t.Fatalf("expected follow-up turn to query knowledge with agent trigger content, got %#v", knowledge.queries)
	}
}

func assertContainsAll(t *testing.T, content string, expected ...string) {
	t.Helper()
	for _, item := range expected {
		if !strings.Contains(content, item) {
			t.Fatalf("expected %q to contain %q", content, item)
		}
	}
}

type recordingKnowledgeProvider struct {
	chunks  []model.KnowledgeChunk
	queries []string
}

func (p *recordingKnowledgeProvider) SearchForAgent(_ context.Context, roomID string, agentID string, query string) ([]model.KnowledgeChunk, error) {
	p.queries = append(p.queries, query)

	result := make([]model.KnowledgeChunk, 0, len(p.chunks))
	for _, chunk := range p.chunks {
		if (chunk.Scope == model.KnowledgeScopeRoom && chunk.ScopeID == roomID) ||
			(chunk.Scope == model.KnowledgeScopeAgent && chunk.ScopeID == agentID) {
			result = append(result, chunk)
		}
	}
	return result, nil
}
