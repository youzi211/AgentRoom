package collaboration

import "time"

type EventKind string

const (
	EventAccepted              EventKind = "accepted"
	EventCollaborationStarted  EventKind = "collaboration_started"
	EventSpeakerSelected       EventKind = "speaker_selected"
	EventAgentTurnStarted      EventKind = "agent_turn_started"
	EventModelStarted          EventKind = "model_started"
	EventModelCompleted        EventKind = "model_completed"
	EventToolStarted           EventKind = "tool_started"
	EventToolCompleted         EventKind = "tool_completed"
	EventToolFailed            EventKind = "tool_failed"
	EventOutputDelta           EventKind = "output_delta"
	EventArtifactReady         EventKind = "artifact_ready"
	EventHandoffRequested      EventKind = "handoff_requested"
	EventAgentMessageCompleted EventKind = "agent_message_completed"
	EventCheckpoint            EventKind = "checkpoint"
	EventCompleted             EventKind = "completed"
	EventStopped               EventKind = "stopped"
	EventCancelled             EventKind = "cancelled"
	EventFailed                EventKind = "failed"
)

type Event struct {
	ProtocolVersion    string
	CollaborationRunID string
	Sequence           uint64
	OccurredAt         time.Time
	TurnID             string
	AgentID            string
	Kind               EventKind

	ReasonCategory   string
	ModelReferenceID string
	Usage            Usage
	Tool             *ToolActivity
	OutputDelta      string
	Artifact         *Artifact
	Handoff          *Handoff
	Message          *AgentMessage
	Checkpoint       *Checkpoint
	Terminal         *Terminal
}

type ToolActivity struct {
	CallID        string
	Name          string
	InputSummary  string
	OutputSummary string
	Failure       *Failure
}

type Handoff struct {
	TargetAgentID  string
	ReasonCategory string
}

type AgentMessage struct {
	Content          string
	Artifacts        []Artifact
	KnowledgeSources []KnowledgeSource
	Model            ModelAudit
	Usage            Usage
}

type Terminal struct {
	TurnCount uint32
	Reason    StopReason
	Failure   *Failure
}
