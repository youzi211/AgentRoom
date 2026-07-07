package mysql

import (
	"context"
	"fmt"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
	"gorm.io/gorm"
)

func (s *MySQLStore) AddMessage(ctx context.Context, message model.Message) (model.Message, error) {
	m := messageToModel(message)
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return model.Message{}, fmt.Errorf("insert message: %w", err)
	}
	return m.toDomain(), nil
}

func (s *MySQLStore) GetMessage(ctx context.Context, roomID string, messageID string) (model.Message, error) {
	var m MessageModel
	if err := s.db.WithContext(ctx).
		Where("room_id = ? AND id = ?", roomID, messageID).
		First(&m).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return model.Message{}, fmt.Errorf("%w: %s", store.ErrMessageNotFound, messageID)
		}
		return model.Message{}, fmt.Errorf("get message: %w", err)
	}
	return m.toDomain(), nil
}

func (s *MySQLStore) ListMessages(ctx context.Context, query store.ListMessagesQuery) ([]model.Message, error) {
	limit := normalizedMessageLimit(query.Limit)

	q := s.db.WithContext(ctx).Where("room_id = ?", query.RoomID)
	if query.Before != "" {
		var cursor MessageModel
		if err := s.db.WithContext(ctx).Select("created_at, id").Where("id = ?", query.Before).First(&cursor).Error; err == nil {
			q = q.Where("(created_at, id) < (?, ?)", cursor.CreatedAt, cursor.ID)
		}
	}

	var models []MessageModel
	if err := q.Order("created_at ASC, id ASC").Limit(limit).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}

	messages := make([]model.Message, len(models))
	for i, m := range models {
		messages[i] = m.toDomain()
	}
	return messages, nil
}

func (s *MySQLStore) ListMessagesPage(ctx context.Context, query store.ListMessagesQuery) (store.MessagePage, error) {
	limit := normalizedMessageLimit(query.Limit)
	base := s.db.WithContext(ctx).Where("room_id = ?", query.RoomID)

	if query.Before != "" {
		var cursor MessageModel
		if err := s.db.WithContext(ctx).
			Select("id, room_id, created_at").
			Where("id = ?", query.Before).
			First(&cursor).Error; err != nil {
			return store.MessagePage{}, store.ErrInvalidMessageCursor
		}
		if cursor.RoomID != query.RoomID {
			return store.MessagePage{}, store.ErrInvalidMessageCursor
		}
		base = base.Where("(created_at, id) < (?, ?)", cursor.CreatedAt, cursor.ID)
	}

	var models []MessageModel
	if err := base.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&models).Error; err != nil {
		return store.MessagePage{}, fmt.Errorf("list messages page: %w", err)
	}

	hasMore := len(models) > limit
	if hasMore {
		models = models[:limit]
	}

	messages := make([]model.Message, len(models))
	for i, m := range models {
		messages[len(models)-1-i] = m.toDomain()
	}

	page := store.MessagePage{
		Messages: messages,
		HasMore:  hasMore,
	}
	if hasMore && len(messages) > 0 {
		page.NextBefore = messages[0].ID
	}
	return page, nil
}

func normalizedMessageLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}
