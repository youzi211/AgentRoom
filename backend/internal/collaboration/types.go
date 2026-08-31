package collaboration

import (
	"errors"
	"time"
)

type Engine string

const (
	EngineNative  Engine = "native"
	EngineAutoGen Engine = "autogen"
)

type TriggerMode string

const (
	TriggerMentionOnly TriggerMode = "mention_only"
	TriggerAutomatic   TriggerMode = "automatic"
)

type SenderType string

const (
	SenderHuman  SenderType = "human"
	SenderAgent  SenderType = "agent"
	SenderSystem SenderType = "system"
)

type StopReason string

const (
	StopReasonCompleted        StopReason = "completed"
	StopReasonMaxTurns         StopReason = "max_turns"
	StopReasonMaxTurnsPerAgent StopReason = "max_turns_per_agent"
	StopReasonEmptyOutput      StopReason = "empty_output"
	StopReasonDuplicateOutput  StopReason = "duplicate_output"
	StopReasonNoEligibleAgent  StopReason = "no_eligible_agent"
	StopReasonCancelled        StopReason = "cancelled"
	StopReasonDeadlineExceeded StopReason = "deadline_exceeded"
	StopReasonInterrupted      StopReason = "interrupted"
	StopReasonEngineFailure    StopReason = "engine_failure"
	StopReasonProtocolError    StopReason = "protocol_error"
)

type ErrorCode string

const (
	ErrorInvalidRequest            ErrorCode = "invalid_request"
	ErrorUnsupportedVersion        ErrorCode = "unsupported_version"
	ErrorEngineUnavailable         ErrorCode = "engine_unavailable"
	ErrorResourceExhausted         ErrorCode = "resource_exhausted"
	ErrorDuplicateRun              ErrorCode = "duplicate_run"
	ErrorRoomBusy                  ErrorCode = "room_busy"
	ErrorModelNotConfigured        ErrorCode = "model_not_configured"
	ErrorModelAuthenticationFailed ErrorCode = "model_authentication_failed"
	ErrorModelRateLimited          ErrorCode = "model_rate_limited"
	ErrorModelTimeout              ErrorCode = "model_timeout"
	ErrorToolFailed                ErrorCode = "tool_failed"
	ErrorOutputInvalid             ErrorCode = "output_invalid"
	ErrorCheckpointInvalid         ErrorCode = "checkpoint_invalid"
	ErrorProtocol                  ErrorCode = "protocol_error"
	ErrorCancelled                 ErrorCode = "cancelled"
	ErrorDeadlineExceeded          ErrorCode = "deadline_exceeded"
	ErrorInternal                  ErrorCode = "internal"
)

type Request struct {
	ProtocolVersion    string
	CollaborationRunID string
	TraceID            string
	Engine             Engine
	Snapshot           ConversationSnapshot
	Checkpoint         *Checkpoint
}

type ConversationSnapshot struct {
	Room                     RoomSnapshot
	Agents                   []AgentSnapshot
	Trigger                  MessageSnapshot
	Transcript               []MessageSnapshot
	KnowledgeChunks          []KnowledgeChunk
	ModelReferences          []ModelReference
	Policy                   PolicySnapshot
	Limits                   ExecutionLimits
	InitialCandidateAgentIDs []string
}

type RoomSnapshot struct {
	ID     string
	Name   string
	Status string
}

type AgentSnapshot struct {
	ID               string
	Name             string
	Mention          string
	Role             string
	Description      string
	SystemPrompt     string
	Runtime          string
	ModelReferenceID string
	ToolNames        []string
}

type MessageSnapshot struct {
	ID                 string
	SenderID           string
	SenderName         string
	SenderType         SenderType
	Content            string
	CreatedAt          time.Time
	CollaborationRunID string
	TurnIndex          uint32
	ParentMessageID    string
}

type KnowledgeChunk struct {
	ID           string
	DocumentID   string
	DocumentName string
	Scope        string
	ScopeID      string
	ChunkIndex   uint32
	Content      string
}

type ModelReference struct {
	ID           string
	ProfileID    string
	Source       string
	Protocol     string
	ModelName    string
	RuntimeScope string
}

type PolicySnapshot struct {
	Version              string
	Engine               Engine
	TriggerMode          TriggerMode
	MaxTurns             uint32
	MaxTurnsPerAgent     uint32
	AllowAgentHandoff    bool
	AllowSelfFollowup    bool
	Cooldown             time.Duration
	StopOnEmptyOutput    bool
	StopOnRepeatedOutput bool
}

type ExecutionLimits struct {
	Timeout            time.Duration
	MaxOutputBytes     uint32
	MaxArtifactBytes   uint32
	MaxToolSteps       uint32
	MaxRequestBytes    uint32
	MaxEventBytes      uint32
	MaxCheckpointBytes uint32
}

type Checkpoint struct {
	Engine        Engine
	EngineVersion string
	FormatVersion string
	SHA256        string
	SizeBytes     uint64
	Payload       []byte
}

type Usage struct {
	InputTokens  uint64
	OutputTokens uint64
	TotalTokens  uint64
}

type ModelAudit struct {
	ModelReferenceID string
	ProfileID        string
	Source           string
	ModelName        string
}

type KnowledgeSource struct {
	DocumentID   string
	DocumentName string
	Scope        string
}

type Artifact struct {
	ID          string
	Type        string
	Title       string
	FileName    string
	MIMEType    string
	Content     []byte
	ExternalURI string
}

type Failure struct {
	Code      ErrorCode
	Message   string
	Retryable bool
}


// Shared errors used across collaboration package files.
var (
)




// Shared errors used across collaboration package files.
var (
	ErrCapacity     = errors.New("collaboration capacity exhausted")
	ErrDuplicateRun = errors.New("collaboration run is already active")
	ErrProtocol     = errors.New("collaboration protocol violation")
)
