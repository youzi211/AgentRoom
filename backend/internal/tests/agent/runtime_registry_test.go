package agent_test

import (
	"context"
	"errors"
	"testing"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/model"
)

type fakeRuntime struct {
	name string
}

func (r fakeRuntime) Name() string { return r.name }

func (r fakeRuntime) Respond(context.Context, agent.AgentRuntimeRequest, ...agent.AgentEventObserver) (agent.AgentRuntimeResponse, error) {
	return agent.AgentRuntimeResponse{Content: "ok"}, nil
}

func TestRuntimeRegistryResolvesDefaultLLMRuntime(t *testing.T) {
	registry := agent.NewRuntimeRegistry(fakeRuntime{name: model.AgentRuntimeLLM})

	got, err := registry.Resolve(model.Agent{})
	if err != nil {
		t.Fatalf("expected default runtime to resolve, got %v", err)
	}
	if got.Name() != model.AgentRuntimeLLM {
		t.Fatalf("expected llm runtime, got %q", got.Name())
	}
}

func TestRuntimeRegistryRejectsUnregisteredDeepAgentRuntime(t *testing.T) {
	registry := agent.NewRuntimeRegistry(fakeRuntime{name: model.AgentRuntimeLLM})

	_, err := registry.Resolve(model.Agent{Runtime: model.AgentRuntimeDeepAgent})
	if !errors.Is(err, agent.ErrRuntimeNotConfigured) {
		t.Fatalf("expected ErrRuntimeNotConfigured, got %v", err)
	}
}

func TestNormalizeAgentRuntimeDefaultsAndValidatesValues(t *testing.T) {
	if got := model.NormalizeAgentRuntime(""); got != model.AgentRuntimeLLM {
		t.Fatalf("expected empty runtime to default to llm, got %q", got)
	}
	if got := model.NormalizeAgentRuntime("  DEEPAGENT  "); got != model.AgentRuntimeDeepAgent {
		t.Fatalf("expected deepagent runtime to normalize, got %q", got)
	}
	if model.IsValidAgentRuntime("unknown") {
		t.Fatal("expected unknown runtime to be invalid")
	}
}
