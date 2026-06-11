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
	room := New(roomID, strings.TrimSpace(name), m.agents)

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

func (m *Manager) Agents() []model.Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agents := make([]model.Agent, len(m.agents))
	copy(agents, m.agents)
	return agents
}
