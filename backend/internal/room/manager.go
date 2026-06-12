package room

import (
	"context"
	"errors"
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
	mu     sync.RWMutex
	store  store.Store
	agents []model.Agent
	rooms  map[string]*Room
	logger *slog.Logger
}

var (
	ErrAgentNotFound      = errors.New("agent not found")
	ErrAgentMentionExists = errors.New("agent mention already exists")
)

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
		logger: logging.Component("room_manager"),
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

	agents := m.resolveAgentsForRoom(agentIDs)

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
func (m *Manager) UpdateAgent(ctx context.Context, agentID string, input UpdateAgentInput) (model.Agent, error) {
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
		return model.Agent{}, ErrAgentNotFound
	}

	updated := applyAgentUpdate(*current, input)
	if hasAgentMentionConflict(m.agents, updated.ID, updated.Mention) {
		m.mu.Unlock()
		return model.Agent{}, ErrAgentMentionExists
	}
	m.agents = replaceAgentInSlice(m.agents, updated)
	m.mu.Unlock()

	// Persist to store
	result, err := m.store.UpdateAgent(ctx, updated)
	if err != nil {
		m.logger.Error("persist agent update", "agent_id", agentID, "error", err)
		return model.Agent{}, fmt.Errorf("persist agent update: %w", err)
	}

	return result, nil
}

// CreateAgent adds a new global agent config and persists it.
func (m *Manager) CreateAgent(ctx context.Context, name, role, description, systemPrompt string, enabled bool) (model.Agent, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return model.Agent{}, fmt.Errorf("agent name is required")
	}
	mention := "@" + trimmedName

	m.mu.RLock()
	if hasAgentMentionConflict(m.agents, "", mention) {
		m.mu.RUnlock()
		return model.Agent{}, ErrAgentMentionExists
	}
	m.mu.RUnlock()

	a := model.Agent{
		ID:           model.NewID("agent"),
		Name:         trimmedName,
		Mention:      mention,
		Role:         strings.TrimSpace(role),
		Description:  strings.TrimSpace(description),
		SystemPrompt: strings.TrimSpace(systemPrompt),
		Enabled:      enabled,
	}

	result, err := m.store.CreateAgent(ctx, a)
	if err != nil {
		return model.Agent{}, fmt.Errorf("persist new agent: %w", err)
	}

	m.mu.Lock()
	m.agents = append(m.agents, result)
	m.mu.Unlock()

	return result, nil
}

// DeleteAgent removes a global agent config. It does NOT affect room_agents snapshots.
func (m *Manager) DeleteAgent(ctx context.Context, agentID string) error {
	if err := m.store.DeleteAgent(ctx, agentID); err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}

	m.mu.Lock()
	m.agents = removeAgentFromSlice(m.agents, agentID)
	m.mu.Unlock()

	return nil
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

// resolveAgentsForRoom returns the agents to snapshot into a new room.
// If agentIDs is nil, all enabled agents are returned (backward compatible).
// If agentIDs is an empty but non-nil slice, no agents are returned.
// Only enabled agents matching the given IDs are included; unknown IDs are ignored.
func (m *Manager) resolveAgentsForRoom(agentIDs []string) []model.Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if agentIDs == nil {
		return enabledAgents(m.agents)
	}
	if len(agentIDs) == 0 {
		return []model.Agent{}
	}

	agentSet := make(map[string]struct{}, len(agentIDs))
	for _, id := range agentIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			agentSet[trimmed] = struct{}{}
		}
	}

	var selected []model.Agent
	for _, a := range m.agents {
		if !a.Enabled {
			continue
		}
		if _, ok := agentSet[a.ID]; ok {
			selected = append(selected, a)
		}
	}

	return selected
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

func removeAgentFromSlice(agents []model.Agent, agentID string) []model.Agent {
	result := make([]model.Agent, 0, len(agents))
	for _, a := range agents {
		if a.ID != agentID {
			result = append(result, a)
		}
	}
	return result
}

func hasAgentMentionConflict(agents []model.Agent, currentAgentID string, mention string) bool {
	for _, a := range agents {
		if a.ID == currentAgentID {
			continue
		}
		if a.Mention == mention {
			return true
		}
	}
	return false
}

func ptr[T any](value T) *T {
	return &value
}
