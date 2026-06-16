package model

import "time"

const (
	SenderTypeHuman  = "human"
	SenderTypeAgent  = "agent"
	SenderTypeSystem = "system"
)

const (
	EventTypeMessage           = "message"
	EventTypeRoomSnapshot      = "room_snapshot"
	EventTypeRoomClosed        = "room_closed"
	EventTypeRoomArchived      = "room_archived"
	EventTypeParticipantJoined = "participant_joined"
	EventTypeParticipantLeft   = "participant_left"
	EventTypeError             = "error"
	EventTypeFocusUpdate       = "focus_update"
	EventTypeAgentActivity     = "agent_activity"
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

type RoomState struct {
	Room         RoomMeta      `json:"room"`
	Participants []Participant `json:"participants"`
	Agents       []Agent       `json:"agents"`
	Messages     []Message     `json:"messages,omitempty"`
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

type UpdateAgentRequest struct {
	Name         string `json:"name"`
	Role         string `json:"role"`
	Description  string `json:"description"`
	SystemPrompt string `json:"systemPrompt"`
	Enabled      *bool  `json:"enabled"`
}

type CreateAgentRequest struct {
	Name         string `json:"name"`
	Role         string `json:"role"`
	Description  string `json:"description"`
	SystemPrompt string `json:"systemPrompt"`
	Enabled      *bool  `json:"enabled"`
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

type HealthResponse struct {
	OK bool `json:"ok"`
}

type AgentsResponse struct {
	Agents []AgentConfig `json:"agents"`
}

type KnowledgeDocumentsResponse struct {
	Documents []KnowledgeDocument `json:"documents"`
}

type UploadKnowledgeResponse struct {
	Document KnowledgeDocument `json:"document"`
}

type CreateRoomRequest struct {
	Name           string               `json:"name"`
	AgentIDs       []string             `json:"agentIds"`
	Passcode       string               `json:"passcode"`
	DialoguePolicy *DialoguePolicyInput `json:"dialoguePolicy,omitempty"`
}

type CreateRoomResponse struct {
	Room RoomMeta `json:"room"`
}

type GetRoomResponse struct {
	Room         RoomMeta      `json:"room"`
	Participants []Participant `json:"participants"`
	Agents       []Agent       `json:"agents"`
}

type GetMessagesResponse struct {
	Messages   []Message `json:"messages"`
	HasMore    bool      `json:"hasMore"`
	NextBefore string    `json:"nextBefore,omitempty"`
}

type RoomActivityResponse struct {
	AgentRuns    []AgentRunActivity    `json:"agentRuns"`
	DialogueRuns []DialogueRunActivity `json:"dialogueRuns"`
}

type AgentRunActivity struct {
	ID               string     `json:"id"`
	RoomID           string     `json:"roomID"`
	AgentID          string     `json:"agentID"`
	AgentName        string     `json:"agentName"`
	TriggerMessageID string     `json:"triggerMessageID"`
	Status           string     `json:"status"`
	ErrorText        string     `json:"errorText,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
}

type DialogueRunActivity struct {
	ID               string     `json:"id"`
	RoomID           string     `json:"roomID"`
	TriggerMessageID string     `json:"triggerMessageID"`
	Mode             string     `json:"mode"`
	TurnCount        int        `json:"turnCount"`
	Status           string     `json:"status"`
	CreatedAt        time.Time  `json:"createdAt"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
}

type AgentActivityEvent struct {
	Kind             string     `json:"kind"`
	Phase            string     `json:"phase"`
	ID               string     `json:"id"`
	RoomID           string     `json:"roomID"`
	AgentID          string     `json:"agentID,omitempty"`
	AgentName        string     `json:"agentName,omitempty"`
	TriggerMessageID string     `json:"triggerMessageID,omitempty"`
	Status           string     `json:"status,omitempty"`
	ErrorText        string     `json:"errorText,omitempty"`
	TurnCount        int        `json:"turnCount,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
}

type GenerateMinutesResponse struct {
	Markdown string          `json:"markdown"`
	Minutes  *MeetingMinutes `json:"minutes,omitempty"`
}

type ListRoomsResponse struct {
	Rooms []RoomSummary `json:"rooms"`
}

type MinutesHistoryResponse struct {
	Minutes []MeetingMinutes `json:"minutes"`
}

type SaveMinutesRequest struct {
	Content string `json:"content"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type ClientEvent struct {
	Type          string `json:"type"`
	Content       string `json:"content,omitempty"`
	ParticipantID string `json:"participantID,omitempty"`
}

type ServerEvent struct {
	Type          string              `json:"type"`
	Room          *RoomMeta           `json:"room,omitempty"`
	Participants  []Participant       `json:"participants,omitempty"`
	Agents        []Agent             `json:"agents,omitempty"`
	Messages      []Message           `json:"messages,omitempty"`
	Message       *Message            `json:"message,omitempty"`
	Participant   *Participant        `json:"participant,omitempty"`
	ParticipantID string              `json:"participantID,omitempty"`
	FocusPoints   []FocusPoint        `json:"focusPoints,omitempty"`
	Activity      *AgentActivityEvent `json:"activity,omitempty"`
	Error         string              `json:"error,omitempty"`
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
