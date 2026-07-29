package model_test

import (
	"strings"
	"testing"

	"agentroom/backend/internal/model"
)

func TestCollaborationPolicyDefaultsPreserveLegacyRoomBehavior(t *testing.T) {
	got := (model.CollaborationPolicy{}).WithDefaults()
	want := model.DefaultCollaborationPolicy()
	if got != want {
		t.Fatalf("expected default collaboration policy %#v, got %#v", want, got)
	}
	if got.Engine != model.CollaborationEngineNative || got.TriggerMode != model.CollaborationTriggerMentionOnly {
		t.Fatalf("expected native mention-only compatibility defaults, got %#v", got)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("expected default collaboration policy to be valid: %v", err)
	}
}

func TestCollaborationPolicyNormalizesEnums(t *testing.T) {
	policy := model.CollaborationPolicy{
		Engine:           " AutoGen ",
		TriggerMode:      " AUTOMATIC ",
		MaxTurns:         4,
		MaxTurnsPerAgent: 2,
	}.WithDefaults()

	if policy.Engine != model.CollaborationEngineAutoGen || policy.TriggerMode != model.CollaborationTriggerAutomatic {
		t.Fatalf("expected normalized collaboration enums, got %#v", policy)
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("expected normalized policy to be valid: %v", err)
	}
}

func TestCollaborationPolicyRejectsInvalidDomainValues(t *testing.T) {
	tests := []struct {
		name    string
		policy  model.CollaborationPolicy
		message string
	}{
		{name: "engine", policy: model.CollaborationPolicy{Engine: "direct_provider"}, message: "engine"},
		{name: "trigger mode", policy: model.CollaborationPolicy{TriggerMode: "fanout"}, message: "trigger mode"},
		{name: "max turns", policy: model.CollaborationPolicy{MaxTurns: -1}, message: "max turns"},
		{name: "max turns per Agent", policy: model.CollaborationPolicy{MaxTurnsPerAgent: -1}, message: "max turns per Agent"},
		{name: "per Agent exceeds total", policy: model.CollaborationPolicy{MaxTurns: 1, MaxTurnsPerAgent: 2}, message: "cannot exceed"},
		{name: "cooldown", policy: model.CollaborationPolicy{CooldownMS: -1}, message: "cooldown"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.policy.Validate(); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("expected validation error containing %q, got %v", test.message, err)
			}
		})
	}
}

func TestCollaborationStopReasonsAreStableAndValidated(t *testing.T) {
	reasons := []string{
		model.CollaborationStopReasonCompleted,
		model.CollaborationStopReasonMaxTurns,
		model.CollaborationStopReasonMaxTurnsPerAgent,
		model.CollaborationStopReasonEmptyOutput,
		model.CollaborationStopReasonDuplicateOutput,
		model.CollaborationStopReasonNoEligibleAgent,
		model.CollaborationStopReasonCancelled,
		model.CollaborationStopReasonDeadlineExceeded,
		model.CollaborationStopReasonInterrupted,
		model.CollaborationStopReasonEngineFailure,
		model.CollaborationStopReasonProtocolError,
	}
	for _, reason := range reasons {
		if !model.IsValidCollaborationStopReason(reason) {
			t.Fatalf("expected stop reason %q to be valid", reason)
		}
	}
	if model.IsValidCollaborationStopReason("provider_error_with_secret") {
		t.Fatal("expected provider-specific stop reason to be rejected")
	}
}

func TestDialoguePolicyMapsToCompatibleCollaborationPolicy(t *testing.T) {
	legacy := model.DialoguePolicy{
		Mode:                      model.DialogueModeGuided,
		MaxAutonomousTurns:        5,
		MaxTurnsPerAgent:          2,
		AllowSelfFollowup:         true,
		AllowAgentToAgentMentions: true,
		CooldownMS:                25,
	}

	got := legacy.ToCollaborationPolicy()
	if got.Engine != model.CollaborationEngineNative || got.TriggerMode != model.CollaborationTriggerMentionOnly {
		t.Fatalf("expected legacy policy to use native mention-only collaboration, got %#v", got)
	}
	if got.MaxTurns != 5 || got.MaxTurnsPerAgent != 2 || !got.AllowAgentHandoff || !got.AllowSelfFollowup || got.CooldownMS != 25 {
		t.Fatalf("expected legacy limits to map to collaboration policy, got %#v", got)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("expected mapped collaboration policy to be valid: %v", err)
	}
}

func TestEmptyDialoguePolicyMapsToCollaborationDefaults(t *testing.T) {
	got := (model.DialoguePolicy{}).ToCollaborationPolicy()
	if got != model.DefaultCollaborationPolicy() {
		t.Fatalf("expected empty legacy policy to map to collaboration defaults, got %#v", got)
	}
}
