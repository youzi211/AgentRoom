package mysql

import (
	"os"
	"strings"
	"testing"
)

func TestAgentRunResultMigrationIsNullableUniqueAndReferenced(t *testing.T) {
	payload, err := os.ReadFile("migrations/006_agent_run_results.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := strings.ToLower(string(payload))
	for _, fragment := range []string{
		"add column agent_run_id varchar(64) null",
		"unique key uk_messages_agent_run_id (agent_run_id)",
		"foreign key (agent_run_id) references agent_runs(id)",
	} {
		if !strings.Contains(schema, fragment) {
			t.Fatalf("migration must contain %q", fragment)
		}
	}
}

func TestMessageAgentRunReferenceStaysInternal(t *testing.T) {
	message := MessageModel{AgentRunID: strPtr("run_1")}.toDomain()
	if message.AgentRunID != "run_1" {
		t.Fatalf("expected internal Agent Run reference, got %#v", message)
	}
}
