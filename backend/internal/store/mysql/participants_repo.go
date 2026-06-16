package mysql

import (
	"context"
	"fmt"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

func (s *MySQLStore) AddParticipant(ctx context.Context, input store.AddParticipantInput) (model.Participant, error) {
	now := input.JoinedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	m := participantToModel(input)
	m.JoinedAt = now
	m.LastSeenAt = now

	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return model.Participant{}, fmt.Errorf("insert participant: %w", err)
	}
	return m.toDomain(), nil
}

func (s *MySQLStore) MarkParticipantLeft(ctx context.Context, participantID string, leftAt time.Time) error {
	result := s.db.WithContext(ctx).
		Model(&ParticipantModel{}).
		Where("id = ? AND left_at IS NULL", participantID).
		Update("left_at", leftAt)
	if result.Error != nil {
		return fmt.Errorf("mark participant left: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: %s", store.ErrParticipantNotFound, participantID)
	}
	return nil
}

func (s *MySQLStore) ListActiveParticipants(ctx context.Context, roomID string) ([]model.Participant, error) {
	var models []ParticipantModel
	if err := s.db.WithContext(ctx).
		Where("room_id = ? AND left_at IS NULL", roomID).
		Order("joined_at, id").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list active participants: %w", err)
	}

	participants := make([]model.Participant, len(models))
	for i, m := range models {
		participants[i] = m.toDomain()
	}
	return participants, nil
}

func (s *MySQLStore) MarkAllActiveParticipantsLeft(ctx context.Context, leftAt time.Time) error {
	result := s.db.WithContext(ctx).
		Model(&ParticipantModel{}).
		Where("left_at IS NULL").
		Update("left_at", leftAt)
	if result.Error != nil {
		return fmt.Errorf("mark all active participants left: %w", result.Error)
	}
	return nil
}
