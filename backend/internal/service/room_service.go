package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/logging"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/store"
)

// RoomService coordinates room use cases across runtime room state, persistence, and agents.
// HTTP/WebSocket handlers should depend on this layer instead of orchestrating store writes directly.
type RoomService struct {
	manager *room.Manager
	agents  *AgentService
	runner  *agent.Runner
	store   store.Store
	logger  *slog.Logger
}

func NewRoomService(manager *room.Manager, agents *AgentService, runner *agent.Runner, s store.Store) *RoomService {
	return &RoomService{
		manager: manager,
		agents:  agents,
		runner:  runner,
		store:   s,
		logger:  logging.Component("room_service"),
	}
}

func (s *RoomService) Ping(ctx context.Context) error {
	return s.store.Ping(ctx)
}

func (s *RoomService) CreateRoom(ctx context.Context, name string, agentIDs []string) (*room.Room, error) {
	return s.manager.CreateRoom(ctx, name, agentIDs)
}

func (s *RoomService) GetRoom(ctx context.Context, roomID string) (*room.Room, bool) {
	return s.manager.GetRoom(ctx, roomID)
}

func (s *RoomService) Agents() []model.AgentConfig {
	return s.agents.Agents()
}

func (s *RoomService) CreateAgent(ctx context.Context, name, role, description, systemPrompt string, enabled bool) (model.Agent, error) {
	return s.agents.CreateAgent(ctx, name, role, description, systemPrompt, enabled)
}

func (s *RoomService) UpdateAgent(ctx context.Context, agentID string, input UpdateAgentInput) (model.Agent, error) {
	return s.agents.UpdateAgent(ctx, agentID, input)
}

func (s *RoomService) DeleteAgent(ctx context.Context, agentID string) error {
	return s.agents.DeleteAgent(ctx, agentID)
}

func (s *RoomService) ListMessages(ctx context.Context, currentRoom *room.Room, limit int) []model.Message {
	roomInfo := currentRoom.Info()
	messages, err := s.store.ListMessages(ctx, store.ListMessagesQuery{
		RoomID: roomInfo.ID,
		Limit:  limit,
	})
	if err != nil {
		s.logger.Warn("list messages from store failed; using room cache", "room_id", roomInfo.ID, "error", err)
		return currentRoom.Messages()
	}
	return messages
}

func (s *RoomService) JoinParticipant(ctx context.Context, currentRoom *room.Room, name string) model.Participant {
	participant := currentRoom.NewParticipant(name)
	roomInfo := currentRoom.Info()
	savedParticipant, err := s.store.AddParticipant(ctx, store.AddParticipantInput{
		ID:          participant.ID,
		RoomID:      roomInfo.ID,
		DisplayName: participant.Name,
		JoinedAt:    participant.JoinedAt,
	})
	if err != nil {
		s.logger.Error("persist participant", "room_id", roomInfo.ID, "participant_id", participant.ID, "error", err)
		savedParticipant = participant
	}
	currentRoom.AddParticipantFromStore(savedParticipant)

	return savedParticipant
}

func (s *RoomService) LeaveParticipant(ctx context.Context, currentRoom *room.Room, participantID string) bool {
	removed := currentRoom.RemoveParticipant(participantID)
	if err := s.store.MarkParticipantLeft(ctx, participantID, time.Now().UTC()); err != nil {
		s.logger.Error("mark participant left", "participant_id", participantID, "error", err)
	}
	return removed
}

func (s *RoomService) HandleHumanMessage(ctx context.Context, currentRoom *room.Room, participant model.Participant, content string) (model.Message, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return model.Message{}, fmt.Errorf("message content must not be empty")
	}

	message := currentRoom.NewHumanMessage(participant, trimmed)
	savedMessage, err := s.store.AddMessage(ctx, message)
	if err != nil {
		s.logger.Error("persist human message", "room_id", currentRoom.Info().ID, "participant_id", participant.ID, "error", err)
		return model.Message{}, fmt.Errorf("persist human message: %w", err)
	}

	currentRoom.AppendMessage(savedMessage)
	go s.runner.HandleHumanMessage(context.Background(), currentRoom, savedMessage)

	return savedMessage, nil
}
