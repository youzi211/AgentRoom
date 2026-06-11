package room

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

type Manager struct {
	mu     sync.RWMutex
	store  store.Store
	agents []model.Agent
	rooms  map[string]*Room
}

type UpdateAgentInput struct {
	Name         string
	Role         string
	Description  string
	SystemPrompt string
	Enabled      *bool
}

func NewManager(s store.Store, agents []model.Agent) *Manager {
	copiedAgents := make([]model.Agent, len(agents))
	copy(copiedAgents, agents)

	return &Manager{
		store:  s,
		agents: copiedAgents,
		rooms:  make(map[string]*Room),
	}
}

// CreateRoom creates a new room, persists it to the store, and caches it in memory.
func (m *Manager) CreateRoom(ctx context.Context, name string) (*Room, error) {
	roomID := model.NewID("room")
	trimmed := strings.TrimSpace(name)
	roomName := normalizeRoomName(trimmed, roomID)
	createdAt := time.Now().UTC()

	enabled := enabledAgents(m.agents)

	meta, _, err := m.store.CreateRoom(ctx, store.CreateRoomInput{
		ID:        roomID,
		Name:      roomName,
		Agents:    enabled,
		CreatedAt: createdAt,
	})
	if err != nil {
		return nil, fmt.Errorf("persist room: %w", err)
	}

	r := NewFromState(meta, enabled)

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
		log.Printf("load room agents for %s: %v", roomID, err)
		agents = nil
	}

	messages, err := m.store.ListMessages(ctx, store.ListMessagesQuery{RoomID: roomID, Limit: 100})
	if err != nil {
		log.Printf("load room messages for %s: %v", roomID, err)
		messages = nil
	}

	participants, err := m.store.ListActiveParticipants(ctx, roomID)
	if err != nil {
		log.Printf("load room participants for %s: %v", roomID, err)
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

// Store returns the underlying store for direct use by API handlers.
func (m *Manager) Store() store.Store {
	return m.store
}

func (m *Manager) Agents() []model.AgentConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agents := make([]model.AgentConfig, 0, len(m.agents))
	for _, configuredAgent := range m.agents {
		agents = append(agents, configuredAgent.Config())
	}
	return agents
}

// UpdateAgent updates a global agent config in the store.
// It does NOT propagate changes to existing rooms (room_agents snapshots are frozen).
func (m *Manager) UpdateAgent(ctx context.Context, agentID string, input UpdateAgentInput) (model.Agent, bool) {
	m.mu.Lock()

	var current *model.Agent
	for i := range m.agents {
		if m.agents[i].ID == agentID {
			current = &m.agents[i]
			break
		}
	}
	if current == nil {
		m.mu.Unlock()
		return model.Agent{}, false
	}

	updated := applyAgentUpdate(*current, input)
	m.agents = replaceAgentInSlice(m.agents, updated)
	m.mu.Unlock()

	// Persist to store
	result, err := m.store.UpdateAgent(ctx, updated)
	if err != nil {
		log.Printf("persist agent update %s: %v", agentID, err)
		return model.Agent{}, false
	}

	return result, true
}

func applyAgentUpdate(current model.Agent, input UpdateAgentInput) model.Agent {
	if name := strings.TrimSpace(input.Name); name != "" {
		current.Name = name
		current.Mention = "@" + name
	}
	if role := strings.TrimSpace(input.Role); role != "" {
		current.Role = role
	}
	if description := strings.TrimSpace(input.Description); description != "" {
		current.Description = description
	}
	if systemPrompt := strings.TrimSpace(input.SystemPrompt); systemPrompt != "" {
		current.SystemPrompt = systemPrompt
	}
	if input.Enabled != nil {
		current.Enabled = *input.Enabled
	}
	return current
}

func enabledAgents(agents []model.Agent) []model.Agent {
	enabled := make([]model.Agent, 0, len(agents))
	for _, configuredAgent := range agents {
		if !configuredAgent.Enabled {
			continue
		}
		enabled = append(enabled, configuredAgent)
	}
	return enabled
}

func replaceAgentInSlice(agents []model.Agent, updated model.Agent) []model.Agent {
	result := make([]model.Agent, len(agents))
	copy(result, agents)
	for i, a := range result {
		if a.ID == updated.ID {
			result[i] = updated
			break
		}
	}
	return result
}

func ptr[T any](value T) *T {
	return &value
}
