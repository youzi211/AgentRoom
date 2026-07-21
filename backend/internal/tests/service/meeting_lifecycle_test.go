package service_test

import (
	"context"
	"testing"
	"time"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/tests/teststore"
)

type roomCancelRuntime struct{ rooms []string }

func (r *roomCancelRuntime) Name() string { return model.AgentRuntimeLLM }
func (r *roomCancelRuntime) Respond(context.Context, agent.AgentRuntimeRequest, ...agent.AgentEventObserver) (agent.AgentRuntimeResponse, error) {
	return agent.AgentRuntimeResponse{}, nil
}
func (r *roomCancelRuntime) CancelRoom(roomID string) int {
	r.rooms = append(r.rooms, roomID)
	return 1
}

func TestMeetingLifecycleAssignsInitialOwnerOnFirstJoin(t *testing.T) {
	roomService, currentRoom, store := newLifecycleRoomService(t)

	participant, err := roomService.JoinParticipant(context.Background(), currentRoom, "Alice")
	if err != nil {
		t.Fatalf("join participant: %v", err)
	}

	info := currentRoom.Info()
	if info.OwnerParticipantID != participant.ID {
		t.Fatalf("expected first join to own room, got %#v", info)
	}
	if store.Rooms[currentRoom.Info().ID].OwnerParticipantID != participant.ID {
		t.Fatalf("expected persisted owner, got %#v", store.Rooms[currentRoom.Info().ID])
	}
}

func TestMeetingLifecycleTransfersOwnerOnlyToOnlineParticipant(t *testing.T) {
	roomService, currentRoom, _ := newLifecycleRoomService(t)

	alice, err := roomService.JoinParticipant(context.Background(), currentRoom, "Alice")
	if err != nil {
		t.Fatalf("join alice: %v", err)
	}
	bob, err := roomService.JoinParticipant(context.Background(), currentRoom, "Bob")
	if err != nil {
		t.Fatalf("join bob: %v", err)
	}

	if err := roomService.TransferRoomOwner(context.Background(), currentRoom, alice.ID, "offline"); err != service.ErrOwnerTargetNotOnline {
		t.Fatalf("expected ErrOwnerTargetNotOnline, got %v", err)
	}
	if err := roomService.TransferRoomOwner(context.Background(), currentRoom, alice.ID, bob.ID); err != nil {
		t.Fatalf("transfer owner: %v", err)
	}
	if currentRoom.Info().OwnerParticipantID != bob.ID {
		t.Fatalf("expected bob to become owner, got %#v", currentRoom.Info())
	}
}

func TestMeetingLifecycleClosesAfterGraceWindowWhenLastHumanLeaves(t *testing.T) {
	roomService, currentRoom, store := newLifecycleRoomService(t)

	alice, err := roomService.JoinParticipant(context.Background(), currentRoom, "Alice")
	if err != nil {
		t.Fatalf("join alice: %v", err)
	}

	if !roomService.LeaveParticipant(context.Background(), currentRoom, alice.ID) {
		t.Fatal("expected leave to remove participant")
	}

	info := currentRoom.Info()
	if !info.IsActive() {
		t.Fatalf("expected grace window to keep room active, got %#v", info)
	}
	if info.OwnerParticipantID != "" {
		t.Fatalf("expected owner cleared during grace window, got %#v", info)
	}
	if info.AutoCloseDeadlineAt == nil {
		t.Fatalf("expected auto-close deadline, got %#v", info)
	}

	pastDeadline := time.Now().UTC().Add(-time.Second)
	currentRoom.ApplyLifecycle(room.LifecycleState{
		Status:              model.RoomStatusActive,
		AutoCloseDeadlineAt: &pastDeadline,
	})
	store.Rooms[currentRoom.Info().ID] = currentRoom.Info()

	reloaded, ok := roomService.GetRoom(context.Background(), currentRoom.Info().ID)
	if !ok {
		t.Fatal("expected room reload to succeed")
	}
	info = reloaded.Info()
	if !info.IsClosed() {
		t.Fatalf("expected room closed after reconcile, got %#v", info)
	}
	if info.ClosedReason != model.RoomClosedReasonLastHumanLeft {
		t.Fatalf("expected last human left close reason, got %#v", info)
	}
	if info.AutoCloseDeadlineAt != nil {
		t.Fatalf("expected auto-close deadline cleared, got %#v", info)
	}
}

func TestMeetingLifecycleManualCloseClearsOwnerAndDeadline(t *testing.T) {
	roomService, currentRoom, _ := newLifecycleRoomService(t)

	alice, err := roomService.JoinParticipant(context.Background(), currentRoom, "Alice")
	if err != nil {
		t.Fatalf("join alice: %v", err)
	}
	if err := roomService.CloseRoomByOwner(context.Background(), currentRoom, alice.ID); err != nil {
		t.Fatalf("close room: %v", err)
	}

	info := currentRoom.Info()
	if !info.IsClosed() {
		t.Fatalf("expected room closed, got %#v", info)
	}
	if info.OwnerParticipantID != "" || info.AutoCloseDeadlineAt != nil {
		t.Fatalf("expected owner/deadline cleared, got %#v", info)
	}
	if info.ClosedReason != model.RoomClosedReasonManual {
		t.Fatalf("expected manual close reason, got %#v", info)
	}
	if len(currentRoom.Participants()) != 0 {
		t.Fatalf("expected participants cleared, got %#v", currentRoom.Participants())
	}
}

func TestMeetingLifecycleCancelsAgentRunsWhenRoomStops(t *testing.T) {
	store := &teststore.Store{
		Rooms: make(map[string]model.RoomMeta), RoomAgents: make(map[string][]model.Agent),
		RoomMessages: make(map[string][]model.Message), ActiveParticipants: make(map[string][]model.Participant),
	}
	manager := room.NewManager(store, func([]string) []model.Agent { return nil })
	canceler := &roomCancelRuntime{}
	runner := agent.NewRunner(nil, store).WithRuntimeRegistry(agent.NewRuntimeRegistry(canceler))
	roomService := service.NewRoomService(manager, nil, nil, runner, nil, store)
	currentRoom, err := manager.CreateRoom(context.Background(), "Planning", nil, "", model.DefaultDialoguePolicy())
	if err != nil {
		t.Fatal(err)
	}
	alice, err := roomService.JoinParticipant(context.Background(), currentRoom, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if err := roomService.CloseRoomByOwner(context.Background(), currentRoom, alice.ID); err != nil {
		t.Fatal(err)
	}
	if len(canceler.rooms) != 1 || canceler.rooms[0] != currentRoom.Info().ID {
		t.Fatalf("expected room run cancellation, got %#v", canceler.rooms)
	}
}

func TestMeetingLifecycleRestoreReturnsArchivedRoomToClosed(t *testing.T) {
	roomService, currentRoom, store := newLifecycleRoomService(t)
	now := time.Now().UTC()
	currentRoom.ApplyLifecycle(room.LifecycleState{
		Status:     model.RoomStatusArchived,
		ArchivedAt: &now,
	})
	store.Rooms[currentRoom.Info().ID] = currentRoom.Info()

	if err := roomService.RestoreRoom(context.Background(), currentRoom.Info().ID); err != nil {
		t.Fatalf("restore room: %v", err)
	}

	info := currentRoom.Info()
	if !info.IsClosed() {
		t.Fatalf("expected archived room to restore to closed, got %#v", info)
	}
	if info.ArchivedAt != nil {
		t.Fatalf("expected archived flag cleared, got %#v", info)
	}
	if info.ClosedReason != model.RoomClosedReasonAdminUnarchive || info.ClosedAt == nil {
		t.Fatalf("expected admin unarchive close metadata, got %#v", info)
	}
}

func newLifecycleRoomService(t *testing.T) (*service.RoomService, *room.Room, *teststore.Store) {
	t.Helper()

	store := &teststore.Store{
		Rooms:              make(map[string]model.RoomMeta),
		RoomAgents:         make(map[string][]model.Agent),
		RoomMessages:       make(map[string][]model.Message),
		ActiveParticipants: make(map[string][]model.Participant),
	}
	manager := room.NewManager(store, func([]string) []model.Agent { return nil })
	roomService := service.NewRoomService(manager, nil, nil, nil, nil, store)
	currentRoom, err := manager.CreateRoom(context.Background(), "Planning", []string{}, "", model.DefaultDialoguePolicy())
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	return roomService, currentRoom, store
}
