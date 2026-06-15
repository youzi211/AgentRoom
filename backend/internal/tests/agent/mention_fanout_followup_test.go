package agent_test

import (
	"context"
	"testing"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/model"
)

func TestMentionFanoutFollowsExplicitAgentMentions(t *testing.T) {
	llmClient := &sequenceLLM{
		responses: []string{
			"@Reviewer @Architect please add your concerns.",
			"Reviewer follow-up.",
			"Architect follow-up.",
		},
	}
	store := &dialogueStore{}
	runner := agent.NewRunner(llmClient, store)
	room := newDialogueRuntimeRoom(model.DialoguePolicy{
		Mode:                      model.DialogueModeMentionFanout,
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

	trigger := room.newHumanMessage("Alice", "@Author please coordinate the review.")
	room.AppendMessage(trigger)

	runner.HandleHumanMessage(context.Background(), room, trigger)

	got := room.agentMessages()
	if len(got) != 3 {
		t.Fatalf("expected agent handoff to produce 3 replies in mention_fanout mode, got %#v", got)
	}
	if got[0].SenderID != "author" || got[1].SenderID != "reviewer" || got[2].SenderID != "architect" {
		t.Fatalf("expected Author, Reviewer, then Architect replies, got %#v", got)
	}
	if len(store.dialogueRuns) != 0 {
		t.Fatalf("expected mention_fanout follow-up to avoid guided dialogue runs, got %#v", store.dialogueRuns)
	}
}

func TestMentionFanoutSkipsSelfMentionsWhenSelfFollowupIsDisabled(t *testing.T) {
	llmClient := &sequenceLLM{
		responses: []string{
			"@Author @Reviewer please both verify this.",
			"Reviewer follow-up.",
		},
	}
	store := &dialogueStore{}
	runner := agent.NewRunner(llmClient, store)
	room := newDialogueRuntimeRoom(model.DialoguePolicy{
		Mode:                      model.DialogueModeMentionFanout,
		MaxAutonomousTurns:        3,
		MaxTurnsPerAgent:          2,
		AllowSelfFollowup:         false,
		AllowAgentToAgentMentions: true,
		ResponseStrategy:          model.DialogueResponseStrategyMentionedFirst,
	}, []model.Agent{
		testAgent("author", "Author"),
		testAgent("reviewer", "Reviewer"),
	})

	trigger := room.newHumanMessage("Alice", "@Author please start the validation.")
	room.AppendMessage(trigger)

	runner.HandleHumanMessage(context.Background(), room, trigger)

	got := room.agentMessages()
	if len(got) != 2 {
		t.Fatalf("expected self-mention to be ignored while reviewer still responds, got %#v", got)
	}
	if got[0].SenderID != "author" || got[1].SenderID != "reviewer" {
		t.Fatalf("expected only Author then Reviewer replies, got %#v", got)
	}
}
