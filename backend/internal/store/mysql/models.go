package mysql

import (
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

// ── GORM 模型定义 ─────────────────────────────────────────────────────
// 与 domain model 分离，通过转换方法连接。
// GORM 模型只负责数据库映射，不承载业务逻辑。

// AgentModel maps to the `agents` table (global agent configuration).
type AgentModel struct {
	ID           string    `gorm:"primaryKey;size:64"`
	Name         string    `gorm:"size:128;not null"`
	Mention      string    `gorm:"size:128;uniqueIndex:uk_agents_mention;not null"`
	Role         string    `gorm:"size:128;not null"`
	Description  string    `gorm:"type:text;not null"`
	SystemPrompt string    `gorm:"column:system_prompt;type:text;not null"`
	Enabled      bool      `gorm:"not null;default:true"`
	SortOrder    int       `gorm:"column:sort_order;not null;default:0"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (AgentModel) TableName() string { return "agents" }

// RoomModel maps to the `rooms` table.
type RoomModel struct {
	ID                        string     `gorm:"primaryKey;size:64"`
	Name                      string     `gorm:"size:255;not null"`
	Status                    string     `gorm:"size:32;not null;default:'active'"`
	OwnerParticipantID        *string    `gorm:"column:owner_participant_id;size:64"`
	PasscodeHash              string     `gorm:"column:passcode_hash;size:128;not null;default:''"`
	DialogueMode              string     `gorm:"column:dialogue_mode;size:32;not null;default:'mention_fanout'"`
	MaxAutonomousTurns        int        `gorm:"column:max_autonomous_turns;not null;default:3"`
	MaxTurnsPerAgent          int        `gorm:"column:max_turns_per_agent;not null;default:1"`
	AllowSelfFollowup         bool       `gorm:"column:allow_self_followup;not null;default:false"`
	AllowAgentToAgentMentions bool       `gorm:"column:allow_agent_to_agent_mentions;not null;default:true"`
	ResponseStrategy          string     `gorm:"column:response_strategy;size:32;not null;default:'mentioned_first'"`
	CooldownMS                int        `gorm:"column:cooldown_ms;not null;default:0"`
	ClosedAt                  *time.Time `gorm:"column:closed_at"`
	ClosedReason              string     `gorm:"column:closed_reason;size:32;not null;default:''"`
	AutoCloseDeadlineAt       *time.Time `gorm:"column:auto_close_deadline_at"`
	CreatedAt                 time.Time  `gorm:"not null;index:idx_rooms_created_at"`
	UpdatedAt                 time.Time  `gorm:"not null"`
	ArchivedAt                *time.Time `gorm:""`
}

func (RoomModel) TableName() string { return "rooms" }

// RoomAgentModel maps to the `room_agents` table (per-room agent snapshot).
type RoomAgentModel struct {
	RoomID       string    `gorm:"primaryKey;size:64;uniqueIndex:idx_room_agents_mention"`
	AgentID      string    `gorm:"primaryKey;size:64"`
	Name         string    `gorm:"size:128;not null"`
	Mention      string    `gorm:"size:128;not null;uniqueIndex:idx_room_agents_mention"`
	Role         string    `gorm:"size:128;not null"`
	Description  string    `gorm:"type:text;not null"`
	SystemPrompt string    `gorm:"column:system_prompt;type:text;not null"`
	Enabled      bool      `gorm:"not null;default:true"`
	SortOrder    int       `gorm:"column:sort_order;not null;default:0"`
	CreatedAt    time.Time `gorm:"not null"`
}

func (RoomAgentModel) TableName() string { return "room_agents" }

// ParticipantModel maps to the `participants` table.
type ParticipantModel struct {
	ID          string     `gorm:"primaryKey;size:64"`
	RoomID      string     `gorm:"size:64;not null;index:idx_participants_room_active"`
	DisplayName string     `gorm:"size:128;not null"`
	GuestKey    *string    `gorm:"size:128;index:idx_participants_guest"`
	JoinedAt    time.Time  `gorm:"not null"`
	LastSeenAt  time.Time  `gorm:"not null"`
	LeftAt      *time.Time `gorm:"index:idx_participants_room_active,sort:desc"`
}

func (ParticipantModel) TableName() string { return "participants" }

// MessageModel maps to the `messages` table.
type MessageModel struct {
	ID              string    `gorm:"primaryKey;size:64;index:idx_messages_room_created"`
	RoomID          string    `gorm:"size:64;not null;index:idx_messages_room_created"`
	SenderID        string    `gorm:"size:64;not null"`
	SenderName      string    `gorm:"size:128;not null"`
	SenderType      string    `gorm:"size:32;not null"`
	Content         string    `gorm:"type:text;not null"`
	DialogueRunID   string    `gorm:"column:dialogue_run_id;size:64;index:idx_messages_dialogue_run"`
	TurnIndex       int       `gorm:"column:turn_index;not null;default:0"`
	ParentMessageID string    `gorm:"column:parent_message_id;size:64"`
	CreatedAt       time.Time `gorm:"not null;index:idx_messages_room_created"`
}

func (MessageModel) TableName() string { return "messages" }

type DialogueRunModel struct {
	ID               string     `gorm:"primaryKey;size:64"`
	RoomID           string     `gorm:"size:64;not null;index:idx_dialogue_runs_room"`
	TriggerMessageID string     `gorm:"size:64;not null;index:idx_dialogue_runs_trigger"`
	Mode             string     `gorm:"size:32;not null"`
	TurnCount        int        `gorm:"column:turn_count;not null;default:0"`
	Status           string     `gorm:"size:32;not null"`
	StartedAt        time.Time  `gorm:"not null"`
	CompletedAt      *time.Time `gorm:""`
}

func (DialogueRunModel) TableName() string { return "dialogue_runs" }

// AgentRunModel maps to the `agent_runs` table.
type AgentRunModel struct {
	ID               string     `gorm:"primaryKey;size:64"`
	RoomID           string     `gorm:"size:64;not null;index:idx_agent_runs_room"`
	AgentID          string     `gorm:"size:64;not null"`
	TriggerMessageID string     `gorm:"size:64;not null;index:idx_agent_runs_trigger"`
	Status           string     `gorm:"size:32;not null"`
	Error            *string    `gorm:"type:text"`
	StartedAt        time.Time  `gorm:"not null"`
	CompletedAt      *time.Time `gorm:""`
}

func (AgentRunModel) TableName() string { return "agent_runs" }

// KnowledgeDocumentModel maps to metadata for room-level or agent-level knowledge documents.
type KnowledgeDocumentModel struct {
	ID          string    `gorm:"primaryKey;size:64"`
	Scope       string    `gorm:"size:32;not null;index:idx_knowledge_documents_scope"`
	ScopeID     string    `gorm:"column:scope_id;size:64;not null;index:idx_knowledge_documents_scope"`
	FileName    string    `gorm:"column:file_name;size:255;not null"`
	ContentType string    `gorm:"column:content_type;size:128;not null"`
	SizeBytes   int64     `gorm:"column:size_bytes;not null"`
	Status      string    `gorm:"size:32;not null"`
	CreatedAt   time.Time `gorm:"not null;index:idx_knowledge_documents_created"`
}

func (KnowledgeDocumentModel) TableName() string { return "knowledge_documents" }

// KnowledgeChunkModel maps to parsed document chunks used for prompt grounding.
type KnowledgeChunkModel struct {
	ID         string    `gorm:"primaryKey;size:64"`
	DocumentID string    `gorm:"column:document_id;size:64;not null;index:idx_knowledge_chunks_document"`
	Scope      string    `gorm:"size:32;not null;index:idx_knowledge_chunks_scope"`
	ScopeID    string    `gorm:"column:scope_id;size:64;not null;index:idx_knowledge_chunks_scope"`
	ChunkIndex int       `gorm:"column:chunk_index;not null"`
	Content    string    `gorm:"type:mediumtext;not null"`
	CreatedAt  time.Time `gorm:"not null"`
}

func (KnowledgeChunkModel) TableName() string { return "knowledge_chunks" }

// MeetingMinutesModel maps to the `meeting_minutes` table (versioned minutes per room).
type MeetingMinutesModel struct {
	ID        string    `gorm:"primaryKey;size:64"`
	RoomID    string    `gorm:"column:room_id;size:64;not null;index:idx_minutes_room,priority:1"`
	Version   int       `gorm:"not null;index:idx_minutes_room,priority:2"`
	Content   string    `gorm:"type:mediumtext;not null"`
	Source    string    `gorm:"size:16;not null"`
	CreatedBy string    `gorm:"column:created_by;size:128;not null;default:''"`
	CreatedAt time.Time `gorm:"not null"`
}

func (MeetingMinutesModel) TableName() string { return "meeting_minutes" }

// SchemaMigrationModel maps to the `schema_migrations` table.
type SchemaMigrationModel struct {
	Version   string    `gorm:"primaryKey;size:64"`
	AppliedAt time.Time `gorm:"not null"`
}

func (SchemaMigrationModel) TableName() string { return "schema_migrations" }

// ── Domain → GORM 转换 ───────────────────────────────────────────────

func agentToModel(a model.Agent, sortOrder int) AgentModel {
	return AgentModel{
		ID:           a.ID,
		Name:         a.Name,
		Mention:      a.Mention,
		Role:         a.Role,
		Description:  a.Description,
		SystemPrompt: a.SystemPrompt,
		Enabled:      a.Enabled,
		SortOrder:    sortOrder,
	}
}

func roomAgentToModel(roomID string, a model.Agent, sortOrder int) RoomAgentModel {
	return RoomAgentModel{
		RoomID:       roomID,
		AgentID:      a.ID,
		Name:         a.Name,
		Mention:      a.Mention,
		Role:         a.Role,
		Description:  a.Description,
		SystemPrompt: a.SystemPrompt,
		Enabled:      a.Enabled,
		SortOrder:    sortOrder,
	}
}

func participantToModel(input store.AddParticipantInput) ParticipantModel {
	m := ParticipantModel{
		ID:          input.ID,
		RoomID:      input.RoomID,
		DisplayName: input.DisplayName,
		JoinedAt:    input.JoinedAt,
		LastSeenAt:  input.JoinedAt,
	}
	if input.GuestKey != "" {
		m.GuestKey = strPtr(input.GuestKey)
	}
	return m
}

func messageToModel(msg model.Message) MessageModel {
	return MessageModel{
		ID:              msg.ID,
		RoomID:          msg.RoomID,
		SenderID:        msg.SenderID,
		SenderName:      msg.SenderName,
		SenderType:      msg.SenderType,
		Content:         msg.Content,
		DialogueRunID:   msg.DialogueRunID,
		TurnIndex:       msg.TurnIndex,
		ParentMessageID: msg.ParentMessageID,
		CreatedAt:       msg.CreatedAt,
	}
}

func agentRunToModel(run store.AgentRun) AgentRunModel {
	m := AgentRunModel{
		ID:               run.ID,
		RoomID:           run.RoomID,
		AgentID:          run.AgentID,
		TriggerMessageID: run.TriggerMessageID,
		Status:           run.Status,
		StartedAt:        run.StartedAt,
	}
	if run.Error != "" {
		m.Error = strPtr(run.Error)
	}
	if run.CompletedAt != nil {
		m.CompletedAt = run.CompletedAt
	}
	return m
}

func dialogueRunToModel(run store.DialogueRun) DialogueRunModel {
	m := DialogueRunModel{
		ID:               run.ID,
		RoomID:           run.RoomID,
		TriggerMessageID: run.TriggerMessageID,
		Mode:             run.Mode,
		TurnCount:        run.TurnCount,
		Status:           run.Status,
		StartedAt:        run.StartedAt,
	}
	if run.CompletedAt != nil {
		m.CompletedAt = run.CompletedAt
	}
	return m
}

func knowledgeDocumentToModel(document model.KnowledgeDocument) KnowledgeDocumentModel {
	return KnowledgeDocumentModel{
		ID:          document.ID,
		Scope:       document.Scope,
		ScopeID:     document.ScopeID,
		FileName:    document.FileName,
		ContentType: document.ContentType,
		SizeBytes:   document.SizeBytes,
		Status:      document.Status,
		CreatedAt:   document.CreatedAt,
	}
}

func knowledgeChunkToModel(chunk model.KnowledgeChunk) KnowledgeChunkModel {
	return KnowledgeChunkModel{
		ID:         chunk.ID,
		DocumentID: chunk.DocumentID,
		Scope:      chunk.Scope,
		ScopeID:    chunk.ScopeID,
		ChunkIndex: chunk.ChunkIndex,
		Content:    chunk.Content,
		CreatedAt:  chunk.CreatedAt,
	}
}

// ── GORM → Domain 转换 ───────────────────────────────────────────────

func (m AgentModel) toDomain() model.Agent {
	return model.Agent{
		ID:           m.ID,
		Name:         m.Name,
		Mention:      m.Mention,
		Role:         m.Role,
		Description:  m.Description,
		SystemPrompt: m.SystemPrompt,
		Enabled:      m.Enabled,
	}
}

func (m RoomAgentModel) toDomain() model.Agent {
	return model.Agent{
		ID:           m.AgentID,
		Name:         m.Name,
		Mention:      m.Mention,
		Role:         m.Role,
		Description:  m.Description,
		SystemPrompt: m.SystemPrompt,
		Enabled:      m.Enabled,
	}
}

func (m ParticipantModel) toDomain() model.Participant {
	return model.Participant{
		ID:       m.ID,
		Name:     m.DisplayName,
		JoinedAt: m.JoinedAt,
	}
}

func (m MessageModel) toDomain() model.Message {
	return model.Message{
		ID:              m.ID,
		RoomID:          m.RoomID,
		SenderID:        m.SenderID,
		SenderName:      m.SenderName,
		SenderType:      m.SenderType,
		Content:         m.Content,
		DialogueRunID:   m.DialogueRunID,
		TurnIndex:       m.TurnIndex,
		ParentMessageID: m.ParentMessageID,
		CreatedAt:       m.CreatedAt,
	}
}

func (m AgentRunModel) toStore() store.AgentRun {
	return store.AgentRun{
		ID:               m.ID,
		RoomID:           m.RoomID,
		AgentID:          m.AgentID,
		TriggerMessageID: m.TriggerMessageID,
		Status:           m.Status,
		Error:            strPtrDeref(m.Error),
		StartedAt:        m.StartedAt,
		CompletedAt:      m.CompletedAt,
	}
}

func (m DialogueRunModel) toStore() store.DialogueRun {
	return store.DialogueRun{
		ID:               m.ID,
		RoomID:           m.RoomID,
		TriggerMessageID: m.TriggerMessageID,
		Mode:             m.Mode,
		TurnCount:        m.TurnCount,
		Status:           m.Status,
		StartedAt:        m.StartedAt,
		CompletedAt:      m.CompletedAt,
	}
}

func (m RoomModel) toDomain() model.RoomMeta {
	return model.RoomMeta{
		ID:                  m.ID,
		Name:                m.Name,
		CreatedAt:           m.CreatedAt,
		HasPasscode:         m.PasscodeHash != "",
		PasscodeHash:        m.PasscodeHash,
		Status:              m.Status,
		OwnerParticipantID:  strPtrDeref(m.OwnerParticipantID),
		ClosedAt:            m.ClosedAt,
		ClosedReason:        m.ClosedReason,
		AutoCloseDeadlineAt: m.AutoCloseDeadlineAt,
		ArchivedAt:          m.ArchivedAt,
		DialoguePolicy: model.DialoguePolicy{
			Mode:                      m.DialogueMode,
			MaxAutonomousTurns:        m.MaxAutonomousTurns,
			MaxTurnsPerAgent:          m.MaxTurnsPerAgent,
			AllowSelfFollowup:         m.AllowSelfFollowup,
			AllowAgentToAgentMentions: m.AllowAgentToAgentMentions,
			ResponseStrategy:          m.ResponseStrategy,
			CooldownMS:                m.CooldownMS,
		}.WithDefaults(),
	}
}

func (m KnowledgeDocumentModel) toDomain() model.KnowledgeDocument {
	return model.KnowledgeDocument{
		ID:          m.ID,
		Scope:       m.Scope,
		ScopeID:     m.ScopeID,
		FileName:    m.FileName,
		ContentType: m.ContentType,
		SizeBytes:   m.SizeBytes,
		Status:      m.Status,
		CreatedAt:   m.CreatedAt,
	}
}

func (m KnowledgeChunkModel) toDomain() model.KnowledgeChunk {
	return model.KnowledgeChunk{
		ID:         m.ID,
		DocumentID: m.DocumentID,
		Scope:      m.Scope,
		ScopeID:    m.ScopeID,
		ChunkIndex: m.ChunkIndex,
		Content:    m.Content,
		CreatedAt:  m.CreatedAt,
	}
}

func meetingMinutesToModel(m model.MeetingMinutes) MeetingMinutesModel {
	return MeetingMinutesModel{
		ID:        m.ID,
		RoomID:    m.RoomID,
		Version:   m.Version,
		Content:   m.Content,
		Source:    m.Source,
		CreatedBy: m.CreatedBy,
		CreatedAt: m.CreatedAt,
	}
}

func (m MeetingMinutesModel) toDomain() model.MeetingMinutes {
	return model.MeetingMinutes{
		ID:        m.ID,
		RoomID:    m.RoomID,
		Version:   m.Version,
		Content:   m.Content,
		Source:    m.Source,
		CreatedBy: m.CreatedBy,
		CreatedAt: m.CreatedAt,
	}
}

// ── Helpers ──────────────────────────────────────────────────────────

func strPtr(s string) *string {
	return &s
}

func strPtrDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
