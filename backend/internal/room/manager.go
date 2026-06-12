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
	store         store.Store
	resolveAgents func(agentIDs []string) []model.Agent
	rooms         map[string]*Room
	logger        *slog.Logger
}

func NewManager(s store.Store, resolveAgents func(agentIDs []string) []model.Agent) *Manager {
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
func (m *Manager) CreateRoom(ctx context.Context, name string, agentIDs []string) (*Room, error) {
	roomID := model.NewID("room")
	trimmed := strings.TrimSpace(name)
	roomName := normalizeRoomName(trimmed, roomID)
	createdAt := time.Now().UTC()

	agents := m.resolveAgents(agentIDs)

	meta, _, err := m.store.CreateRoom(ctx, store.CreateRoomInput{
		ID:        roomID,
		Name:      roomName,
		Agents:    agents,
		CreatedAt: createdAt,
	})
	if err != nil {
		return nil, fmt.Errorf("persist room: %w", err)
	}

	r := NewFromState(meta, agents)

	m.mu.Lock()
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

	// Load from store
	meta, err := m.store.GetRoom(ctx, roomID)
	if err != nil {
		return nil, false
	}

	agents, err := m.store.ListRoomAgents(ctx, roomID)
	if err != nil {
		m.logger.Warn("load room agents failed", "room_id", roomID, "error", err)
		agents = nil
	}

	messages, err := m.store.ListMessages(ctx, store.ListMessagesQuery{RoomID: roomID, Limit: 100})
	if err != nil {
		m.logger.Warn("load room messages failed", "room_id", roomID, "error", err)
		messages = nil
	}

	participants, err := m.store.ListActiveParticipants(ctx, roomID)
	if err != nil {
		m.logger.Warn("load room participants failed", "room_id", roomID, "error", err)
		participants = nil
	}

	r = NewFromSnapshot(meta, agents, messages, participants)

	m.mu.Lock()
	// Double-check: another goroutine may have loaded the room
	if existing, ok := m.rooms[roomID]; ok {
		m.mu.Unlock()
		return existing, true
	}
	m.rooms[roomID] = r
	m.mu.Unlock()

	return r, true
}

func ptr[T any](value T) *T {
	return &value
}
