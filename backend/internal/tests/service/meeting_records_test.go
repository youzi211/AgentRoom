package service_test

import (
	"context"
	"testing"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/tests/teststore"
)

func TestArchivedRoomRejectsHumanMessage(t *testing.T) {
	store := &teststore.Store{}
	roomService := service.NewRoomService(nil, nil, nil, nil, nil, store)

	currentRoom := room.New("room_1", "Planning", nil)
	currentRoom.SetStatus(model.RoomStatusArchived, nil)
	participant := currentRoom.NewParticipant("Alice")

	_, _, err := roomService.HandleHumanMessage(context.Background(), currentRoom, participant, "hello")
	if err != service.ErrRoomArchived {
		t.Fatalf("expected ErrRoomArchived, got %v", err)
	}
}

func TestActiveRoomAcceptsHumanMessage(t *testing.T) {
	store := &teststore.Store{}
	roomService := service.NewRoomService(nil, nil, nil, nil, nil, store)

	currentRoom := room.New("room_1", "Planning", nil)
	participant := currentRoom.NewParticipant("Alice")

	if _, _, err := roomService.HandleHumanMessage(context.Background(), currentRoom, participant, "hello"); err != nil {
		t.Fatalf("active room should accept messages, got %v", err)
	}
}

func TestGenerateMinutesPersistsIncrementingVersions(t *testing.T) {
	store := &teststore.Store{}
	roomService := service.NewRoomService(nil, nil, nil, nil, nil, store)
	currentRoom := room.New("room_1", "Planning", nil)

	_, first, err := roomService.GenerateMinutes(context.Background(), currentRoom)
	if err != nil {
		t.Fatalf("generate minutes (1): %v", err)
	}
	if first.Version != 1 || first.Source != model.MinutesSourceAI {
		t.Fatalf("expected v1 ai, got v%d %s", first.Version, first.Source)
	}

	_, second, err := roomService.GenerateMinutes(context.Background(), currentRoom)
	if err != nil {
		t.Fatalf("generate minutes (2): %v", err)
	}
	if second.Version != 2 {
		t.Fatalf("expected v2, got v%d", second.Version)
	}

	manual, err := roomService.SaveManualMinutes(context.Background(), currentRoom, "# Edited minutes")
	if err != nil {
		t.Fatalf("save manual minutes: %v", err)
	}
	if manual.Version != 3 || manual.Source != model.MinutesSourceManual {
		t.Fatalf("expected v3 manual, got v%d %s", manual.Version, manual.Source)
	}

	history, err := roomService.ListMinutes(context.Background(), currentRoom)
	if err != nil {
		t.Fatalf("list minutes: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(history))
	}
	if history[0].Version != 3 {
		t.Fatalf("expected newest first (v3), got v%d", history[0].Version)
	}
}

func TestSaveManualMinutesRejectsEmpty(t *testing.T) {
	store := &teststore.Store{}
	roomService := service.NewRoomService(nil, nil, nil, nil, nil, store)
	currentRoom := room.New("room_1", "Planning", nil)

	if _, err := roomService.SaveManualMinutes(context.Background(), currentRoom, "   "); err == nil {
		t.Fatal("expected error for empty minutes content")
	}
}

func TestListRoomsFiltersByStatus(t *testing.T) {
	memStore := &teststore.Store{
		Rooms: map[string]model.RoomMeta{
			"room_active":   {ID: "room_active", Name: "Active", Status: model.RoomStatusActive, CreatedAt: time.Now().UTC()},
			"room_archived": {ID: "room_archived", Name: "Archived", Status: model.RoomStatusArchived, CreatedAt: time.Now().UTC()},
		},
	}
	roomService := service.NewRoomService(nil, nil, nil, nil, nil, memStore)

	all, err := roomService.ListRooms(context.Background(), service.ListRoomsInput{})
	if err != nil {
		t.Fatalf("list all rooms: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 rooms, got %d", len(all))
	}

	archived, err := roomService.ListRooms(context.Background(), service.ListRoomsInput{Status: model.RoomStatusArchived})
	if err != nil {
		t.Fatalf("list archived rooms: %v", err)
	}
	if len(archived) != 1 || archived[0].ID != "room_archived" {
		t.Fatalf("expected only archived room, got %+v", archived)
	}
}
