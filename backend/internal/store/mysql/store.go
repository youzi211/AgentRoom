package mysql

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

// MySQLStore implements store.Store backed by a MySQL database via GORM.
type MySQLStore struct {
	db *gorm.DB
}

// Open creates a new GORM MySQL connection and verifies it is reachable.
func Open(dsn string) (*MySQLStore, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	return &MySQLStore{db: db}, nil
}

func (s *MySQLStore) Ping(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func (s *MySQLStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Migrate runs AutoMigrate for all models and ensures schema is up to date.
func (s *MySQLStore) Migrate(ctx context.Context) error {
	return s.db.WithContext(ctx).Set(
		"gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
	).AutoMigrate(
		&AgentModel{},
		&RoomModel{},
		&RoomAgentModel{},
		&ParticipantModel{},
		&MessageModel{},
		&AgentRunModel{},
		&SchemaMigrationModel{},
	)
}

// ── Agent configuration ──────────────────────────────────────────────

func (s *MySQLStore) SeedAgents(ctx context.Context, agents []model.Agent) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&AgentModel{}).Count(&count).Error; err != nil {
			return fmt.Errorf("count agents: %w", err)
		}
		if count > 0 {
			// Agents already seeded; do not overwrite user modifications
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

func (s *MySQLStore) UpdateAgent(ctx context.Context, a model.Agent) (model.Agent, error) {
	now := time.Now().UTC()
	var existing AgentModel
	if err := s.db.WithContext(ctx).Where("id = ?", a.ID).First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return model.Agent{}, fmt.Errorf("agent %s not found", a.ID)
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

// ── Room lifecycle ───────────────────────────────────────────────────

func (s *MySQLStore) CreateRoom(ctx context.Context, input store.CreateRoomInput) (model.RoomMeta, []model.Agent, error) {
	now := input.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		roomRecord := RoomModel{
			ID:        input.ID,
			Name:      input.Name,
			Status:    "active",
			CreatedAt: now,
			UpdatedAt: now,
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
		ID:        input.ID,
		Name:      input.Name,
		CreatedAt: now,
	}
	return meta, input.Agents, nil
}

func (s *MySQLStore) GetRoom(ctx context.Context, roomID string) (model.RoomMeta, error) {
	var m RoomModel
	if err := s.db.WithContext(ctx).Where("id = ?", roomID).First(&m).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return model.RoomMeta{}, fmt.Errorf("room %s not found", roomID)
		}
		return model.RoomMeta{}, fmt.Errorf("get room: %w", err)
	}
	return m.toDomain(), nil
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

// ── Participants ─────────────────────────────────────────────────────

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
		return fmt.Errorf("participant %s not found or already left", participantID)
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

// ── Messages ─────────────────────────────────────────────────────────

func (s *MySQLStore) AddMessage(ctx context.Context, message model.Message) (model.Message, error) {
	m := messageToModel(message)
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return model.Message{}, fmt.Errorf("insert message: %w", err)
	}
	return m.toDomain(), nil
}

func (s *MySQLStore) ListMessages(ctx context.Context, query store.ListMessagesQuery) ([]model.Message, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	q := s.db.WithContext(ctx).
		Where("room_id = ?", query.RoomID)

	if query.Before != "" {
		var cursor MessageModel
		if err := s.db.WithContext(ctx).Select("created_at, id").Where("id = ?", query.Before).First(&cursor).Error; err == nil {
			q = q.Where("(created_at, id) < (?, ?)", cursor.CreatedAt, cursor.ID)
		}
		// If cursor not found, just skip the filter (return from beginning)
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

// ── Agent runs ───────────────────────────────────────────────────────

func (s *MySQLStore) CreateAgentRun(ctx context.Context, run store.AgentRun) error {
	m := agentRunToModel(run)
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return fmt.Errorf("insert agent run: %w", err)
	}
	return nil
}

func (s *MySQLStore) FinishAgentRun(ctx context.Context, runID string, status string, errText string, completedAt time.Time) error {
	updates := map[string]interface{}{
		"status":       status,
		"error":        nilIfEmpty(errText),
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
		return fmt.Errorf("agent run %s not found", runID)
	}
	return nil
}

// ── Helpers ──────────────────────────────────────────────────────────

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
