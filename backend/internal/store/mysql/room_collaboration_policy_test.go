package mysql

import (
	"os"
	"strings"
	"testing"

	"agentroom/backend/internal/model"
)

func TestRoomCollaborationPolicyMigrationAndMapping(t *testing.T) {
	payload, err := os.ReadFile("migrations/009_room_collaboration_policy.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := strings.ToLower(string(payload))
	for _, column := range []string{
		"collaboration_engine",
		"collaboration_trigger_mode",
		"collaboration_max_turns",
		"collaboration_max_turns_per_agent",
		"collaboration_allow_agent_handoff",
		"collaboration_allow_self_followup",
		"collaboration_cooldown_ms",
	} {
		if !strings.Contains(schema, column) {
			t.Fatalf("room collaboration migration is missing %q", column)
		}
	}

	want := model.CollaborationPolicy{
		Engine: model.CollaborationEngineAutoGen, TriggerMode: model.CollaborationTriggerAutomatic,
		MaxTurns: 6, MaxTurnsPerAgent: 2, AllowAgentHandoff: true, AllowSelfFollowup: true, CooldownMS: 40,
	}
	got := (RoomModel{
		CollaborationEngine: want.Engine, CollaborationTriggerMode: want.TriggerMode,
		CollaborationMaxTurns: want.MaxTurns, CollaborationMaxTurnsPerAgent: want.MaxTurnsPerAgent,
		CollaborationAllowAgentHandoff: want.AllowAgentHandoff,
		CollaborationAllowSelfFollowup: want.AllowSelfFollowup,
		CollaborationCooldownMS:        want.CooldownMS,
	}).toDomain().CollaborationPolicy
	if got != want {
		t.Fatalf("expected room collaboration policy %#v, got %#v", want, got)
	}
}
