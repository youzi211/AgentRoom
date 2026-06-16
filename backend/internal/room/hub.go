package room

import (
	"sync"

	"agentroom/backend/internal/model"
)

type Client struct {
	ID            string
	ParticipantID string
	Send          chan model.ServerEvent
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[*Client]struct{}),
	}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client] = struct{}{}
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dropLocked(client)
}

func (h *Hub) Broadcast(event model.ServerEvent) {
	h.broadcast(event, nil)
}

func (h *Hub) BroadcastAndClose(event model.ServerEvent) {
	h.mu.Lock()
	clients := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.Unlock()

	for _, client := range clients {
		select {
		case client.Send <- event:
		default:
		}
	}

	h.mu.Lock()
	for _, client := range clients {
		h.dropLocked(client)
	}
	h.mu.Unlock()
}

func (h *Hub) BroadcastExcept(event model.ServerEvent, excluded *Client) {
	h.broadcast(event, excluded)
}

func (h *Hub) broadcast(event model.ServerEvent, excluded *Client) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		if client == excluded {
			continue
		}
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		select {
		case client.Send <- event:
		default:
			h.mu.Lock()
			h.dropLocked(client)
			h.mu.Unlock()
		}
	}
}

func (h *Hub) dropLocked(client *Client) {
	if _, ok := h.clients[client]; !ok {
		return
	}

	delete(h.clients, client)
	close(client.Send)
}
