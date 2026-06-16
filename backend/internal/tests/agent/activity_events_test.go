package agent_test

import (
	"context"
	"testing"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/realtime"
	"agentroom/backend/internal/tests/teststore"
)

func TestAgentActivityEventsWrapMentionFanoutRun(t *testing.T) {
	llmClient := &sequenceLLM{responses: []string{"I can help with the rollout."}}
	runner := agent.NewRunner(llmClient, &teststore.Store{})
	room := newDialogueRuntimeRoom(model.DefaultDialoguePolicy(), []model.Agent{
		testAgent("builder", "Builder"),
	})

	trigger := room.newHumanMessage("Alice", "@Builder please review this.")
	room.AppendMessage(trigger)

	runner.HandleHumanMessage(context.Background(), room, trigger)

	started := activityEvents(room.events, "agent_run", "started")
	if len(started) != 1 {
		t.Fatalf("expected one agent run started event, got %#v", room.events)
	}
	if started[0].Activity.AgentID != "builder" || started[0].Activity.AgentName != "Builder" || started[0].Activity.TriggerMessageID != trigger.ID {
		t.Fatalf("unexpected started activity payload: %#v", started[0].Activity)
	}

	finished := activityEvents(room.events, "agent_run", "finished")
	if len(finished) != 1 {
		t.Fatalf("expected one agent run finished event, got %#v", room.events)
	}
	if finished[0].Activity.Status != "succeeded" || finished[0].Activity.CompletedAt == nil {
		t.Fatalf("unexpected finished activity payload: %#v", finished[0].Activity)
	}
}

func TestDialogueActivityEventsWrapGuidedDialogueRun(t *testing.T) {
	llmClient := &sequenceLLM{responses: []string{"Guided reply."}}
	runner := agent.NewRunner(llmClient, &teststore.Store{})
	room := newDialogueRuntimeRoom(model.DialoguePolicy{
		Mode:                      model.DialogueModeGuided,
		MaxAutonomousTurns:        1,
		MaxTurnsPerAgent:          1,
		AllowSelfFollowup:         false,
		AllowAgentToAgentMentions: true,
		ResponseStrategy:          model.DialogueResponseStrategyMentionedFirst,
	}, []model.Agent{
		testAgent("builder", "Builder"),
	})

	trigger := room.newHumanMessage("Alice", "@Builder please begin.")
	room.AppendMessage(trigger)

	runner.HandleHumanMessage(context.Background(), room, trigger)

	started := activityEvents(room.events, "dialogue_run", "started")
	if len(started) != 1 {
		t.Fatalf("expected one dialogue run started event, got %#v", room.events)
	}
	if started[0].Activity.RoomID != room.Info().ID || started[0].Activity.TriggerMessageID != trigger.ID {
		t.Fatalf("unexpected started dialogue activity: %#v", started[0].Activity)
	}

	finished := activityEvents(room.events, "dialogue_run", "finished")
	if len(finished) != 1 {
		t.Fatalf("expected one dialogue run finished event, got %#v", room.events)
	}
	if finished[0].Activity.Status != model.DialogueRunStatusSucceeded || finished[0].Activity.TurnCount != 1 || finished[0].Activity.CompletedAt == nil {
		t.Fatalf("unexpected finished dialogue activity: %#v", finished[0].Activity)
	}
}

func activityEvents(events []realtime.Event, kind string, phase string) []realtime.Event {
	result := make([]realtime.Event, 0)
	for _, event := range events {
		if event.Type != realtime.EventTypeAgentActivity || event.Activity == nil {
			continue
		}
		if event.Activity.Kind == kind && event.Activity.Phase == phase {
			result = append(result, event)
		}
	}
	return result
}
