package collaboration

import (
	"fmt"
)

type validationTurnState struct {
	agentID   string
	started   bool
	completed bool
}



// Validator applies AgentRoom's stream and turn invariants to neutral events.
type Validator struct {
	runID                string
	allowedAgents        map[string]struct{}
	maxTurns             uint32
	maxTurnsPerAgent     uint32
	allowAgentHandoff    bool
	allowSelfFollowup    bool
	lastSequence         uint64
	accepted             bool
	started              bool
	terminal             bool
	activeTurnID         string
	turns                map[string]*validationTurnState
	selectedTurnsByAgent map[string]uint32
	completedTurns       uint32
}

func NewValidator(request Request) *Validator {
	allowedAgents := make(map[string]struct{}, len(request.Snapshot.Agents))
	for _, agent := range request.Snapshot.Agents {
		allowedAgents[agent.ID] = struct{}{}
	}
	return &Validator{
		runID:                request.CollaborationRunID,
		allowedAgents:        allowedAgents,
		maxTurns:             request.Snapshot.Policy.MaxTurns,
		maxTurnsPerAgent:     request.Snapshot.Policy.MaxTurnsPerAgent,
		allowAgentHandoff:    request.Snapshot.Policy.AllowAgentHandoff,
		allowSelfFollowup:    request.Snapshot.Policy.AllowSelfFollowup,
		turns:                make(map[string]*validationTurnState),
		selectedTurnsByAgent: make(map[string]uint32),
	}
}

func (v *Validator) Validate(event Event) error {
	if err := v.validateEnvelope(event); err != nil {
		return err
	}

	switch event.Kind {
	case EventAccepted:
		if v.accepted {
			return protocolError("accepted event was repeated")
		}
		v.accepted = true
	case EventCollaborationStarted:
		if v.started || len(v.turns) != 0 {
			return protocolError("collaboration_started is out of order")
		}
		v.started = true
	case EventSpeakerSelected:
		if err := v.selectTurn(event); err != nil {
			return err
		}
	case EventAgentTurnStarted:
		turn, err := v.activeTurn(event)
		if err != nil {
			return err
		}
		if turn.started {
			return protocolError("turn %q was started more than once", event.TurnID)
		}
		turn.started = true
	case EventModelStarted, EventModelCompleted, EventOutputDelta:
		if _, err := v.activeTurn(event); err != nil {
			return err
		}
	case EventToolStarted, EventToolCompleted, EventToolFailed:
		if _, err := v.activeTurn(event); err != nil {
			return err
		}
		if event.Tool == nil {
			return protocolError("%s event payload is required", event.Kind)
		}
	case EventArtifactReady:
		if _, err := v.activeTurn(event); err != nil {
			return err
		}
		if event.Artifact == nil {
			return protocolError("artifact_ready event payload is required")
		}
	case EventHandoffRequested:
		if _, err := v.activeTurn(event); err != nil {
			return err
		}
		if err := v.validateHandoff(event); err != nil {
			return err
		}
	case EventAgentMessageCompleted:
		turn, err := v.activeTurn(event)
		if err != nil {
			return err
		}
		if event.Message == nil {
			return protocolError("agent_message_completed event payload is required")
		}
		turn.completed = true
		v.completedTurns++
		v.activeTurnID = ""
	case EventCheckpoint:
		if event.Checkpoint == nil {
			return protocolError("checkpoint event payload is required")
		}
	case EventCompleted, EventStopped, EventCancelled, EventFailed:
		if err := v.validateTerminal(event); err != nil {
			return err
		}
		v.terminal = true
	default:
		return protocolError("unsupported event kind %q", event.Kind)
	}

	v.lastSequence = event.Sequence
	return nil
}

func (v *Validator) TerminalSeen() bool {
	return v.terminal
}

func (v *Validator) CompletedTurns() uint32 {
	return v.completedTurns
}

func (v *Validator) validateEnvelope(event Event) error {
	if event.ProtocolVersion != ProtocolVersion {
		return protocolError("unsupported protocol version")
	}
	if event.CollaborationRunID == "" || event.CollaborationRunID != v.runID {
		return protocolError("unexpected collaboration run ID")
	}
	if v.terminal {
		return protocolError("event received after terminal event")
	}
	if event.Sequence == 0 || event.Sequence <= v.lastSequence {
		return protocolError("event sequence must increase")
	}
	if !v.accepted && event.Kind != EventAccepted {
		return protocolError("first event must be accepted")
	}
	if isTurnScoped(event.Kind) {
		if event.TurnID == "" || event.AgentID == "" {
			return protocolError("turn-scoped event requires turn and Agent IDs")
		}
		if _, ok := v.allowedAgents[event.AgentID]; !ok {
			return protocolError("event references an ineligible Agent")
		}
	} else if event.TurnID != "" || event.AgentID != "" {
		return protocolError("run-scoped event must not include turn identity")
	}
	return nil
}

func (v *Validator) selectTurn(event Event) error {
	if v.activeTurnID != "" {
		return protocolError("turn %q is still active", v.activeTurnID)
	}
	if _, exists := v.turns[event.TurnID]; exists {
		return protocolError("turn ID %q was reused", event.TurnID)
	}
	if v.maxTurns == 0 || uint32(len(v.turns)) >= v.maxTurns {
		return protocolError("maximum collaboration turns exceeded")
	}
	if v.maxTurnsPerAgent == 0 || v.selectedTurnsByAgent[event.AgentID] >= v.maxTurnsPerAgent {
		return protocolError("maximum turns for Agent exceeded")
	}
	v.turns[event.TurnID] = &validationTurnState{agentID: event.AgentID}
	v.selectedTurnsByAgent[event.AgentID]++
	v.activeTurnID = event.TurnID
	return nil
}

func (v *Validator) activeTurn(event Event) (*validationTurnState, error) {
	turn, ok := v.turns[event.TurnID]
	if !ok || v.activeTurnID != event.TurnID {
		return nil, protocolError("event references a turn that is not active")
	}
	if turn.agentID != event.AgentID {
		return nil, protocolError("turn Agent identity changed")
	}
	if turn.completed {
		return nil, protocolError("event references a completed turn")
	}
	return turn, nil
}

func (v *Validator) validateHandoff(event Event) error {
	if !v.allowAgentHandoff || event.Handoff == nil || event.Handoff.TargetAgentID == "" {
		return protocolError("handoff is not allowed or has no target")
	}
	if _, ok := v.allowedAgents[event.Handoff.TargetAgentID]; !ok {
		return protocolError("handoff target is not eligible")
	}
	if event.Handoff.TargetAgentID == event.AgentID && !v.allowSelfFollowup {
		return protocolError("self handoff is not allowed")
	}
	return nil
}

func (v *Validator) validateTerminal(event Event) error {
	if event.Terminal == nil {
		return protocolError("terminal event payload is required")
	}
	if event.Terminal.TurnCount != v.completedTurns {
		return protocolError("terminal turn count does not match completed turns")
	}
	if event.Kind == EventCompleted && v.activeTurnID != "" {
		return protocolError("completed event cannot close an active turn")
	}
	if !terminalReasonAllowed(event.Kind, event.Terminal.Reason) {
		return protocolError("terminal reason is incompatible with event kind")
	}
	if event.Kind == EventFailed {
		if event.Terminal.Failure == nil {
			return protocolError("failed event requires failure details")
		}
	} else if event.Terminal.Failure != nil {
		return protocolError("only failed events may include failure details")
	}
	return nil
}

func isTurnScoped(kind EventKind) bool {
	switch kind {
	case EventSpeakerSelected,
		EventAgentTurnStarted,
		EventModelStarted,
		EventModelCompleted,
		EventToolStarted,
		EventToolCompleted,
		EventToolFailed,
		EventOutputDelta,
		EventArtifactReady,
		EventHandoffRequested,
		EventAgentMessageCompleted:
		return true
	default:
		return false
	}
}

func terminalReasonAllowed(kind EventKind, reason StopReason) bool {
	switch kind {
	case EventCompleted:
		return reason == StopReasonCompleted
	case EventStopped:
		return reason == StopReasonMaxTurns ||
			reason == StopReasonMaxTurnsPerAgent ||
			reason == StopReasonEmptyOutput ||
			reason == StopReasonDuplicateOutput ||
			reason == StopReasonNoEligibleAgent
	case EventCancelled:
		return reason == StopReasonCancelled ||
			reason == StopReasonDeadlineExceeded ||
			reason == StopReasonInterrupted
	case EventFailed:
		return reason == StopReasonEngineFailure || reason == StopReasonProtocolError
	default:
		return false
	}
}

func protocolError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrProtocol, fmt.Sprintf(format, args...))
}
