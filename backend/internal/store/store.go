package store

import (
	"context"
	"time"

	"agentroom/backend/internal/model"
)

// Store defines the persistence interface for all AgentRoom data.
// Business code depends on this interface, not on MySQL directly.
type Store interface {
	Ping(ctx context.Context) error
	Close() error

	// Agent configuration (global)
	SeedAgents(ctx context.Context, agents []model.Agent) error
	ListAgents(ctx context.Context) ([]model.Agent, error)
	CreateAgent(ctx context.Context, agent model.Agent) (model.Agent, error)
	UpdateAgent(ctx context.Context, agent model.Agent) (model.Agent, error)
	DeleteAgent(ctx context.Context, agentID string) error

	// Room lifecycle
	CreateRoom(ctx context.Context, input CreateRoomInput) (model.RoomMeta, []model.Agent, error)
	GetRoom(ctx context.Context, roomID string) (model.RoomMeta, error)
	ListRoomAgents(ctx context.Context, roomID string) ([]model.Agent, error)

	// Participants
	AddParticipant(ctx context.Context, input AddParticipantInput) (model.Participant, error)
	MarkParticipantLeft(ctx context.Context, participantID string, leftAt time.Time) error
	ListActiveParticipants(ctx context.Context, roomID string) ([]model.Participant, error)
	MarkAllActiveParticipantsLeft(ctx context.Context, leftAt time.Time) error

	// Messages
	AddMessage(ctx context.Context, message model.Message) (model.Message, error)
	ListMessages(ctx context.Context, query ListMessagesQuery) ([]model.Message, error)

	// Agent runs
	CreateAgentRun(ctx context.Context, run AgentRun) error
	FinishAgentRun(ctx context.Context, runID string, status string, errText string, completedAt time.Time) error
}

// CreateRoomInput holds the data needed to create a new room with agent snapshots.
type CreateRoomInput struct {
	ID        string
	Name      string
	Agents    []model.Agent
	CreatedAt time.Time
}

// AddParticipantInput holds the data needed to add a participant to a room.
type AddParticipantInput struct {
	ID          string
	RoomID      string
	DisplayName string
	GuestKey    string
	JoinedAt    time.Time
}

// ListMessagesQuery holds query parameters for listing messages.
type ListMessagesQuery struct {
	RoomID string
	Limit  int
	Before string // cursor for future pagination
}

// AgentRun records a single agent execution within a room.
type AgentRun struct {
	ID               string
	RoomID           string
	AgentID          string
	TriggerMessageID string
	Status           string
	Error            string
	StartedAt        time.Time
	CompletedAt      *time.Time
}
