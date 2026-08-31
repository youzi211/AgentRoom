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

func TestMentionFanoutHumanMentionsPreserveTextOrderAndScope(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantIDs  []string
		response []string
	}{
		{
			name:     "single mention",
			content:  "@Reviewer please review this.",
			wantIDs:  []string{"reviewer"},
			response: []string{"Reviewer response."},
		},
		{
			name:     "multiple mentions",
			content:  "@Reviewer then @Author please review this.",
			wantIDs:  []string{"reviewer", "author"},
			response: []string{"Reviewer response.", "Author response."},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			llmClient := &sequenceLLM{responses: test.response}
			store := &dialogueStore{}
			runner := agent.NewRunner(llmClient, store)
			room := newDialogueRuntimeRoom(model.DialoguePolicy{
				Mode:                      model.DialogueModeMentionFanout,
				MaxAutonomousTurns:        2,
				MaxTurnsPerAgent:          1,
				AllowSelfFollowup:         false,
				AllowAgentToAgentMentions: true,
				ResponseStrategy:          model.DialogueResponseStrategyMentionedFirst,
			}, []model.Agent{
				testAgent("author", "Author"),
				testAgent("reviewer", "Reviewer"),
				testAgent("architect", "Architect"),
			})

			trigger := room.newHumanMessage("Alice", test.content)
			room.AppendMessage(trigger)
			runner.HandleHumanMessage(context.Background(), room, trigger)

			got := room.agentMessages()
			if len(got) != len(test.wantIDs) {
				t.Fatalf("expected responders %v, got %#v", test.wantIDs, got)
			}
			for i, wantID := range test.wantIDs {
				if got[i].SenderID != wantID {
					t.Fatalf("expected responder %d to be %q, got %#v", i, wantID, got)
				}
			}
			if llmClient.calls != len(test.wantIDs) || len(store.AgentRuns) != len(test.wantIDs) {
				t.Fatalf("expected one successful run per mentioned Agent, calls=%d runs=%#v", llmClient.calls, store.AgentRuns)
			}
		})
	}
}

func TestMentionFanoutStopsAtAutonomousTurnLimit(t *testing.T) {
	llmClient := &sequenceLLM{responses: []string{
		"@Reviewer please follow up.",
		"@Author please follow up again.",
	}}
	store := &dialogueStore{}
	runner := agent.NewRunner(llmClient, store)
	room := newDialogueRuntimeRoom(model.DialoguePolicy{
		Mode:                      model.DialogueModeMentionFanout,
		MaxAutonomousTurns:        1,
		MaxTurnsPerAgent:          2,
		AllowSelfFollowup:         false,
		AllowAgentToAgentMentions: true,
		ResponseStrategy:          model.DialogueResponseStrategyMentionedFirst,
	}, []model.Agent{
		testAgent("author", "Author"),
		testAgent("reviewer", "Reviewer"),
	})

	trigger := room.newHumanMessage("Alice", "@Author start the review.")
	room.AppendMessage(trigger)
	runner.HandleHumanMessage(context.Background(), room, trigger)

	got := room.agentMessages()
	if len(got) != 2 || got[0].SenderID != "author" || got[1].SenderID != "reviewer" {
		t.Fatalf("expected initial response and one autonomous follow-up, got %#v", got)
	}
	if llmClient.calls != 2 {
		t.Fatalf("expected autonomous turn limit to stop a third call, got %d", llmClient.calls)
	}
}

func TestHumanMessageWithoutMentionDoesNotTriggerCurrentDialogueModes(t *testing.T) {
	for _, mode := range []string{model.DialogueModeMentionFanout, model.DialogueModeGuided} {
		t.Run(mode, func(t *testing.T) {
			llmClient := &sequenceLLM{responses: []string{"must not be called"}}
			store := &dialogueStore{}
			runner := agent.NewRunner(llmClient, store)
			room := newDialogueRuntimeRoom(model.DialoguePolicy{
				Mode:                      mode,
				MaxAutonomousTurns:        3,
				MaxTurnsPerAgent:          1,
				AllowSelfFollowup:         false,
				AllowAgentToAgentMentions: true,
				ResponseStrategy:          model.DialogueResponseStrategyMentionedFirst,
			}, []model.Agent{testAgent("author", "Author")})

			trigger := room.newHumanMessage("Alice", "Please review this without assigning an Agent.")
			room.AppendMessage(trigger)
			runner.HandleHumanMessage(context.Background(), room, trigger)

			if llmClient.calls != 0 || len(room.agentMessages()) != 0 || len(store.AgentRuns) != 0 || len(store.dialogueRuns) != 0 {
				t.Fatalf("expected no Agent work without a mention, calls=%d runs=%#v dialogues=%#v messages=%#v", llmClient.calls, store.AgentRuns, store.dialogueRuns, room.Messages())
			}
		})
	}
}
