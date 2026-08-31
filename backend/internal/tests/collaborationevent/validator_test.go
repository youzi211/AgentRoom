package collaborationevent_test

import (
	"errors"
	"testing"

	"agentroom/backend/internal/collaboration"
	"agentroom/backend/internal/collaborationevent"
)

func TestValidatorAcceptsOrderedEligibleTurnsAndUniqueTerminal(t *testing.T) {
	validator := collaborationevent.NewValidator(request())
	events := []collaboration.Event{
		runEvent(1, collaboration.EventAccepted),
		runEvent(2, collaboration.EventCollaborationStarted),
		turnEvent(3, collaboration.EventSpeakerSelected, "turn_1", "agent_1"),
		turnEvent(4, collaboration.EventAgentTurnStarted, "turn_1", "agent_1"),
		turnEvent(5, collaboration.EventModelStarted, "turn_1", "agent_1"),
		withHandoff(turnEvent(6, collaboration.EventHandoffRequested, "turn_1", "agent_1"), "agent_2"),
		withMessage(turnEvent(7, collaboration.EventAgentMessageCompleted, "turn_1", "agent_1")),
		turnEvent(8, collaboration.EventSpeakerSelected, "turn_2", "agent_2"),
		withMessage(turnEvent(9, collaboration.EventAgentMessageCompleted, "turn_2", "agent_2")),
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
			validator := collaborationevent.NewValidator(request())
			if err := validator.Validate(test.event); !errors.Is(err, collaborationevent.ErrProtocol) {
				t.Fatalf("expected protocol error, got %v", err)
			}
			if err := validator.Validate(runEvent(1, collaboration.EventAccepted)); err != nil {
				t.Fatalf("failed event advanced validator state: %v", err)
			}
		})
	}

	validator := collaborationevent.NewValidator(request())
	mustValidate(t, validator, runEvent(1, collaboration.EventAccepted))
	mustValidate(t, validator, runEvent(3, collaboration.EventCollaborationStarted))
	if err := validator.Validate(runEvent(2, collaboration.EventCheckpoint)); !errors.Is(err, collaborationevent.ErrProtocol) {
		t.Fatalf("expected decreasing sequence error, got %v", err)
	}
}

func TestValidatorRejectsIneligibleAgentsAndInvalidTurnTransitions(t *testing.T) {
	tests := []struct {
		name   string
		events []collaboration.Event
	}{
		{name: "unknown Agent", events: []collaboration.Event{turnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "outside")}},
		{name: "unknown turn", events: []collaboration.Event{turnEvent(2, collaboration.EventAgentTurnStarted, "turn_1", "agent_1")}},
		{name: "Agent changed", events: []collaboration.Event{
			turnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "agent_1"),
			turnEvent(3, collaboration.EventAgentTurnStarted, "turn_1", "agent_2"),
		}},
		{name: "concurrent turn", events: []collaboration.Event{
			turnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "agent_1"),
			turnEvent(3, collaboration.EventSpeakerSelected, "turn_2", "agent_2"),
		}},
		{name: "duplicate turn completion", events: []collaboration.Event{
			turnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "agent_1"),
			withMessage(turnEvent(3, collaboration.EventAgentMessageCompleted, "turn_1", "agent_1")),
			withMessage(turnEvent(4, collaboration.EventAgentMessageCompleted, "turn_1", "agent_1")),
		}},
		{name: "turn ID reused", events: []collaboration.Event{
			turnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "agent_1"),
			withMessage(turnEvent(3, collaboration.EventAgentMessageCompleted, "turn_1", "agent_1")),
			turnEvent(4, collaboration.EventSpeakerSelected, "turn_1", "agent_2"),
		}},
		{name: "per Agent limit", events: []collaboration.Event{
			turnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "agent_1"),
			withMessage(turnEvent(3, collaboration.EventAgentMessageCompleted, "turn_1", "agent_1")),
			turnEvent(4, collaboration.EventSpeakerSelected, "turn_2", "agent_1"),
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator := collaborationevent.NewValidator(request())
			mustValidate(t, validator, runEvent(1, collaboration.EventAccepted))
			for index, event := range test.events {
				err := validator.Validate(event)
				if index == len(test.events)-1 {
					if !errors.Is(err, collaborationevent.ErrProtocol) {
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
		setup func(*collaborationevent.Validator)
		event collaboration.Event
	}{
		{
			name: "outside handoff",
			setup: func(v *collaborationevent.Validator) {
				mustValidate(t, v, turnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "agent_1"))
			},
			event: withHandoff(turnEvent(3, collaboration.EventHandoffRequested, "turn_1", "agent_1"), "outside"),
		},
		{
			name: "self handoff",
			setup: func(v *collaborationevent.Validator) {
				mustValidate(t, v, turnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "agent_1"))
			},
			event: withHandoff(turnEvent(3, collaboration.EventHandoffRequested, "turn_1", "agent_1"), "agent_1"),
		},
		{
			name: "completed with active turn",
			setup: func(v *collaborationevent.Validator) {
				mustValidate(t, v, turnEvent(2, collaboration.EventSpeakerSelected, "turn_1", "agent_1"))
			},
			event: withTerminal(runEvent(3, collaboration.EventCompleted), 0, collaboration.StopReasonCompleted, nil),
		},
		{
			name:  "turn count mismatch",
			setup: func(*collaborationevent.Validator) {},
			event: withTerminal(runEvent(2, collaboration.EventCompleted), 1, collaboration.StopReasonCompleted, nil),
		},
		{
			name:  "terminal reason mismatch",
			setup: func(*collaborationevent.Validator) {},
			event: withTerminal(runEvent(2, collaboration.EventStopped), 0, collaboration.StopReasonCompleted, nil),
		},
		{
			name:  "failed without failure",
			setup: func(*collaborationevent.Validator) {},
			event: withTerminal(runEvent(2, collaboration.EventFailed), 0, collaboration.StopReasonEngineFailure, nil),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator := collaborationevent.NewValidator(request())
			mustValidate(t, validator, runEvent(1, collaboration.EventAccepted))
			test.setup(validator)
			if err := validator.Validate(test.event); !errors.Is(err, collaborationevent.ErrProtocol) {
				t.Fatalf("expected protocol error, got %v", err)
			}
		})
	}

	validator := collaborationevent.NewValidator(request())
	mustValidate(t, validator, runEvent(1, collaboration.EventAccepted))
	mustValidate(t, validator, withTerminal(runEvent(2, collaboration.EventCancelled), 0, collaboration.StopReasonCancelled, nil))
	if err := validator.Validate(withTerminal(runEvent(3, collaboration.EventCompleted), 0, collaboration.StopReasonCompleted, nil)); !errors.Is(err, collaborationevent.ErrProtocol) {
		t.Fatalf("expected event-after-terminal error, got %v", err)
	}
}

func request() collaboration.Request {
	return collaboration.Request{
		ProtocolVersion:    collaboration.ProtocolVersion,
		CollaborationRunID: "collab_1",
		Snapshot: collaboration.ConversationSnapshot{
			Agents: []collaboration.AgentSnapshot{{ID: "agent_1"}, {ID: "agent_2"}},
			Policy: collaboration.PolicySnapshot{
				MaxTurns: 3, MaxTurnsPerAgent: 1, AllowAgentHandoff: true,
			},
		},
	}
}

func runEvent(sequence uint64, kind collaboration.EventKind) collaboration.Event {
	return collaboration.Event{
		ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: "collab_1", Sequence: sequence, Kind: kind,
	}
}

func turnEvent(sequence uint64, kind collaboration.EventKind, turnID, agentID string) collaboration.Event {
	event := runEvent(sequence, kind)
	event.TurnID, event.AgentID = turnID, agentID
	return event
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

func mustValidate(t *testing.T, validator *collaborationevent.Validator, event collaboration.Event) {
	t.Helper()
	if err := validator.Validate(event); err != nil {
		t.Fatalf("validate %s: %v", event.Kind, err)
	}
}
