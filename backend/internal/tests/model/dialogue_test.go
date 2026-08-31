package model_test

import (
	"encoding/json"
	"testing"

	"agentroom/backend/internal/api/contracts"
	"agentroom/backend/internal/model"
)

// A partial policy from a client (only the mode) must inherit the enabled
// agent-to-agent handoff default. This is the regression guard for rooms where
// agents could not reply to each other's @mentions.
func TestDialoguePolicyInputResolveKeepsAgentHandoffByDefault(t *testing.T) {
	var input contracts.DialoguePolicyInput
	if err := json.Unmarshal([]byte(`{"mode":"guided_dialogue"}`), &input); err != nil {
		t.Fatalf("unmarshal partial policy: %v", err)
	}

	policy := input.Resolve()

	if policy.Mode != model.DialogueModeGuided {
		t.Fatalf("expected guided mode, got %q", policy.Mode)
	}
	if !policy.AllowAgentToAgentMentions {
		t.Fatal("expected agent-to-agent mentions to stay enabled for a partial policy")
	}
	if policy.MaxAutonomousTurns != model.DefaultDialoguePolicy().MaxAutonomousTurns {
		t.Fatalf("expected default MaxAutonomousTurns, got %d", policy.MaxAutonomousTurns)
	}
}

// An explicit false must still be honored — the pointer DTO distinguishes it
// from an omitted field.
func TestDialoguePolicyInputResolveHonorsExplicitFalse(t *testing.T) {
	var input contracts.DialoguePolicyInput
	if err := json.Unmarshal([]byte(`{"mode":"guided_dialogue","allowAgentToAgentMentions":false}`), &input); err != nil {
		t.Fatalf("unmarshal policy: %v", err)
	}

	if input.Resolve().AllowAgentToAgentMentions {
		t.Fatal("expected explicit false to disable agent-to-agent mentions")
	}
}

// A nil input (no dialoguePolicy in the request) resolves to the full defaults.
func TestDialoguePolicyInputResolveNilUsesDefaults(t *testing.T) {
	var input *contracts.DialoguePolicyInput

	if input.Resolve() != model.DefaultDialoguePolicy() {
		t.Fatal("expected nil input to resolve to default dialogue policy")
	}
}

func TestCollaborationPolicyInputResolveOverlaysCompatibilityPolicy(t *testing.T) {
	base := model.DialoguePolicy{
		Mode:                      model.DialogueModeGuided,
		MaxAutonomousTurns:        5,
		MaxTurnsPerAgent:          2,
		AllowAgentToAgentMentions: true,
		CooldownMS:                25,
	}.ToCollaborationPolicy()
	var input contracts.CollaborationPolicyInput
	if err := json.Unmarshal([]byte(`{"engine":"AUTOGEN","triggerMode":"AUTOMATIC","allowAgentHandoff":false}`), &input); err != nil {
		t.Fatalf("unmarshal collaboration policy: %v", err)
	}

	policy, err := input.Resolve(base)
	if err != nil {
		t.Fatalf("resolve collaboration policy: %v", err)
	}
	if policy.Engine != model.CollaborationEngineAutoGen || policy.TriggerMode != model.CollaborationTriggerAutomatic {
		t.Fatalf("expected normalized engine and trigger mode, got %#v", policy)
	}
	if policy.MaxTurns != 5 || policy.MaxTurnsPerAgent != 2 || policy.CooldownMS != 25 {
		t.Fatalf("expected omitted fields from compatibility policy, got %#v", policy)
	}
	if policy.AllowAgentHandoff {
		t.Fatal("expected explicit false to disable Agent handoff")
	}
}

func TestCollaborationPolicyInputResolveRejectsExplicitInvalidValues(t *testing.T) {
	tests := []string{
		`{"engine":"unknown"}`,
		`{"triggerMode":"fanout"}`,
		`{"maxTurns":0}`,
		`{"maxTurnsPerAgent":0}`,
		`{"cooldownMs":-1}`,
	}
	for _, body := range tests {
		var input contracts.CollaborationPolicyInput
		if err := json.Unmarshal([]byte(body), &input); err != nil {
			t.Fatalf("unmarshal %s: %v", body, err)
		}
		if _, err := input.Resolve(model.DefaultCollaborationPolicy()); err == nil {
			t.Fatalf("expected invalid collaboration policy %s to fail", body)
		}
	}
}
