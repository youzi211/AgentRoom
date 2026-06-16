package service

import (
	"context"
	"errors"
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
	lifecycle *MeetingLifecycle
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
		lifecycle: NewMeetingLifecycle(s),
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
	currentRoom, ok := s.manager.GetRoom(ctx, roomID)
	if !ok {
		return nil, false
	}
	if err := s.lifecycle.ReconcileLoadedRoom(ctx, currentRoom); err != nil {
		s.logger.Warn("reconcile room lifecycle", "room_id", roomID, "error", err)
	}
	return currentRoom, true
}

var (
	ErrRoomNotFound          = errors.New("room not found")
	ErrRoomClosed            = errors.New("room is closed")
	ErrRoomArchived          = errors.New("room is archived")
	ErrInvalidRoomTransition = errors.New("invalid room transition")
	ErrNotRoomOwner          = errors.New("not room owner")
	ErrOwnerTargetNotOnline  = errors.New("owner transfer target is not an online participant")
)

func (s *RoomService) ListRooms(ctx context.Context, query store.ListRoomsQuery) ([]model.RoomSummary, error) {
	return s.store.ListRooms(ctx, query)
}

func (s *RoomService) ArchiveRoom(ctx context.Context, roomID string) error {
	currentRoom, ok := s.GetRoom(ctx, roomID)
	if !ok {
		return ErrRoomNotFound
	}
	return s.lifecycle.Archive(ctx, currentRoom)
}

func (s *RoomService) RestoreRoom(ctx context.Context, roomID string) error {
	currentRoom, ok := s.GetRoom(ctx, roomID)
	if !ok {
		return ErrRoomNotFound
	}
	return s.lifecycle.Restore(ctx, currentRoom)
}

func (s *RoomService) ReopenRoom(ctx context.Context, roomID string) error {
	currentRoom, ok := s.GetRoom(ctx, roomID)
	if !ok {
		return ErrRoomNotFound
	}
	return s.lifecycle.Reopen(ctx, currentRoom)
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

func (s *RoomService) ListMessagesPage(ctx context.Context, currentRoom *room.Room, limit int, before string) (store.MessagePage, error) {
	return s.store.ListMessagesPage(ctx, store.ListMessagesQuery{
		RoomID: currentRoom.Info().ID,
		Limit:  limit,
		Before: before,
	})
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

// LatestPersistedMinutesMarkdown returns the latest saved minutes content
// without generating a new version as a side effect.
func (s *RoomService) LatestPersistedMinutesMarkdown(ctx context.Context, currentRoom *room.Room) (string, bool, error) {
	latest, ok, err := s.store.LatestMinutes(ctx, currentRoom.Info().ID)
	if err != nil {
		return "", false, err
	}
	if ok {
		return latest.Content, true, nil
	}
	return "", false, nil
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

func (s *RoomService) JoinParticipant(ctx context.Context, currentRoom *room.Room, name string) (model.Participant, error) {
	if currentRoom == nil {
		return model.Participant{}, ErrRoomNotFound
	}
	if !currentRoom.Info().IsActive() {
		return model.Participant{}, ErrRoomClosed
	}

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
		return model.Participant{}, err
	}
	currentRoom.AddParticipantFromStore(savedParticipant)
	if err := s.lifecycle.OnParticipantJoined(ctx, currentRoom, savedParticipant); err != nil {
		return model.Participant{}, err
	}

	return savedParticipant, nil
}

func (s *RoomService) LeaveParticipant(ctx context.Context, currentRoom *room.Room, participantID string) bool {
	removed := currentRoom.RemoveParticipant(participantID)
	if err := s.store.MarkParticipantLeft(ctx, participantID, time.Now().UTC()); err != nil {
		s.logger.Error("mark participant left", "participant_id", participantID, "error", err)
	}
	if removed {
		if err := s.lifecycle.OnParticipantLeft(ctx, currentRoom, participantID); err != nil {
			s.logger.Warn("room lifecycle on leave", "room_id", currentRoom.Info().ID, "participant_id", participantID, "error", err)
		}
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
	if currentRoom.Info().IsClosed() {
		return model.Message{}, nil, ErrRoomClosed
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

func (s *RoomService) TransferRoomOwner(ctx context.Context, currentRoom *room.Room, callerParticipantID string, targetParticipantID string) error {
	return s.lifecycle.TransferOwner(ctx, currentRoom, callerParticipantID, targetParticipantID)
}

func (s *RoomService) CloseRoomByOwner(ctx context.Context, currentRoom *room.Room, callerParticipantID string) error {
	return s.lifecycle.CloseByOwner(ctx, currentRoom, callerParticipantID)
}

func (s *RoomService) agentByID(agentID string) (model.AgentConfig, bool) {
	for _, configuredAgent := range s.agents.Agents() {
		if configuredAgent.ID == agentID {
			return configuredAgent, true
		}
	}
	return model.AgentConfig{}, false
}
