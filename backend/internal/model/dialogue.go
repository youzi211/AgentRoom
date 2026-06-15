package model

import "strings"

const (
	DialogueModeMentionFanout = "mention_fanout"
	DialogueModeGuided        = "guided_dialogue"
)

const (
	DialogueResponseStrategyMentionedFirst = "mentioned_first"
)

const (
	DialogueRunStatusRunning          = "running"
	DialogueRunStatusSucceeded        = "succeeded"
	DialogueRunStatusFailed           = "failed"
	DialogueRunStatusTimeout          = "timeout"
	DialogueRunStatusStoppedLimit     = "stopped_limit"
	DialogueRunStatusStoppedDuplicate = "stopped_duplicate"
	DialogueRunStatusStoppedEmpty     = "stopped_empty"
)

type DialoguePolicy struct {
	Mode                      string `json:"mode"`
	MaxAutonomousTurns        int    `json:"maxAutonomousTurns"`
	MaxTurnsPerAgent          int    `json:"maxTurnsPerAgent"`
	AllowSelfFollowup         bool   `json:"allowSelfFollowup"`
	AllowAgentToAgentMentions bool   `json:"allowAgentToAgentMentions"`
	ResponseStrategy          string `json:"responseStrategy"`
	CooldownMS                int    `json:"cooldownMs"`
}

func DefaultDialoguePolicy() DialoguePolicy {
	return DialoguePolicy{
		Mode:                      DialogueModeMentionFanout,
		MaxAutonomousTurns:        3,
		MaxTurnsPerAgent:          1,
		AllowSelfFollowup:         false,
		AllowAgentToAgentMentions: true,
		ResponseStrategy:          DialogueResponseStrategyMentionedFirst,
		CooldownMS:                0,
	}
}

func (p DialoguePolicy) WithDefaults() DialoguePolicy {
	defaults := DefaultDialoguePolicy()

	mode := strings.TrimSpace(p.Mode)
	if mode != DialogueModeGuided && mode != DialogueModeMentionFanout {
		p.Mode = defaults.Mode
	}
	if p.MaxAutonomousTurns <= 0 {
		p.MaxAutonomousTurns = defaults.MaxAutonomousTurns
	}
	if p.MaxTurnsPerAgent <= 0 {
		p.MaxTurnsPerAgent = defaults.MaxTurnsPerAgent
	}
	if strings.TrimSpace(p.ResponseStrategy) == "" {
		p.ResponseStrategy = defaults.ResponseStrategy
	}
	if p.CooldownMS < 0 {
		p.CooldownMS = defaults.CooldownMS
	}
	return p
}

func (p DialoguePolicy) IsGuided() bool {
	return p.WithDefaults().Mode == DialogueModeGuided
}

// DialoguePolicyInput is the request-side view of a dialogue policy. Every field
// is optional: pointer fields let the API tell "omitted" apart from an explicit
// zero value (notably a `false` boolean), which a plain struct cannot do. Resolve
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

func (in *DialoguePolicyInput) Resolve() DialoguePolicy {
	policy := DefaultDialoguePolicy()
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
