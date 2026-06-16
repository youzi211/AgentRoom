package contracts

import (
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
	Name           string               `json:"name"`
	AgentIDs       []string             `json:"agentIds"`
	Passcode       string               `json:"passcode"`
	DialoguePolicy *DialoguePolicyInput `json:"dialoguePolicy,omitempty"`
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
	AgentRuns    []AgentRunActivity    `json:"agentRuns"`
	DialogueRuns []DialogueRunActivity `json:"dialogueRuns"`
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

type GenerateMinutesResponse struct {
	Markdown string               `json:"markdown"`
	Minutes  *model.MeetingMinutes `json:"minutes,omitempty"`
}

type ListRoomsResponse struct {
	Rooms []model.RoomSummary `json:"rooms"`
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
