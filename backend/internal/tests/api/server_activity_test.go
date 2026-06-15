package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/api"
	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/store"
	"agentroom/backend/internal/tests/teststore"
)

func TestRoomActivityListsAgentAndDialogueRuns(t *testing.T) {
	server, backingStore := newActivityTestServer(t, api.Config{})
	created := createActivityRoom(t, server, `{"name":"Activity room"}`)
	completedAt := time.Date(2026, 6, 15, 10, 30, 1, 0, time.UTC)
	startedAt := completedAt.Add(-time.Second)

	backingStore.AgentRuns = append(backingStore.AgentRuns, store.AgentRun{
		ID:               "run_1",
		RoomID:           created.Room.ID,
		AgentID:          "secretary",
		TriggerMessageID: "msg_1",
		Status:           "succeeded",
		StartedAt:        startedAt,
		CompletedAt:      &completedAt,
	})
	backingStore.DialogueRuns = append(backingStore.DialogueRuns, store.DialogueRun{
		ID:               "dialogue_1",
		RoomID:           created.Room.ID,
		TriggerMessageID: "msg_1",
		Mode:             model.DialogueModeGuided,
		TurnCount:        2,
		Status:           model.DialogueRunStatusStoppedLimit,
		StartedAt:        startedAt,
		CompletedAt:      &completedAt,
	})

	request := httptest.NewRequest(http.MethodGet, "/api/rooms/"+created.Room.ID+"/activity", nil)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected activity response 200, got %d body=%s", response.Code, response.Body.String())
	}

	var payload struct {
		AgentRuns []struct {
			ID               string     `json:"id"`
			RoomID           string     `json:"roomID"`
			AgentID          string     `json:"agentID"`
			AgentName        string     `json:"agentName"`
			TriggerMessageID string     `json:"triggerMessageID"`
			Status           string     `json:"status"`
			ErrorText        string     `json:"errorText"`
			CreatedAt        time.Time  `json:"createdAt"`
			CompletedAt      *time.Time `json:"completedAt"`
		} `json:"agentRuns"`
		DialogueRuns []struct {
			ID               string     `json:"id"`
			RoomID           string     `json:"roomID"`
			TriggerMessageID string     `json:"triggerMessageID"`
			Mode             string     `json:"mode"`
			TurnCount        int        `json:"turnCount"`
			Status           string     `json:"status"`
			CreatedAt        time.Time  `json:"createdAt"`
			CompletedAt      *time.Time `json:"completedAt"`
		} `json:"dialogueRuns"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode activity response: %v", err)
	}
	if len(payload.AgentRuns) != 1 {
		t.Fatalf("expected one agent run, got %#v", payload.AgentRuns)
	}
	if got := payload.AgentRuns[0]; got.ID != "run_1" || got.RoomID != created.Room.ID || got.AgentID != "secretary" || got.AgentName == "" || got.Status != "succeeded" {
		t.Fatalf("unexpected agent run payload: %#v", got)
	}
	if payload.AgentRuns[0].CompletedAt == nil {
		t.Fatalf("expected completed agent run timestamp, got %#v", payload.AgentRuns[0])
	}
	if len(payload.DialogueRuns) != 1 {
		t.Fatalf("expected one dialogue run, got %#v", payload.DialogueRuns)
	}
	if got := payload.DialogueRuns[0]; got.ID != "dialogue_1" || got.Mode != model.DialogueModeGuided || got.TurnCount != 2 || got.Status != model.DialogueRunStatusStoppedLimit {
		t.Fatalf("unexpected dialogue run payload: %#v", got)
	}
}

func TestRoomActivityRequiresPasscodeForProtectedRooms(t *testing.T) {
	server, _ := newActivityTestServer(t, api.Config{})
	created := createActivityRoom(t, server, `{"name":"Protected activity","passcode":"open-sesame"}`)

	request := httptest.NewRequest(http.MethodGet, "/api/rooms/"+created.Room.ID+"/activity", nil)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected missing passcode to return 403, got %d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/rooms/"+created.Room.ID+"/activity", nil)
	request.Header.Set("X-Room-Passcode", "wrong")
	response = httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected wrong passcode to return 403, got %d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/rooms/"+created.Room.ID+"/activity?passcode=open-sesame", nil)
	response = httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected correct passcode to return 200, got %d body=%s", response.Code, response.Body.String())
	}
}

func newActivityTestServer(t *testing.T, config api.Config) (*api.Server, *teststore.Store) {
	t.Helper()

	backingStore := &teststore.Store{}
	agents := agent.PredefinedAgents()
	if err := backingStore.SeedAgents(context.Background(), agents); err != nil {
		t.Fatalf("seed agents: %v", err)
	}

	agentService := service.NewAgentService(backingStore, agents)
	knowledgeService := service.NewKnowledgeService(backingStore)
	manager := room.NewManager(backingStore, agentService.ResolveForRoom)
	llmClient := activityStubLLM{response: "ok"}
	runner := agent.NewRunner(llmClient, backingStore).WithKnowledge(knowledgeService)
	focusService := service.NewFocusService(llmClient)
	roomService := service.NewRoomService(manager, agentService, knowledgeService, runner, focusService, backingStore)
	return api.NewServerWithConfig(roomService, config), backingStore
}

func createActivityRoom(t *testing.T, server *api.Server, body string) struct {
	Room struct {
		ID string `json:"id"`
	} `json:"room"`
} {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/api/rooms", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create room failed: %d body=%s", response.Code, response.Body.String())
	}

	var created struct {
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create room response: %v", err)
	}
	return created
}

type activityStubLLM struct {
	response string
}

func (s activityStubLLM) Complete(context.Context, []llm.ChatMessage) (string, error) {
	return s.response, nil
}
