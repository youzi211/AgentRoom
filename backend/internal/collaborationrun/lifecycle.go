package collaborationrun

import (
	"context"
	"errors"
	"fmt"
	"time"

	"agentroom/backend/internal/collaboration"
	"agentroom/backend/internal/collaborationevent"
	"agentroom/backend/internal/collaborationgrpc"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

var ErrInvalidLifecycle = errors.New("invalid collaboration run lifecycle")

const terminalWriteTimeout = 5 * time.Second

type lifecycleStore interface {
	StartCollaborationRun(context.Context, string, string, time.Time) error
	FinishCollaborationRun(context.Context, store.FinishCollaborationRunInput) error
}

// Lifecycle maps one validated Runtime stream to its collaboration run audit.
type Lifecycle struct {
	store             lifecycleStore
	runID             string
	engineVersion     string
	completedTurnIDs  map[string]struct{}
	pendingTerminal   *store.FinishCollaborationRunInput
	terminalCommitted bool
}

func New(store lifecycleStore, request collaboration.Request, engineVersion string) (*Lifecycle, error) {
	if store == nil {
		return nil, fmt.Errorf("%w: store is required", ErrInvalidLifecycle)
	}
	if request.CollaborationRunID == "" || engineVersion == "" {
		return nil, fmt.Errorf("%w: run ID and engine version are required", ErrInvalidLifecycle)
	}
	return &Lifecycle{
		store:            store,
		runID:            request.CollaborationRunID,
		engineVersion:    engineVersion,
		completedTurnIDs: make(map[string]struct{}),
	}, nil
}

func (l *Lifecycle) Handle(ctx context.Context, event collaboration.Event) error {
	if event.CollaborationRunID != l.runID {
		return fmt.Errorf("%w: unexpected collaboration run ID", ErrInvalidLifecycle)
	}
	if l.terminalCommitted {
		return nil
	}
	switch event.Kind {
	case collaboration.EventAccepted:
		return l.store.StartCollaborationRun(ctx, l.runID, l.engineVersion, eventTime(event.OccurredAt))
	case collaboration.EventAgentMessageCompleted:
		if event.TurnID != "" {
			l.completedTurnIDs[event.TurnID] = struct{}{}
		}
		return nil
	case collaboration.EventCompleted, collaboration.EventStopped, collaboration.EventCancelled, collaboration.EventFailed:
		terminal, err := terminalInput(l.runID, event)
		if err != nil {
			return err
		}
		l.pendingTerminal = &terminal
		if err := l.store.FinishCollaborationRun(ctx, terminal); err != nil {
			return err
		}
		l.terminalCommitted = true
		return nil
	default:
		return nil
	}
}

// Converge persists a terminal audit when execution returns without a committed terminal event.
func (l *Lifecycle) Converge(ctx context.Context, executionErr error) error {
	if l.terminalCommitted {
		return nil
	}
	terminalCtx, cancel := context.WithTimeout(withoutCancel(ctx), terminalWriteTimeout)
	defer cancel()

	if l.pendingTerminal != nil {
		if err := l.store.FinishCollaborationRun(terminalCtx, *l.pendingTerminal); err != nil {
			return err
		}
		l.terminalCommitted = true
		return nil
	}

	terminal := terminalFromError(l.runID, len(l.completedTurnIDs), executionErr, time.Now().UTC())
	l.pendingTerminal = &terminal
	if err := l.store.FinishCollaborationRun(terminalCtx, terminal); err != nil {
		return err
	}
	l.terminalCommitted = true
	return nil
}

func terminalInput(runID string, event collaboration.Event) (store.FinishCollaborationRunInput, error) {
	if event.Terminal == nil {
		return store.FinishCollaborationRunInput{}, fmt.Errorf("%w: terminal payload is required", ErrInvalidLifecycle)
	}
	input := store.FinishCollaborationRunInput{
		RunID: runID, TurnCount: int(event.Terminal.TurnCount), CompletedAt: eventTime(event.OccurredAt),
	}
	switch event.Kind {
	case collaboration.EventCompleted:
		input.Status = model.CollaborationRunStatusSucceeded
		input.StopReason = model.CollaborationStopReasonCompleted
	case collaboration.EventStopped:
		input.Status = model.CollaborationRunStatusStopped
		input.StopReason = string(event.Terminal.Reason)
	case collaboration.EventCancelled:
		input.StopReason = string(event.Terminal.Reason)
		switch event.Terminal.Reason {
		case collaboration.StopReasonCancelled:
			input.Status = model.CollaborationRunStatusCancelled
		case collaboration.StopReasonDeadlineExceeded:
			input.Status = model.CollaborationRunStatusTimeout
		case collaboration.StopReasonInterrupted:
			input.Status = model.CollaborationRunStatusInterrupted
		default:
			return store.FinishCollaborationRunInput{}, fmt.Errorf("%w: unsupported cancellation reason", ErrInvalidLifecycle)
		}
	case collaboration.EventFailed:
		input.Status = model.CollaborationRunStatusFailed
		input.StopReason = string(event.Terminal.Reason)
		input.Error = terminalFailureAudit(event.Terminal.Failure)
	default:
		return store.FinishCollaborationRunInput{}, fmt.Errorf("%w: event is not terminal", ErrInvalidLifecycle)
	}
	if !model.IsCollaborationStopReasonForStatus(input.Status, input.StopReason) {
		return store.FinishCollaborationRunInput{}, fmt.Errorf("%w: incompatible terminal status and reason", ErrInvalidLifecycle)
	}
	return input, nil
}

func terminalFromError(runID string, turnCount int, err error, completedAt time.Time) store.FinishCollaborationRunInput {
	input := store.FinishCollaborationRunInput{RunID: runID, TurnCount: turnCount, CompletedAt: completedAt}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		input.Status = model.CollaborationRunStatusTimeout
		input.StopReason = model.CollaborationStopReasonDeadlineExceeded
		input.Error = "collaboration deadline exceeded"
	case errors.Is(err, context.Canceled):
		input.Status = model.CollaborationRunStatusCancelled
		input.StopReason = model.CollaborationStopReasonCancelled
	case errors.Is(err, collaborationevent.ErrProtocol), errors.Is(err, collaborationgrpc.ErrProtocol):
		input.Status = model.CollaborationRunStatusFailed
		input.StopReason = model.CollaborationStopReasonProtocolError
		input.Error = "collaboration protocol validation failed"
	case errors.Is(err, collaborationgrpc.ErrUnavailable):
		input.Status = model.CollaborationRunStatusInterrupted
		input.StopReason = model.CollaborationStopReasonInterrupted
		input.Error = "collaboration Runtime became unavailable"
	default:
		input.Status = model.CollaborationRunStatusFailed
		input.StopReason = model.CollaborationStopReasonEngineFailure
		input.Error = "collaboration execution failed"
	}
	return input
}

func terminalFailureAudit(failure *collaboration.Failure) string {
	if failure == nil || failure.Code == "" {
		return "collaboration Engine failed"
	}
	return "collaboration Engine failed: " + string(failure.Code)
}

func withoutCancel(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

func eventTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}
