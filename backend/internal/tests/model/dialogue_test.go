package model_test

import (
	"encoding/json"
	"testing"

	"agentroom/backend/internal/model"
)

// A partial policy from a client (only the mode) must inherit the enabled
// agent-to-agent handoff default. This is the regression guard for rooms where
// agents could not reply to each other's @mentions.
func TestDialoguePolicyInputResolveKeepsAgentHandoffByDefault(t *testing.T) {
	var input model.DialoguePolicyInput
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
	var input model.DialoguePolicyInput
	if err := json.Unmarshal([]byte(`{"mode":"guided_dialogue","allowAgentToAgentMentions":false}`), &input); err != nil {
		t.Fatalf("unmarshal policy: %v", err)
	}

	if input.Resolve().AllowAgentToAgentMentions {
		t.Fatal("expected explicit false to disable agent-to-agent mentions")
	}
}

// A nil input (no dialoguePolicy in the request) resolves to the full defaults.
func TestDialoguePolicyInputResolveNilUsesDefaults(t *testing.T) {
	var input *model.DialoguePolicyInput

	if input.Resolve() != model.DefaultDialoguePolicy() {
		t.Fatal("expected nil input to resolve to default dialogue policy")
	}
}
