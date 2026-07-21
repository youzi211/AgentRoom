package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *MySQLStore) CreateAgentRun(ctx context.Context, run store.AgentRun) error {
	m := agentRunToModel(run)
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return fmt.Errorf("insert agent run: %w", err)
	}
	return nil
}

func (s *MySQLStore) CommitAgentRunSuccess(ctx context.Context, input store.CommitAgentRunSuccessInput) (model.Message, error) {
	var saved model.Message
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run AgentRunModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", input.RunID).First(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: %s", store.ErrAgentRunNotFound, input.RunID)
			}
			return fmt.Errorf("lock agent run: %w", err)
		}

		if run.Status == "succeeded" {
			var existing MessageModel
			if err := tx.Where("agent_run_id = ?", input.RunID).First(&existing).Error; err != nil {
				return fmt.Errorf("load committed agent run message: %w", err)
			}
			saved = existing.toDomain()
			return nil
		}
		if run.Status != "running" {
			return fmt.Errorf("%w: %s is %s", store.ErrAgentRunAlreadyFinished, input.RunID, run.Status)
		}

		message := input.Message
		message.AgentRunID = input.RunID
		messageModel := messageToModel(message)
		if err := tx.Create(&messageModel).Error; err != nil {
			return fmt.Errorf("insert agent run message: %w", err)
		}

		var profile any
		if input.ModelProfileID != "" {
			profile = input.ModelProfileID
		}
		result := tx.Model(&AgentRunModel{}).
			Where("id = ? AND status = ?", input.RunID, "running").
			Updates(map[string]any{
				"status": "succeeded", "error": nil, "completed_at": input.CompletedAt,
				"model_profile_id": profile, "model_source": input.ModelSource, "model_name": input.ModelName,
			})
		if result.Error != nil {
			return fmt.Errorf("finish successful agent run: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("%w: %s", store.ErrAgentRunAlreadyFinished, input.RunID)
		}
		saved = messageModel.toDomain()
		return nil
	})
	if err != nil {
		return model.Message{}, err
	}
	return saved, nil
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
		Where("id = ? AND status = ?", runID, "running").
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("finish agent run: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := s.db.WithContext(ctx).Model(&AgentRunModel{}).Where("id = ?", runID).Count(&count).Error; err != nil {
			return fmt.Errorf("check agent run terminal state: %w", err)
		}
		if count == 0 {
			return fmt.Errorf("%w: %s", store.ErrAgentRunNotFound, runID)
		}
		return fmt.Errorf("%w: %s", store.ErrAgentRunAlreadyFinished, runID)
	}
	return nil
}

func (s *MySQLStore) ReconcileActiveAgentRuns(ctx context.Context, completedAt time.Time) (int64, error) {
	result := s.db.WithContext(ctx).Model(&AgentRunModel{}).
		Where("status = ?", "running").
		Updates(map[string]any{
			"status": "interrupted", "error": "backend restarted during Agent Runtime execution", "completed_at": completedAt,
		})
	if result.Error != nil {
		return 0, fmt.Errorf("reconcile active agent runs: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (s *MySQLStore) UpdateAgentRunModel(ctx context.Context, runID, profileID, source, modelName string) error {
	var profile any = nil
	if profileID != "" {
		profile = profileID
	}
	return s.db.WithContext(ctx).Model(&AgentRunModel{}).Where("id = ?", runID).Updates(map[string]any{"model_profile_id": profile, "model_source": source, "model_name": modelName}).Error
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
