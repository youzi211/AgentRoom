package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agentroom/backend/internal/api"
	"agentroom/backend/internal/model"
)

func TestRecentRoomsArePublicAndContainEntryFields(t *testing.T) {
	server, _, _ := newActivityTestServer(t, api.Config{AdminAPIKey: "secret"})
	created := createActivityRoom(t, server, `{"name":"Entry room","passcode":"open-sesame","dialoguePolicy":{"mode":"guided_dialogue"}}`)

	request := httptest.NewRequest(http.MethodGet, "/api/recent-rooms?limit=3", nil)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected public recent rooms response 200, got %d body=%s", response.Code, response.Body.String())
	}

	var payload struct {
		Rooms []struct {
			ID             string               `json:"id"`
			Name           string               `json:"name"`
			Status         string               `json:"status"`
			HasPasscode    bool                 `json:"hasPasscode"`
			DialoguePolicy model.DialoguePolicy `json:"dialoguePolicy"`
			AgentCount     int                  `json:"agentCount"`
		} `json:"rooms"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode recent rooms response: %v", err)
	}
	if len(payload.Rooms) != 1 {
		t.Fatalf("expected one recent room, got %#v", payload.Rooms)
	}
	got := payload.Rooms[0]
	if got.ID != created.Room.ID || got.Name != "Entry room" || got.Status != model.RoomStatusActive {
		t.Fatalf("unexpected recent room payload: %#v", got)
	}
	if !got.HasPasscode {
		t.Fatalf("expected passcode status in public summary, got %#v", got)
	}
	if got.DialoguePolicy.Mode != model.DialogueModeGuided {
		t.Fatalf("expected dialogue mode in public summary, got %#v", got.DialoguePolicy)
	}
	if got.AgentCount == 0 {
		t.Fatalf("expected agent count in public summary, got %#v", got)
	}
}

func TestAdminRoomListingRemainsProtected(t *testing.T) {
	server, _, _ := newActivityTestServer(t, api.Config{AdminAPIKey: "secret"})

	request := httptest.NewRequest(http.MethodGet, "/api/rooms?status=active", nil)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected admin room listing to require admin key, got %d body=%s", response.Code, response.Body.String())
	}
}
