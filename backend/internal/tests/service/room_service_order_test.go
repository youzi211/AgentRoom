package service_test

import (
	"context"
	"testing"
	"time"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/tests/teststore"
)

func TestHandleHumanMessageDoesNotStartAgentResponsesBeforeCallerBroadcasts(t *testing.T) {
	store := &teststore.Store{}
	llmClient := &blockingLLM{called: make(chan struct{}, 1)}
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

	if _, _, err := roomService.HandleHumanMessage(context.Background(), currentRoom, participant, "@Product please review this"); err != nil {
		t.Fatalf("HandleHumanMessage returned error: %v", err)
	}

	select {
	case <-llmClient.called:
		t.Fatal("expected agent responses to wait until the caller broadcasts the human message")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestTriggerAgentResponsesStartsAgentWorkAfterCallerBroadcasts(t *testing.T) {
	store := &teststore.Store{}
	llmClient := &blockingLLM{called: make(chan struct{}, 1)}
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

	roomService.TriggerAgentResponses(context.Background(), currentRoom, savedMessage)

	select {
	case <-llmClient.called:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected explicit trigger to start agent responses")
	}
}

func TestTriggerAgentResponsesUsesCollaborationSchedulerInsteadOfLegacyRunner(t *testing.T) {
	store := &teststore.Store{}
	llmClient := &blockingLLM{called: make(chan struct{}, 1)}
	runner := agent.NewRunner(llmClient, store)
	scheduler := &blockingCollaborationScheduler{
		called:  make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	roomService := service.NewRoomService(nil, nil, nil, runner, nil, store).WithCollaborationScheduler(scheduler)

	currentRoom := room.New("room_1", "Planning", []model.Agent{
		{
			ID:           "pm",
			Name:         "Product",
			Mention:      "@Product",
			SystemPrompt: "You are the product manager.",
			Enabled:      true,
		},
	})
	message := model.Message{
		ID:         "message_1",
		RoomID:     currentRoom.Info().ID,
		SenderType: model.SenderTypeHuman,
		Content:    "@Product please review this",
	}

	roomService.TriggerAgentResponses(context.Background(), currentRoom, message)

	select {
	case <-scheduler.called:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected collaboration scheduler to receive the human message")
	}
	select {
	case <-llmClient.called:
		t.Fatal("expected remote collaboration routing to bypass the legacy Runner")
	default:
	}

	close(scheduler.release)
	select {
	case <-llmClient.called:
		t.Fatal("expected remote collaboration routing not to fall back to the legacy Runner")
	case <-time.After(50 * time.Millisecond):
	}
}

type blockingLLM struct {
	called chan struct{}
}

func (b *blockingLLM) Complete(context.Context, []llm.ChatMessage) (string, error) {
	select {
	case b.called <- struct{}{}:
	default:
	}
	return "Ack", nil
}

type blockingCollaborationScheduler struct {
	called  chan struct{}
	release chan struct{}
}

func (s *blockingCollaborationScheduler) HandleHumanMessage(context.Context, *room.Room, model.Message) error {
	select {
	case s.called <- struct{}{}:
	default:
	}
	<-s.release
	return nil
}
