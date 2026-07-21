package mysql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

func (s *MySQLStore) SeedAgents(ctx context.Context, agents []model.Agent) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing []AgentModel
		if err := tx.Model(&AgentModel{}).Find(&existing).Error; err != nil {
			return fmt.Errorf("list existing agents: %w", err)
		}
		existingIDs := make(map[string]struct{}, len(existing))
		existingMentions := make(map[string]struct{}, len(existing))
		for _, agent := range existing {
			existingIDs[agent.ID] = struct{}{}
			if mention := strings.ToLower(strings.TrimSpace(agent.Mention)); mention != "" {
				existingMentions[mention] = struct{}{}
			}
		}

		now := time.Now().UTC()
		sortOrder := len(existing)
		for _, a := range agents {
			if _, ok := existingIDs[a.ID]; ok {
				continue
			}
			mention := strings.ToLower(strings.TrimSpace(a.Mention))
			if mention != "" {
				if _, ok := existingMentions[mention]; ok {
					continue
				}
			}
			m := agentToModel(a, sortOrder)
			m.CreatedAt = now
			m.UpdatedAt = now
			if err := tx.Create(&m).Error; err != nil {
				return fmt.Errorf("insert agent %s: %w", a.ID, err)
			}
			sortOrder++
			existingIDs[a.ID] = struct{}{}
			if mention != "" {
				existingMentions[mention] = struct{}{}
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
	existing.Runtime = model.NormalizeAgentRuntime(a.Runtime)
	existing.Source = model.NormalizeAgentSource(a.Source)
	existing.Description = a.Description
	existing.SystemPrompt = a.SystemPrompt
	existing.Enabled = a.Enabled
	if a.ModelProfileID == "" {
		existing.ModelProfileID = nil
	} else {
		existing.ModelProfileID = strPtr(a.ModelProfileID)
	}
	existing.UpdatedAt = now

	if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
		return model.Agent{}, fmt.Errorf("update agent: %w", err)
	}
	return existing.toDomain(), nil
}
