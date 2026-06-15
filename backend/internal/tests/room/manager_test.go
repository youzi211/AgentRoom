package room_test

import (
	"context"
	"testing"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/tests/teststore"
)

func TestCreateRoomAgentSelectionModes(t *testing.T) {
	agents := []model.Agent{
		{ID: "pm", Name: "Product", Mention: "@Product", Enabled: true},
		{ID: "qa", Name: "QA", Mention: "@QA", Enabled: true},
		{ID: "off", Name: "Disabled", Mention: "@Disabled", Enabled: false},
	}

	t.Run("nil agent ids uses all enabled agents", func(t *testing.T) {
		fake := &teststore.Store{}
		manager := room.NewManager(fake, resolveForTest(agents))

		created, err := manager.CreateRoom(context.Background(), "planning", nil, "", model.DefaultDialoguePolicy())
		if err != nil {
			t.Fatalf("CreateRoom returned error: %v", err)
		}

		got := created.AgentsWithPrompts()
		if len(got) != 2 || got[0].ID != "pm" || got[1].ID != "qa" {
			t.Fatalf("expected enabled agents pm, qa; got %#v", got)
		}
	})

	t.Run("empty agent ids creates a room without agents", func(t *testing.T) {
		fake := &teststore.Store{}
		manager := room.NewManager(fake, resolveForTest(agents))

		created, err := manager.CreateRoom(context.Background(), "quiet", []string{}, "", model.DefaultDialoguePolicy())
		if err != nil {
			t.Fatalf("CreateRoom returned error: %v", err)
		}

		if got := created.AgentsWithPrompts(); len(got) != 0 {
			t.Fatalf("expected no room agents, got %#v", got)
		}
	})

	t.Run("specified agent ids include only enabled matches", func(t *testing.T) {
		fake := &teststore.Store{}
		manager := room.NewManager(fake, resolveForTest(agents))

		created, err := manager.CreateRoom(context.Background(), "focused", []string{"qa", "off", "missing"}, "", model.DefaultDialoguePolicy())
		if err != nil {
			t.Fatalf("CreateRoom returned error: %v", err)
		}

		got := created.AgentsWithPrompts()
		if len(got) != 1 || got[0].ID != "qa" {
			t.Fatalf("expected only qa, got %#v", got)
		}
	})
}

func resolveForTest(agents []model.Agent) func(agentIDs []string) []model.Agent {
	return func(agentIDs []string) []model.Agent {
		if agentIDs == nil {
			selected := make([]model.Agent, 0, len(agents))
			for _, a := range agents {
				if a.Enabled {
					selected = append(selected, a)
				}
			}
			return selected
		}
		if len(agentIDs) == 0 {
			return []model.Agent{}
		}

		ids := make(map[string]struct{}, len(agentIDs))
		for _, id := range agentIDs {
			ids[id] = struct{}{}
		}

		selected := make([]model.Agent, 0, len(agents))
		for _, a := range agents {
			if !a.Enabled {
				continue
			}
			if _, ok := ids[a.ID]; ok {
				selected = append(selected, a)
			}
		}
		return selected
	}
}
