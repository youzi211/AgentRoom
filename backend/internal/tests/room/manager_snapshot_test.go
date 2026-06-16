package room_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/tests/teststore"
)

func TestGetRoomFailsAndDoesNotCachePartialSnapshotWhenMessageLoadFails(t *testing.T) {
	backingStore := &teststore.Store{
		Rooms: map[string]model.RoomMeta{
			"room_1": {
				ID:        "room_1",
				Name:      "Planning",
				Status:    model.RoomStatusActive,
				CreatedAt: time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC),
			},
		},
		RoomAgents: map[string][]model.Agent{
			"room_1": {
				{ID: "qa", Name: "QA", Mention: "@QA", Enabled: true},
			},
		},
		RoomMessages: map[string][]model.Message{
			"room_1": {
				{
					ID:         "msg_1",
					RoomID:     "room_1",
					SenderID:   "participant_1",
					SenderName: "Alice",
					SenderType: model.SenderTypeHuman,
					Content:    "hello",
					CreatedAt:  time.Date(2026, 6, 16, 9, 1, 0, 0, time.UTC),
				},
			},
		},
		ActiveParticipants: map[string][]model.Participant{
			"room_1": {
				{
					ID:       "participant_1",
					Name:     "Alice",
					JoinedAt: time.Date(2026, 6, 16, 9, 0, 30, 0, time.UTC),
				},
			},
		},
		ListMessagesErr: errors.New("messages unavailable"),
	}
	manager := room.NewManager(backingStore, func([]string) []model.Agent { return nil })

	currentRoom, ok := manager.GetRoom(context.Background(), "room_1")
	if ok || currentRoom != nil {
		t.Fatalf("expected room hydration failure to return no room, got ok=%v room=%#v", ok, currentRoom)
	}

	backingStore.ListMessagesErr = nil
	currentRoom, ok = manager.GetRoom(context.Background(), "room_1")
	if !ok || currentRoom == nil {
		t.Fatal("expected room to load after store recovery")
	}
	if len(currentRoom.Messages()) != 1 {
		t.Fatalf("expected recovered load to include persisted messages, got %#v", currentRoom.Messages())
	}
	if len(currentRoom.Participants()) != 1 {
		t.Fatalf("expected recovered load to include persisted participants, got %#v", currentRoom.Participants())
	}
	if len(currentRoom.Agents()) != 1 {
		t.Fatalf("expected recovered load to include persisted agents, got %#v", currentRoom.Agents())
	}
}
