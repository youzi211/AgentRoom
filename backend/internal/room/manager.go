package room

import (
	"strings"
	"sync"

	"agentroom/backend/internal/model"
)

type Manager struct {
	mu     sync.RWMutex
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

func NewManager(agents []model.Agent) *Manager {
	copiedAgents := make([]model.Agent, len(agents))
	copy(copiedAgents, agents)

	return &Manager{
		agents: copiedAgents,
		rooms:  make(map[string]*Room),
	}
}

func (m *Manager) CreateRoom(name string) *Room {
	roomID := model.NewID("room")
	room := New(roomID, strings.TrimSpace(name), enabledAgents(m.agents))

	m.mu.Lock()
	m.rooms[roomID] = room
	m.mu.Unlock()

	return room
}

func (m *Manager) GetRoom(roomID string) (*Room, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	room, ok := m.rooms[roomID]
	return room, ok
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

func (m *Manager) UpdateAgent(agentID string, input UpdateAgentInput) (model.Agent, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for index := range m.agents {
		if m.agents[index].ID != agentID {
			continue
		}

		updated := applyAgentUpdate(m.agents[index], input)
		m.agents[index] = updated
		enabled := enabledAgents(m.agents)
		for _, currentRoom := range m.rooms {
			currentRoom.ReplaceAgents(enabled)
			currentRoom.Hub().Broadcast(model.ServerEvent{
				Type:         model.EventTypeRoomSnapshot,
				Room:         ptr(currentRoom.Info()),
				Participants: currentRoom.Participants(),
				Agents:       currentRoom.Agents(),
				Messages:     currentRoom.Messages(),
			})
		}
		return updated, true
	}

	return model.Agent{}, false
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

func ptr[T any](value T) *T {
	return &value
}
