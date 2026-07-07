package agent_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

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

func TestAgentRunFailsWhenRuntimeIsNotConfigured(t *testing.T) {
	llmClient := &sequenceLLM{responses: []string{"this must not be called"}}
	store := &teststore.Store{}
	runner := agent.NewRunner(llmClient, store)
	room := newDialogueRuntimeRoom(model.DefaultDialoguePolicy(), []model.Agent{
		{
			ID:           "research",
			Name:         "Research",
			Mention:      "@Research",
			Role:         "Research",
			Runtime:      model.AgentRuntimeDeepAgent,
			SystemPrompt: "Research deeply.",
			Enabled:      true,
		},
	})

	trigger := room.newHumanMessage("Alice", "@Research 当前大模型的参数量")
	room.AppendMessage(trigger)

	runner.HandleHumanMessage(context.Background(), room, trigger)

	if llmClient.calls != 0 {
		t.Fatalf("expected unconfigured deepagent runtime not to call llm, got %d calls", llmClient.calls)
	}
	if len(store.AgentRuns) != 1 || store.AgentRuns[0].Status != "failed" {
		t.Fatalf("expected one failed agent run, got %#v", store.AgentRuns)
	}
	if !strings.Contains(store.AgentRuns[0].Error, "runtime deepagent is not configured") {
		t.Fatalf("expected runtime error to be persisted, got %#v", store.AgentRuns[0])
	}
	messages := room.Messages()
	last := messages[len(messages)-1]
	if last.SenderType != model.SenderTypeSystem || !strings.Contains(last.Content, "runtime deepagent is not configured") {
		t.Fatalf("expected system runtime failure message, got %#v", last)
	}
}

func TestAgentRunUsesRegisteredDeepAgentRuntime(t *testing.T) {
	llmClient := &sequenceLLM{responses: []string{"this must not be called"}}
	store := &teststore.Store{}
	deepRuntime := agent.NewDeepAgentRuntime(agent.DeepAgentRuntimeConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestDeepAgentRuntimeHelperProcess", "--"},
		Env:     []string{"AGENTROOM_DEEPAGENT_HELPER=1"},
		WorkDir: t.TempDir(),
		Config:  "deepagent.toml",
		Timeout: 5 * time.Second,
	})
	runner := agent.NewRunner(llmClient, store).
		WithRuntimeRegistry(agent.NewRuntimeRegistry(agent.NewLLMAgentRuntime(llmClient, 45*time.Second), deepRuntime))
	room := newDialogueRuntimeRoom(model.DefaultDialoguePolicy(), []model.Agent{
		{
			ID:           "research",
			Name:         "Research",
			Mention:      "@Research",
			Role:         "Research",
			Runtime:      model.AgentRuntimeDeepAgent,
			SystemPrompt: "Research deeply.",
			Enabled:      true,
		},
	})

	trigger := room.newHumanMessage("Alice", "@Research 当前大模型的参数量")
	room.AppendMessage(trigger)

	runner.HandleHumanMessage(context.Background(), room, trigger)

	if llmClient.calls != 0 {
		t.Fatalf("expected deepagent runtime not to call llm, got %d calls", llmClient.calls)
	}
	if len(store.AgentRuns) != 1 || store.AgentRuns[0].Status != "succeeded" {
		t.Fatalf("expected succeeded deepagent run, got %#v", store.AgentRuns)
	}
	messages := room.Messages()
	last := messages[len(messages)-1]
	if last.SenderType != model.SenderTypeAgent || strings.Contains(last.Content, "# Report") {
		t.Fatalf("expected short deepagent message, got %#v", last)
	}
	if len(last.Artifacts) != 1 || last.Artifacts[0].ID != "report" || !strings.Contains(last.Artifacts[0].Content, "# Report") {
		t.Fatalf("expected deepagent report artifact on message, got %#v", last.Artifacts)
	}
}

func TestGuidedDialoguePreservesDeepAgentRuntimeArtifacts(t *testing.T) {
	llmClient := &sequenceLLM{responses: []string{"this must not be called"}}
	store := &teststore.Store{}
	deepRuntime := agent.NewDeepAgentRuntime(agent.DeepAgentRuntimeConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestDeepAgentRuntimeHelperProcess", "--"},
		Env:     []string{"AGENTROOM_DEEPAGENT_HELPER=1"},
		WorkDir: t.TempDir(),
		Config:  "deepagent.toml",
		Timeout: 5 * time.Second,
	})
	runner := agent.NewRunner(llmClient, store).
		WithRuntimeRegistry(agent.NewRuntimeRegistry(agent.NewLLMAgentRuntime(llmClient, 45*time.Second), deepRuntime))
	room := newDialogueRuntimeRoom(model.DialoguePolicy{
		Mode:                      model.DialogueModeGuided,
		MaxAutonomousTurns:        1,
		MaxTurnsPerAgent:          1,
		AllowSelfFollowup:         false,
		AllowAgentToAgentMentions: true,
		ResponseStrategy:          model.DialogueResponseStrategyMentionedFirst,
	}, []model.Agent{
		{
			ID:           "research",
			Name:         "Research",
			Mention:      "@Research",
			Role:         "Research",
			Runtime:      model.AgentRuntimeDeepAgent,
			SystemPrompt: "Research deeply.",
			Enabled:      true,
		},
	})

	trigger := room.newHumanMessage("Alice", "@Research current model parameter counts")
	room.AppendMessage(trigger)

	runner.HandleHumanMessage(context.Background(), room, trigger)

	messages := room.agentMessages()
	if len(messages) != 1 {
		t.Fatalf("expected one guided dialogue agent message, got %#v", messages)
	}
	if len(messages[0].Artifacts) != 1 || messages[0].Artifacts[0].ID != "report" || !strings.Contains(messages[0].Artifacts[0].Content, "# Report") {
		t.Fatalf("expected guided dialogue report artifact, got %#v", messages[0].Artifacts)
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
