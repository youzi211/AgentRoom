package agent_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/realtime"
	"agentroom/backend/internal/tests/teststore"
)

type canceledAgentRuntime struct{}

func (canceledAgentRuntime) Name() string { return model.AgentRuntimeLLM }
func (canceledAgentRuntime) Respond(context.Context, agent.AgentRuntimeRequest, ...agent.AgentEventObserver) (agent.AgentRuntimeResponse, error) {
	return agent.AgentRuntimeResponse{}, context.Canceled
}

type artifactAuditRuntime struct{}

func (artifactAuditRuntime) Name() string { return model.AgentRuntimeDeepAgent }
func (artifactAuditRuntime) Respond(_ context.Context, request agent.AgentRuntimeRequest, _ ...agent.AgentEventObserver) (agent.AgentRuntimeResponse, error) {
	content := "review complete"
	if request.Agent.ID == "research" {
		content = "@Reviewer verify the research"
	}
	return agent.AgentRuntimeResponse{
		Content: content,
		Artifacts: []agent.AgentRuntimeArtifact{{
			ID: "report-" + request.Agent.ID, Type: "markdown_report", Title: "Report",
			FileName: request.Agent.ID + ".md", MIMEType: "text/markdown", Content: "# " + request.Agent.Name,
		}},
		Metadata: map[string]string{
			"model_profile_id": "profile-" + request.Agent.ID,
			"model_source":     "database",
			"model_name":       "research-model",
		},
	}, nil
}

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
	for _, phase := range []string{"accepted", "model_started", "model_completed", "completed"} {
		if got := activityEvents(room.events, "agent_runtime", phase); len(got) != 1 {
			t.Fatalf("expected one %s runtime activity, got %#v", phase, room.events)
		}
	}
	if messages := room.agentMessages(); len(messages) != 1 {
		t.Fatalf("runtime lifecycle events must not create chat messages, got %#v", messages)
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

func TestAgentMessageIsNotPublishedWhenAtomicSuccessCommitFails(t *testing.T) {
	llmClient := &sequenceLLM{responses: []string{"Uncommitted response"}}
	store := &teststore.Store{CommitAgentRunErr: errors.New("database commit failed")}
	runner := agent.NewRunner(llmClient, store)
	room := newDialogueRuntimeRoom(model.DefaultDialoguePolicy(), []model.Agent{testAgent("builder", "Builder")})
	trigger := room.newHumanMessage("Alice", "@Builder please review this.")
	room.AppendMessage(trigger)

	runner.HandleHumanMessage(context.Background(), room, trigger)

	if messages := room.agentMessages(); len(messages) != 0 {
		t.Fatalf("failed success commit must not publish an Agent message: %#v", messages)
	}
	messages := room.Messages()
	if messages[len(messages)-1].SenderType != model.SenderTypeSystem || !strings.Contains(messages[len(messages)-1].Content, "could not be saved") {
		t.Fatalf("expected visible system failure, got %#v", messages)
	}
	if len(store.AgentRuns) != 1 || store.AgentRuns[0].Status != "failed" {
		t.Fatalf("expected failed terminal Run after rollback, got %#v", store.AgentRuns)
	}
}

func TestCancelledAgentRunPersistsCanceledTerminalState(t *testing.T) {
	store := &teststore.Store{}
	runner := agent.NewRunner(&sequenceLLM{}, store).
		WithRuntimeRegistry(agent.NewRuntimeRegistry(canceledAgentRuntime{}))
	room := newDialogueRuntimeRoom(model.DefaultDialoguePolicy(), []model.Agent{testAgent("builder", "Builder")})
	trigger := room.newHumanMessage("Alice", "@Builder please review this.")
	room.AppendMessage(trigger)

	runner.HandleHumanMessage(context.Background(), room, trigger)

	if len(store.AgentRuns) != 1 || store.AgentRuns[0].Status != "canceled" || store.AgentRuns[0].CompletedAt == nil {
		t.Fatalf("expected durable canceled terminal state, got %#v", store.AgentRuns)
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

func TestDeepAgentArtifactsAndAuditSurviveHumanAndAgentMentionsInAllDialogueModes(t *testing.T) {
	tests := []struct {
		name   string
		policy model.DialoguePolicy
	}{
		{
			name: "mention fanout",
			policy: model.DialoguePolicy{
				Mode: model.DialogueModeMentionFanout, MaxAutonomousTurns: 2, MaxTurnsPerAgent: 1,
				AllowAgentToAgentMentions: true, ResponseStrategy: model.DialogueResponseStrategyMentionedFirst,
			},
		},
		{
			name: "guided dialogue",
			policy: model.DialoguePolicy{
				Mode: model.DialogueModeGuided, MaxAutonomousTurns: 2, MaxTurnsPerAgent: 1,
				AllowAgentToAgentMentions: true, ResponseStrategy: model.DialogueResponseStrategyMentionedFirst,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &teststore.Store{}
			runner := agent.NewRunner(&sequenceLLM{}, store).
				WithRuntimeRegistry(agent.NewRuntimeRegistry(artifactAuditRuntime{}))
			room := newDialogueRuntimeRoom(test.policy, []model.Agent{
				{ID: "research", Name: "Research", Mention: "@Research", Runtime: model.AgentRuntimeDeepAgent, Enabled: true},
				{ID: "reviewer", Name: "Reviewer", Mention: "@Reviewer", Runtime: model.AgentRuntimeDeepAgent, Enabled: true},
			})
			trigger := room.newHumanMessage("Alice", "@Research investigate the topic")
			room.AppendMessage(trigger)

			runner.HandleHumanMessage(context.Background(), room, trigger)

			messages := room.agentMessages()
			if len(messages) != 2 || messages[0].SenderID != "research" || messages[1].SenderID != "reviewer" {
				t.Fatalf("expected human mention followed by Agent mention, got %#v", messages)
			}
			for _, message := range messages {
				if len(message.Artifacts) != 1 || !strings.Contains(message.Artifacts[0].Content, "# ") {
					t.Fatalf("expected saved Markdown artifact, got %#v", message.Artifacts)
				}
			}
			if len(store.AgentRuns) != 2 {
				t.Fatalf("expected two durable Agent runs, got %#v", store.AgentRuns)
			}
			for _, run := range store.AgentRuns {
				if run.Status != "succeeded" || run.ModelSource != "database" || run.ModelName != "research-model" || run.ModelProfileID == "" {
					t.Fatalf("expected persisted DeepAgent audit, got %#v", run)
				}
			}
		})
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
