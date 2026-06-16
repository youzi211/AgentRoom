package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/store"
	"agentroom/backend/internal/tests/teststore"
)

const expectedAgentResponseWorkers = 4

type barrierLLM struct {
	mu       sync.Mutex
	active   int
	maxActive int
	started  chan struct{}
	release  chan struct{}
	done     sync.WaitGroup
}

func newBarrierLLM(total int) *barrierLLM {
	client := &barrierLLM{
		started: make(chan struct{}, total),
		release: make(chan struct{}),
	}
	client.done.Add(total)
	return client
}

func (c *barrierLLM) Complete(context.Context, []llm.ChatMessage) (string, error) {
	c.mu.Lock()
	c.active++
	if c.active > c.maxActive {
		c.maxActive = c.active
	}
	c.mu.Unlock()

	select {
	case c.started <- struct{}{}:
	default:
	}

	<-c.release

	c.mu.Lock()
	c.active--
	c.mu.Unlock()
	c.done.Done()
	return "Ack", nil
}

func (c *barrierLLM) MaxActive() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.maxActive
}

type synchronizedStore struct {
	mu sync.Mutex
	teststore.Store
}

func (s *synchronizedStore) AddMessage(ctx context.Context, message model.Message) (model.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Store.AddMessage(ctx, message)
}

func (s *synchronizedStore) CreateAgentRun(ctx context.Context, run store.AgentRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Store.CreateAgentRun(ctx, run)
}

func (s *synchronizedStore) FinishAgentRun(ctx context.Context, runID string, status string, errText string, completedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Store.FinishAgentRun(ctx, runID, status, errText, completedAt)
}

func TestTriggerAgentResponsesUsesBoundedConcurrency(t *testing.T) {
	const totalTriggers = expectedAgentResponseWorkers + 6

	store := &synchronizedStore{}
	llmClient := newBarrierLLM(totalTriggers)
	runner := agent.NewRunner(llmClient, store)
	roomService := service.NewRoomService(nil, nil, nil, runner, nil, store)

	currentRoom := room.New("room_1", "Planning", []model.Agent{
		{
			ID:           "pm",
			Name:         "Product",
			Mention:      "@Product",
			SystemPrompt: "You are the product manager.",
			Enabled:      true,
		},
	})
	participant := currentRoom.NewParticipant("Alice")
	savedMessage, _, err := roomService.HandleHumanMessage(context.Background(), currentRoom, participant, "@Product please review this")
	if err != nil {
		t.Fatalf("HandleHumanMessage returned error: %v", err)
	}

	for i := 0; i < totalTriggers; i++ {
		roomService.TriggerAgentResponses(context.Background(), currentRoom, savedMessage)
	}

	for i := 0; i < expectedAgentResponseWorkers; i++ {
		select {
		case <-llmClient.started:
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("expected at least %d concurrent worker starts", expectedAgentResponseWorkers)
		}
	}

	time.Sleep(100 * time.Millisecond)
	if llmClient.MaxActive() > expectedAgentResponseWorkers {
		t.Fatalf("expected bounded agent response concurrency, got maxActive=%d", llmClient.MaxActive())
	}

	close(llmClient.release)

	done := make(chan struct{})
	go func() {
		llmClient.done.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected queued agent responses to drain after releasing the barrier")
	}
}
