package model

import (
	"fmt"
	"strings"
)

const (
	CollaborationEngineNative  = "native"
	CollaborationEngineAutoGen = "autogen"
	CollaborationPolicyVersion = "v1"
)

const (
	CollaborationTriggerMentionOnly = "mention_only"
	CollaborationTriggerAutomatic   = "automatic"
)

const (
	CollaborationStopReasonCompleted        = "completed"
	CollaborationStopReasonMaxTurns         = "max_turns"
	CollaborationStopReasonMaxTurnsPerAgent = "max_turns_per_agent"
	CollaborationStopReasonEmptyOutput      = "empty_output"
	CollaborationStopReasonDuplicateOutput  = "duplicate_output"
	CollaborationStopReasonNoEligibleAgent  = "no_eligible_agent"
	CollaborationStopReasonCancelled        = "cancelled"
	CollaborationStopReasonDeadlineExceeded = "deadline_exceeded"
	CollaborationStopReasonInterrupted      = "interrupted"
	CollaborationStopReasonEngineFailure    = "engine_failure"
	CollaborationStopReasonProtocolError    = "protocol_error"
)

const (
	CollaborationRunStatusCreated     = "created"
	CollaborationRunStatusRunning     = "running"
	CollaborationRunStatusSucceeded   = "succeeded"
	CollaborationRunStatusStopped     = "stopped"
	CollaborationRunStatusFailed      = "failed"
	CollaborationRunStatusCancelled   = "cancelled"
	CollaborationRunStatusTimeout     = "timeout"
	CollaborationRunStatusInterrupted = "interrupted"
)

type CollaborationPolicy struct {
	Engine            string `json:"engine"`
	TriggerMode       string `json:"triggerMode"`
	MaxTurns          int    `json:"maxTurns"`
	MaxTurnsPerAgent  int    `json:"maxTurnsPerAgent"`
	AllowAgentHandoff bool   `json:"allowAgentHandoff"`
	AllowSelfFollowup bool   `json:"allowSelfFollowup"`
	CooldownMS        int    `json:"cooldownMs"`
}

func DefaultCollaborationPolicy() CollaborationPolicy {
	return CollaborationPolicy{
		Engine:            CollaborationEngineNative,
		TriggerMode:       CollaborationTriggerMentionOnly,
		MaxTurns:          3,
		MaxTurnsPerAgent:  1,
		AllowAgentHandoff: true,
	}
}

func (p CollaborationPolicy) WithDefaults() CollaborationPolicy {
	defaults := DefaultCollaborationPolicy()
	if p == (CollaborationPolicy{}) {
		return defaults
	}
	if strings.TrimSpace(p.Engine) == "" {
		p.Engine = defaults.Engine
	} else {
		p.Engine = strings.ToLower(strings.TrimSpace(p.Engine))
	}
	if strings.TrimSpace(p.TriggerMode) == "" {
		p.TriggerMode = defaults.TriggerMode
	} else {
		p.TriggerMode = strings.ToLower(strings.TrimSpace(p.TriggerMode))
	}
	if p.MaxTurns == 0 {
		p.MaxTurns = defaults.MaxTurns
	}
	if p.MaxTurnsPerAgent == 0 {
		p.MaxTurnsPerAgent = defaults.MaxTurnsPerAgent
	}
	return p
}

func (p CollaborationPolicy) Validate() error {
	p = p.WithDefaults()
	if !IsValidCollaborationEngine(p.Engine) {
		return fmt.Errorf("unsupported collaboration engine %q", p.Engine)
	}
	if !IsValidCollaborationTriggerMode(p.TriggerMode) {
		return fmt.Errorf("unsupported collaboration trigger mode %q", p.TriggerMode)
	}
	if p.MaxTurns < 1 {
		return fmt.Errorf("collaboration max turns must be positive")
	}
	if p.MaxTurnsPerAgent < 1 {
		return fmt.Errorf("collaboration max turns per Agent must be positive")
	}
	if p.MaxTurnsPerAgent > p.MaxTurns {
		return fmt.Errorf("collaboration max turns per Agent cannot exceed max turns")
	}
	if p.CooldownMS < 0 {
		return fmt.Errorf("collaboration cooldown must not be negative")
	}
	return nil
}

func IsValidCollaborationEngine(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case CollaborationEngineNative, CollaborationEngineAutoGen:
		return true
	default:
		return false
	}
}

func IsValidCollaborationTriggerMode(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case CollaborationTriggerMentionOnly, CollaborationTriggerAutomatic:
		return true
	default:
		return false
	}
}

func IsValidCollaborationStopReason(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case CollaborationStopReasonCompleted,
		CollaborationStopReasonMaxTurns,
		CollaborationStopReasonMaxTurnsPerAgent,
		CollaborationStopReasonEmptyOutput,
		CollaborationStopReasonDuplicateOutput,
		CollaborationStopReasonNoEligibleAgent,
		CollaborationStopReasonCancelled,
		CollaborationStopReasonDeadlineExceeded,
		CollaborationStopReasonInterrupted,
		CollaborationStopReasonEngineFailure,
		CollaborationStopReasonProtocolError:
		return true
	default:
		return false
	}
}

func IsValidCollaborationRunStatus(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case CollaborationRunStatusCreated,
		CollaborationRunStatusRunning,
		CollaborationRunStatusSucceeded,
		CollaborationRunStatusStopped,
		CollaborationRunStatusFailed,
		CollaborationRunStatusCancelled,
		CollaborationRunStatusTimeout,
		CollaborationRunStatusInterrupted:
		return true
	default:
		return false
	}
}

func IsTerminalCollaborationRunStatus(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case CollaborationRunStatusSucceeded,
		CollaborationRunStatusStopped,
		CollaborationRunStatusFailed,
		CollaborationRunStatusCancelled,
		CollaborationRunStatusTimeout,
		CollaborationRunStatusInterrupted:
		return true
	default:
		return false
	}
}

func CanTransitionCollaborationRunStatus(from string, to string) bool {
	from = strings.ToLower(strings.TrimSpace(from))
	to = strings.ToLower(strings.TrimSpace(to))
	if !IsValidCollaborationRunStatus(from) || !IsValidCollaborationRunStatus(to) || from == to {
		return false
	}
	switch from {
	case CollaborationRunStatusCreated:
		return to == CollaborationRunStatusRunning ||
			to == CollaborationRunStatusFailed ||
			to == CollaborationRunStatusCancelled ||
			to == CollaborationRunStatusTimeout ||
			to == CollaborationRunStatusInterrupted
	case CollaborationRunStatusRunning:
		return IsTerminalCollaborationRunStatus(to)
	default:
		return false
	}
}

func IsCollaborationStopReasonForStatus(status string, reason string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	reason = strings.ToLower(strings.TrimSpace(reason))
	switch status {
	case CollaborationRunStatusSucceeded:
		return reason == CollaborationStopReasonCompleted
	case CollaborationRunStatusStopped:
		switch reason {
		case CollaborationStopReasonMaxTurns,
			CollaborationStopReasonMaxTurnsPerAgent,
			CollaborationStopReasonEmptyOutput,
			CollaborationStopReasonDuplicateOutput,
			CollaborationStopReasonNoEligibleAgent:
			return true
		}
	case CollaborationRunStatusFailed:
		return reason == CollaborationStopReasonEngineFailure || reason == CollaborationStopReasonProtocolError
	case CollaborationRunStatusCancelled:
		return reason == CollaborationStopReasonCancelled
	case CollaborationRunStatusTimeout:
		return reason == CollaborationStopReasonDeadlineExceeded
	case CollaborationRunStatusInterrupted:
		return reason == CollaborationStopReasonInterrupted
	}
	return false
}
