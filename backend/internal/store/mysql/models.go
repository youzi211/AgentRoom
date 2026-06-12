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
	ID         string     `gorm:"primaryKey;size:64"`
	Name       string     `gorm:"size:255;not null"`
	Status     string     `gorm:"size:32;not null;default:'active'"`
	CreatedAt  time.Time  `gorm:"not null;index:idx_rooms_created_at"`
	UpdatedAt  time.Time  `gorm:"not null"`
	ArchivedAt *time.Time `gorm:""`
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
	ID         string    `gorm:"primaryKey;size:64;index:idx_messages_room_created"`
	RoomID     string    `gorm:"size:64;not null;index:idx_messages_room_created"`
	SenderID   string    `gorm:"size:64;not null"`
	SenderName string    `gorm:"size:128;not null"`
	SenderType string    `gorm:"size:32;not null"`
	Content    string    `gorm:"type:text;not null"`
	CreatedAt  time.Time `gorm:"not null;index:idx_messages_room_created"`
}

func (MessageModel) TableName() string { return "messages" }

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
		ID:         msg.ID,
		RoomID:     msg.RoomID,
		SenderID:   msg.SenderID,
		SenderName: msg.SenderName,
		SenderType: msg.SenderType,
		Content:    msg.Content,
		CreatedAt:  msg.CreatedAt,
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
		ID:         m.ID,
		RoomID:     m.RoomID,
		SenderID:   m.SenderID,
		SenderName: m.SenderName,
		SenderType: m.SenderType,
		Content:    m.Content,
		CreatedAt:  m.CreatedAt,
	}
}

func (m RoomModel) toDomain() model.RoomMeta {
	return model.RoomMeta{
		ID:        m.ID,
		Name:      m.Name,
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
