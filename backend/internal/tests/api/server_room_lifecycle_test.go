package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agentroom/backend/internal/api"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
)

func TestRoomLifecycleClosedRoomReadOnlyAccessAndMinutesRules(t *testing.T) {
	server, roomService, backingStore := newActivityTestServer(t, api.Config{AdminAPIKey: "secret"})
	roomID := createRoomForTest(t, server, "Closed room")
	currentRoom, ok := roomService.GetRoom(context.Background(), roomID)
	if !ok {
		t.Fatal("expected room to exist")
	}

	closedAt := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	currentRoom.ApplyLifecycle(room.LifecycleState{
		Status:       model.RoomStatusClosed,
		ClosedAt:     &closedAt,
		ClosedReason: model.RoomClosedReasonManual,
	})
	backingStore.Rooms[roomID] = currentRoom.Info()

	getRoom := httptest.NewRequest(http.MethodGet, "/api/rooms/"+roomID, nil)
	getRoomResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(getRoomResponse, getRoom)
	if getRoomResponse.Code != http.StatusOK {
		t.Fatalf("expected closed room metadata, got %d body=%s", getRoomResponse.Code, getRoomResponse.Body.String())
	}

	getMessages := httptest.NewRequest(http.MethodGet, "/api/rooms/"+roomID+"/messages", nil)
	getMessagesResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(getMessagesResponse, getMessages)
	if getMessagesResponse.Code != http.StatusOK {
		t.Fatalf("expected closed room history, got %d body=%s", getMessagesResponse.Code, getMessagesResponse.Body.String())
	}

	downloadMinutes := httptest.NewRequest(http.MethodGet, "/api/rooms/"+roomID+"/minutes.md", nil)
	downloadMinutesResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(downloadMinutesResponse, downloadMinutes)
	if downloadMinutesResponse.Code != http.StatusNotFound {
		t.Fatalf("expected no persisted minutes yet, got %d body=%s", downloadMinutesResponse.Code, downloadMinutesResponse.Body.String())
	}

	generateMinutes := httptest.NewRequest(http.MethodPost, "/api/rooms/"+roomID+"/minutes", nil)
	generateMinutesResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(generateMinutesResponse, generateMinutes)
	if generateMinutesResponse.Code != http.StatusForbidden {
		t.Fatalf("expected ordinary closed-room minutes generation to fail, got %d body=%s", generateMinutesResponse.Code, generateMinutesResponse.Body.String())
	}

	adminGenerateMinutes := httptest.NewRequest(http.MethodPost, "/api/rooms/"+roomID+"/minutes", nil)
	adminGenerateMinutes.Header.Set("X-Admin-Key", "secret")
	adminGenerateMinutesResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(adminGenerateMinutesResponse, adminGenerateMinutes)
	if adminGenerateMinutesResponse.Code != http.StatusOK {
		t.Fatalf("expected admin closed-room minutes generation, got %d body=%s", adminGenerateMinutesResponse.Code, adminGenerateMinutesResponse.Body.String())
	}

	downloadMinutes = httptest.NewRequest(http.MethodGet, "/api/rooms/"+roomID+"/minutes.md", nil)
	downloadMinutesResponse = httptest.NewRecorder()
	server.Routes().ServeHTTP(downloadMinutesResponse, downloadMinutes)
	if downloadMinutesResponse.Code != http.StatusOK {
		t.Fatalf("expected closed-room minutes download after generation, got %d body=%s", downloadMinutesResponse.Code, downloadMinutesResponse.Body.String())
	}

	listClosed := httptest.NewRequest(http.MethodGet, "/api/rooms?status=closed", nil)
	listClosed.Header.Set("X-Admin-Key", "secret")
	listClosedResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(listClosedResponse, listClosed)
	if listClosedResponse.Code != http.StatusOK {
		t.Fatalf("expected closed room list, got %d body=%s", listClosedResponse.Code, listClosedResponse.Body.String())
	}

	var payload struct {
		Rooms []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"rooms"`
	}
	if err := json.Unmarshal(listClosedResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode closed rooms: %v", err)
	}
	if len(payload.Rooms) != 1 || payload.Rooms[0].ID != roomID || payload.Rooms[0].Status != model.RoomStatusClosed {
		t.Fatalf("expected single closed room, got %#v", payload.Rooms)
	}
}

func TestRoomLifecycleArchivedRoomReadDeniedAndRestoreReturnsClosed(t *testing.T) {
	server := newTestServer(t, api.Config{AdminAPIKey: "secret"})
	roomID := createRoomForTest(t, server, "Archive me")

	archive := httptest.NewRequest(http.MethodPost, "/api/rooms/"+roomID+"/archive", nil)
	archive.Header.Set("X-Admin-Key", "secret")
	archiveResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(archiveResponse, archive)
	if archiveResponse.Code != http.StatusOK {
		t.Fatalf("archive room: %d body=%s", archiveResponse.Code, archiveResponse.Body.String())
	}

	readRoom := httptest.NewRequest(http.MethodGet, "/api/rooms/"+roomID, nil)
	readRoomResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(readRoomResponse, readRoom)
	if readRoomResponse.Code != http.StatusForbidden {
		t.Fatalf("expected archived room read denial, got %d body=%s", readRoomResponse.Code, readRoomResponse.Body.String())
	}

	adminReadRoom := httptest.NewRequest(http.MethodGet, "/api/rooms/"+roomID, nil)
	adminReadRoom.Header.Set("X-Admin-Key", "secret")
	adminReadRoomResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(adminReadRoomResponse, adminReadRoom)
	if adminReadRoomResponse.Code != http.StatusOK {
		t.Fatalf("expected admin archived room read, got %d body=%s", adminReadRoomResponse.Code, adminReadRoomResponse.Body.String())
	}

	restore := httptest.NewRequest(http.MethodPost, "/api/rooms/"+roomID+"/restore", nil)
	restore.Header.Set("X-Admin-Key", "secret")
	restoreResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(restoreResponse, restore)
	if restoreResponse.Code != http.StatusOK {
		t.Fatalf("restore room: %d body=%s", restoreResponse.Code, restoreResponse.Body.String())
	}

	var payload struct {
		Room struct {
			Status       string `json:"status"`
			ClosedReason string `json:"closedReason"`
		} `json:"room"`
	}
	if err := json.Unmarshal(restoreResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode restore payload: %v", err)
	}
	if payload.Room.Status != model.RoomStatusClosed {
		t.Fatalf("expected archived room to restore to closed, got %#v", payload.Room)
	}
	if payload.Room.ClosedReason != model.RoomClosedReasonAdminUnarchive {
		t.Fatalf("expected admin unarchive reason, got %#v", payload.Room)
	}
}

func TestRoomMessagesPaginationAndInvalidCursor(t *testing.T) {
	server, _, backingStore := newActivityTestServer(t, api.Config{})
	roomID := createRoomForTest(t, server, "Paginated room")
	otherRoomID := createRoomForTest(t, server, "Other room")

	base := time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC)
	for i, id := range []string{"msg_1", "msg_2", "msg_3", "msg_4"} {
		message := model.Message{
			ID:         id,
			RoomID:     roomID,
			SenderID:   "participant_1",
			SenderName: "Alice",
			SenderType: model.SenderTypeHuman,
			Content:    id,
			CreatedAt:  base.Add(time.Duration(i) * time.Minute),
		}
		backingStore.RoomMessages[roomID] = append(backingStore.RoomMessages[roomID], message)
	}
	backingStore.RoomMessages[otherRoomID] = append(backingStore.RoomMessages[otherRoomID], model.Message{
		ID:         "other_msg",
		RoomID:     otherRoomID,
		SenderID:   "participant_2",
		SenderName: "Bob",
		SenderType: model.SenderTypeHuman,
		Content:    "other",
		CreatedAt:  base,
	})

	request := httptest.NewRequest(http.MethodGet, "/api/rooms/"+roomID+"/messages?limit=2", nil)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("load newest message page: %d body=%s", response.Code, response.Body.String())
	}

	var page struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
		HasMore    bool   `json:"hasMore"`
		NextBefore string `json:"nextBefore"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode messages page: %v", err)
	}
	if len(page.Messages) != 2 || page.Messages[0].ID != "msg_3" || page.Messages[1].ID != "msg_4" {
		t.Fatalf("expected newest page ordered oldest-to-newest, got %#v", page.Messages)
	}
	if !page.HasMore || page.NextBefore != "msg_3" {
		t.Fatalf("expected pagination metadata, got %#v", page)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/rooms/"+roomID+"/messages?limit=2&before=msg_3", nil)
	response = httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("load older message page: %d body=%s", response.Code, response.Body.String())
	}
	page = struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
		HasMore    bool   `json:"hasMore"`
		NextBefore string `json:"nextBefore"`
	}{}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode older messages page: %v", err)
	}
	if len(page.Messages) != 2 || page.Messages[0].ID != "msg_1" || page.Messages[1].ID != "msg_2" {
		t.Fatalf("expected older page ordered oldest-to-newest, got %#v", page.Messages)
	}
	if page.HasMore || page.NextBefore != "" {
		t.Fatalf("expected final page metadata, got %#v", page)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/rooms/"+roomID+"/messages?before=missing", nil)
	response = httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected malformed cursor to fail, got %d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/rooms/"+roomID+"/messages?before=other_msg", nil)
	response = httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected foreign-room cursor to fail, got %d body=%s", response.Code, response.Body.String())
	}
}

func TestRoomLifecycleReopenOnlyWorksForClosedRooms(t *testing.T) {
	server, roomService, backingStore := newActivityTestServer(t, api.Config{AdminAPIKey: "secret"})
	roomID := createRoomForTest(t, server, "Needs reopen")
	currentRoom, ok := roomService.GetRoom(context.Background(), roomID)
	if !ok {
		t.Fatal("expected room to exist")
	}

	closedAt := time.Date(2026, 6, 16, 11, 0, 0, 0, time.UTC)
	currentRoom.ApplyLifecycle(room.LifecycleState{
		Status:       model.RoomStatusClosed,
		ClosedAt:     &closedAt,
		ClosedReason: model.RoomClosedReasonManual,
	})
	backingStore.Rooms[roomID] = currentRoom.Info()

	reopen := httptest.NewRequest(http.MethodPost, "/api/rooms/"+roomID+"/reopen", nil)
	reopen.Header.Set("X-Admin-Key", "secret")
	reopenResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(reopenResponse, reopen)
	if reopenResponse.Code != http.StatusOK {
		t.Fatalf("reopen room: %d body=%s", reopenResponse.Code, reopenResponse.Body.String())
	}

	var payload struct {
		Room struct {
			Status              string     `json:"status"`
			ClosedAt            *time.Time `json:"closedAt"`
			OwnerParticipantID  string     `json:"ownerParticipantID"`
			AutoCloseDeadlineAt *time.Time `json:"autoCloseDeadlineAt"`
		} `json:"room"`
	}
	if err := json.Unmarshal(reopenResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode reopen payload: %v", err)
	}
	if payload.Room.Status != model.RoomStatusActive || payload.Room.ClosedAt != nil || payload.Room.OwnerParticipantID != "" || payload.Room.AutoCloseDeadlineAt != nil {
		t.Fatalf("expected reopened active room without close metadata, got %#v", payload.Room)
	}
}

func TestClosedRoomMinutesWriteForAdminSupportsManualSave(t *testing.T) {
	server, roomService, backingStore := newActivityTestServer(t, api.Config{AdminAPIKey: "secret"})
	roomID := createRoomForTest(t, server, "Closed save")
	currentRoom, ok := roomService.GetRoom(context.Background(), roomID)
	if !ok {
		t.Fatal("expected room to exist")
	}

	closedAt := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	currentRoom.ApplyLifecycle(room.LifecycleState{
		Status:       model.RoomStatusClosed,
		ClosedAt:     &closedAt,
		ClosedReason: model.RoomClosedReasonManual,
	})
	backingStore.Rooms[roomID] = currentRoom.Info()

	saveBody := bytes.NewBufferString(`{"content":"# Edited minutes\n\n- decision"}`)
	save := httptest.NewRequest(http.MethodPut, "/api/rooms/"+roomID+"/minutes", saveBody)
	save.Header.Set("Content-Type", "application/json")
	save.Header.Set("X-Admin-Key", "secret")
	saveResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(saveResponse, save)
	if saveResponse.Code != http.StatusOK {
		t.Fatalf("expected admin save to succeed on closed room, got %d body=%s", saveResponse.Code, saveResponse.Body.String())
	}
}
