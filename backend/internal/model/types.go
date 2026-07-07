package model

import (
	"strings"
	"time"
)

const (
	SenderTypeHuman  = "human"
	SenderTypeAgent  = "agent"
	SenderTypeSystem = "system"
)

const (
	AgentRuntimeLLM       = "llm"
	AgentRuntimeDeepAgent = "deepagent"
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
	ID                  string         `json:"id"`
	Name                string         `json:"name"`
	Status              string         `json:"status"`
	HasPasscode         bool           `json:"hasPasscode"`
	CreatedAt           time.Time      `json:"createdAt"`
	DialoguePolicy      DialoguePolicy `json:"dialoguePolicy,omitempty"`
	AgentCount          int            `json:"agentCount"`
	OwnerParticipantID  string         `json:"ownerParticipantID,omitempty"`
	ClosedAt            *time.Time     `json:"closedAt,omitempty"`
	ClosedReason        string         `json:"closedReason,omitempty"`
	AutoCloseDeadlineAt *time.Time     `json:"autoCloseDeadlineAt,omitempty"`
	ArchivedAt          *time.Time     `json:"archivedAt,omitempty"`
	MessageCount        int            `json:"messageCount"`
	LastMessageAt       *time.Time     `json:"lastMessageAt,omitempty"`
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
	Runtime      string `json:"runtime"`
	Source       string `json:"source"`
	Description  string `json:"description"`
	Enabled      bool   `json:"enabled"`
	SystemPrompt string `json:"-"`
}

type AgentConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Mention      string `json:"mention"`
	Role         string `json:"role"`
	Runtime      string `json:"runtime"`
	Source       string `json:"source"`
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
	ID           string    `json:"id"`
	DocumentID   string    `json:"documentId"`
	DocumentName string    `json:"documentName,omitempty"`
	Scope        string    `json:"scope"`
	ScopeID      string    `json:"scopeId"`
	ChunkIndex   int       `json:"chunkIndex"`
	Content      string    `json:"content"`
	CreatedAt    time.Time `json:"createdAt"`
}

type MessageKnowledgeSource struct {
	DocumentID   string `json:"documentId"`
	DocumentName string `json:"documentName"`
	Scope        string `json:"scope"`
}

type MessageArtifact struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	FileName string `json:"fileName"`
	MIMEType string `json:"mimeType"`
	Content  string `json:"-"`
}

type Message struct {
	ID               string                   `json:"id"`
	RoomID           string                   `json:"roomID"`
	SenderID         string                   `json:"senderID"`
	SenderName       string                   `json:"senderName"`
	SenderType       string                   `json:"senderType"`
	Content          string                   `json:"content"`
	CreatedAt        time.Time                `json:"createdAt"`
	DialogueRunID    string                   `json:"dialogueRunID,omitempty"`
	TurnIndex        int                      `json:"turnIndex,omitempty"`
	ParentMessageID  string                   `json:"parentMessageID,omitempty"`
	KnowledgeSources []MessageKnowledgeSource `json:"knowledgeSources,omitempty"`
	Artifacts        []MessageArtifact        `json:"artifacts,omitempty"`
}

type FocusPoint struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Category  string    `json:"category,omitempty"`
}

func (a Agent) Public() Agent {
	a.SystemPrompt = ""
	a.Runtime = NormalizeAgentRuntime(a.Runtime)
	return a
}

func (a Agent) Config() AgentConfig {
	return AgentConfig{
		ID:           a.ID,
		Name:         a.Name,
		Mention:      a.Mention,
		Role:         a.Role,
		Runtime:      NormalizeAgentRuntime(a.Runtime),
		Source:       NormalizeAgentSource(a.Source),
		Description:  a.Description,
		Enabled:      a.Enabled,
		SystemPrompt: a.SystemPrompt,
	}
}

const (
	AgentSourceBuiltin   = "builtin"
	AgentSourceDeepAgent = "deepagent"
)

func NormalizeAgentSource(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case AgentSourceDeepAgent:
		return AgentSourceDeepAgent
	default:
		return AgentSourceBuiltin
	}
}

func NormalizeAgentRuntime(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", AgentRuntimeLLM:
		return AgentRuntimeLLM
	case AgentRuntimeDeepAgent:
		return AgentRuntimeDeepAgent
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func IsValidAgentRuntime(value string) bool {
	switch NormalizeAgentRuntime(value) {
	case AgentRuntimeLLM, AgentRuntimeDeepAgent:
		return true
	default:
		return false
	}
}
