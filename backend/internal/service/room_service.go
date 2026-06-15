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

func (s *RoomService) CreateRoom(ctx context.Context, name string, agentIDs []string, passcode string, dialoguePolicy model.DialoguePolicy) (*room.Room, error) {
	return s.manager.CreateRoom(ctx, name, agentIDs, HashRoomPasscode(passcode), dialoguePolicy)
}

func (s *RoomService) GetRoom(ctx context.Context, roomID string) (*room.Room, bool) {
	return s.manager.GetRoom(ctx, roomID)
}

// ErrRoomArchived is returned when a write is attempted on an archived room.
var ErrRoomArchived = fmt.Errorf("room is archived")

func (s *RoomService) ListRooms(ctx context.Context, query store.ListRoomsQuery) ([]model.RoomSummary, error) {
	return s.store.ListRooms(ctx, query)
}

func (s *RoomService) ArchiveRoom(ctx context.Context, roomID string) error {
	return s.setRoomStatus(ctx, roomID, model.RoomStatusArchived)
}

func (s *RoomService) RestoreRoom(ctx context.Context, roomID string) error {
	return s.setRoomStatus(ctx, roomID, model.RoomStatusActive)
}

func (s *RoomService) setRoomStatus(ctx context.Context, roomID string, status string) error {
	if _, ok := s.GetRoom(ctx, roomID); !ok {
		return fmt.Errorf("room not found")
	}
	var archivedAt *time.Time
	if status == model.RoomStatusArchived {
		now := time.Now().UTC()
		archivedAt = &now
	}
	if err := s.store.SetRoomStatus(ctx, roomID, status, archivedAt); err != nil {
		return err
	}
	// Reflect the change in the live room so it takes effect without a reload.
	if currentRoom, ok := s.GetRoom(ctx, roomID); ok {
		currentRoom.SetStatus(status, archivedAt)
	}
	return nil
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

func (s *RoomService) ListRoomActivity(ctx context.Context, currentRoom *room.Room, limit int) (model.RoomActivityResponse, error) {
	roomInfo := currentRoom.Info()
	query := store.ListRunsQuery{RoomID: roomInfo.ID, Limit: limit}

	agentRuns, err := s.store.ListAgentRuns(ctx, query)
	if err != nil {
		return model.RoomActivityResponse{}, err
	}
	dialogueRuns, err := s.store.ListDialogueRuns(ctx, query)
	if err != nil {
		return model.RoomActivityResponse{}, err
	}

	agentNameByID := make(map[string]string)
	for _, roomAgent := range currentRoom.Agents() {
		agentNameByID[roomAgent.ID] = roomAgent.Name
	}

	activity := model.RoomActivityResponse{
		AgentRuns:    make([]model.AgentRunActivity, 0, len(agentRuns)),
		DialogueRuns: make([]model.DialogueRunActivity, 0, len(dialogueRuns)),
	}
	for _, run := range agentRuns {
		activity.AgentRuns = append(activity.AgentRuns, model.AgentRunActivity{
			ID:               run.ID,
			RoomID:           run.RoomID,
			AgentID:          run.AgentID,
			AgentName:        agentNameByID[run.AgentID],
			TriggerMessageID: run.TriggerMessageID,
			Status:           run.Status,
			ErrorText:        run.Error,
			CreatedAt:        run.StartedAt,
			CompletedAt:      run.CompletedAt,
		})
	}
	for _, run := range dialogueRuns {
		activity.DialogueRuns = append(activity.DialogueRuns, model.DialogueRunActivity{
			ID:               run.ID,
			RoomID:           run.RoomID,
			TriggerMessageID: run.TriggerMessageID,
			Mode:             run.Mode,
			TurnCount:        run.TurnCount,
			Status:           run.Status,
			CreatedAt:        run.StartedAt,
			CompletedAt:      run.CompletedAt,
		})
	}
	return activity, nil
}

// GenerateMinutes produces meeting minutes from the room transcript and
// persists them as a new AI-sourced version. The returned markdown is the
// content of the saved version (or the freshly generated content if
// persistence fails).
func (s *RoomService) GenerateMinutes(ctx context.Context, currentRoom *room.Room) (string, model.MeetingMinutes, error) {
	messages := s.ListMessages(ctx, currentRoom, 500)
	roomInfo := currentRoom.Info()

	var markdown string
	if s.minutes != nil {
		generated, err := s.minutes.Generate(ctx, roomInfo, messages)
		if err != nil {
			return "", model.MeetingMinutes{}, err
		}
		markdown = generated
	} else {
		markdown = fallbackMinutes(roomInfo, messages)
	}

	saved, err := s.persistMinutes(ctx, roomInfo.ID, markdown, model.MinutesSourceAI)
	if err != nil {
		s.logger.Warn("persist generated minutes failed", "room_id", roomInfo.ID, "error", err)
		return markdown, model.MeetingMinutes{}, nil
	}
	return saved.Content, saved, nil
}

// ListMinutes returns all persisted minutes versions for a room, newest first.
func (s *RoomService) ListMinutes(ctx context.Context, currentRoom *room.Room) ([]model.MeetingMinutes, error) {
	return s.store.ListMinutes(ctx, currentRoom.Info().ID)
}

// SaveManualMinutes stores an admin-edited minutes body as a new manual version.
func (s *RoomService) SaveManualMinutes(ctx context.Context, currentRoom *room.Room, content string) (model.MeetingMinutes, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return model.MeetingMinutes{}, fmt.Errorf("minutes content must not be empty")
	}
	return s.persistMinutes(ctx, currentRoom.Info().ID, trimmed, model.MinutesSourceManual)
}

// LatestMinutesMarkdown returns the latest persisted minutes content, falling
// back to a freshly generated (and persisted) version when none exist yet.
func (s *RoomService) LatestMinutesMarkdown(ctx context.Context, currentRoom *room.Room) (string, error) {
	latest, ok, err := s.store.LatestMinutes(ctx, currentRoom.Info().ID)
	if err != nil {
		return "", err
	}
	if ok {
		return latest.Content, nil
	}
	markdown, _, err := s.GenerateMinutes(ctx, currentRoom)
	return markdown, err
}

func (s *RoomService) persistMinutes(ctx context.Context, roomID string, content string, source string) (model.MeetingMinutes, error) {
	return s.store.CreateMinutes(ctx, model.MeetingMinutes{
		ID:        model.NewID("minutes"),
		RoomID:    roomID,
		Content:   content,
		Source:    source,
		CreatedAt: time.Now().UTC(),
	})
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

	if currentRoom.Info().IsArchived() {
		return model.Message{}, nil, ErrRoomArchived
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
