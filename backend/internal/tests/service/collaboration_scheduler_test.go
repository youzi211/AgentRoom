package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"agentroom/backend/internal/collaboration"
	"agentroom/backend/internal/collaborationcoordinator"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/realtime"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/tests/teststore"
)

type capturingCollaborationCoordinator struct {
	mu       sync.Mutex
	requests []collaboration.Request
	events   func(collaboration.Request) []collaboration.Event
	err      error
}

func (c *capturingCollaborationCoordinator) Execute(ctx context.Context, request collaboration.Request, handler collaborationcoordinator.EventHandler) error {
	c.mu.Lock()
	c.requests = append(c.requests, request)
	events := c.events
	err := c.err
	c.mu.Unlock()
	if events != nil {
		for _, event := range events(request) {
			if handleErr := handler(ctx, event); handleErr != nil {
				return handleErr
			}
		}
	}
	return err
}

func (c *capturingCollaborationCoordinator) Requests() []collaboration.Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]collaboration.Request(nil), c.requests...)
}

type staticCollaborationModelResolver struct{}

func (staticCollaborationModelResolver) Resolve(_ context.Context, scope string, explicitID string) (service.ResolvedModelConfig, error) {
	profileID := explicitID
	if profileID == "" {
		profileID = "default-" + scope
	}
	return service.ResolvedModelConfig{
		ProfileID: profileID,
		Source:    "database",
		ModelName: "test-model",
	}, nil
}

func TestRemoteCollaborationSchedulerMentionOnlySkipsUnmentionedMessage(t *testing.T) {
	coordinator := &capturingCollaborationCoordinator{}
	store := &teststore.Store{}
	scheduler := newTestCollaborationScheduler(t, coordinator, store)
	currentRoom, trigger := collaborationTestRoom(model.CollaborationTriggerMentionOnly, "please review this")

	if err := scheduler.HandleHumanMessage(context.Background(), currentRoom, trigger); err != nil {
		t.Fatal(err)
	}
	if got := len(coordinator.Requests()); got != 0 {
		t.Fatalf("expected no remote run without a mention, got %d", got)
	}
	if got := len(store.CollaborationRuns); got != 0 {
		t.Fatalf("expected no collaboration audit, got %d", got)
	}
}

func TestRemoteCollaborationSchedulerAutomaticStartsWithoutMention(t *testing.T) {
	coordinator := &capturingCollaborationCoordinator{events: terminalCollaborationEvents}
	store := &teststore.Store{}
	scheduler := newTestCollaborationScheduler(t, coordinator, store)
	currentRoom, trigger := collaborationTestRoom(model.CollaborationTriggerAutomatic, "please review this")

	if err := scheduler.HandleHumanMessage(context.Background(), currentRoom, trigger); err != nil {
		t.Fatal(err)
	}
	requests := coordinator.Requests()
	if len(requests) != 1 {
		t.Fatalf("expected one remote run, got %d", len(requests))
	}
	if len(requests[0].Snapshot.InitialCandidateAgentIDs) != 0 {
		t.Fatalf("expected Engine-selected first speaker, got %#v", requests[0].Snapshot.InitialCandidateAgentIDs)
	}
	if len(store.CollaborationRuns) != 1 || store.CollaborationRuns[0].Status != model.CollaborationRunStatusSucceeded {
		t.Fatalf("unexpected collaboration audit: %#v", store.CollaborationRuns)
	}
	if len(store.DialogueRuns) != 0 {
		t.Fatalf("expected new remote collaboration to skip legacy dialogue_runs, got %#v", store.DialogueRuns)
	}
}

func TestRemoteCollaborationSchedulerDoesNotRetryStartedRunAfterRuntimeError(t *testing.T) {
	coordinator := &capturingCollaborationCoordinator{
		events: func(request collaboration.Request) []collaboration.Event {
			now := time.Now().UTC()
			return []collaboration.Event{
				{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 1, OccurredAt: now, Kind: collaboration.EventAccepted},
				{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 2, OccurredAt: now, Kind: collaboration.EventCollaborationStarted},
			}
		},
		err: errors.New("runtime failed after model start"),
	}
	store := &teststore.Store{}
	scheduler := newTestCollaborationScheduler(t, coordinator, store)
	currentRoom, trigger := collaborationTestRoom(model.CollaborationTriggerAutomatic, "please review this")

	if err := scheduler.HandleHumanMessage(context.Background(), currentRoom, trigger); err == nil {
		t.Fatal("expected started runtime error to be returned")
	}
	if got := len(coordinator.Requests()); got != 1 {
		t.Fatalf("expected no retry after started runtime error, got %d requests", got)
	}
	if len(store.CollaborationRuns) != 1 {
		t.Fatalf("expected one collaboration audit, got %#v", store.CollaborationRuns)
	}
	run := store.CollaborationRuns[0]
	if run.Status != model.CollaborationRunStatusFailed || run.StopReason != model.CollaborationStopReasonEngineFailure {
		t.Fatalf("expected failed run without fallback retry, got %#v", run)
	}
	if len(store.DialogueRuns) != 0 {
		t.Fatalf("expected no legacy dialogue_runs fallback, got %#v", store.DialogueRuns)
	}
}

func TestRemoteCollaborationSchedulerUsesMentionsAsOrderedCandidates(t *testing.T) {
	coordinator := &capturingCollaborationCoordinator{events: terminalCollaborationEvents}
	scheduler := newTestCollaborationScheduler(t, coordinator, &teststore.Store{})
	currentRoom, trigger := collaborationTestRoom(model.CollaborationTriggerMentionOnly, "@Engineer then @Product")

	if err := scheduler.HandleHumanMessage(context.Background(), currentRoom, trigger); err != nil {
		t.Fatal(err)
	}
	requests := coordinator.Requests()
	if len(requests) != 1 {
		t.Fatalf("expected one remote run, got %d", len(requests))
	}
	want := []string{"engineer", "product"}
	got := requests[0].Snapshot.InitialCandidateAgentIDs
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("unexpected mention candidates: got %#v want %#v", got, want)
	}
}

func TestRemoteCollaborationSchedulerBroadcastsOnlyCommittedAgentMessage(t *testing.T) {
	coordinator := &capturingCollaborationCoordinator{events: completedMessageEvents}
	store := &teststore.Store{}
	scheduler := newTestCollaborationScheduler(t, coordinator, store)
	currentRoom, trigger := collaborationTestRoom(model.CollaborationTriggerMentionOnly, "@Product please review")
	client := &room.Client{ID: "client-1", Send: make(chan realtime.Event, 16)}
	currentRoom.Events().Register(client)
	defer currentRoom.Events().Unregister(client)

	if err := scheduler.HandleHumanMessage(context.Background(), currentRoom, trigger); err != nil {
		t.Fatal(err)
	}
	messages := currentRoom.Messages()
	if len(messages) != 2 || messages[1].Content != "Approved" {
		t.Fatalf("expected committed Agent message in room state, got %#v", messages)
	}
	timeout := time.After(time.Second)
	for {
		select {
		case event := <-client.Send:
			if event.Type != realtime.EventTypeMessage {
				continue
			}
			if event.Message == nil || event.Message.Content != "Approved" {
				t.Fatalf("unexpected message event: %#v", event)
			}
			return
		case <-timeout:
			t.Fatal("timed out waiting for committed Agent message broadcast")
		}
	}
}

func TestRemoteCollaborationSchedulerBroadcastsSafeCollaborationActivities(t *testing.T) {
	coordinator := &capturingCollaborationCoordinator{events: collaborationActivityEvents}
	store := &teststore.Store{}
	scheduler := newTestCollaborationScheduler(t, coordinator, store)
	currentRoom, trigger := collaborationTestRoom(model.CollaborationTriggerMentionOnly, "@Product please review")
	client := &room.Client{ID: "client-1", Send: make(chan realtime.Event, 16)}
	currentRoom.Events().Register(client)
	defer currentRoom.Events().Unregister(client)

	if err := scheduler.HandleHumanMessage(context.Background(), currentRoom, trigger); err != nil {
		t.Fatal(err)
	}

	var activities []realtime.CollaborationActivity
	var messages []model.Message
	for len(client.Send) > 0 {
		event := <-client.Send
		if event.Type == realtime.EventTypeCollaborationActivity && event.Collaboration != nil {
			activities = append(activities, *event.Collaboration)
		}
		if event.Type == realtime.EventTypeMessage && event.Message != nil {
			messages = append(messages, *event.Message)
		}
	}
	wantKinds := []string{
		string(collaboration.EventCollaborationStarted),
		string(collaboration.EventSpeakerSelected),
		string(collaboration.EventAgentTurnStarted),
		string(collaboration.EventHandoffRequested),
		string(collaboration.EventAgentMessageCompleted),
		string(collaboration.EventCompleted),
	}
	if len(activities) != len(wantKinds) {
		t.Fatalf("unexpected collaboration activities: %#v", activities)
	}
	for index, want := range wantKinds {
		if activities[index].Kind != want {
			t.Fatalf("activity %d kind = %q, want %q", index, activities[index].Kind, want)
		}
		if activities[index].CollaborationRunID == "" || activities[index].TriggerMessageID != trigger.ID {
			t.Fatalf("activity %d missing run correlation: %#v", index, activities[index])
		}
	}
	if got := activities[1]; got.AgentID != "product" || got.AgentName != "Product" {
		t.Fatalf("unexpected speaker activity: %#v", got)
	}
	if got := activities[3]; got.TargetAgentID != "engineer" || got.TargetAgentName != "Engineer" || got.ReasonCategory != "delegation" {
		t.Fatalf("unexpected handoff activity: %#v", got)
	}
	if got := activities[len(activities)-1]; got.StopReason != string(collaboration.StopReasonCompleted) || got.TurnCount != 1 {
		t.Fatalf("unexpected terminal activity: %#v", got)
	}
	if len(messages) != 1 || messages[0].Content != "Approved" {
		t.Fatalf("expected only the committed Agent message, got %#v", messages)
	}
}

func newTestCollaborationScheduler(t *testing.T, coordinator *capturingCollaborationCoordinator, store *teststore.Store) *service.RemoteCollaborationScheduler {
	t.Helper()
	scheduler, err := service.NewRemoteCollaborationScheduler(coordinator, store, staticCollaborationModelResolver{}, nil, service.RemoteCollaborationConfig{
		TranscriptLimit: 30,
		EngineVersions: map[string]string{
			model.CollaborationEngineNative: "native-v1",
		},
		Limits: collaboration.ExecutionLimits{
			Timeout: 30 * time.Second, MaxOutputBytes: 1 << 20, MaxArtifactBytes: 4 << 20,
			MaxToolSteps: 32, MaxRequestBytes: 8 << 20, MaxEventBytes: 4 << 20, MaxCheckpointBytes: 1 << 20,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return scheduler
}

func collaborationTestRoom(triggerMode string, content string) (*room.Room, model.Message) {
	currentRoom := room.New("room-1", "Planning", []model.Agent{
		{ID: "product", Name: "Product", Mention: "@Product", Runtime: model.AgentRuntimeLLM, SystemPrompt: "Product prompt", Enabled: true},
		{ID: "engineer", Name: "Engineer", Mention: "@Engineer", Runtime: model.AgentRuntimeLLM, SystemPrompt: "Engineer prompt", Enabled: true},
	})
	policy := model.DefaultCollaborationPolicy()
	policy.TriggerMode = triggerMode
	currentRoom.SetCollaborationPolicy(policy)
	trigger := model.Message{
		ID: "message-1", RoomID: currentRoom.Info().ID, SenderID: "human-1", SenderName: "Alice",
		SenderType: model.SenderTypeHuman, Content: content, CreatedAt: time.Now().UTC(),
	}
	currentRoom.AppendMessage(trigger)
	return currentRoom, trigger
}

func terminalCollaborationEvents(request collaboration.Request) []collaboration.Event {
	now := time.Now().UTC()
	return []collaboration.Event{
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 1, OccurredAt: now, Kind: collaboration.EventAccepted},
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 2, OccurredAt: now, Kind: collaboration.EventCollaborationStarted},
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 3, OccurredAt: now, Kind: collaboration.EventCompleted, Terminal: &collaboration.Terminal{Reason: collaboration.StopReasonCompleted}},
	}
}

func completedMessageEvents(request collaboration.Request) []collaboration.Event {
	now := time.Now().UTC()
	return []collaboration.Event{
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 1, OccurredAt: now, Kind: collaboration.EventAccepted},
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 2, OccurredAt: now, Kind: collaboration.EventCollaborationStarted},
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 3, OccurredAt: now, Kind: collaboration.EventSpeakerSelected, TurnID: "turn-1", AgentID: "product"},
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 4, OccurredAt: now, Kind: collaboration.EventAgentTurnStarted, TurnID: "turn-1", AgentID: "product"},
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 5, OccurredAt: now, Kind: collaboration.EventAgentMessageCompleted, TurnID: "turn-1", AgentID: "product", Message: &collaboration.AgentMessage{Content: "Approved", Model: collaboration.ModelAudit{ProfileID: "default-go", Source: "database", ModelName: "test-model"}}},
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 6, OccurredAt: now, Kind: collaboration.EventCompleted, Terminal: &collaboration.Terminal{TurnCount: 1, Reason: collaboration.StopReasonCompleted}},
	}
}

func collaborationActivityEvents(request collaboration.Request) []collaboration.Event {
	now := time.Now().UTC()
	return []collaboration.Event{
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 1, OccurredAt: now, Kind: collaboration.EventAccepted},
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 2, OccurredAt: now, Kind: collaboration.EventCollaborationStarted},
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 3, OccurredAt: now, Kind: collaboration.EventOutputDelta, TurnID: "turn-1", AgentID: "product", OutputDelta: "internal partial output"},
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 4, OccurredAt: now, Kind: collaboration.EventSpeakerSelected, TurnID: "turn-1", AgentID: "product", ReasonCategory: "explicit_mention"},
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 5, OccurredAt: now, Kind: collaboration.EventAgentTurnStarted, TurnID: "turn-1", AgentID: "product"},
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 6, OccurredAt: now, Kind: collaboration.EventHandoffRequested, TurnID: "turn-1", AgentID: "product", Handoff: &collaboration.Handoff{TargetAgentID: "engineer", ReasonCategory: "delegation"}},
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 7, OccurredAt: now, Kind: collaboration.EventCheckpoint, Checkpoint: &collaboration.Checkpoint{Payload: []byte("opaque internal state")}},
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 8, OccurredAt: now, Kind: collaboration.EventAgentMessageCompleted, TurnID: "turn-1", AgentID: "product", Message: &collaboration.AgentMessage{Content: "Approved", Model: collaboration.ModelAudit{ProfileID: "default-go", Source: "database", ModelName: "test-model"}}},
		{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: request.CollaborationRunID, Sequence: 9, OccurredAt: now, Kind: collaboration.EventCompleted, Terminal: &collaboration.Terminal{TurnCount: 1, Reason: collaboration.StopReasonCompleted}},
	}
}
