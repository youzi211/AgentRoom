package contracts

import (
	"fmt"
	"time"

	"agentroom/backend/internal/model"
)

type KnowledgeDocumentsResponse struct {
	Documents []model.KnowledgeDocument `json:"documents"`
}

type UploadKnowledgeResponse struct {
	Document model.KnowledgeDocument `json:"document"`
}

type CreateRoomRequest struct {
	Name                string                    `json:"name"`
	AgentIDs            []string                  `json:"agentIds"`
	Passcode            string                    `json:"passcode"`
	DialoguePolicy      *DialoguePolicyInput      `json:"dialoguePolicy,omitempty"`
	CollaborationPolicy *CollaborationPolicyInput `json:"collaborationPolicy,omitempty"`
}

type CreateRoomResponse struct {
	Room model.RoomMeta `json:"room"`
}

type GetRoomResponse struct {
	Room         model.RoomMeta      `json:"room"`
	Participants []model.Participant `json:"participants"`
	Agents       []model.Agent       `json:"agents"`
}

type GetMessagesResponse struct {
	Messages   []model.Message `json:"messages"`
	HasMore    bool            `json:"hasMore"`
	NextBefore string          `json:"nextBefore,omitempty"`
}

type RoomActivityResponse struct {
	AgentRuns         []AgentRunActivity         `json:"agentRuns"`
	DialogueRuns      []DialogueRunActivity      `json:"dialogueRuns"`
	CollaborationRuns []CollaborationRunActivity `json:"collaborationRuns"`
}

type AgentRunActivity struct {
	ID               string     `json:"id"`
	RoomID           string     `json:"roomID"`
	AgentID          string     `json:"agentID"`
	AgentName        string     `json:"agentName"`
	TriggerMessageID string     `json:"triggerMessageID"`
	Status           string     `json:"status"`
	ErrorText        string     `json:"errorText,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
}

type DialogueRunActivity struct {
	ID               string     `json:"id"`
	RoomID           string     `json:"roomID"`
	TriggerMessageID string     `json:"triggerMessageID"`
	Mode             string     `json:"mode"`
	TurnCount        int        `json:"turnCount"`
	Status           string     `json:"status"`
	CreatedAt        time.Time  `json:"createdAt"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
}

type CollaborationRunActivity struct {
	ID            string     `json:"id"`
	RoomID        string     `json:"roomID"`
	RootMessageID string     `json:"rootMessageID"`
	Engine        string     `json:"engine"`
	EngineVersion string     `json:"engineVersion"`
	PolicyVersion string     `json:"policyVersion"`
	Status        string     `json:"status"`
	StopReason    string     `json:"stopReason,omitempty"`
	TurnCount     int        `json:"turnCount"`
	ErrorText     string     `json:"errorText,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
}

type GenerateMinutesResponse struct {
	Markdown string                `json:"markdown"`
	Minutes  *model.MeetingMinutes `json:"minutes,omitempty"`
}

type ListRoomsResponse struct {
	Rooms []model.RoomSummary `json:"rooms"`
}

type PublicRoomSummary struct {
	ID                  string                    `json:"id"`
	Name                string                    `json:"name"`
	Status              string                    `json:"status"`
	HasPasscode         bool                      `json:"hasPasscode"`
	CreatedAt           time.Time                 `json:"createdAt"`
	DialoguePolicy      model.DialoguePolicy      `json:"dialoguePolicy"`
	CollaborationPolicy model.CollaborationPolicy `json:"collaborationPolicy"`
	AgentCount          int                       `json:"agentCount"`
}

type ListRecentRoomsResponse struct {
	Rooms []PublicRoomSummary `json:"rooms"`
}

type EntrySummaryResponse struct {
	ActiveRooms        int `json:"activeRooms"`
	TodayRooms         int `json:"todayRooms"`
	KnowledgeDocuments int `json:"knowledgeDocuments"`
	EnabledAgents      int `json:"enabledAgents"`
}

type MinutesHistoryResponse struct {
	Minutes []model.MeetingMinutes `json:"minutes"`
}

type SaveMinutesRequest struct {
	Content string `json:"content"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// DialoguePolicyInput is the request-side view of a dialogue policy. Every field
// is optional: pointer fields let the API tell "omitted" apart from an explicit
// zero value (notably a false boolean), which a plain struct cannot do. Resolve
// overlays only the fields a client actually sent onto DefaultDialoguePolicy, so a
// partial policy such as {"mode":"guided_dialogue"} keeps agent-to-agent handoff
// enabled instead of silently disabling it.
type DialoguePolicyInput struct {
	Mode                      *string `json:"mode"`
	MaxAutonomousTurns        *int    `json:"maxAutonomousTurns"`
	MaxTurnsPerAgent          *int    `json:"maxTurnsPerAgent"`
	AllowSelfFollowup         *bool   `json:"allowSelfFollowup"`
	AllowAgentToAgentMentions *bool   `json:"allowAgentToAgentMentions"`
	ResponseStrategy          *string `json:"responseStrategy"`
	CooldownMS                *int    `json:"cooldownMs"`
}

func (in *DialoguePolicyInput) Resolve() model.DialoguePolicy {
	policy := model.DefaultDialoguePolicy()
	if in == nil {
		return policy
	}
	if in.Mode != nil {
		policy.Mode = *in.Mode
	}
	if in.MaxAutonomousTurns != nil {
		policy.MaxAutonomousTurns = *in.MaxAutonomousTurns
	}
	if in.MaxTurnsPerAgent != nil {
		policy.MaxTurnsPerAgent = *in.MaxTurnsPerAgent
	}
	if in.AllowSelfFollowup != nil {
		policy.AllowSelfFollowup = *in.AllowSelfFollowup
	}
	if in.AllowAgentToAgentMentions != nil {
		policy.AllowAgentToAgentMentions = *in.AllowAgentToAgentMentions
	}
	if in.ResponseStrategy != nil {
		policy.ResponseStrategy = *in.ResponseStrategy
	}
	if in.CooldownMS != nil {
		policy.CooldownMS = *in.CooldownMS
	}
	return policy.WithDefaults()
}

// CollaborationPolicyInput overlays explicitly supplied collaboration fields
// onto the compatibility policy derived from dialoguePolicy.
type CollaborationPolicyInput struct {
	Engine            *string `json:"engine"`
	TriggerMode       *string `json:"triggerMode"`
	MaxTurns          *int    `json:"maxTurns"`
	MaxTurnsPerAgent  *int    `json:"maxTurnsPerAgent"`
	AllowAgentHandoff *bool   `json:"allowAgentHandoff"`
	AllowSelfFollowup *bool   `json:"allowSelfFollowup"`
	CooldownMS        *int    `json:"cooldownMs"`
}

func (in *CollaborationPolicyInput) Resolve(base model.CollaborationPolicy) (model.CollaborationPolicy, error) {
	policy := base.WithDefaults()
	if in == nil {
		return policy, policy.Validate()
	}
	if in.Engine != nil {
		if !model.IsValidCollaborationEngine(*in.Engine) {
			return model.CollaborationPolicy{}, fmt.Errorf("unsupported collaboration engine %q", *in.Engine)
		}
		policy.Engine = *in.Engine
	}
	if in.TriggerMode != nil {
		if !model.IsValidCollaborationTriggerMode(*in.TriggerMode) {
			return model.CollaborationPolicy{}, fmt.Errorf("unsupported collaboration trigger mode %q", *in.TriggerMode)
		}
		policy.TriggerMode = *in.TriggerMode
	}
	if in.MaxTurns != nil {
		if *in.MaxTurns < 1 {
			return model.CollaborationPolicy{}, fmt.Errorf("collaboration max turns must be positive")
		}
		policy.MaxTurns = *in.MaxTurns
	}
	if in.MaxTurnsPerAgent != nil {
		if *in.MaxTurnsPerAgent < 1 {
			return model.CollaborationPolicy{}, fmt.Errorf("collaboration max turns per Agent must be positive")
		}
		policy.MaxTurnsPerAgent = *in.MaxTurnsPerAgent
	}
	if in.AllowAgentHandoff != nil {
		policy.AllowAgentHandoff = *in.AllowAgentHandoff
	}
	if in.AllowSelfFollowup != nil {
		policy.AllowSelfFollowup = *in.AllowSelfFollowup
	}
	if in.CooldownMS != nil {
		if *in.CooldownMS < 0 {
			return model.CollaborationPolicy{}, fmt.Errorf("collaboration cooldown must not be negative")
		}
		policy.CooldownMS = *in.CooldownMS
	}

	policy = policy.WithDefaults()
	return policy, policy.Validate()
}
