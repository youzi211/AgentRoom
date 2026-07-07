package agent_test

import (
	"os"
	"path/filepath"
	"testing"

	"agentroom/backend/internal/agent"
)

func TestRoleTemplatesAreStableAndComplete(t *testing.T) {
	templates := agent.RoleTemplates()
	if len(templates) < 5 {
		t.Fatalf("expected meeting role templates, got %#v", templates)
	}

	wantIDs := []string{"product_manager", "architect", "qa_reviewer", "risk_reviewer", "meeting_scribe"}
	for i, id := range wantIDs {
		if templates[i].ID != id {
			t.Fatalf("template %d should have stable id %q, got %#v", i, id, templates[i])
		}
		if templates[i].Name == "" || templates[i].Role == "" || templates[i].Description == "" || templates[i].SystemPrompt == "" {
			t.Fatalf("template %q is incomplete: %#v", id, templates[i])
		}
	}
}

func TestPredefinedAgentsDeriveFromRoleTemplates(t *testing.T) {
	templates := agent.RoleTemplates()
	agents := agent.PredefinedAgents()
	if len(agents) != len(templates) {
		t.Fatalf("expected one predefined agent per template, got templates=%d agents=%d", len(templates), len(agents))
	}

	for i, template := range templates {
		got := agents[i]
		if got.ID == "" || got.Name != template.Name || got.Role != template.Role || got.Description != template.Description || got.SystemPrompt != template.SystemPrompt {
			t.Fatalf("agent %d should derive from template %#v, got %#v", i, template, got)
		}
		if got.Mention != "@"+got.Name || !got.Enabled {
			t.Fatalf("agent should have enabled mention from template name, got %#v", got)
		}
	}
	if agents[0].ID != "pm" || agents[2].ID != "qa" || agents[4].ID != "secretary" {
		t.Fatalf("predefined agent ids should preserve runtime compatibility, got %#v", agents)
	}
}

func TestLoadDeepAgentRegistryAgents(t *testing.T) {
	registryPath := filepath.Join(t.TempDir(), "agents.json")
	if err := os.WriteFile(registryPath, []byte(`{
		"agents": [
			{
				"id": "research",
				"name": "Research",
				"mention": "@Research",
				"role": "Deep Research",
				"description": "Runs DeepAgent research.",
				"systemPrompt": "Use the DeepAgent research graph."
			}
		]
	}`), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	agents, err := agent.LoadDeepAgentRegistryAgents(registryPath)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected one registry agent, got %#v", agents)
	}
	got := agents[0]
	if got.ID != "research" || got.Name != "Research" || got.Mention != "@Research" || got.Runtime != "deepagent" || !got.Enabled {
		t.Fatalf("unexpected registry agent: %#v", got)
	}
}
