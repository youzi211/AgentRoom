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
