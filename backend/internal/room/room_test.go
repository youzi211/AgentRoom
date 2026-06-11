package room

import (
	"testing"

	"agentroom/backend/internal/model"
)

func TestRoomStoresMessagesInOrder(t *testing.T) {
	currentRoom := New("room_1", "Demo Room", nil)
	participant := currentRoom.AddParticipant("Alice")

	first := currentRoom.AddHumanMessage(participant, "hello")
	second := currentRoom.AddSystemMessage("system note")
	third := currentRoom.AddAgentMessage(model.Agent{ID: "pm", Name: "产品经理"}, "建议先收范围")

	messages := currentRoom.Messages()
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}
	if messages[0].ID != first.ID {
		t.Fatalf("expected first message %q, got %q", first.ID, messages[0].ID)
	}
	if messages[1].ID != second.ID {
		t.Fatalf("expected second message %q, got %q", second.ID, messages[1].ID)
	}
	if messages[2].ID != third.ID {
		t.Fatalf("expected third message %q, got %q", third.ID, messages[2].ID)
	}
}
