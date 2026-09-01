package collaboration_test

import (
	"errors"
	"testing"
	"agentroom/backend/internal/collaboration"

)

func TestValidatorAcceptsOrderedEligibleTurnsAndUniqueTerminal(t *testing.T) {
	validator := collaboration.NewValidator(validationRequest())
	events := []collaboration.Event{
		runEvent(1, collaboration.EventAccepted),
		runEvent(2, collaboration.EventCollaborationStarted),
		validationTurnEvent(3, collaboration.EventSpeakerSelected, "turn_1", "agent_1"),
		validationTurnEvent(4, collaboration.EventAgentTurnStarted, "turn_1", "agent_1"),
		validationTurnEvent(5, collaboration.EventModelStarted, "turn_1", "agent_1"),
		withHandoff(validationTurnEvent(6, collaboration.EventHandoffRequested, "turn_1", "agent_1"), "agent_2"),
		withMessage(validationTurnEvent(7, collaboration.EventAgentMessageCompleted, "turn_1", "agent_1")),
		validationTurnEvent(8, collaboration.EventSpeakerSelected, "turn_2", "agent_2"),
		withMessage(validationTurnEvent(9, collaboration.EventAgentMessageCompleted, "turn_2", "agent_2")),
		withTerminal(runEvent(10, collaboration.EventCompleted), 2, collaboration.StopReasonCompleted, nil),
	}
	for _, event := range events {
		if err := validator.Validate(event); err != nil {
			t.Fatalf("validate %s: %v", event.Kind, err)
		}
	}
	if !validator.TerminalSeen() || validator.CompletedTurns() != 2 {
		t.Fatalf("unexpected validator state: terminal=%v turns=%d", validator.TerminalSeen(), validator.CompletedTurns())
	}
}

func TestValidatorRejectsEnvelopeViolationsWithoutAdvancingSequence(t *testing.T) {
	tests := []struct {
		name  string
		event collaboration.Event
	}{
		{name: "unsupported version", event: collaboration.Event{ProtocolVersion: "v2", CollaborationRunID: "collab_1", Sequence: 1, Kind: collaboration.EventAccepted}},
		{name: "wrong run", event: collaboration.Event{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: "other", Sequence: 1, Kind: collaboration.EventAccepted}},
		{name: "zero sequence", event: collaboration.Event{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: "collab_1", Kind: collaboration.EventAccepted}},
		{name: "first event", event: runEvent(1, collaboration.EventCollaborationStarted)},
		{name: "run identity", event: collaboration.Event{ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: "collab_1", Sequence: 1, Kind: collaboration.EventAccepted, TurnID: "turn_1", AgentID: "agent_1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator := collaboration.NewValidator(validationRequest())
			if err := validator.Validate(test.event); !errors.Is(err, collaboration.ErrProtocol) {
				t.Fatalf("expected protocol error, got %v", err)
			}
			if err := validator.Validate(runEvent(1, collaboration.EventAccepted)); err != nil {
				t.Fatalf("failed event advanced validator state: %v", err)
			}
		})
	}

	validator := collaboration.NewValidator(validationRequest())
	mustValidate(t, validator, runEvent(1, collaboration.EventAccepted))
	mustValidate(t, validator, runEvent(3, collaboration.EventCollaborationStarted))
	if err := validator.Validate(runEvent(2, collaboration.EventCheckpoint)); !errors.Is(err, collaboration.ErrProtocol) {
		t.Fatalf("expected decreasing sequence error, got %v", err)
	}
}

func TestValidatorRejectsIneligibleAgentsAndInvalidTurnTransitions(t *testing.T) {
	tests := []struct {
		name   string
		events []collaboration.Event
	}{
		{name: "unknown Agent", events: []collaboration.Event{validationTurnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "outside")}},
		{name: "unknown turn", events: []collaboration.Event{validationTurnEvent(2, collaboration.EventAgentTurnStarted, "turn_1", "agent_1")}},
		{name: "Agent changed", events: []collaboration.Event{
			validationTurnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "agent_1"),
			validationTurnEvent(3, collaboration.EventAgentTurnStarted, "turn_1", "agent_2"),
		}},
		{name: "concurrent turn", events: []collaboration.Event{
			validationTurnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "agent_1"),
			validationTurnEvent(3, collaboration.EventSpeakerSelected, "turn_2", "agent_2"),
		}},
		{name: "duplicate turn completion", events: []collaboration.Event{
			validationTurnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "agent_1"),
			withMessage(validationTurnEvent(3, collaboration.EventAgentMessageCompleted, "turn_1", "agent_1")),
			withMessage(validationTurnEvent(4, collaboration.EventAgentMessageCompleted, "turn_1", "agent_1")),
		}},
		{name: "turn ID reused", events: []collaboration.Event{
			validationTurnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "agent_1"),
			withMessage(validationTurnEvent(3, collaboration.EventAgentMessageCompleted, "turn_1", "agent_1")),
			validationTurnEvent(4, collaboration.EventSpeakerSelected, "turn_1", "agent_2"),
		}},
		{name: "per Agent limit", events: []collaboration.Event{
			validationTurnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "agent_1"),
			withMessage(validationTurnEvent(3, collaboration.EventAgentMessageCompleted, "turn_1", "agent_1")),
			validationTurnEvent(4, collaboration.EventSpeakerSelected, "turn_2", "agent_1"),
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator := collaboration.NewValidator(validationRequest())
			mustValidate(t, validator, runEvent(1, collaboration.EventAccepted))
			for index, event := range test.events {
				err := validator.Validate(event)
				if index == len(test.events)-1 {
					if !errors.Is(err, collaboration.ErrProtocol) {
						t.Fatalf("expected protocol error, got %v", err)
					}
					return
				}
				if err != nil {
					t.Fatalf("setup event %s failed: %v", event.Kind, err)
				}
			}
		})
	}
}

func TestValidatorEnforcesHandoffPolicyAndTerminalState(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*collaboration.Validator)
		event collaboration.Event
	}{
		{
			name: "outside handoff",
			setup: func(v *collaboration.Validator) {
				mustValidate(t, v, validationTurnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "agent_1"))
			},
			event: withHandoff(validationTurnEvent(3, collaboration.EventHandoffRequested, "turn_1", "agent_1"), "outside"),
		},
		{
			name: "self handoff",
			setup: func(v *collaboration.Validator) {
				mustValidate(t, v, validationTurnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "agent_1"))
			},
			event: withHandoff(validationTurnEvent(3, collaboration.EventHandoffRequested, "turn_1", "agent_1"), "agent_1"),
		},
		{
			name: "completed with active turn",
			setup: func(v *collaboration.Validator) {
				mustValidate(t, v, validationTurnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "agent_1"))
			},
			event: withTerminal(runEvent(3, collaboration.EventCompleted), 0, collaboration.StopReasonCompleted, nil),
		},
		{
			name:  "turn count mismatch",
			setup: func(*collaboration.Validator) {},
			event: withTerminal(runEvent(2, collaboration.EventCompleted), 1, collaboration.StopReasonCompleted, nil),
		},
		{
			name:  "terminal reason mismatch",
			setup: func(*collaboration.Validator) {},
			event: withTerminal(runEvent(2, collaboration.EventStopped), 0, collaboration.StopReasonCompleted, nil),
		},
		{
			name:  "failed without failure",
			setup: func(*collaboration.Validator) {},
			event: withTerminal(runEvent(2, collaboration.EventFailed), 0, collaboration.StopReasonEngineFailure, nil),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator := collaboration.NewValidator(validationRequest())
			mustValidate(t, validator, runEvent(1, collaboration.EventAccepted))
			test.setup(validator)
			if err := validator.Validate(test.event); !errors.Is(err, collaboration.ErrProtocol) {
				t.Fatalf("expected protocol error, got %v", err)
			}
		})
	}

	validator := collaboration.NewValidator(validationRequest())
	mustValidate(t, validator, runEvent(1, collaboration.EventAccepted))
	mustValidate(t, validator, withTerminal(runEvent(2, collaboration.EventCancelled), 0, collaboration.StopReasonCancelled, nil))
	if err := validator.Validate(withTerminal(runEvent(3, collaboration.EventCompleted), 0, collaboration.StopReasonCompleted, nil)); !errors.Is(err, collaboration.ErrProtocol) {
		t.Fatalf("expected event-after-terminal error, got %v", err)
	}
}


func runEvent(sequence uint64, kind collaboration.EventKind) collaboration.Event {
	return collaboration.Event{
		ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: "collab_1", Sequence: sequence, Kind: kind,
	}
}


func withMessage(event collaboration.Event) collaboration.Event {
	event.Message = &collaboration.AgentMessage{Content: "done"}
	return event
}

func withHandoff(event collaboration.Event, target string) collaboration.Event {
	event.Handoff = &collaboration.Handoff{TargetAgentID: target}
	return event
}

func withTerminal(event collaboration.Event, turns uint32, reason collaboration.StopReason, failure *collaboration.Failure) collaboration.Event {
	event.Terminal = &collaboration.Terminal{TurnCount: turns, Reason: reason, Failure: failure}
	return event
}

func mustValidate(t *testing.T, validator *collaboration.Validator, event collaboration.Event) {
	t.Helper()
	if err := validator.Validate(event); err != nil {
		t.Fatalf("validate %s: %v", event.Kind, err)
	}
}
func validationTurnEvent(sequence int, kind collaboration.EventKind, turnID string, agentID string) collaboration.Event {
	return collaboration.Event{
		ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: "collab_1", Sequence: uint64(sequence), Kind: kind,
		TurnID: turnID, AgentID: agentID,
	}
}
func validationRequest() collaboration.Request {
	req := request("room_1", "collab_1")
	req.Snapshot.Policy.AllowAgentHandoff = true
	req.Snapshot.Policy.MaxTurns = 10
	req.Snapshot.Policy.MaxTurnsPerAgent = 1
	req.Snapshot.Agents = append(req.Snapshot.Agents, collaboration.AgentSnapshot{ID: "agent_2"})
	return req
}

// ---------------------------------------------------------------------------
// Task 7: Event-order contract for preparation failures
// ---------------------------------------------------------------------------

func TestValidatorAcceptsPreparationFailureSequence(t *testing.T) {
	validator := collaboration.NewValidator(validationRequest())
	events := []collaboration.Event{
		runEvent(1, collaboration.EventAccepted),
		runEvent(2, collaboration.EventCollaborationStarted),
		validationTurnEvent(3, collaboration.EventSpeakerSelected, "turn_1", "agent_1"),
		validationTurnEvent(4, collaboration.EventAgentTurnStarted, "turn_1", "agent_1"),
		withTerminal(
			runEvent(5, collaboration.EventFailed),
			0,
			collaboration.StopReasonEngineFailure,
			&collaboration.Failure{Code: collaboration.ErrorModelNotConfigured, Message: "model not configured", Retryable: false},
		),
	}
	for _, event := range events {
		if err := validator.Validate(event); err != nil {
			t.Fatalf("validate %s: %v", event.Kind, err)
		}
	}
	if !validator.TerminalSeen() {
		t.Fatal("expected terminal to be seen after preparation failure")
	}
}

func TestValidatorRejectsModelStartedAfterPreparationFailure(t *testing.T) {
	validator := collaboration.NewValidator(validationRequest())
	mustValidate(t, validator, runEvent(1, collaboration.EventAccepted))
	mustValidate(t, validator, runEvent(2, collaboration.EventCollaborationStarted))
	mustValidate(t, validator, validationTurnEvent(3, collaboration.EventSpeakerSelected, "turn_1", "agent_1"))
	mustValidate(t, validator, validationTurnEvent(4, collaboration.EventAgentTurnStarted, "turn_1", "agent_1"))
	failed := withTerminal(
		runEvent(5, collaboration.EventFailed),
		0,
		collaboration.StopReasonEngineFailure,
		&collaboration.Failure{Code: collaboration.ErrorModelNotConfigured, Retryable: false},
	)
	mustValidate(t, validator, failed)
	lateEvent := validationTurnEvent(6, collaboration.EventModelStarted, "turn_1", "agent_1")
	if err := validator.Validate(lateEvent); !errors.Is(err, collaboration.ErrProtocol) {
		t.Fatalf("expected protocol error for event after terminal, got %v", err)
	}
}
