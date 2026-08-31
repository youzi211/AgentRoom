package collaborationcoordinator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"agentroom/backend/internal/collaboration"
	"agentroom/backend/internal/collaborationevent"
)

var (
	ErrInvalidConfig = errors.New("collaboration coordinator configuration is invalid")
	ErrCapacity      = errors.New("collaboration coordinator capacity exhausted")
	ErrDuplicateRun  = errors.New("collaboration run is already active")
	ErrClosed        = errors.New("collaboration coordinator is closed")
)

type Config struct {
	MaxConcurrent int
	MaxPending    int
}

func (c Config) Validate() error {
	if c.MaxConcurrent <= 0 {
		return fmt.Errorf("%w: max concurrency must be positive", ErrInvalidConfig)
	}
	if c.MaxPending < 0 {
		return fmt.Errorf("%w: max pending must not be negative", ErrInvalidConfig)
	}
	return nil
}

type EventHandler func(context.Context, collaboration.Event) error

type Coordinator struct {
	runtime collaboration.CollaborationRuntime
	config  Config
	slots   chan struct{}

	mu      sync.Mutex
	active  map[string]*activeRun
	pending int
	closed  bool
}

type activeRun struct {
	runID  string
	cancel context.CancelFunc
	done   chan struct{}
}

func New(runtime collaboration.CollaborationRuntime, config Config) (*Coordinator, error) {
	if runtime == nil {
		return nil, fmt.Errorf("%w: runtime is required", ErrInvalidConfig)
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &Coordinator{
		runtime: runtime,
		config:  config,
		slots:   make(chan struct{}, config.MaxConcurrent),
		active:  make(map[string]*activeRun),
	}, nil
}

// Execute runs one collaboration synchronously. A newer call for the same room
// cancels this call and waits for it to finish before entering the Runtime.
func (c *Coordinator) Execute(ctx context.Context, request collaboration.Request, handler EventHandler) error {
	if ctx == nil {
		ctx = context.Background()
	}
	roomID := request.Snapshot.Room.ID
	if roomID == "" || request.CollaborationRunID == "" {
		return fmt.Errorf("%w: room and run IDs are required", ErrInvalidConfig)
	}

	callCtx, cancel := context.WithCancel(ctx)
	current := &activeRun{runID: request.CollaborationRunID, cancel: cancel, done: make(chan struct{})}
	previous, err := c.replace(roomID, current)
	if err != nil {
		cancel()
		return err
	}
	defer c.finish(roomID, current)

	if previous != nil {
		previous.cancel()
		// Preserve the predecessor chain even if this call is itself replaced.
		// Otherwise a third call could overtake a slow-to-cancel first call.
		<-previous.done
		if err := callCtx.Err(); err != nil {
			return err
		}
	}
	if !c.isCurrent(roomID, current) {
		return context.Canceled
	}

	if err := c.acquire(callCtx); err != nil {
		return err
	}
	defer c.release()
	if err := callCtx.Err(); err != nil {
		return err
	}
	if !c.isCurrent(roomID, current) {
		return context.Canceled
	}

	stream, err := c.runtime.ExecuteConversation(callCtx, request)
	if err != nil {
		return err
	}
	validator := collaborationevent.NewValidator(request)
	for {
		event, recvErr := stream.Recv()
		if err := callCtx.Err(); err != nil {
			return err
		}
		if !c.isCurrent(roomID, current) {
			return context.Canceled
		}
		if errors.Is(recvErr, io.EOF) {
			if !validator.TerminalSeen() {
				return fmt.Errorf("%w: stream ended without a terminal event", collaborationevent.ErrProtocol)
			}
			return nil
		}
		if recvErr != nil {
			return recvErr
		}
		if err := validator.Validate(event); err != nil {
			return err
		}
		if handler != nil {
			if err := handler(callCtx, event); err != nil {
				return err
			}
		}
		if validator.TerminalSeen() {
			return nil
		}
	}
}

// CancelRoom cancels and waits for every run that was active or replacing an
// active run for the room before the call returns.
func (c *Coordinator) CancelRoom(ctx context.Context, roomID string) error {
	if roomID == "" {
		return fmt.Errorf("%w: room ID is required", ErrInvalidConfig)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		c.mu.Lock()
		current := c.active[roomID]
		if current != nil {
			current.cancel()
		}
		c.mu.Unlock()
		if current == nil {
			return nil
		}
		select {
		case <-current.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Shutdown rejects new runs, cancels all active rooms, and waits for cleanup.
func (c *Coordinator) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	c.mu.Lock()
	c.closed = true
	active := make([]*activeRun, 0, len(c.active))
	for _, current := range c.active {
		current.cancel()
		active = append(active, current)
	}
	c.mu.Unlock()

	for _, current := range active {
		select {
		case <-current.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (c *Coordinator) replace(roomID string, current *activeRun) (*activeRun, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, ErrClosed
	}
	previous := c.active[roomID]
	if previous != nil && previous.runID == current.runID {
		return nil, ErrDuplicateRun
	}
	c.active[roomID] = current
	return previous, nil
}

func (c *Coordinator) finish(roomID string, current *activeRun) {
	current.cancel()
	c.mu.Lock()
	if c.active[roomID] == current {
		delete(c.active, roomID)
	}
	close(current.done)
	c.mu.Unlock()
}

func (c *Coordinator) isCurrent(roomID string, current *activeRun) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.active[roomID] == current
}

func (c *Coordinator) acquire(ctx context.Context) error {
	select {
	case c.slots <- struct{}{}:
		return nil
	default:
	}

	c.mu.Lock()
	if c.pending >= c.config.MaxPending {
		c.mu.Unlock()
		return ErrCapacity
	}
	c.pending++
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.pending--
		c.mu.Unlock()
	}()

	select {
	case c.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Coordinator) release() {
	<-c.slots
}
