package room

import (
	"sort"
	"strings"
	"sync"
	"time"

	"agentroom/backend/internal/model"
)

type Room struct {
	mu           sync.RWMutex
	id           string
	name         string
	createdAt    time.Time
	participants map[string]*model.Participant
	agents       map[string]*model.Agent
	agentOrder   []string
	messages     []model.Message
	hub          *Hub
}

// New creates a brand-new room from scratch (used for new room creation).
func New(id string, name string, agents []model.Agent) *Room {
	createdAt := time.Now().UTC()
	agentMap := make(map[string]*model.Agent, len(agents))
	agentOrder := make([]string, 0, len(agents))
	for _, agent := range agents {
		copyAgent := agent
		agentMap[agent.ID] = &copyAgent
		agentOrder = append(agentOrder, agent.ID)
	}

	return &Room{
		id:           id,
		name:         normalizeRoomName(name, id),
		createdAt:    createdAt,
		participants: make(map[string]*model.Participant),
		agents:       agentMap,
		agentOrder:   agentOrder,
		messages:     make([]model.Message, 0),
		hub:          NewHub(),
	}
}

// NewFromState creates a Room from persisted metadata and agent list.
// Used after creating a room that was already persisted to the store.
func NewFromState(meta model.RoomMeta, agents []model.Agent) *Room {
	agentMap := make(map[string]*model.Agent, len(agents))
	agentOrder := make([]string, 0, len(agents))
	for _, a := range agents {
		copyAgent := a
		agentMap[a.ID] = &copyAgent
		agentOrder = append(agentOrder, a.ID)
	}

	return &Room{
		id:           meta.ID,
		name:         meta.Name,
		createdAt:    meta.CreatedAt,
		participants: make(map[string]*model.Participant),
		agents:       agentMap,
		agentOrder:   agentOrder,
		messages:     make([]model.Message, 0),
		hub:          NewHub(),
	}
}

// NewFromSnapshot creates a Room from a full persisted snapshot (meta, agents, messages, participants).
// Used when loading a room from the store after a backend restart.
func NewFromSnapshot(meta model.RoomMeta, agents []model.Agent, messages []model.Message, participants []model.Participant) *Room {
	agentMap := make(map[string]*model.Agent, len(agents))
	agentOrder := make([]string, 0, len(agents))
	for _, a := range agents {
		copyAgent := a
		agentMap[a.ID] = &copyAgent
		agentOrder = append(agentOrder, a.ID)
	}

	participantMap := make(map[string]*model.Participant, len(participants))
	for _, p := range participants {
		copyP := p
		participantMap[p.ID] = &copyP
	}

	msgCopy := make([]model.Message, len(messages))
	copy(msgCopy, messages)

	return &Room{
		id:           meta.ID,
		name:         meta.Name,
		createdAt:    meta.CreatedAt,
		participants: participantMap,
		agents:       agentMap,
		agentOrder:   agentOrder,
		messages:     msgCopy,
		hub:          NewHub(),
	}
}

func (r *Room) Hub() *Hub {
	return r.hub
}

func (r *Room) Info() model.RoomMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return model.RoomMeta{
		ID:        r.id,
		Name:      r.name,
		CreatedAt: r.createdAt,
	}
}

func (r *Room) Snapshot() model.RoomState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return model.RoomState{
		Room:         model.RoomMeta{ID: r.id, Name: r.name, CreatedAt: r.createdAt},
		Participants: cloneParticipants(r.participants),
		Agents:       cloneAgents(r.agents, r.agentOrder),
		Messages:     cloneMessages(r.messages),
	}
}

func (r *Room) Participants() []model.Participant {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneParticipants(r.participants)
}

func (r *Room) Agents() []model.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneAgents(r.agents, r.agentOrder)
}

// AgentsWithPrompts returns agents including their system prompts (for Agent Runner).
func (r *Room) AgentsWithPrompts() []model.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]model.Agent, 0, len(r.agentOrder))
	for _, id := range r.agentOrder {
		agent, ok := r.agents[id]
		if !ok {
			continue
		}
		copyAgent := *agent
		result = append(result, copyAgent)
	}
	return result
}

func (r *Room) ReplaceAgents(agents []model.Agent) {
	agentMap := make(map[string]*model.Agent, len(agents))
	agentOrder := make([]string, 0, len(agents))
	for _, agent := range agents {
		copyAgent := agent
		agentMap[agent.ID] = &copyAgent
		agentOrder = append(agentOrder, agent.ID)
	}

	r.mu.Lock()
	r.agents = agentMap
	r.agentOrder = agentOrder
	r.mu.Unlock()
}

func (r *Room) Messages() []model.Message {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneMessages(r.messages)
}

func (r *Room) RecentMessages(limit int) []model.Message {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 || len(r.messages) <= limit {
		return cloneMessages(r.messages)
	}

	start := len(r.messages) - limit
	return cloneMessages(r.messages[start:])
}

// AddParticipantFromStore adds a participant that was already persisted to the store.
func (r *Room) AddParticipantFromStore(participant model.Participant) {
	r.mu.Lock()
	copyP := participant
	r.participants[participant.ID] = &copyP
	r.mu.Unlock()
}

// NewParticipant creates a new participant model (ID is generated) but does NOT add it to the room.
// The caller is responsible for persisting via Store and then calling AddParticipantFromStore.
func (r *Room) NewParticipant(name string) model.Participant {
	return model.Participant{
		ID:       model.NewID("participant"),
		Name:     strings.TrimSpace(name),
		JoinedAt: time.Now().UTC(),
	}
}

func (r *Room) RemoveParticipant(participantID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.participants[participantID]; !ok {
		return false
	}
	delete(r.participants, participantID)
	return true
}

// AppendMessage adds an already-persisted message to the in-memory list.
func (r *Room) AppendMessage(message model.Message) {
	r.mu.Lock()
	r.messages = append(r.messages, message)
	r.mu.Unlock()
}

// NewHumanMessage creates a human message model without adding it to the room.
func (r *Room) NewHumanMessage(participant model.Participant, content string) model.Message {
	return model.Message{
		ID:         model.NewID("msg"),
		RoomID:     r.id,
		SenderID:   participant.ID,
		SenderName: participant.Name,
		SenderType: model.SenderTypeHuman,
		Content:    strings.TrimSpace(content),
		CreatedAt:  time.Now().UTC(),
	}
}

// NewAgentMessage creates an agent message model without adding it to the room.
func (r *Room) NewAgentMessage(agent model.Agent, content string) model.Message {
	return model.Message{
		ID:         model.NewID("msg"),
		RoomID:     r.id,
		SenderID:   agent.ID,
		SenderName: agent.Name,
		SenderType: model.SenderTypeAgent,
		Content:    strings.TrimSpace(content),
		CreatedAt:  time.Now().UTC(),
	}
}

// NewSystemMessage creates a system message model without adding it to the room.
func (r *Room) NewSystemMessage(content string) model.Message {
	return model.Message{
		ID:         model.NewID("msg"),
		RoomID:     r.id,
		SenderID:   "system",
		SenderName: "System",
		SenderType: model.SenderTypeSystem,
		Content:    strings.TrimSpace(content),
		CreatedAt:  time.Now().UTC(),
	}
}

// addMessage appends a message directly to the in-memory list.
// This should only be used internally by AppendMessage; external callers
// should persist via Store first, then call AppendMessage.
func (r *Room) addMessage(message model.Message) model.Message {
	r.mu.Lock()
	r.messages = append(r.messages, message)
	r.mu.Unlock()
	return message
}

func cloneParticipants(participants map[string]*model.Participant) []model.Participant {
	items := make([]model.Participant, 0, len(participants))
	for _, participant := range participants {
		items = append(items, *participant)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].JoinedAt.Equal(items[j].JoinedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].JoinedAt.Before(items[j].JoinedAt)
	})

	return items
}

func cloneAgents(agents map[string]*model.Agent, order []string) []model.Agent {
	items := make([]model.Agent, 0, len(order))
	for _, id := range order {
		agent, ok := agents[id]
		if !ok {
			continue
		}
		items = append(items, agent.Public())
	}
	return items
}

func cloneMessages(messages []model.Message) []model.Message {
	items := make([]model.Message, len(messages))
	copy(items, messages)
	return items
}

func normalizeRoomName(name string, roomID string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed != "" {
		return trimmed
	}
	return "Room " + roomID
}
