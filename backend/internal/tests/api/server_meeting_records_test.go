package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agentroom/backend/internal/api"
	"agentroom/backend/internal/model"
)

func createRoomForTest(t *testing.T, server *api.Server, name string) string {
	t.Helper()
	body := bytes.NewBufferString(`{"name":"` + name + `"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/rooms", body)
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
		t.Fatalf("decode create room: %v", err)
	}
	return created.Room.ID
}

func TestListRoomsRequiresAdminKeyWhenConfigured(t *testing.T) {
	server := newTestServer(t, api.Config{AdminAPIKey: "secret"})

	request := httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without admin key, got %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
	request.Header.Set("X-Admin-Key", "secret")
	response = httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200 with admin key, got %d body=%s", response.Code, response.Body.String())
	}
}

func TestArchiveRoomBlocksFurtherListingUnderActiveFilter(t *testing.T) {
	server := newTestServer(t, api.Config{AdminAPIKey: "secret"})
	roomID := createRoomForTest(t, server, "Archivable")

	archive := httptest.NewRequest(http.MethodPost, "/api/rooms/"+roomID+"/archive", nil)
	archive.Header.Set("X-Admin-Key", "secret")
	archiveResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(archiveResponse, archive)
	if archiveResponse.Code != http.StatusOK {
		t.Fatalf("archive failed: %d body=%s", archiveResponse.Code, archiveResponse.Body.String())
	}

	listActive := httptest.NewRequest(http.MethodGet, "/api/rooms?status=active", nil)
	listActive.Header.Set("X-Admin-Key", "secret")
	listActiveResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(listActiveResponse, listActive)
	var activePayload struct {
		Rooms []struct {
			ID string `json:"id"`
		} `json:"rooms"`
	}
	if err := json.Unmarshal(listActiveResponse.Body.Bytes(), &activePayload); err != nil {
		t.Fatalf("decode active rooms: %v", err)
	}
	for _, r := range activePayload.Rooms {
		if r.ID == roomID {
			t.Fatal("archived room should not appear under active filter")
		}
	}

	listArchived := httptest.NewRequest(http.MethodGet, "/api/rooms?status=archived", nil)
	listArchived.Header.Set("X-Admin-Key", "secret")
	listArchivedResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(listArchivedResponse, listArchived)
	var archivedPayload struct {
		Rooms []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"rooms"`
	}
	if err := json.Unmarshal(listArchivedResponse.Body.Bytes(), &archivedPayload); err != nil {
		t.Fatalf("decode archived rooms: %v", err)
	}
	found := false
	for _, r := range archivedPayload.Rooms {
		if r.ID == roomID && r.Status == "archived" {
			found = true
		}
	}
	if !found {
		t.Fatal("archived room should appear under archived filter")
	}
}

func TestSaveAndListMinutesHistory(t *testing.T) {
	server := newTestServer(t, api.Config{AdminAPIKey: "secret"})
	roomID := createRoomForTest(t, server, "Minutes room")

	saveBody := bytes.NewBufferString(`{"content":"# Edited minutes\n\n- decision"}`)
	save := httptest.NewRequest(http.MethodPut, "/api/rooms/"+roomID+"/minutes", saveBody)
	save.Header.Set("Content-Type", "application/json")
	save.Header.Set("X-Admin-Key", "secret")
	saveResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(saveResponse, save)
	if saveResponse.Code != http.StatusOK {
		t.Fatalf("save minutes failed: %d body=%s", saveResponse.Code, saveResponse.Body.String())
	}

	history := httptest.NewRequest(http.MethodGet, "/api/rooms/"+roomID+"/minutes/history", nil)
	history.Header.Set("X-Admin-Key", "secret")
	historyResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(historyResponse, history)
	if historyResponse.Code != http.StatusOK {
		t.Fatalf("list minutes failed: %d body=%s", historyResponse.Code, historyResponse.Body.String())
	}

	var payload struct {
		Minutes []struct {
			Version int    `json:"version"`
			Source  string `json:"source"`
		} `json:"minutes"`
	}
	if err := json.Unmarshal(historyResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode minutes history: %v", err)
	}
	if len(payload.Minutes) != 1 || payload.Minutes[0].Version != 1 || payload.Minutes[0].Source != "manual" {
		t.Fatalf("expected one manual v1 minutes, got %+v", payload.Minutes)
	}
}

func TestDownloadMessageArtifactReturnsPersistedReport(t *testing.T) {
	server, _, backingStore := newActivityTestServer(t, api.Config{AdminAPIKey: "secret"})
	roomID := createRoomForTest(t, server, "Research room")
	report := "# Report\n\n- parameter counts"
	backingStore.RoomMessages[roomID] = append(backingStore.RoomMessages[roomID], model.Message{
		ID:         "msg_report",
		RoomID:     roomID,
		SenderID:   "research",
		SenderName: "Research",
		SenderType: model.SenderTypeAgent,
		Content:    "Research report is ready.",
		CreatedAt:  time.Now().UTC(),
		Artifacts: []model.MessageArtifact{
			{
				ID:       "report",
				Type:     "markdown_report",
				Title:    "DeepAgent Research Report",
				FileName: "research-report.md",
				MIMEType: "text/markdown",
				Content:  report,
			},
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/rooms/"+roomID+"/messages/msg_report/artifacts/report", nil)
	request.Header.Set("X-Admin-Key", "secret")
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected artifact download 200, got %d body=%s", response.Code, response.Body.String())
	}
	if body := response.Body.String(); body != report {
		t.Fatalf("expected report body %q, got %q", report, body)
	}
	if disposition := response.Header().Get("Content-Disposition"); disposition != "attachment; filename=\"research-report.md\"" {
		t.Fatalf("expected attachment filename, got %q", disposition)
	}
}
