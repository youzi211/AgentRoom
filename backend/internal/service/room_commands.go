package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/store"
)

func (s *RoomService) CreateRoom(ctx context.Context, name string, agentIDs []string, passcode string, dialoguePolicy model.DialoguePolicy) (*room.Room, error) {
	return s.manager.CreateRoom(ctx, name, agentIDs, HashRoomPasscode(passcode), dialoguePolicy)
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
		return model.KnowledgeDocument{}, ErrRoomNotFound
	}
	return s.knowledge.UploadMarkdown(ctx, UploadKnowledgeInput{
		Scope:    model.KnowledgeScopeRoom,
		ScopeID:  roomID,
		FileName: fileName,
		Content:  content,
	})
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

func (s *RoomService) DeleteKnowledgeDocument(ctx context.Context, documentID string) error {
	return s.knowledge.DeleteDocument(ctx, documentID)
}

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

func (s *RoomService) SaveManualMinutes(ctx context.Context, currentRoom *room.Room, content string) (model.MeetingMinutes, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return model.MeetingMinutes{}, ErrMinutesContentEmpty
	}
	return s.persistMinutes(ctx, currentRoom.Info().ID, trimmed, model.MinutesSourceManual)
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
		return model.Message{}, nil, ErrMessageContentEmpty
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
	if ctx == nil {
		ctx = context.Background()
	}
	s.ensureResponseWorkers()
	select {
	case s.responseJobs <- agentResponseJob{ctx: ctx, room: currentRoom, message: message}:
	case <-ctx.Done():
		s.logger.Warn("skip queued agent response", "room_id", currentRoom.Info().ID, "message_id", message.ID, "error", ctx.Err())
	}
}

func (s *RoomService) TransferRoomOwner(ctx context.Context, currentRoom *room.Room, callerParticipantID string, targetParticipantID string) error {
	return s.lifecycle.TransferOwner(ctx, currentRoom, callerParticipantID, targetParticipantID)
}

func (s *RoomService) CloseRoomByOwner(ctx context.Context, currentRoom *room.Room, callerParticipantID string) error {
	return s.lifecycle.CloseByOwner(ctx, currentRoom, callerParticipantID)
}

func (s *RoomService) ensureResponseWorkers() {
	s.responseStart.Do(func() {
		s.responseJobs = make(chan agentResponseJob, defaultAgentResponseQueue)
		for i := 0; i < defaultAgentResponseWorkers; i++ {
			go s.runAgentResponseWorker()
		}
	})
}

func (s *RoomService) runAgentResponseWorker() {
	for job := range s.responseJobs {
		s.runner.HandleHumanMessage(job.ctx, job.room, job.message)
	}
}
