package mysql

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

func (s *MySQLStore) CreateRoom(ctx context.Context, input store.CreateRoomInput) (model.RoomMeta, []model.Agent, error) {
	now := input.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		policy := input.DialoguePolicy.WithDefaults()
		roomRecord := RoomModel{
			ID:                        input.ID,
			Name:                      input.Name,
			Status:                    model.RoomStatusActive,
			PasscodeHash:              input.PasscodeHash,
			DialogueMode:              policy.Mode,
			MaxAutonomousTurns:        policy.MaxAutonomousTurns,
			MaxTurnsPerAgent:          policy.MaxTurnsPerAgent,
			AllowSelfFollowup:         policy.AllowSelfFollowup,
			AllowAgentToAgentMentions: policy.AllowAgentToAgentMentions,
			ResponseStrategy:          policy.ResponseStrategy,
			CooldownMS:                policy.CooldownMS,
			CreatedAt:                 now,
			UpdatedAt:                 now,
		}
		if err := tx.Create(&roomRecord).Error; err != nil {
			return fmt.Errorf("insert room: %w", err)
		}

		for i, a := range input.Agents {
			ra := roomAgentToModel(input.ID, a, i)
			ra.CreatedAt = now
			if err := tx.Create(&ra).Error; err != nil {
				return fmt.Errorf("insert room_agent %s: %w", a.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		return model.RoomMeta{}, nil, fmt.Errorf("create room: %w", err)
	}

	meta := model.RoomMeta{
		ID:             input.ID,
		Name:           input.Name,
		CreatedAt:      now,
		HasPasscode:    input.PasscodeHash != "",
		PasscodeHash:   input.PasscodeHash,
		DialoguePolicy: input.DialoguePolicy.WithDefaults(),
		Status:         model.RoomStatusActive,
	}
	return meta, input.Agents, nil
}

func (s *MySQLStore) GetRoom(ctx context.Context, roomID string) (model.RoomMeta, error) {
	var m RoomModel
	if err := s.db.WithContext(ctx).Where("id = ?", roomID).First(&m).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return model.RoomMeta{}, fmt.Errorf("%w: %s", store.ErrRoomNotFound, roomID)
		}
		return model.RoomMeta{}, fmt.Errorf("get room: %w", err)
	}
	return m.toDomain(), nil
}

func (s *MySQLStore) LoadRoomSnapshot(ctx context.Context, roomID string, messageLimit int) (store.RoomSnapshot, error) {
	meta, err := s.GetRoom(ctx, roomID)
	if err != nil {
		return store.RoomSnapshot{}, err
	}

	agents, err := s.ListRoomAgents(ctx, roomID)
	if err != nil {
		return store.RoomSnapshot{}, fmt.Errorf("load snapshot agents: %w", err)
	}

	messages, err := s.ListMessages(ctx, store.ListMessagesQuery{
		RoomID: roomID,
		Limit:  messageLimit,
	})
	if err != nil {
		return store.RoomSnapshot{}, fmt.Errorf("load snapshot messages: %w", err)
	}

	participants, err := s.ListActiveParticipants(ctx, roomID)
	if err != nil {
		return store.RoomSnapshot{}, fmt.Errorf("load snapshot participants: %w", err)
	}

	return store.RoomSnapshot{
		Meta:         meta,
		Agents:       agents,
		Messages:     messages,
		Participants: participants,
	}, nil
}

func (s *MySQLStore) ListRoomAgents(ctx context.Context, roomID string) ([]model.Agent, error) {
	var models []RoomAgentModel
	if err := s.db.WithContext(ctx).Where("room_id = ?", roomID).Order("sort_order, agent_id").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list room agents: %w", err)
	}

	agents := make([]model.Agent, len(models))
	for i, m := range models {
		agents[i] = m.toDomain()
	}
	return agents, nil
}

func (s *MySQLStore) ListRooms(ctx context.Context, query store.ListRoomsQuery) ([]model.RoomSummary, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	q := s.db.WithContext(ctx).Model(&RoomModel{})
	switch query.Status {
	case model.RoomStatusActive, model.RoomStatusClosed, model.RoomStatusArchived:
		q = q.Where("status = ?", query.Status)
	}

	var rooms []RoomModel
	if err := q.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rooms).Error; err != nil {
		return nil, fmt.Errorf("list rooms: %w", err)
	}
	if len(rooms) == 0 {
		return []model.RoomSummary{}, nil
	}

	roomIDs := make([]string, len(rooms))
	for i, r := range rooms {
		roomIDs[i] = r.ID
	}

	type messageStat struct {
		RoomID string
		Total  int
		LastAt *time.Time
	}
	type agentStat struct {
		RoomID string
		Total  int
	}

	var stats []messageStat
	if err := s.db.WithContext(ctx).
		Model(&MessageModel{}).
		Select("room_id AS room_id, COUNT(*) AS total, MAX(created_at) AS last_at").
		Where("room_id IN ?", roomIDs).
		Group("room_id").
		Scan(&stats).Error; err != nil {
		return nil, fmt.Errorf("aggregate room messages: %w", err)
	}

	statByRoom := make(map[string]messageStat, len(stats))
	for _, stat := range stats {
		statByRoom[stat.RoomID] = stat
	}

	var agentStats []agentStat
	if err := s.db.WithContext(ctx).
		Model(&RoomAgentModel{}).
		Select("room_id AS room_id, COUNT(*) AS total").
		Where("room_id IN ?", roomIDs).
		Group("room_id").
		Scan(&agentStats).Error; err != nil {
		return nil, fmt.Errorf("aggregate room agents: %w", err)
	}
	agentCountByRoom := make(map[string]int, len(agentStats))
	for _, stat := range agentStats {
		agentCountByRoom[stat.RoomID] = stat.Total
	}

	summaries := make([]model.RoomSummary, len(rooms))
	for i, r := range rooms {
		stat := statByRoom[r.ID]
		summaries[i] = model.RoomSummary{
			ID:                  r.ID,
			Name:                r.Name,
			Status:              r.Status,
			HasPasscode:         r.PasscodeHash != "",
			CreatedAt:           r.CreatedAt,
			DialoguePolicy:      r.toDomain().DialoguePolicy,
			AgentCount:          agentCountByRoom[r.ID],
			OwnerParticipantID:  strPtrDeref(r.OwnerParticipantID),
			ClosedAt:            r.ClosedAt,
			ClosedReason:        r.ClosedReason,
			AutoCloseDeadlineAt: r.AutoCloseDeadlineAt,
			ArchivedAt:          r.ArchivedAt,
			MessageCount:        stat.Total,
			LastMessageAt:       stat.LastAt,
		}
	}
	return summaries, nil
}

func (s *MySQLStore) UpdateRoomLifecycle(ctx context.Context, input store.UpdateRoomLifecycleInput) error {
	ownerParticipantID := any(nil)
	if input.OwnerParticipantID != "" {
		ownerParticipantID = input.OwnerParticipantID
	}

	result := s.db.WithContext(ctx).
		Model(&RoomModel{}).
		Where("id = ?", input.RoomID).
		Updates(map[string]interface{}{
			"status":                 input.Status,
			"owner_participant_id":   ownerParticipantID,
			"closed_at":              input.ClosedAt,
			"closed_reason":          input.ClosedReason,
			"auto_close_deadline_at": input.AutoCloseDeadlineAt,
			"archived_at":            input.ArchivedAt,
			"updated_at":             time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("update room lifecycle: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: %s", store.ErrRoomNotFound, input.RoomID)
	}
	return nil
}
