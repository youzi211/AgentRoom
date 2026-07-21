package mysql

import (
	"context"
	"errors"
	"fmt"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *MySQLStore) ListModelProfiles(ctx context.Context) ([]model.ModelProfile, error) {
	var rows []ModelProfileModel
	if err := s.db.WithContext(ctx).Order("runtime_scope, name, id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list model profiles: %w", err)
	}
	out := make([]model.ModelProfile, len(rows))
	for i := range rows {
		out[i] = rows[i].toDomain()
	}
	return out, nil
}
func (s *MySQLStore) GetModelProfile(ctx context.Context, id string) (model.ModelProfile, error) {
	var row ModelProfileModel
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ModelProfile{}, store.ErrModelProfileNotFound
		}
		return model.ModelProfile{}, fmt.Errorf("get model profile: %w", err)
	}
	return row.toDomain(), nil
}
func (s *MySQLStore) GetDefaultModelProfile(ctx context.Context, scope string) (model.ModelProfile, error) {
	var row ModelProfileModel
	if err := s.db.WithContext(ctx).Where("runtime_scope = ? AND is_default = ?", scope, true).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ModelProfile{}, store.ErrModelProfileNotFound
		}
		return model.ModelProfile{}, fmt.Errorf("get default model profile: %w", err)
	}
	return row.toDomain(), nil
}
func (s *MySQLStore) CreateModelProfile(ctx context.Context, p model.ModelProfile) (model.ModelProfile, error) {
	row := modelProfileToModel(p)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if p.IsDefault {
			if err := tx.Model(&ModelProfileModel{}).
				Where("runtime_scope = ? AND is_default = ?", p.RuntimeScope, true).
				Updates(map[string]any{"is_default": false, "default_slot": nil}).Error; err != nil {
				return err
			}
		}
		return tx.Create(&row).Error
	})
	if err != nil {
		return model.ModelProfile{}, fmt.Errorf("create model profile: %w", err)
	}
	return row.toDomain(), nil
}
func (s *MySQLStore) UpdateModelProfile(ctx context.Context, p model.ModelProfile) (model.ModelProfile, error) {
	row := modelProfileToModel(p)
	result := s.db.WithContext(ctx).Model(&ModelProfileModel{}).Where("id = ?", p.ID).Select("name", "base_url", "model_name", "api_key_ciphertext", "api_key_hint", "enabled", "updated_at").Updates(&row)
	if result.Error != nil {
		return model.ModelProfile{}, fmt.Errorf("update model profile: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.ModelProfile{}, store.ErrModelProfileNotFound
	}
	return s.GetModelProfile(ctx, p.ID)
}
func (s *MySQLStore) SetDefaultModelProfile(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row ModelProfileModel
		if err := tx.Where("id = ?", id).First(&row).Error; err != nil {
			return store.ErrModelProfileNotFound
		}
		if !row.Enabled {
			return fmt.Errorf("default model profile must be enabled")
		}
		if err := tx.Model(&ModelProfileModel{}).Where("runtime_scope = ? AND is_default = ?", row.RuntimeScope, true).Updates(map[string]any{"is_default": false, "default_slot": nil}).Error; err != nil {
			return err
		}
		return tx.Model(&ModelProfileModel{}).Where("id = ?", id).Updates(map[string]any{"is_default": true, "default_slot": row.RuntimeScope}).Error
	})
}
func (s *MySQLStore) CountModelProfileReferences(ctx context.Context, id string) (int64, error) {
	var agents, rooms int64
	if err := s.db.WithContext(ctx).Model(&AgentModel{}).Where("model_profile_id = ?", id).Count(&agents).Error; err != nil {
		return 0, err
	}
	if err := s.db.WithContext(ctx).Model(&RoomAgentModel{}).Where("model_profile_id = ?", id).Count(&rooms).Error; err != nil {
		return 0, err
	}
	return agents + rooms, nil
}
func (s *MySQLStore) DeleteModelProfile(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var profile ModelProfileModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&profile).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return store.ErrModelProfileNotFound
			}
			return err
		}
		if profile.IsDefault {
			return store.ErrModelProfileReferenced
		}
		var agents, rooms int64
		if err := tx.Model(&AgentModel{}).Where("model_profile_id = ?", id).Count(&agents).Error; err != nil {
			return err
		}
		if err := tx.Model(&RoomAgentModel{}).Where("model_profile_id = ?", id).Count(&rooms).Error; err != nil {
			return err
		}
		if agents+rooms > 0 {
			return store.ErrModelProfileReferenced
		}
		return tx.Where("id = ?", id).Delete(&ModelProfileModel{}).Error
	})
}
