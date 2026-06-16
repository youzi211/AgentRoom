package mysql

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

func (s *MySQLStore) SeedAgents(ctx context.Context, agents []model.Agent) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&AgentModel{}).Count(&count).Error; err != nil {
			return fmt.Errorf("count agents: %w", err)
		}
		if count > 0 {
			return nil
		}

		now := time.Now().UTC()
		for i, a := range agents {
			m := agentToModel(a, i)
			m.CreatedAt = now
			m.UpdatedAt = now
			if err := tx.Create(&m).Error; err != nil {
				return fmt.Errorf("insert agent %s: %w", a.ID, err)
			}
		}
		return nil
	})
}

func (s *MySQLStore) ListAgents(ctx context.Context) ([]model.Agent, error) {
	var models []AgentModel
	if err := s.db.WithContext(ctx).Order("sort_order, id").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}

	agents := make([]model.Agent, len(models))
	for i, m := range models {
		agents[i] = m.toDomain()
	}
	return agents, nil
}

func (s *MySQLStore) CreateAgent(ctx context.Context, a model.Agent) (model.Agent, error) {
	now := time.Now().UTC()

	var maxOrder int
	if err := s.db.WithContext(ctx).Model(&AgentModel{}).Select("COALESCE(MAX(sort_order), -1)").Scan(&maxOrder).Error; err != nil {
		return model.Agent{}, fmt.Errorf("get max sort order: %w", err)
	}

	m := agentToModel(a, maxOrder+1)
	m.CreatedAt = now
	m.UpdatedAt = now

	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return model.Agent{}, fmt.Errorf("insert agent: %w", err)
	}
	return m.toDomain(), nil
}

func (s *MySQLStore) DeleteAgent(ctx context.Context, agentID string) error {
	result := s.db.WithContext(ctx).
		Where("id = ?", agentID).
		Delete(&AgentModel{})
	if result.Error != nil {
		return fmt.Errorf("delete agent: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: %s", store.ErrAgentNotFound, agentID)
	}
	return nil
}

func (s *MySQLStore) UpdateAgent(ctx context.Context, a model.Agent) (model.Agent, error) {
	now := time.Now().UTC()

	var existing AgentModel
	if err := s.db.WithContext(ctx).Where("id = ?", a.ID).First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return model.Agent{}, fmt.Errorf("%w: %s", store.ErrAgentNotFound, a.ID)
		}
		return model.Agent{}, fmt.Errorf("find agent: %w", err)
	}

	existing.Name = a.Name
	existing.Mention = a.Mention
	existing.Role = a.Role
	existing.Description = a.Description
	existing.SystemPrompt = a.SystemPrompt
	existing.Enabled = a.Enabled
	existing.UpdatedAt = now

	if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
		return model.Agent{}, fmt.Errorf("update agent: %w", err)
	}
	return existing.toDomain(), nil
}
