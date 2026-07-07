package config_test

import (
	"testing"
	"time"

	"agentroom/backend/internal/config"
)

func TestLoadDeepAgentConfigDefaults(t *testing.T) {
	t.Setenv("DEEPAGENT_COMMAND", "")
	t.Setenv("DEEPAGENT_WORKDIR", "")
	t.Setenv("DEEPAGENT_CONFIG", "")
	t.Setenv("DEEPAGENT_REGISTRY", "")
	t.Setenv("DEEPAGENT_TIMEOUT_SECONDS", "")

	got := config.LoadDeepAgentConfig()
	if got.Command != "uv" {
		t.Fatalf("expected default command uv, got %#v", got)
	}
	if got.WorkDir != "../deepagent" {
		t.Fatalf("expected default workdir ../deepagent, got %#v", got)
	}
	if got.Config != "deepagent.toml" {
		t.Fatalf("expected default config deepagent.toml, got %#v", got)
	}
	if got.Registry != "agents.json" {
		t.Fatalf("expected default registry agents.json, got %#v", got)
	}
	if got.Timeout != 5*time.Minute {
		t.Fatalf("expected default timeout 5 minutes, got %#v", got)
	}
}

func TestLoadDeepAgentConfigFromEnv(t *testing.T) {
	t.Setenv("DEEPAGENT_COMMAND", "python")
	t.Setenv("DEEPAGENT_WORKDIR", "../runtime")
	t.Setenv("DEEPAGENT_CONFIG", "custom.toml")
	t.Setenv("DEEPAGENT_REGISTRY", "custom-agents.json")
	t.Setenv("DEEPAGENT_TIMEOUT_SECONDS", "42")

	got := config.LoadDeepAgentConfig()
	if got.Command != "python" || got.WorkDir != "../runtime" || got.Config != "custom.toml" || got.Registry != "custom-agents.json" || got.Timeout != 42*time.Second {
		t.Fatalf("unexpected deepagent config: %#v", got)
	}
}
