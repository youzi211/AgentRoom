package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agentroom/backend/internal/api"
	"agentroom/backend/internal/model"
)

func TestEntrySummaryIsPublicAndReturnsRealCounts(t *testing.T) {
	server, _, backingStore := newActivityTestServer(t, api.Config{AdminAPIKey: "secret"})
	todayStart := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	backingStore.Rooms = map[string]model.RoomMeta{
		"active-today": {
			ID:        "active-today",
			Status:    model.RoomStatusActive,
			CreatedAt: todayStart.Add(time.Hour),
		},
		"closed-today": {
			ID:        "closed-today",
			Status:    model.RoomStatusClosed,
			CreatedAt: todayStart.Add(2 * time.Hour),
		},
		"active-yesterday": {
			ID:        "active-yesterday",
			Status:    model.RoomStatusActive,
			CreatedAt: todayStart.AddDate(0, 0, -1),
		},
	}
	backingStore.Documents = []model.KnowledgeDocument{{ID: "doc-1"}, {ID: "doc-2"}}
	backingStore.Agents = []model.Agent{
		{ID: "enabled-1", Enabled: true},
		{ID: "disabled-1", Enabled: false},
	}

	request := httptest.NewRequest(http.MethodGet, "/api/entry-summary", nil)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected entry summary 200 without admin key, got %d body=%s", response.Code, response.Body.String())
	}

	var payload struct {
		ActiveRooms        int `json:"activeRooms"`
		TodayRooms         int `json:"todayRooms"`
		KnowledgeDocuments int `json:"knowledgeDocuments"`
		EnabledAgents      int `json:"enabledAgents"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode entry summary: %v", err)
	}
	if payload.ActiveRooms != 2 || payload.TodayRooms != 2 || payload.KnowledgeDocuments != 2 || payload.EnabledAgents != 1 {
		t.Fatalf("unexpected entry summary: %#v", payload)
	}
}

func TestEntrySummaryLegacyRouteAlsoPublic(t *testing.T) {
	server, _, _ := newActivityTestServer(t, api.Config{AdminAPIKey: "secret"})

	request := httptest.NewRequest(http.MethodGet, "/entry-summary", nil)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected legacy entry summary 200 without admin key, got %d body=%s", response.Code, response.Body.String())
	}
}
