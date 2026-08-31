package testcollaboration

import (
	"context"
	"io"
	"sync"

	"agentroom/backend/internal/collaboration"
)

type Runtime struct {
	mu sync.Mutex

	Events              []collaboration.Event
	ExecuteErr          error
	StreamErr           error
	WaitForCancellation bool
	Started             chan struct{}
	Cancelled           chan struct{}

	requests    []collaboration.Request
	startedOnce sync.Once
	cancelOnce  sync.Once
}

var _ collaboration.CollaborationRuntime = (*Runtime)(nil)

func (f *Runtime) ExecuteConversation(ctx context.Context, request collaboration.Request) (collaboration.EventStream, error) {
	f.mu.Lock()
	f.requests = append(f.requests, request)
	events := append([]collaboration.Event(nil), f.Events...)
	executeErr := f.ExecuteErr
	streamErr := f.StreamErr
	waitForCancellation := f.WaitForCancellation
	f.mu.Unlock()

	if f.Started != nil {
		f.startedOnce.Do(func() { close(f.Started) })
	}
	if executeErr != nil {
		return nil, executeErr
	}
	return &eventStream{
		ctx:                 ctx,
		events:              events,
		finalErr:            streamErr,
		waitForCancellation: waitForCancellation,
		onCancelled:         f.signalCancelled,
	}, nil
}

func (f *Runtime) Requests() []collaboration.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]collaboration.Request(nil), f.requests...)
}

func (f *Runtime) signalCancelled() {
	if f.Cancelled != nil {
		f.cancelOnce.Do(func() { close(f.Cancelled) })
	}
}

type eventStream struct {
	ctx                 context.Context
	events              []collaboration.Event
	next                int
	finalErr            error
	waitForCancellation bool
	onCancelled         func()
}

func (s *eventStream) Recv() (collaboration.Event, error) {
	select {
	case <-s.ctx.Done():
		s.onCancelled()
		return collaboration.Event{}, s.ctx.Err()
	default:
	}

	if s.next < len(s.events) {
		event := s.events[s.next]
		s.next++
		return event, nil
	}
	if s.waitForCancellation {
		<-s.ctx.Done()
		s.onCancelled()
		return collaboration.Event{}, s.ctx.Err()
	}
	if s.finalErr != nil {
		return collaboration.Event{}, s.finalErr
	}
	return collaboration.Event{}, io.EOF
}
