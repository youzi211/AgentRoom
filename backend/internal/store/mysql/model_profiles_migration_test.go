package mysql

import (
	"os"
	"strings"
	"testing"

	"agentroom/backend/internal/model"
)

func TestLegacyRowsWithoutModelProfileFieldsRemainReadable(t *testing.T) {
	agent := (AgentModel{ID: "legacy-agent", Runtime: model.AgentRuntimeLLM}).toDomain()
	if agent.ModelProfileID != "" {
		t.Fatalf("legacy agent profile id = %q, want empty", agent.ModelProfileID)
	}

	roomAgent := (RoomAgentModel{RoomID: "legacy-room", AgentID: "legacy-agent", Runtime: model.AgentRuntimeLLM}).toDomain()
	if roomAgent.ModelProfileID != "" {
		t.Fatalf("legacy room agent profile id = %q, want empty", roomAgent.ModelProfileID)
	}

	run := (AgentRunModel{ID: "legacy-run"}).toStore()
	if run.ModelProfileID != "" || run.ModelSource != "" || run.ModelName != "" {
		t.Fatalf("legacy run model audit must remain empty: %+v", run)
	}
}

func TestModelProfileMigrationDeclaresDefaultUniquenessAndNullableReferences(t *testing.T) {
	payload, err := os.ReadFile("migrations/005_model_profiles.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := strings.ToLower(string(payload))
	for _, fragment := range []string{
		"create table model_profiles",
		"unique key uk_model_profiles_default_slot (default_slot)",
		"alter table agents add column model_profile_id varchar(64) null",
		"alter table room_agents add column model_profile_id varchar(64) null",
		"alter table agent_runs add column model_profile_id varchar(64) null",
		"alter table agent_runs add column model_source",
		"alter table agent_runs add column model_name",
	} {
		if !strings.Contains(schema, fragment) {
			t.Fatalf("migration is missing %q", fragment)
		}
	}

	defaultProfile := modelProfileToModel(model.ModelProfile{RuntimeScope: model.ModelRuntimeGo, IsDefault: true})
	if defaultProfile.DefaultSlot == nil || *defaultProfile.DefaultSlot != model.ModelRuntimeGo {
		t.Fatalf("default slot = %#v, want go", defaultProfile.DefaultSlot)
	}
	nonDefault := modelProfileToModel(model.ModelProfile{RuntimeScope: model.ModelRuntimeGo})
	if nonDefault.DefaultSlot != nil {
		t.Fatalf("non-default slot = %#v, want nil so MySQL allows multiple non-default rows", nonDefault.DefaultSlot)
	}
}
