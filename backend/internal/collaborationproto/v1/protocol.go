package collaborationruntimev1

import "fmt"

const ProtocolVersion = "v1"

func ValidateProtocolVersion(version string) error {
	if version != ProtocolVersion {
		return fmt.Errorf("unsupported Collaboration Runtime protocol version %q", version)
	}
	return nil
}

// EventSequenceValidator validates only the versioned stream envelope. Agent
// eligibility and business state transitions remain control-plane concerns.
type EventSequenceValidator struct {
	runID        string
	lastSequence uint64
	terminalSeen bool
}

func NewEventSequenceValidator(runID string) *EventSequenceValidator {
	return &EventSequenceValidator{runID: runID}
}

func (v *EventSequenceValidator) Validate(event *CollaborationEvent) error {
	if event == nil {
		return fmt.Errorf("collaboration event is required")
	}
	if err := ValidateProtocolVersion(event.GetProtocolVersion()); err != nil {
		return err
	}
	if event.GetCollaborationRunId() == "" || event.GetCollaborationRunId() != v.runID {
		return fmt.Errorf("unexpected collaboration run ID %q", event.GetCollaborationRunId())
	}
	if event.GetPayload() == nil {
		return fmt.Errorf("collaboration event payload is required")
	}
	if v.terminalSeen {
		return fmt.Errorf("collaboration event received after terminal event")
	}
	if event.GetSequence() == 0 || event.GetSequence() <= v.lastSequence {
		return fmt.Errorf("collaboration event sequence must increase")
	}
	if v.lastSequence == 0 && event.GetAccepted() == nil {
		return fmt.Errorf("first collaboration event must be accepted")
	}
	if err := validateEventIdentity(event); err != nil {
		return err
	}

	v.lastSequence = event.GetSequence()
	v.terminalSeen = isTerminalEvent(event)
	return nil
}

func (v *EventSequenceValidator) TerminalSeen() bool {
	return v.terminalSeen
}

func validateEventIdentity(event *CollaborationEvent) error {
	turnScoped := event.GetSpeakerSelected() != nil ||
		event.GetAgentTurnStarted() != nil ||
		event.GetModelStarted() != nil ||
		event.GetModelCompleted() != nil ||
		event.GetToolStarted() != nil ||
		event.GetToolCompleted() != nil ||
		event.GetToolFailed() != nil ||
		event.GetOutputDelta() != nil ||
		event.GetArtifactReady() != nil ||
		event.GetHandoffRequested() != nil ||
		event.GetAgentMessageCompleted() != nil
	if turnScoped {
		if event.GetTurnId() == "" || event.GetAgentId() == "" {
			return fmt.Errorf("turn-scoped collaboration event requires turn and Agent IDs")
		}
		return nil
	}
	if event.GetTurnId() != "" || event.GetAgentId() != "" {
		return fmt.Errorf("run-scoped collaboration event must not include turn identity")
	}
	return nil
}

func isTerminalEvent(event *CollaborationEvent) bool {
	return event.GetCompleted() != nil || event.GetStopped() != nil || event.GetCancelled() != nil || event.GetFailed() != nil
}
