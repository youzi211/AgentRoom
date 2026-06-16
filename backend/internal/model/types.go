package model

import "time"

const (
	SenderTypeHuman  = "human"
	SenderTypeAgent  = "agent"
	SenderTypeSystem = "system"
)

const (
	KnowledgeScopeRoom  = "room"
	KnowledgeScopeAgent = "agent"

	KnowledgeStatusReady = "ready"
)

const (
	RoomStatusActive   = "active"
	RoomStatusClosed   = "closed"
	RoomStatusArchived = "archived"

	RoomClosedReasonManual         = "manual"
	RoomClosedReasonLastHumanLeft  = "last_human_left"
	RoomClosedReasonAdminUnarchive = "admin_unarchive"

	MinutesSourceAI     = "ai"
	MinutesSourceManual = "manual"
)

type Room struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Participants map[string]*Participant `json:"-"`
	Agents       map[string]*Agent       `json:"-"`
	Messages     []Message               `json:"-"`
	CreatedAt    time.Time               `json:"createdAt"`
}

type RoomMeta struct {
	ID                  string         `json:"id"`
	Name                string         `json:"name"`
	CreatedAt           time.Time      `json:"createdAt"`
	HasPasscode         bool           `json:"hasPasscode"`
	PasscodeHash        string         `json:"-"`
	DialoguePolicy      DialoguePolicy `json:"dialoguePolicy"`
	Status              string         `json:"status,omitempty"`
	OwnerParticipantID  string         `json:"ownerParticipantID,omitempty"`
	ClosedAt            *time.Time     `json:"closedAt,omitempty"`
	ClosedReason        string         `json:"closedReason,omitempty"`
	AutoCloseDeadlineAt *time.Time     `json:"autoCloseDeadlineAt,omitempty"`
	ArchivedAt          *time.Time     `json:"archivedAt,omitempty"`
}

// IsArchived reports whether the room has been archived and should reject new turns.
func (r RoomMeta) IsArchived() bool {
	return r.Status == RoomStatusArchived
}

// IsClosed reports whether the meeting has ended and is only available in read-only mode.
func (r RoomMeta) IsClosed() bool {
	return r.Status == RoomStatusClosed
}

// IsActive reports whether the room is currently live.
func (r RoomMeta) IsActive() bool {
	return r.Status == "" || r.Status == RoomStatusActive
}

// RoomSummary is a lightweight room listing entry for the admin meeting list.
type RoomSummary struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	Status              string     `json:"status"`
	HasPasscode         bool       `json:"hasPasscode"`
	CreatedAt           time.Time  `json:"createdAt"`
	OwnerParticipantID  string     `json:"ownerParticipantID,omitempty"`
	ClosedAt            *time.Time `json:"closedAt,omitempty"`
	ClosedReason        string     `json:"closedReason,omitempty"`
	AutoCloseDeadlineAt *time.Time `json:"autoCloseDeadlineAt,omitempty"`
	ArchivedAt          *time.Time `json:"archivedAt,omitempty"`
	MessageCount        int        `json:"messageCount"`
	LastMessageAt       *time.Time `json:"lastMessageAt,omitempty"`
}

// MeetingMinutes is a persisted, versioned meeting minutes record.
type MeetingMinutes struct {
	ID        string    `json:"id"`
	RoomID    string    `json:"roomID"`
	Version   int       `json:"version"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`
	CreatedBy string    `json:"createdBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type Participant struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	JoinedAt time.Time `json:"joinedAt"`
}

type Agent struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Mention      string `json:"mention"`
	Role         string `json:"role"`
	Description  string `json:"description"`
	Enabled      bool   `json:"enabled"`
	SystemPrompt string `json:"-"`
}

type AgentConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Mention      string `json:"mention"`
	Role         string `json:"role"`
	Description  string `json:"description"`
	Enabled      bool   `json:"enabled"`
	SystemPrompt string `json:"systemPrompt"`
}

type KnowledgeDocument struct {
	ID          string    `json:"id"`
	Scope       string    `json:"scope"`
	ScopeID     string    `json:"scopeId"`
	FileName    string    `json:"fileName"`
	ContentType string    `json:"contentType"`
	SizeBytes   int64     `json:"sizeBytes"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

type KnowledgeChunk struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"documentId"`
	Scope      string    `json:"scope"`
	ScopeID    string    `json:"scopeId"`
	ChunkIndex int       `json:"chunkIndex"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"createdAt"`
}

type Message struct {
	ID              string    `json:"id"`
	RoomID          string    `json:"roomID"`
	SenderID        string    `json:"senderID"`
	SenderName      string    `json:"senderName"`
	SenderType      string    `json:"senderType"`
	Content         string    `json:"content"`
	CreatedAt       time.Time `json:"createdAt"`
	DialogueRunID   string    `json:"dialogueRunID,omitempty"`
	TurnIndex       int       `json:"turnIndex,omitempty"`
	ParentMessageID string    `json:"parentMessageID,omitempty"`
}

type FocusPoint struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Category  string    `json:"category,omitempty"`
}

func (a Agent) Public() Agent {
	a.SystemPrompt = ""
	return a
}

func (a Agent) Config() AgentConfig {
	return AgentConfig{
		ID:           a.ID,
		Name:         a.Name,
		Mention:      a.Mention,
		Role:         a.Role,
		Description:  a.Description,
		Enabled:      a.Enabled,
		SystemPrompt: a.SystemPrompt,
	}
}
