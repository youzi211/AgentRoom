package mysql

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"agentroom/backend/internal/model"
)

func (s *MySQLStore) CreateMinutes(ctx context.Context, minutes model.MeetingMinutes) (model.MeetingMinutes, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var maxVersion int
		if err := tx.Model(&MeetingMinutesModel{}).
			Where("room_id = ?", minutes.RoomID).
			Select("COALESCE(MAX(version), 0)").
			Scan(&maxVersion).Error; err != nil {
			return fmt.Errorf("read max minutes version: %w", err)
		}
		minutes.Version = maxVersion + 1
		m := meetingMinutesToModel(minutes)
		if err := tx.Create(&m).Error; err != nil {
			return fmt.Errorf("insert minutes: %w", err)
		}
		return nil
	})
	if err != nil {
		return model.MeetingMinutes{}, err
	}
	return minutes, nil
}

func (s *MySQLStore) ListMinutes(ctx context.Context, roomID string) ([]model.MeetingMinutes, error) {
	var models []MeetingMinutesModel
	if err := s.db.WithContext(ctx).
		Where("room_id = ?", roomID).
		Order("version DESC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list minutes: %w", err)
	}

	minutes := make([]model.MeetingMinutes, len(models))
	for i, m := range models {
		minutes[i] = m.toDomain()
	}
	return minutes, nil
}

func (s *MySQLStore) LatestMinutes(ctx context.Context, roomID string) (model.MeetingMinutes, bool, error) {
	var m MeetingMinutesModel
	if err := s.db.WithContext(ctx).
		Where("room_id = ?", roomID).
		Order("version DESC").
		First(&m).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return model.MeetingMinutes{}, false, nil
		}
		return model.MeetingMinutes{}, false, fmt.Errorf("latest minutes: %w", err)
	}
	return m.toDomain(), true, nil
}
