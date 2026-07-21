package service_test

import (
	"context"
	"testing"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/tests/teststore"
)

func TestEntrySummaryUsesServerLocalDayBoundary(t *testing.T) {
	now := time.Date(2026, 7, 8, 15, 30, 0, 0, time.Local)
	todayStart := time.Date(2026, 7, 8, 0, 0, 0, 0, time.Local)
	memStore := &teststore.Store{
		Agents: []model.Agent{
			{ID: "enabled", Enabled: true},
			{ID: "disabled", Enabled: false},
		},
		Rooms: map[string]model.RoomMeta{
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
				CreatedAt: todayStart.Add(-time.Nanosecond),
			},
			"archived-tomorrow": {
				ID:        "archived-tomorrow",
				Status:    model.RoomStatusArchived,
				CreatedAt: todayStart.AddDate(0, 0, 1),
			},
		},
		Documents: []model.KnowledgeDocument{
			{ID: "doc-1"},
			{ID: "doc-2"},
		},
	}
	roomService := service.NewRoomService(nil, nil, nil, nil, nil, memStore)

	summary, err := roomService.EntrySummary(context.Background(), service.EntrySummaryInput{Now: now})
	if err != nil {
		t.Fatalf("entry summary: %v", err)
	}
	if summary.ActiveRooms != 2 {
		t.Fatalf("expected 2 active rooms, got %d", summary.ActiveRooms)
	}
	if summary.TodayRooms != 2 {
		t.Fatalf("expected 2 rooms created today, got %d", summary.TodayRooms)
	}
	if summary.KnowledgeDocuments != 2 {
		t.Fatalf("expected 2 knowledge documents, got %d", summary.KnowledgeDocuments)
	}
	if summary.EnabledAgents != 1 {
		t.Fatalf("expected 1 enabled agent, got %d", summary.EnabledAgents)
	}
}
