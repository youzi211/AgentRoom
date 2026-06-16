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
		&DialogueRunModel{},
		&KnowledgeDocumentModel{},
		&KnowledgeChunkModel{},
		&MeetingMinutesModel{},
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

func (s *MySQLStore) CreateAgent(ctx context.Context, a model.Agent) (model.Agent, error) {
	now := time.Now().UTC()

	// Determine the next sort order
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
		return fmt.Errorf("agent %s not found", agentID)
	}
	return nil
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
		policy := input.DialoguePolicy.WithDefaults()
		roomRecord := RoomModel{
			ID:                        input.ID,
			Name:                      input.Name,
			Status:                    "active",
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

	summaries := make([]model.RoomSummary, len(rooms))
	for i, r := range rooms {
		stat := statByRoom[r.ID]
		summaries[i] = model.RoomSummary{
			ID:                  r.ID,
			Name:                r.Name,
			Status:              r.Status,
			HasPasscode:         r.PasscodeHash != "",
			CreatedAt:           r.CreatedAt,
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
	result := s.db.WithContext(ctx).
		Model(&RoomModel{}).
		Where("id = ?", input.RoomID).
		Updates(map[string]interface{}{
			"status":                 input.Status,
			"owner_participant_id":   nilIfEmpty(input.OwnerParticipantID),
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
		return fmt.Errorf("room %s not found", input.RoomID)
	}
	return nil
}

// ── Meeting minutes ──────────────────────────────────────────────────

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
	limit := normalizedMessageLimit(query.Limit)

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
		return fmt.Errorf("dialogue run %s not found", runID)
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

// Knowledge documents

func (s *MySQLStore) CreateKnowledgeDocument(ctx context.Context, document model.KnowledgeDocument, chunks []model.KnowledgeChunk) (model.KnowledgeDocument, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(knowledgeDocumentToModel(document)).Error; err != nil {
			return fmt.Errorf("insert knowledge document: %w", err)
		}
		for _, chunk := range chunks {
			if err := tx.Create(knowledgeChunkToModel(chunk)).Error; err != nil {
				return fmt.Errorf("insert knowledge chunk: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return model.KnowledgeDocument{}, err
	}
	return document, nil
}

func (s *MySQLStore) ListKnowledgeDocuments(ctx context.Context, query store.ListKnowledgeDocumentsQuery) ([]model.KnowledgeDocument, error) {
	var models []KnowledgeDocumentModel
	if err := s.db.WithContext(ctx).
		Where("scope = ? AND scope_id = ?", query.Scope, query.ScopeID).
		Order("created_at DESC, id DESC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list knowledge documents: %w", err)
	}

	documents := make([]model.KnowledgeDocument, len(models))
	for i, m := range models {
		documents[i] = m.toDomain()
	}
	return documents, nil
}

func (s *MySQLStore) DeleteKnowledgeDocument(ctx context.Context, documentID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("document_id = ?", documentID).Delete(&KnowledgeChunkModel{}).Error; err != nil {
			return fmt.Errorf("delete knowledge chunks: %w", err)
		}
		result := tx.Where("id = ?", documentID).Delete(&KnowledgeDocumentModel{})
		if result.Error != nil {
			return fmt.Errorf("delete knowledge document: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("knowledge document %s not found", documentID)
		}
		return nil
	})
}

func (s *MySQLStore) SearchKnowledgeChunks(ctx context.Context, query store.SearchKnowledgeChunksQuery) ([]model.KnowledgeChunk, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 6
	}
	if limit > 20 {
		limit = 20
	}

	var models []KnowledgeChunkModel
	if err := s.db.WithContext(ctx).
		Where("scope = ? AND scope_id = ?", query.Scope, query.ScopeID).
		Order("created_at DESC, chunk_index ASC").
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("search knowledge chunks: %w", err)
	}

	chunks := make([]model.KnowledgeChunk, len(models))
	for i, m := range models {
		chunks[i] = m.toDomain()
	}
	return chunks, nil
}

// ── Helpers ──────────────────────────────────────────────────────────

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
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

func normalizedMessageLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}
