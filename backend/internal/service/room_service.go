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
	manager   *room.Manager
	agents    *AgentService
	knowledge *KnowledgeService
	runner    *agent.Runner
	focus     *FocusService
	minutes   *MinutesService
	store     store.Store
	logger    *slog.Logger
}

func NewRoomService(manager *room.Manager, agents *AgentService, knowledge *KnowledgeService, runner *agent.Runner, focus *FocusService, s store.Store) *RoomService {
	return &RoomService{
		manager:   manager,
		agents:    agents,
		knowledge: knowledge,
		runner:    runner,
		focus:     focus,
		store:     s,
		logger:    logging.Component("room_service"),
	}
}

func (s *RoomService) WithMinutes(minutes *MinutesService) *RoomService {
	s.minutes = minutes
	return s
}

func (s *RoomService) Ping(ctx context.Context) error {
	return s.store.Ping(ctx)
}

func (s *RoomService) CreateRoom(ctx context.Context, name string, agentIDs []string, passcode string) (*room.Room, error) {
	return s.manager.CreateRoom(ctx, name, agentIDs, HashRoomPasscode(passcode))
}

func (s *RoomService) GetRoom(ctx context.Context, roomID string) (*room.Room, bool) {
	return s.manager.GetRoom(ctx, roomID)
}

func (s *RoomService) CanAccessRoom(currentRoom *room.Room, passcode string) bool {
	if currentRoom == nil {
		return false
	}
	return RoomPasscodeMatches(currentRoom.PasscodeHash(), passcode)
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

func (s *RoomService) UploadRoomKnowledge(ctx context.Context, roomID string, fileName string, content []byte) (model.KnowledgeDocument, error) {
	if _, ok := s.GetRoom(ctx, roomID); !ok {
		return model.KnowledgeDocument{}, fmt.Errorf("room not found")
	}
	return s.knowledge.UploadMarkdown(ctx, UploadKnowledgeInput{
		Scope:    model.KnowledgeScopeRoom,
		ScopeID:  roomID,
		FileName: fileName,
		Content:  content,
	})
}

func (s *RoomService) ListRoomKnowledge(ctx context.Context, roomID string) ([]model.KnowledgeDocument, error) {
	if _, ok := s.GetRoom(ctx, roomID); !ok {
		return nil, fmt.Errorf("room not found")
	}
	return s.knowledge.ListDocuments(ctx, model.KnowledgeScopeRoom, roomID)
}

func (s *RoomService) UploadAgentKnowledge(ctx context.Context, agentID string, fileName string, content []byte) (model.KnowledgeDocument, error) {
	if _, ok := s.agentByID(agentID); !ok {
		return model.KnowledgeDocument{}, ErrAgentNotFound
	}
	return s.knowledge.UploadMarkdown(ctx, UploadKnowledgeInput{
		Scope:    model.KnowledgeScopeAgent,
		ScopeID:  agentID,
		FileName: fileName,
		Content:  content,
	})
}

func (s *RoomService) ListAgentKnowledge(ctx context.Context, agentID string) ([]model.KnowledgeDocument, error) {
	if _, ok := s.agentByID(agentID); !ok {
		return nil, ErrAgentNotFound
	}
	return s.knowledge.ListDocuments(ctx, model.KnowledgeScopeAgent, agentID)
}

func (s *RoomService) DeleteKnowledgeDocument(ctx context.Context, documentID string) error {
	return s.knowledge.DeleteDocument(ctx, documentID)
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

func (s *RoomService) GenerateMinutes(ctx context.Context, currentRoom *room.Room) (string, error) {
	messages := s.ListMessages(ctx, currentRoom, 500)
	if s.minutes != nil {
		return s.minutes.Generate(ctx, currentRoom.Info(), messages)
	}
	return fallbackMinutes(currentRoom.Info(), messages), nil
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

func (s *RoomService) HandleHumanMessage(ctx context.Context, currentRoom *room.Room, participant model.Participant, content string) (model.Message, []model.FocusPoint, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return model.Message{}, nil, fmt.Errorf("message content must not be empty")
	}

	message := currentRoom.NewHumanMessage(participant, trimmed)
	savedMessage, err := s.store.AddMessage(ctx, message)
	if err != nil {
		s.logger.Error("persist human message", "room_id", currentRoom.Info().ID, "participant_id", participant.ID, "error", err)
		return model.Message{}, nil, fmt.Errorf("persist human message: %w", err)
	}

	currentRoom.AppendMessage(savedMessage)

	var focusPoints []model.FocusPoint
	if s.focus != nil {
		focusPoints = s.focus.AddMessage(ctx, currentRoom.Info().ID, savedMessage)
	}

	return savedMessage, focusPoints, nil
}

func (s *RoomService) TriggerAgentResponses(ctx context.Context, currentRoom *room.Room, message model.Message) {
	if s == nil || s.runner == nil || currentRoom == nil {
		return
	}
	go s.runner.HandleHumanMessage(ctx, currentRoom, message)
}

func (s *RoomService) agentByID(agentID string) (model.AgentConfig, bool) {
	for _, configuredAgent := range s.agents.Agents() {
		if configuredAgent.ID == agentID {
			return configuredAgent, true
		}
	}
	return model.AgentConfig{}, false
}
