package collaboration_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
	"agentroom/backend/internal/collaboration"

)

type controlledRuntime struct {
	started chan startedRun
}

type startedRun struct {
	request collaboration.Request
	finish  chan struct{}
}

func (r *controlledRuntime) ExecuteConversation(ctx context.Context, request collaboration.Request) (collaboration.EventStream, error) {
	control := startedRun{request: request, finish: make(chan struct{})}
	r.started <- control
	return &controlledStream{ctx: ctx, runID: request.CollaborationRunID, finish: control.finish}, nil
}

type controlledStream struct {
	ctx      context.Context
	runID    string
	finish   chan struct{}
	sequence uint64
}

func (s *controlledStream) Recv() (collaboration.Event, error) {
	s.sequence++
	if s.sequence == 1 {
		return event(s.runID, 1, collaboration.EventAccepted), nil
	}
	if s.sequence == 2 {
		select {
		case <-s.ctx.Done():
			return collaboration.Event{}, s.ctx.Err()
		case <-s.finish:
			terminal := event(s.runID, 2, collaboration.EventCompleted)
			terminal.Terminal = &collaboration.Terminal{Reason: collaboration.StopReasonCompleted}
			return terminal, nil
		}
	}
	return collaboration.Event{}, io.EOF
}

func TestCoordinatorBoundsGlobalConcurrencyAndPendingQueue(t *testing.T) {
	runtime := &controlledRuntime{started: make(chan startedRun, 3)}
	coordinator, err := collaboration.NewCoordinator(runtime, collaboration.Config{MaxConcurrent: 2, MaxPending: 0})
	if err != nil {
		t.Fatal(err)
	}

	results := make(chan error, 2)
	for _, roomID := range []string{"room_1", "room_2"} {
		request := request(roomID, "run_"+roomID)
		go func() { results <- coordinator.Execute(context.Background(), request, nil) }()
	}
	first := receiveStart(t, runtime.started)
	second := receiveStart(t, runtime.started)
	assertRooms(t, first.request.Snapshot.Room.ID, second.request.Snapshot.Room.ID, "room_1", "room_2")

	thirdErr := coordinator.Execute(context.Background(), request("room_3", "run_room_3"), nil)
	if !errors.Is(thirdErr, collaboration.ErrCapacity) {
		t.Fatalf("expected bounded capacity rejection, got %v", thirdErr)
	}

	close(first.finish)
	close(second.finish)
	for range 2 {
		if err := receiveResult(t, results); err != nil {
			t.Fatalf("execute collaboration: %v", err)
		}
	}
}

func TestCoordinatorStartsPendingRunAfterCapacityIsReleased(t *testing.T) {
	runtime := &controlledRuntime{started: make(chan startedRun, 2)}
	coordinator, err := collaboration.NewCoordinator(runtime, collaboration.Config{MaxConcurrent: 1, MaxPending: 1})
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	go func() { results <- coordinator.Execute(context.Background(), request("room_1", "run_1"), nil) }()
	first := receiveStart(t, runtime.started)
	go func() { results <- coordinator.Execute(context.Background(), request("room_2", "run_2"), nil) }()

	select {
	case run := <-runtime.started:
		t.Fatalf("pending run started before capacity was released: %q", run.request.CollaborationRunID)
	case <-time.After(20 * time.Millisecond):
	}
	close(first.finish)
	second := receiveStart(t, runtime.started)
	if second.request.CollaborationRunID != "run_2" {
		t.Fatalf("unexpected pending run: %q", second.request.CollaborationRunID)
	}
	close(second.finish)
	for range 2 {
		if err := receiveResult(t, results); err != nil {
			t.Fatalf("execute collaboration: %v", err)
		}
	}
}

func TestCoordinatorPreemptsSameRoomBeforeStartingReplacement(t *testing.T) {
	runtime := &controlledRuntime{started: make(chan startedRun, 2)}
	coordinator, err := collaboration.NewCoordinator(runtime, collaboration.Config{MaxConcurrent: 1, MaxPending: 1})
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)

	go func() { results <- coordinator.Execute(context.Background(), request("room_1", "run_1"), nil) }()
	first := receiveStart(t, runtime.started)
	go func() { results <- coordinator.Execute(context.Background(), request("room_1", "run_2"), nil) }()

	if err := receiveResult(t, results); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected old run cancellation, got %v", err)
	}
	second := receiveStart(t, runtime.started)
	if second.request.CollaborationRunID != "run_2" {
		t.Fatalf("unexpected replacement run: %q", second.request.CollaborationRunID)
	}
	select {
	case <-first.finish:
		t.Fatal("replacement must cancel the old context, not complete its stream")
	default:
	}
	close(second.finish)
	if err := receiveResult(t, results); err != nil {
		t.Fatalf("replacement failed: %v", err)
	}
}

func TestCoordinatorDropsLateEventsBeforeStartingReplacement(t *testing.T) {
	runtime := &lateAfterCancelRuntime{started: make(chan string, 2)}
	coordinator, err := collaboration.NewCoordinator(runtime, collaboration.Config{MaxConcurrent: 1, MaxPending: 1})
	if err != nil {
		t.Fatal(err)
	}
	oldResult := make(chan error, 1)
	newResult := make(chan error, 1)
	oldEvents := make(chan collaboration.EventKind, 8)

	go func() {
		oldResult <- coordinator.Execute(context.Background(), request("room_1", "run_1"), func(_ context.Context, event collaboration.Event) error {
			oldEvents <- event.Kind
			return nil
		})
	}()
	if runID := receiveRunID(t, runtime.started); runID != "run_1" {
		t.Fatalf("unexpected first run: %q", runID)
	}
	waitForEventKind(t, oldEvents, collaboration.EventAgentTurnStarted)

	go func() {
		newResult <- coordinator.Execute(context.Background(), request("room_1", "run_2"), nil)
	}()
	if err := receiveResult(t, oldResult); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected old run cancellation, got %v", err)
	}
	if runID := receiveRunID(t, runtime.started); runID != "run_2" {
		t.Fatalf("unexpected replacement run: %q", runID)
	}
	if err := receiveResult(t, newResult); err != nil {
		t.Fatalf("replacement failed: %v", err)
	}
	close(oldEvents)
	for kind := range oldEvents {
		if kind == collaboration.EventAgentMessageCompleted {
			t.Fatal("late Agent message reached the old run handler after cancellation")
		}
	}
}

func TestCoordinatorRejectsDuplicateRunAndMissingTerminal(t *testing.T) {
	runtime := &controlledRuntime{started: make(chan startedRun, 2)}
	coordinator, err := collaboration.NewCoordinator(runtime, collaboration.Config{MaxConcurrent: 1, MaxPending: 1})
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 1)
	go func() { results <- coordinator.Execute(context.Background(), request("room_1", "run_1"), nil) }()
	first := receiveStart(t, runtime.started)
	if err := coordinator.Execute(context.Background(), request("room_1", "run_1"), nil); !errors.Is(err, collaboration.ErrDuplicateRun) {
		t.Fatalf("expected duplicate run error, got %v", err)
	}
	close(first.finish)
	if err := receiveResult(t, results); err != nil {
		t.Fatal(err)
	}

	missingTerminal := staticRuntime{events: []collaboration.Event{event("run_2", 1, collaboration.EventAccepted)}}
	coordinator, err = collaboration.NewCoordinator(missingTerminal, collaboration.Config{MaxConcurrent: 1})
	if err != nil {
		t.Fatal(err)
	}
	err = coordinator.Execute(context.Background(), request("room_2", "run_2"), nil)
	if err == nil || !strings.Contains(err.Error(), "without a terminal") {
		t.Fatalf("expected missing terminal error, got %v", err)
	}
}

func TestCoordinatorCancelsRuntimeWhenHandlerFails(t *testing.T) {
	runtime := &cancellationRuntime{cancelled: make(chan struct{})}
	coordinator, err := collaboration.NewCoordinator(runtime, collaboration.Config{MaxConcurrent: 1})
	if err != nil {
		t.Fatal(err)
	}
	handlerErr := errors.New("commit failed")
	err = coordinator.Execute(context.Background(), request("room_1", "run_1"), func(context.Context, collaboration.Event) error {
		return handlerErr
	})
	if !errors.Is(err, handlerErr) {
		t.Fatalf("expected handler error, got %v", err)
	}
	select {
	case <-runtime.cancelled:
	case <-time.After(time.Second):
		t.Fatal("runtime did not observe handler cancellation")
	}
}

func TestCoordinatorCancelRoomCancelsAndWaitsForActiveRun(t *testing.T) {
	runtime := &controlledRuntime{started: make(chan startedRun, 1)}
	coordinator, err := collaboration.NewCoordinator(runtime, collaboration.Config{MaxConcurrent: 1})
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- coordinator.Execute(context.Background(), request("room_1", "run_1"), nil) }()
	receiveStart(t, runtime.started)

	if err := coordinator.CancelRoom(context.Background(), "room_1"); err != nil {
		t.Fatal(err)
	}
	if err := receiveResult(t, result); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected active run cancellation, got %v", err)
	}
	if err := coordinator.CancelRoom(context.Background(), "room_1"); err != nil {
		t.Fatalf("cancel inactive room: %v", err)
	}
}

func TestCoordinatorShutdownCancelsAllRunsAndRejectsNewWork(t *testing.T) {
	runtime := &controlledRuntime{started: make(chan startedRun, 2)}
	coordinator, err := collaboration.NewCoordinator(runtime, collaboration.Config{MaxConcurrent: 2})
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for _, roomID := range []string{"room_1", "room_2"} {
		go func() { results <- coordinator.Execute(context.Background(), request(roomID, "run_"+roomID), nil) }()
	}
	receiveStart(t, runtime.started)
	receiveStart(t, runtime.started)

	if err := coordinator.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := receiveResult(t, results); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected shutdown cancellation, got %v", err)
		}
	}
	if err := coordinator.Execute(context.Background(), request("room_3", "run_3"), nil); !errors.Is(err, collaboration.ErrClosed) {
		t.Fatalf("expected closed coordinator error, got %v", err)
	}
}

func TestCoordinatorPropagatesDeadlineCancellation(t *testing.T) {
	runtime := &controlledRuntime{started: make(chan startedRun, 1)}
	coordinator, err := collaboration.NewCoordinator(runtime, collaboration.Config{MaxConcurrent: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- coordinator.Execute(ctx, request("room_1", "run_1"), nil) }()
	receiveStart(t, runtime.started)
	if err := receiveResult(t, result); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline cancellation, got %v", err)
	}
}

type staticRuntime struct {
	events []collaboration.Event
}

func (r staticRuntime) ExecuteConversation(context.Context, collaboration.Request) (collaboration.EventStream, error) {
	return &staticStream{events: r.events}, nil
}

type staticStream struct {
	events []collaboration.Event
	next   int
}

func (s *staticStream) Recv() (collaboration.Event, error) {
	if s.next >= len(s.events) {
		return collaboration.Event{}, io.EOF
	}
	event := s.events[s.next]
	s.next++
	return event, nil
}

type cancellationRuntime struct {
	cancelled chan struct{}
	once      sync.Once
}

func (r *cancellationRuntime) ExecuteConversation(ctx context.Context, request collaboration.Request) (collaboration.EventStream, error) {
	go func() {
		<-ctx.Done()
		r.once.Do(func() { close(r.cancelled) })
	}()
	return &cancellationStream{runID: request.CollaborationRunID}, nil
}

type cancellationStream struct {
	runID    string
	accepted bool
}

type lateAfterCancelRuntime struct {
	started chan string
}

func (r *lateAfterCancelRuntime) ExecuteConversation(ctx context.Context, request collaboration.Request) (collaboration.EventStream, error) {
	r.started <- request.CollaborationRunID
	if request.CollaborationRunID == "run_1" {
		return &lateAfterCancelStream{ctx: ctx, runID: request.CollaborationRunID}, nil
	}
	return &staticStream{events: []collaboration.Event{
		event(request.CollaborationRunID, 1, collaboration.EventAccepted),
		event(request.CollaborationRunID, 2, collaboration.EventCollaborationStarted),
		terminalEvent(request.CollaborationRunID, 3),
	}}, nil
}

type lateAfterCancelStream struct {
	ctx   context.Context
	runID string
	next  uint64
}

func (s *lateAfterCancelStream) Recv() (collaboration.Event, error) {
	s.next++
	switch s.next {
	case 1:
		return event(s.runID, s.next, collaboration.EventAccepted), nil
	case 2:
		return event(s.runID, s.next, collaboration.EventCollaborationStarted), nil
	case 3:
		selected := event(s.runID, s.next, collaboration.EventSpeakerSelected)
		selected.TurnID = "turn_1"
		selected.AgentID = "agent_1"
		return selected, nil
	case 4:
		started := event(s.runID, s.next, collaboration.EventAgentTurnStarted)
		started.TurnID = "turn_1"
		started.AgentID = "agent_1"
		return started, nil
	case 5:
		<-s.ctx.Done()
		completed := event(s.runID, s.next, collaboration.EventAgentMessageCompleted)
		completed.TurnID = "turn_1"
		completed.AgentID = "agent_1"
		completed.Message = &collaboration.AgentMessage{Content: "late reply"}
		return completed, nil
	default:
		return collaboration.Event{}, io.EOF
	}
}

func (s *cancellationStream) Recv() (collaboration.Event, error) {
	if !s.accepted {
		s.accepted = true
		return event(s.runID, 1, collaboration.EventAccepted), nil
	}
	return collaboration.Event{}, io.EOF
}

func request(roomID, runID string) collaboration.Request {
	return collaboration.Request{
		ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: runID,
		Snapshot: collaboration.ConversationSnapshot{
			Room:   collaboration.RoomSnapshot{ID: roomID},
			Agents: []collaboration.AgentSnapshot{{ID: "agent_1"}},
			Policy: collaboration.PolicySnapshot{MaxTurns: 1, MaxTurnsPerAgent: 1},
		},
	}
}

func event(runID string, sequence uint64, kind collaboration.EventKind) collaboration.Event {
	return collaboration.Event{
		ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: runID, Sequence: sequence, Kind: kind,
	}
}

func terminalEvent(runID string, sequence uint64) collaboration.Event {
	terminal := event(runID, sequence, collaboration.EventCompleted)
	terminal.Terminal = &collaboration.Terminal{Reason: collaboration.StopReasonCompleted}
	return terminal
}

func receiveStart(t *testing.T, started <-chan startedRun) startedRun {
	t.Helper()
	select {
	case run := <-started:
		return run
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Runtime start")
		return startedRun{}
	}
}

func receiveRunID(t *testing.T, started <-chan string) string {
	t.Helper()
	select {
	case runID := <-started:
		return runID
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Runtime start")
		return ""
	}
}

func waitForEventKind(t *testing.T, events <-chan collaboration.EventKind, want collaboration.EventKind) {
	t.Helper()
	timeout := time.After(time.Second)
	for {
		select {
		case kind := <-events:
			if kind == want {
				return
			}
		case <-timeout:
			t.Fatalf("timed out waiting for event %q", want)
		}
	}
}

func receiveResult(t *testing.T, results <-chan error) error {
	t.Helper()
	select {
	case err := <-results:
		return err
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for execution result")
		return nil
	}
}

func assertRooms(t *testing.T, first, second string, expected ...string) {
	t.Helper()
	allowed := make(map[string]bool, len(expected))
	for _, roomID := range expected {
		allowed[roomID] = true
	}
	if first == second || !allowed[first] || !allowed[second] {
		t.Fatalf("unexpected started rooms: %q, %q", first, second)
	}
}
