package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"agentroom/backend/internal/api"
	"agentroom/backend/internal/store"
)

func TestDeleteAgentNotFoundReturnsTyped404(t *testing.T) {
	server, _, backingStore := newActivityTestServer(t, api.Config{AdminAPIKey: "secret"})
	backingStore.DeleteAgentErr = fmt.Errorf("delete failed: %w", store.ErrAgentNotFound)

	request := httptest.NewRequest(http.MethodDelete, "/api/agents/agent_missing", nil)
	request.Header.Set("X-Admin-Key", "secret")
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected typed not-found delete to return 404, got %d body=%s", response.Code, response.Body.String())
	}
}

func TestUploadRoomKnowledgeMissingRoomReturnsTyped404(t *testing.T) {
	server := newTestServer(t, api.Config{AdminAPIKey: "secret"})

	request := multipartRequest(t, http.MethodPost, "/api/rooms/room_missing/knowledge", "notes.md", "# missing room")
	request.Header.Set("X-Admin-Key", "secret")
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected missing room knowledge upload to return 404, got %d body=%s", response.Code, response.Body.String())
	}
}

func TestSaveMinutesBlankContentReturnsTyped400(t *testing.T) {
	server := newTestServer(t, api.Config{AdminAPIKey: "secret"})
	roomID := createRoomForTest(t, server, "Minutes validation")

	request := httptest.NewRequest(http.MethodPut, "/api/rooms/"+roomID+"/minutes", bytes.NewBufferString(`{"content":" \n\t "}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Admin-Key", "secret")
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected blank minutes save to return 400, got %d body=%s", response.Code, response.Body.String())
	}
}

func TestReopenActiveRoomReturnsTyped409(t *testing.T) {
	server := newTestServer(t, api.Config{AdminAPIKey: "secret"})
	roomID := createRoomForTest(t, server, "Already active")

	request := httptest.NewRequest(http.MethodPost, "/api/rooms/"+roomID+"/reopen", nil)
	request.Header.Set("X-Admin-Key", "secret")
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("expected invalid lifecycle transition to return 409, got %d body=%s", response.Code, response.Body.String())
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if payload.Error != "invalid room transition" {
		t.Fatalf("expected invalid room transition payload, got %#v", payload)
	}
}
