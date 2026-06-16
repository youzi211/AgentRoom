package mysql

import (
	"context"
	"fmt"
	"time"

	"agentroom/backend/internal/store"
)

func (s *MySQLStore) CreateAgentRun(ctx context.Context, run store.AgentRun) error {
	m := agentRunToModel(run)
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return fmt.Errorf("insert agent run: %w", err)
	}
	return nil
}

func (s *MySQLStore) FinishAgentRun(ctx context.Context, runID string, status string, errText string, completedAt time.Time) error {
	errorValue := any(nil)
	if errText != "" {
		errorValue = errText
	}

	updates := map[string]interface{}{
		"status":       status,
		"error":        errorValue,
		"completed_at": completedAt,
	}
	result := s.db.WithContext(ctx).
		Model(&AgentRunModel{}).
		Where("id = ?", runID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("finish agent run: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: %s", store.ErrAgentRunNotFound, runID)
	}
	return nil
}

func (s *MySQLStore) ListAgentRuns(ctx context.Context, query store.ListRunsQuery) ([]store.AgentRun, error) {
	limit := normalizedRunLimit(query.Limit)
	var models []AgentRunModel
	if err := s.db.WithContext(ctx).
		Where("room_id = ?", query.RoomID).
		Order("started_at DESC, id DESC").
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list agent runs: %w", err)
	}

	runs := make([]store.AgentRun, len(models))
	for i, m := range models {
		runs[i] = m.toStore()
	}
	return runs, nil
}

func (s *MySQLStore) CreateDialogueRun(ctx context.Context, run store.DialogueRun) error {
	m := dialogueRunToModel(run)
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return fmt.Errorf("insert dialogue run: %w", err)
	}
	return nil
}

func (s *MySQLStore) FinishDialogueRun(ctx context.Context, runID string, status string, turnCount int, completedAt time.Time) error {
	updates := map[string]interface{}{
		"status":       status,
		"turn_count":   turnCount,
		"completed_at": completedAt,
	}
	result := s.db.WithContext(ctx).
		Model(&DialogueRunModel{}).
		Where("id = ?", runID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("finish dialogue run: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: %s", store.ErrDialogueRunNotFound, runID)
	}
	return nil
}

func (s *MySQLStore) ListDialogueRuns(ctx context.Context, query store.ListRunsQuery) ([]store.DialogueRun, error) {
	limit := normalizedRunLimit(query.Limit)
	var models []DialogueRunModel
	if err := s.db.WithContext(ctx).
		Where("room_id = ?", query.RoomID).
		Order("started_at DESC, id DESC").
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list dialogue runs: %w", err)
	}

	runs := make([]store.DialogueRun, len(models))
	for i, m := range models {
		runs[i] = m.toStore()
	}
	return runs, nil
}

func normalizedRunLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}
