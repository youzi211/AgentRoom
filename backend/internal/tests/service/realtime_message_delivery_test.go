package service_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/realtime"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/tests/teststore"
)

type blockingFocusClient struct {
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (c *blockingFocusClient) Complete(ctx context.Context, _ []llm.ChatMessage) (string, error) {
	c.calls.Add(1)
	select {
	case c.started <- struct{}{}:
	default:
	}
	select {
	case <-c.release:
		return `[{"content":"排期风险","category":"风险"}]`, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (c *blockingFocusClient) Calls() int32 {
	return c.calls.Load()
}

type responseFocusClient struct {
	response string
	err      error
	calls    atomic.Int32
}

func (c *responseFocusClient) Complete(context.Context, []llm.ChatMessage) (string, error) {
	c.calls.Add(1)
	return c.response, c.err
}

func (c *responseFocusClient) Calls() int32 {
	return c.calls.Load()
}

type failingMessageStore struct {
	*teststore.Store
	err error
}

func (s *failingMessageStore) AddMessage(context.Context, model.Message) (model.Message, error) {
	return model.Message{}, s.err
}

func TestPostRealtimeMessageBroadcastsBeforeFocusAnalysisCompletes(t *testing.T) {
	focusClient := &blockingFocusClient{
		started: make(chan struct{}, 8),
		release: make(chan struct{}),
	}
	roomService, session, cleanup := newRealtimeSession(t, focusClient, &teststore.Store{})
	defer cleanup()

	for i := 0; i < 2; i++ {
		postRealtimeMessage(t, roomService, session, fmt.Sprintf("warmup-%d", i))
	}

	postDone := make(chan error, 1)
	go func() {
		postDone <- roomService.PostRealtimeMessage(context.Background(), session, "trigger")
	}()

	message := waitForRealtimeEvent(t, session, func(event realtime.Event) bool {
		return event.Type == realtime.EventTypeMessage && event.Message != nil && event.Message.Content == "trigger"
	})
	if message.Message == nil || message.Message.Content != "trigger" {
		t.Fatalf("expected trigger message event, got %#v", message)
	}
	select {
	case err := <-postDone:
		if err != nil {
			t.Fatalf("post realtime message: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected PostRealtimeMessage to return without waiting for focus analysis")
	}

	select {
	case <-focusClient.started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected focus analysis to start in the background")
	}
	close(focusClient.release)
	focusEvent := waitForRealtimeEvent(t, session, func(event realtime.Event) bool {
		return event.Type == realtime.EventTypeFocusUpdate && len(event.FocusPoints) > 0
	})
	if focusEvent.FocusPoints[0].Content != "排期风险" {
		t.Fatalf("expected focus update after release, got %#v", focusEvent.FocusPoints)
	}
}

func TestPostRealtimeMessagePersistenceFailureDoesNotBroadcastOrAnalyze(t *testing.T) {
	storeErr := errors.New("message store unavailable")
	focusClient := &responseFocusClient{response: `[{"content":"不会执行","category":"错误"}]`}
	backingStore := &failingMessageStore{Store: &teststore.Store{}, err: storeErr}
	roomService := service.NewRoomService(nil, nil, nil, nil, service.NewFocusService(focusClient), backingStore)
	currentRoom := room.New("room-1", "Planning", nil)
	backingStore.Store.Rooms = map[string]model.RoomMeta{currentRoom.Info().ID: currentRoom.Info()}
	session, cleanup := openRealtimeSession(t, roomService, currentRoom)
	defer cleanup()

	if err := roomService.PostRealtimeMessage(context.Background(), session, "not saved"); !errors.Is(err, storeErr) {
		t.Fatalf("expected persistence error, got %v", err)
	}
	select {
	case event := <-session.Events():
		if event.Type == realtime.EventTypeMessage {
			t.Fatalf("did not expect message event after persistence failure: %#v", event)
		}
	case <-time.After(100 * time.Millisecond):
	}
	if got := len(session.Room().Messages()); got != 0 {
		t.Fatalf("expected failed message not to enter room state, got %d messages", got)
	}
	if got := focusClient.Calls(); got != 0 {
		t.Fatalf("expected focus analysis not to run, got %d calls", got)
	}
}

func TestFocusAnalysisTerminalResultAdvancesCursorWithoutImmediateRetry(t *testing.T) {
	tests := []struct {
		name     string
		response string
		err      error
	}{
		{name: "model error", err: errors.New("model unavailable")},
		{name: "invalid response", response: "invalid json"},
		{name: "empty result", response: "[]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			focusClient := &responseFocusClient{response: test.response, err: test.err}
			roomService, session, cleanup := newRealtimeSession(t, focusClient, &teststore.Store{})
			defer cleanup()

			for i := 0; i < 3; i++ {
				postRealtimeMessage(t, roomService, session, fmt.Sprintf("message-%d", i))
			}
			waitForFocusCalls(t, focusClient.Calls, 1)

			postRealtimeMessage(t, roomService, session, "below-threshold")
			time.Sleep(50 * time.Millisecond)
			if got := focusClient.Calls(); got != 1 {
				t.Fatalf("expected completed analysis cursor to suppress immediate retry, got %d calls", got)
			}

			postRealtimeMessage(t, roomService, session, "next-1")
			postRealtimeMessage(t, roomService, session, "next-2")
			waitForFocusCalls(t, focusClient.Calls, 2)
		})
	}
}

func TestFocusAnalysisDoesNotRunConcurrentlyForOneRoom(t *testing.T) {
	focusClient := &blockingFocusClient{
		started: make(chan struct{}, 8),
		release: make(chan struct{}),
	}
	roomService, session, cleanup := newRealtimeSession(t, focusClient, &teststore.Store{})
	defer cleanup()

	for i := 0; i < 6; i++ {
		postRealtimeMessage(t, roomService, session, fmt.Sprintf("message-%d", i))
	}
	select {
	case <-focusClient.started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected focus analysis to start")
	}
	time.Sleep(50 * time.Millisecond)
	if got := focusClient.Calls(); got != 1 {
		t.Fatalf("expected one in-flight focus analysis per room, got %d calls", got)
	}
	close(focusClient.release)
	waitForFocusCalls(t, focusClient.Calls, 2)
}

func TestMessageBroadcastDoesNotWaitWhenFocusQueueIsFull(t *testing.T) {
	const roomCount = 70
	focusClient := &blockingFocusClient{
		started: make(chan struct{}, roomCount),
		release: make(chan struct{}),
	}
	store := &teststore.Store{}
	roomService := service.NewRoomService(nil, nil, nil, nil, service.NewFocusService(focusClient), store)
	sessions := make([]*service.RealtimeSession, 0, roomCount)
	rooms := make([]*room.Room, 0, roomCount)
	for i := 0; i < roomCount; i++ {
		currentRoom := room.New(fmt.Sprintf("room-%d", i), "Planning", nil)
		store.Rooms = ensureRoomMap(store.Rooms)
		store.Rooms[currentRoom.Info().ID] = currentRoom.Info()
		session, cleanup := openRealtimeSession(t, roomService, currentRoom)
		sessions = append(sessions, session)
		rooms = append(rooms, currentRoom)
		t.Cleanup(cleanup)
	}

	for i := range sessions {
		for messageIndex := 0; messageIndex < 3; messageIndex++ {
			postRealtimeMessage(t, roomService, sessions[i], fmt.Sprintf("room-%d-message-%d", i, messageIndex))
		}
	}

	for i, session := range sessions {
		for messageCount := 0; messageCount < 3; messageCount++ {
			waitForRealtimeEvent(t, session, func(event realtime.Event) bool {
				return event.Type == realtime.EventTypeMessage
			})
		}
		if got := len(rooms[i].Messages()); got != 3 {
			t.Fatalf("room %d expected all messages in memory, got %d", i, got)
		}
	}
	close(focusClient.release)
}

func newRealtimeSession(t *testing.T, focusClient llm.Client, backingStore *teststore.Store) (*service.RoomService, *service.RealtimeSession, func()) {
	currentRoom := room.New("room-1", "Planning", nil)
	roomService := service.NewRoomService(nil, nil, nil, nil, service.NewFocusService(focusClient), backingStore)
	backingStore.Rooms = map[string]model.RoomMeta{currentRoom.Info().ID: currentRoom.Info()}
	session, cleanup := openRealtimeSession(t, roomService, currentRoom)
	return roomService, session, cleanup
}

func openRealtimeSession(t *testing.T, roomService *service.RoomService, currentRoom *room.Room) (*service.RealtimeSession, func()) {
	session, err := roomService.OpenRealtimeSession(context.Background(), currentRoom, "Alice")
	if err != nil {
		t.Fatalf("open realtime session: %v", err)
	}
	select {
	case <-session.Events():
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected initial room snapshot")
	}
	cleanup := func() {
		roomService.CloseRealtimeSession(context.Background(), session)
	}
	return session, cleanup
}

func ensureRoomMap(rooms map[string]model.RoomMeta) map[string]model.RoomMeta {
	if rooms == nil {
		return make(map[string]model.RoomMeta)
	}
	return rooms
}

func postRealtimeMessage(t *testing.T, roomService *service.RoomService, session *service.RealtimeSession, content string) {
	t.Helper()
	if err := roomService.PostRealtimeMessage(context.Background(), session, content); err != nil {
		t.Fatalf("post realtime message %q: %v", content, err)
	}
}

func waitForRealtimeEvent(t *testing.T, session *service.RealtimeSession, match func(realtime.Event) bool) realtime.Event {
	t.Helper()
	deadline := time.NewTimer(1 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case event := <-session.Events():
			if match(event) {
				return event
			}
		case <-deadline.C:
			t.Fatal("timed out waiting for realtime event")
		}
	}
}

func waitForFocusCalls(t *testing.T, calls func() int32, expected int32) {
	t.Helper()
	deadline := time.NewTimer(1 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if calls() >= expected {
			return
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("timed out waiting for %d focus calls, got %d", expected, calls())
		}
	}
}
