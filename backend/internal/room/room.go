package room

import (
	"sort"
	"strings"
	"sync"
	"time"

	"agentroom/backend/internal/model"
)

type Room struct {
	mu            sync.RWMutex
	id            string
	name          string
	createdAt     time.Time
	participants  map[string]*model.Participant
	agents        map[string]*model.Agent
	agentOrder    []string
	messages      []model.Message
	hub           *Hub
}

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

func (r *Room) AddParticipant(name string) model.Participant {
	participant := model.Participant{
		ID:       model.NewID("participant"),
		Name:     strings.TrimSpace(name),
		JoinedAt: time.Now().UTC(),
	}

	r.mu.Lock()
	r.participants[participant.ID] = &participant
	r.mu.Unlock()

	return participant
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

func (r *Room) AddHumanMessage(participant model.Participant, content string) model.Message {
	return r.addMessage(model.Message{
		ID:         model.NewID("msg"),
		RoomID:     r.id,
		SenderID:   participant.ID,
		SenderName: participant.Name,
		SenderType: model.SenderTypeHuman,
		Content:    strings.TrimSpace(content),
		CreatedAt:  time.Now().UTC(),
	})
}

func (r *Room) AddAgentMessage(agent model.Agent, content string) model.Message {
	return r.addMessage(model.Message{
		ID:         model.NewID("msg"),
		RoomID:     r.id,
		SenderID:   agent.ID,
		SenderName: agent.Name,
		SenderType: model.SenderTypeAgent,
		Content:    strings.TrimSpace(content),
		CreatedAt:  time.Now().UTC(),
	})
}

func (r *Room) AddSystemMessage(content string) model.Message {
	return r.addMessage(model.Message{
		ID:         model.NewID("msg"),
		RoomID:     r.id,
		SenderID:   "system",
		SenderName: "System",
		SenderType: model.SenderTypeSystem,
		Content:    strings.TrimSpace(content),
		CreatedAt:  time.Now().UTC(),
	})
}

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
