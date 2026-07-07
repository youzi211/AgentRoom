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
	LoadRoomSnapshot(ctx context.Context, roomID string, messageLimit int) (RoomSnapshot, error)
	ListRoomAgents(ctx context.Context, roomID string) ([]model.Agent, error)
	ListRooms(ctx context.Context, query ListRoomsQuery) ([]model.RoomSummary, error)
	UpdateRoomLifecycle(ctx context.Context, input UpdateRoomLifecycleInput) error

	// Meeting minutes (versioned, persisted)
	CreateMinutes(ctx context.Context, minutes model.MeetingMinutes) (model.MeetingMinutes, error)
	ListMinutes(ctx context.Context, roomID string) ([]model.MeetingMinutes, error)
	LatestMinutes(ctx context.Context, roomID string) (model.MeetingMinutes, bool, error)

	// Participants
	AddParticipant(ctx context.Context, input AddParticipantInput) (model.Participant, error)
	MarkParticipantLeft(ctx context.Context, participantID string, leftAt time.Time) error
	ListActiveParticipants(ctx context.Context, roomID string) ([]model.Participant, error)
	MarkAllActiveParticipantsLeft(ctx context.Context, leftAt time.Time) error

	// Messages
	AddMessage(ctx context.Context, message model.Message) (model.Message, error)
	GetMessage(ctx context.Context, roomID string, messageID string) (model.Message, error)
	ListMessages(ctx context.Context, query ListMessagesQuery) ([]model.Message, error)
	ListMessagesPage(ctx context.Context, query ListMessagesQuery) (MessagePage, error)

	// Agent runs
	CreateAgentRun(ctx context.Context, run AgentRun) error
	FinishAgentRun(ctx context.Context, runID string, status string, errText string, completedAt time.Time) error
	ListAgentRuns(ctx context.Context, query ListRunsQuery) ([]AgentRun, error)
	CreateDialogueRun(ctx context.Context, run DialogueRun) error
	FinishDialogueRun(ctx context.Context, runID string, status string, turnCount int, completedAt time.Time) error
	ListDialogueRuns(ctx context.Context, query ListRunsQuery) ([]DialogueRun, error)

	// Knowledge documents
	CreateKnowledgeDocument(ctx context.Context, document model.KnowledgeDocument, chunks []model.KnowledgeChunk) (model.KnowledgeDocument, error)
	ListKnowledgeDocuments(ctx context.Context, query ListKnowledgeDocumentsQuery) ([]model.KnowledgeDocument, error)
	DeleteKnowledgeDocument(ctx context.Context, documentID string) error
	SearchKnowledgeChunks(ctx context.Context, query SearchKnowledgeChunksQuery) ([]model.KnowledgeChunk, error)
}

// CreateRoomInput holds the data needed to create a new room with agent snapshots.
type CreateRoomInput struct {
	ID             string
	Name           string
	Agents         []model.Agent
	PasscodeHash   string
	CreatedAt      time.Time
	DialoguePolicy model.DialoguePolicy
}

// AddParticipantInput holds the data needed to add a participant to a room.
type AddParticipantInput struct {
	ID          string
	RoomID      string
	DisplayName string
	GuestKey    string
	JoinedAt    time.Time
}

// ListRoomsQuery holds query parameters for listing rooms in the admin console.
type ListRoomsQuery struct {
	Status string // "active", "closed", "archived", or "" / "all" for no filter
	Limit  int
	Offset int
}

// ListMessagesQuery holds query parameters for listing messages.
type ListMessagesQuery struct {
	RoomID string
	Limit  int
	Before string // cursor for future pagination
}

type UpdateRoomLifecycleInput struct {
	RoomID              string
	Status              string
	OwnerParticipantID  string
	ClosedAt            *time.Time
	ClosedReason        string
	AutoCloseDeadlineAt *time.Time
	ArchivedAt          *time.Time
}

type MessagePage struct {
	Messages   []model.Message
	HasMore    bool
	NextBefore string
}

type RoomSnapshot struct {
	Meta         model.RoomMeta
	Agents       []model.Agent
	Messages     []model.Message
	Participants []model.Participant
}

type ListRunsQuery struct {
	RoomID string
	Limit  int
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

type DialogueRun struct {
	ID               string
	RoomID           string
	TriggerMessageID string
	Mode             string
	TurnCount        int
	Status           string
	StartedAt        time.Time
	CompletedAt      *time.Time
}

type ListKnowledgeDocumentsQuery struct {
	Scope   string
	ScopeID string
}

type SearchKnowledgeChunksQuery struct {
	Scope   string
	ScopeID string
	Query   string
	Limit   int
}
