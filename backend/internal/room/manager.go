package room

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"agentroom/backend/internal/logging"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

type Manager struct {
	mu            sync.RWMutex
	store         managerStore
	resolveAgents func(agentIDs []string) []model.Agent
	rooms         map[string]*Room
	logger        *slog.Logger
}

type managerStore interface {
	CreateRoom(ctx context.Context, input store.CreateRoomInput) (model.RoomMeta, []model.Agent, error)
	LoadRoomSnapshot(ctx context.Context, roomID string, messageLimit int) (store.RoomSnapshot, error)
}

func NewManager(s managerStore, resolveAgents func(agentIDs []string) []model.Agent) *Manager {
	return &Manager{
		store:         s,
		resolveAgents: resolveAgents,
		rooms:         make(map[string]*Room),
		logger:        logging.Component("room_manager"),
	}
}

// CreateRoom creates a new room, persists it to the store, and caches it in memory.
// If agentIDs is nil, all enabled agents are included (backward compatible).
// If agentIDs is an empty but non-nil slice, the room starts without agents.
func (m *Manager) CreateRoom(ctx context.Context, name string, agentIDs []string, passcodeHash string, dialoguePolicy model.DialoguePolicy) (*Room, error) {
	roomID := model.NewID("room")
	trimmed := strings.TrimSpace(name)
	roomName := normalizeRoomName(trimmed, roomID)
	createdAt := time.Now().UTC()
	policy := dialoguePolicy.WithDefaults()

	agents := m.resolveAgents(agentIDs)

	meta, _, err := m.store.CreateRoom(ctx, store.CreateRoomInput{
		ID:             roomID,
		Name:           roomName,
		Agents:         agents,
		PasscodeHash:   passcodeHash,
		CreatedAt:      createdAt,
		DialoguePolicy: policy,
	})
	if err != nil {
		return nil, fmt.Errorf("persist room: %w", err)
	}

	r := NewFromState(meta, agents)

	m.mu.Lock()
	m.pruneInactiveRoomsLocked(roomID)
	m.rooms[roomID] = r
	m.mu.Unlock()

	return r, nil
}

// GetRoom returns a room by ID. If the room is not in memory, it loads it from the store.
func (m *Manager) GetRoom(ctx context.Context, roomID string) (*Room, bool) {
	m.mu.RLock()
	r, ok := m.rooms[roomID]
	m.mu.RUnlock()
	if ok {
		return r, true
	}

	snapshot, err := m.store.LoadRoomSnapshot(ctx, roomID, 100)
	if err != nil {
		m.logger.Warn("load room snapshot failed", "room_id", roomID, "error", err)
		return nil, false
	}

	r = NewFromSnapshot(snapshot.Meta, snapshot.Agents, snapshot.Messages, snapshot.Participants)

	m.mu.Lock()
	// Double-check: another goroutine may have loaded the room
	if existing, ok := m.rooms[roomID]; ok {
		m.mu.Unlock()
		return existing, true
	}
	m.pruneInactiveRoomsLocked(roomID)
	m.rooms[roomID] = r
	m.mu.Unlock()

	return r, true
}

func ptr[T any](value T) *T {
	return &value
}

func (m *Manager) pruneInactiveRoomsLocked(exemptRoomID string) {
	for roomID, currentRoom := range m.rooms {
		if roomID == exemptRoomID || currentRoom == nil {
			continue
		}
		info := currentRoom.Info()
		if info.IsActive() {
			continue
		}
		if len(currentRoom.Participants()) > 0 {
			continue
		}
		delete(m.rooms, roomID)
	}
}
