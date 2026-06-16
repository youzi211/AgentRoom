package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"agentroom/backend/internal/api"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"

	"github.com/gorilla/websocket"
)

func TestRoomWebSocketOwnerTransferAndCloseLifecycle(t *testing.T) {
	server, backingStore := newActivityTestServer(t, api.Config{})
	roomID := createRoomForTest(t, server, "WebSocket room")
	httpServer := httptest.NewServer(server.Routes())
	defer httpServer.Close()

	alice := dialRoomSocket(t, httpServer.URL, roomID, "Alice")
	defer alice.Close()
	aliceSnapshot := waitForServerEvent(t, alice, 2*time.Second, func(event model.ServerEvent) bool {
		return event.Type == model.EventTypeRoomSnapshot && event.ParticipantID != ""
	})
	aliceID := aliceSnapshot.ParticipantID
	if aliceSnapshot.Room == nil || aliceSnapshot.Room.OwnerParticipantID != aliceID {
		t.Fatalf("expected first participant to become owner, got %#v", aliceSnapshot.Room)
	}

	bob := dialRoomSocket(t, httpServer.URL, roomID, "Bob")
	defer bob.Close()
	bobSnapshot := waitForServerEvent(t, bob, 2*time.Second, func(event model.ServerEvent) bool {
		return event.Type == model.EventTypeRoomSnapshot && event.ParticipantID != ""
	})
	bobID := bobSnapshot.ParticipantID
	if bobSnapshot.Room == nil || bobSnapshot.Room.OwnerParticipantID != aliceID || len(bobSnapshot.Participants) != 2 {
		t.Fatalf("expected second participant snapshot to preserve Alice as owner, got %#v", bobSnapshot)
	}

	waitForServerEvent(t, alice, 2*time.Second, func(event model.ServerEvent) bool {
		return event.Type == model.EventTypeRoomSnapshot && event.Room != nil && event.Room.OwnerParticipantID == aliceID && len(event.Participants) == 2
	})

	if err := alice.WriteJSON(model.ClientEvent{Type: "transfer_owner", ParticipantID: bobID}); err != nil {
		t.Fatalf("transfer owner request: %v", err)
	}

	aliceOwnerSnapshot := waitForServerEvent(t, alice, 2*time.Second, func(event model.ServerEvent) bool {
		return event.Type == model.EventTypeRoomSnapshot && event.Room != nil && event.Room.OwnerParticipantID == bobID
	})
	if aliceOwnerSnapshot.Room.Status != model.RoomStatusActive {
		t.Fatalf("expected room to stay active during owner transfer, got %#v", aliceOwnerSnapshot.Room)
	}
	waitForServerEvent(t, bob, 2*time.Second, func(event model.ServerEvent) bool {
		return event.Type == model.EventTypeRoomSnapshot && event.Room != nil && event.Room.OwnerParticipantID == bobID
	})

	if err := alice.WriteJSON(model.ClientEvent{Type: "close_room"}); err != nil {
		t.Fatalf("non-owner close request: %v", err)
	}
	ownerError := waitForServerEvent(t, alice, 2*time.Second, func(event model.ServerEvent) bool {
		return event.Type == model.EventTypeError
	})
	if !strings.Contains(ownerError.Error, "only the current meeting owner") {
		t.Fatalf("expected non-owner close to fail with owner error, got %#v", ownerError)
	}

	if err := bob.WriteJSON(model.ClientEvent{Type: "close_room"}); err != nil {
		t.Fatalf("owner close request: %v", err)
	}

	closedForAlice := waitForServerEvent(t, alice, 2*time.Second, func(event model.ServerEvent) bool {
		return event.Type == model.EventTypeRoomClosed
	})
	closedForBob := waitForServerEvent(t, bob, 2*time.Second, func(event model.ServerEvent) bool {
		return event.Type == model.EventTypeRoomClosed
	})
	if closedForAlice.Room == nil || closedForAlice.Room.Status != model.RoomStatusClosed {
		t.Fatalf("expected room_closed payload for Alice, got %#v", closedForAlice)
	}
	if closedForBob.Room == nil || closedForBob.Room.ClosedReason != model.RoomClosedReasonManual {
		t.Fatalf("expected manual close payload for Bob, got %#v", closedForBob)
	}

	waitForSocketClosure(t, alice, 2*time.Second)
	waitForSocketClosure(t, bob, 2*time.Second)

	currentRoom, ok := server.RoomsForTest().GetRoom(context.Background(), roomID)
	if !ok {
		t.Fatal("expected room to still resolve after close")
	}
	if !currentRoom.Info().IsClosed() || len(currentRoom.Participants()) != 0 {
		t.Fatalf("expected closed room with cleared participants, got room=%#v participants=%#v", currentRoom.Info(), currentRoom.Participants())
	}
	if backingStore.Rooms[roomID].ClosedReason != model.RoomClosedReasonManual {
		t.Fatalf("expected store to persist manual close, got %#v", backingStore.Rooms[roomID])
	}
}

func TestRoomWebSocketRejectsClosedRoomJoin(t *testing.T) {
	server, backingStore := newActivityTestServer(t, api.Config{})
	roomID := createRoomForTest(t, server, "Closed socket room")
	currentRoom, ok := server.RoomsForTest().GetRoom(context.Background(), roomID)
	if !ok {
		t.Fatal("expected room to exist")
	}

	closedAt := time.Date(2026, 6, 16, 13, 0, 0, 0, time.UTC)
	currentRoom.ApplyLifecycle(room.LifecycleState{
		Status:       model.RoomStatusClosed,
		ClosedAt:     &closedAt,
		ClosedReason: model.RoomClosedReasonManual,
	})
	backingStore.Rooms[roomID] = currentRoom.Info()

	httpServer := httptest.NewServer(server.Routes())
	defer httpServer.Close()

	socketURL := roomSocketURL(t, httpServer.URL, roomID, "Alice")
	connection, response, err := websocket.DefaultDialer.Dial(socketURL, nil)
	if connection != nil {
		connection.Close()
	}
	if response != nil {
		defer response.Body.Close()
	}
	if err == nil {
		t.Fatal("expected closed room websocket handshake to fail")
	}
	if response == nil || response.StatusCode != http.StatusConflict {
		t.Fatalf("expected closed room websocket rejection to return 409, got response=%v err=%v", response, err)
	}
}

func TestRoomWebSocketArchiveBroadcastsAndDisconnectsLiveClients(t *testing.T) {
	server := newTestServer(t, api.Config{AdminAPIKey: "secret"})
	roomID := createRoomForTest(t, server, "Archive socket room")
	httpServer := httptest.NewServer(server.Routes())
	defer httpServer.Close()

	alice := dialRoomSocket(t, httpServer.URL, roomID, "Alice")
	defer alice.Close()
	waitForServerEvent(t, alice, 2*time.Second, func(event model.ServerEvent) bool {
		return event.Type == model.EventTypeRoomSnapshot && event.ParticipantID != ""
	})

	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/rooms/%s/archive", httpServer.URL, roomID), nil)
	if err != nil {
		t.Fatalf("build archive request: %v", err)
	}
	request.Header.Set("X-Admin-Key", "secret")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("archive room request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected archive request to succeed, got %d", response.StatusCode)
	}

	archivedEvent := waitForServerEvent(t, alice, 2*time.Second, func(event model.ServerEvent) bool {
		return event.Type == model.EventTypeRoomArchived
	})
	if archivedEvent.Room == nil || archivedEvent.Room.Status != model.RoomStatusArchived {
		t.Fatalf("expected room_archived payload, got %#v", archivedEvent)
	}
	waitForSocketClosure(t, alice, 2*time.Second)
}

func dialRoomSocket(t *testing.T, baseURL string, roomID string, participantName string) *websocket.Conn {
	t.Helper()

	socketURL := roomSocketURL(t, baseURL, roomID, participantName)
	connection, response, err := websocket.DefaultDialer.Dial(socketURL, nil)
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial websocket %s: %v", socketURL, err)
	}
	return connection
}

func roomSocketURL(t *testing.T, baseURL string, roomID string, participantName string) string {
	t.Helper()

	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse base url %q: %v", baseURL, err)
	}
	if parsed.Scheme == "https" {
		parsed.Scheme = "wss"
	} else {
		parsed.Scheme = "ws"
	}
	parsed.Path = fmt.Sprintf("/api/rooms/%s/ws", roomID)
	query := parsed.Query()
	query.Set("name", participantName)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func waitForServerEvent(t *testing.T, connection *websocket.Conn, timeout time.Duration, match func(model.ServerEvent) bool) model.ServerEvent {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for {
		if err := connection.SetReadDeadline(deadline); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}

		var event model.ServerEvent
		if err := connection.ReadJSON(&event); err != nil {
			t.Fatalf("read websocket event: %v", err)
		}
		if match(event) {
			return event
		}
	}
}

func waitForSocketClosure(t *testing.T, connection *websocket.Conn, timeout time.Duration) {
	t.Helper()

	if err := connection.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatalf("set close read deadline: %v", err)
	}
	var event model.ServerEvent
	err := connection.ReadJSON(&event)
	if err == nil {
		payload, marshalErr := json.Marshal(event)
		if marshalErr != nil {
			t.Fatalf("expected websocket closure, got extra event %#v", event)
		}
		t.Fatalf("expected websocket closure, got extra event %s", payload)
	}
}
