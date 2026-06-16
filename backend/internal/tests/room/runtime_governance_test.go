package room_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/tests/teststore"
)

func TestRoomRetainsRecentMessagesOnly(t *testing.T) {
	currentRoom := room.New("room_1", "Planning", nil)
	base := time.Now().UTC()

	for i := 0; i < 600; i++ {
		currentRoom.AppendMessage(model.Message{
			ID:         fmt.Sprintf("msg_%03d", i),
			RoomID:     "room_1",
			SenderID:   "human_1",
			SenderName: "Alice",
			SenderType: model.SenderTypeHuman,
			Content:    fmt.Sprintf("message %d", i),
			CreatedAt:  base.Add(time.Duration(i) * time.Second),
		})
	}

	messages := currentRoom.Messages()
	if len(messages) != 500 {
		t.Fatalf("expected room to retain 500 recent messages, got %d", len(messages))
	}
	if messages[0].ID != "msg_100" {
		t.Fatalf("expected oldest retained message to be msg_100, got %#v", messages[0])
	}
	if messages[len(messages)-1].ID != "msg_599" {
		t.Fatalf("expected newest retained message to be msg_599, got %#v", messages[len(messages)-1])
	}
}

func TestManagerPrunesClosedRoomsFromCacheWhenAddingNewRooms(t *testing.T) {
	fake := &teststore.Store{}
	manager := room.NewManager(fake, resolveForTest(nil))

	closedRoom, err := manager.CreateRoom(context.Background(), "Closed room", []string{}, "", model.DefaultDialoguePolicy())
	if err != nil {
		t.Fatalf("create closed room: %v", err)
	}
	closedAt := time.Now().UTC()
	closedRoom.ApplyLifecycle(room.LifecycleState{
		Status:       model.RoomStatusClosed,
		ClosedAt:     &closedAt,
		ClosedReason: model.RoomClosedReasonManual,
	})
	fake.Rooms[closedRoom.Info().ID] = closedRoom.Info()

	if _, err := manager.CreateRoom(context.Background(), "Active room", []string{}, "", model.DefaultDialoguePolicy()); err != nil {
		t.Fatalf("create active room: %v", err)
	}

	reloaded, ok := manager.GetRoom(context.Background(), closedRoom.Info().ID)
	if !ok {
		t.Fatal("expected closed room to remain loadable from store after cache prune")
	}
	if reloaded == closedRoom {
		t.Fatal("expected closed room cache entry to be pruned and reloaded, but manager returned the original cached instance")
	}
}
