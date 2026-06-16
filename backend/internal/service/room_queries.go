package service

import (
	"context"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/store"
)

func (s *RoomService) Ping(ctx context.Context) error {
	return s.store.Ping(ctx)
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

func (s *RoomService) ListRooms(ctx context.Context, query ListRoomsInput) ([]model.RoomSummary, error) {
	return s.store.ListRooms(ctx, store.ListRoomsQuery{
		Status: query.Status,
		Limit:  query.Limit,
		Offset: query.Offset,
	})
}

func (s *RoomService) Agents() []model.AgentConfig {
	return s.agents.Agents()
}

func (s *RoomService) ListRoomKnowledge(ctx context.Context, roomID string) ([]model.KnowledgeDocument, error) {
	if _, ok := s.GetRoom(ctx, roomID); !ok {
		return nil, ErrRoomNotFound
	}
	return s.knowledge.ListDocuments(ctx, model.KnowledgeScopeRoom, roomID)
}

func (s *RoomService) ListAgentKnowledge(ctx context.Context, agentID string) ([]model.KnowledgeDocument, error) {
	if _, ok := s.agentByID(agentID); !ok {
		return nil, ErrAgentNotFound
	}
	return s.knowledge.ListDocuments(ctx, model.KnowledgeScopeAgent, agentID)
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

func (s *RoomService) ListMessagesPage(ctx context.Context, currentRoom *room.Room, limit int, before string) (MessagePage, error) {
	page, err := s.store.ListMessagesPage(ctx, store.ListMessagesQuery{
		RoomID: currentRoom.Info().ID,
		Limit:  limit,
		Before: before,
	})
	if err != nil {
		return MessagePage{}, err
	}
	return MessagePage{
		Messages:   page.Messages,
		HasMore:    page.HasMore,
		NextBefore: page.NextBefore,
	}, nil
}

func (s *RoomService) ListRoomActivity(ctx context.Context, currentRoom *room.Room, limit int) (RoomActivity, error) {
	roomInfo := currentRoom.Info()
	query := store.ListRunsQuery{RoomID: roomInfo.ID, Limit: limit}

	agentRuns, err := s.store.ListAgentRuns(ctx, query)
	if err != nil {
		return RoomActivity{}, err
	}
	dialogueRuns, err := s.store.ListDialogueRuns(ctx, query)
	if err != nil {
		return RoomActivity{}, err
	}

	agentNameByID := make(map[string]string)
	for _, roomAgent := range currentRoom.Agents() {
		agentNameByID[roomAgent.ID] = roomAgent.Name
	}

	activity := RoomActivity{
		AgentRuns:    make([]AgentRunActivity, 0, len(agentRuns)),
		DialogueRuns: make([]DialogueRunActivity, 0, len(dialogueRuns)),
	}
	for _, run := range agentRuns {
		activity.AgentRuns = append(activity.AgentRuns, AgentRunActivity{
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
		activity.DialogueRuns = append(activity.DialogueRuns, DialogueRunActivity{
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

func (s *RoomService) ListMinutes(ctx context.Context, currentRoom *room.Room) ([]model.MeetingMinutes, error) {
	return s.store.ListMinutes(ctx, currentRoom.Info().ID)
}

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
