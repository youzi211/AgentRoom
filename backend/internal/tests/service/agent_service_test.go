package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/store"
	"agentroom/backend/internal/tests/teststore"
)

func TestAgentServiceResolveForRoomSelectionModes(t *testing.T) {
	agents := []model.Agent{
		{ID: "pm", Name: "Product", Mention: "@Product", Enabled: true},
		{ID: "qa", Name: "QA", Mention: "@QA", Enabled: true},
		{ID: "off", Name: "Disabled", Mention: "@Disabled", Enabled: false},
	}
	svc := service.NewAgentService(&teststore.Store{}, agents)

	allEnabled := svc.ResolveForRoom(nil)
	if len(allEnabled) != 2 || allEnabled[0].ID != "pm" || allEnabled[1].ID != "qa" {
		t.Fatalf("expected enabled agents pm, qa; got %#v", allEnabled)
	}

	none := svc.ResolveForRoom([]string{})
	if len(none) != 0 {
		t.Fatalf("expected empty explicit selection, got %#v", none)
	}

	selected := svc.ResolveForRoom([]string{"qa", "off", "missing"})
	if len(selected) != 1 || selected[0].ID != "qa" {
		t.Fatalf("expected only qa, got %#v", selected)
	}
}

func TestAgentServiceRejectsDuplicateMentionOnUpdate(t *testing.T) {
	agents := []model.Agent{
		{ID: "pm", Name: "Product", Mention: "@Product", Enabled: true},
		{ID: "qa", Name: "QA", Mention: "@QA", Enabled: true},
	}
	svc := service.NewAgentService(&teststore.Store{Agents: append([]model.Agent(nil), agents...)}, agents)

	_, err := svc.UpdateAgent(context.Background(), "qa", service.UpdateAgentInput{Name: "Product"})
	if !errors.Is(err, service.ErrAgentMentionExists) {
		t.Fatalf("expected ErrAgentMentionExists, got %v", err)
	}
}

func TestAgentServiceCreateAllowsBlankRoleTemplate(t *testing.T) {
	store := &teststore.Store{}
	svc := service.NewAgentService(store, nil)

	created, err := svc.CreateAgent(context.Background(), "Reviewer", "QA Engineer", "Finds regression risk.", "   ", true, "")
	if err != nil {
		t.Fatalf("expected create to succeed, got %v", err)
	}
	if created.SystemPrompt != "" {
		t.Fatalf("expected blank role template to remain blank, got %q", created.SystemPrompt)
	}
	if len(store.Agents) != 1 || store.Agents[0].SystemPrompt != "" {
		t.Fatalf("expected persisted blank role template, got %#v", store.Agents)
	}
}

func TestAgentServiceDefaultsAndValidatesRuntime(t *testing.T) {
	store := &teststore.Store{}
	svc := service.NewAgentService(store, nil)

	created, err := svc.CreateAgent(context.Background(), "Research", "Researcher", "Finds sources.", "   ", true, model.AgentRuntimeDeepAgent)
	if err != nil {
		t.Fatalf("expected create to succeed, got %v", err)
	}
	if created.Runtime != model.AgentRuntimeDeepAgent {
		t.Fatalf("expected deepagent runtime, got %#v", created)
	}

	_, err = svc.CreateAgent(context.Background(), "Broken", "Role", "Description", "", true, "unknown")
	if !errors.Is(err, service.ErrInvalidAgentRuntime) {
		t.Fatalf("expected ErrInvalidAgentRuntime, got %v", err)
	}
}

func TestAgentServiceUpdateRuntime(t *testing.T) {
	agents := []model.Agent{
		{ID: "research", Name: "Research", Mention: "@Research", Runtime: model.AgentRuntimeLLM, Enabled: true},
	}
	store := &teststore.Store{Agents: append([]model.Agent(nil), agents...)}
	svc := service.NewAgentService(store, agents)

	updated, err := svc.UpdateAgent(context.Background(), "research", service.UpdateAgentInput{
		Runtime: model.AgentRuntimeDeepAgent,
	})
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}
	if updated.Runtime != model.AgentRuntimeDeepAgent {
		t.Fatalf("expected updated runtime, got %#v", updated)
	}
}

func TestAgentServiceBlankUpdatePreservesExistingRoleTemplate(t *testing.T) {
	agents := []model.Agent{
		{
			ID:           "qa",
			Name:         "QA",
			Mention:      "@QA",
			Role:         "QA Engineer",
			Description:  "Finds risk.",
			SystemPrompt: "Keep replies brief.",
			Enabled:      true,
		},
	}
	store := &teststore.Store{Agents: append([]model.Agent(nil), agents...)}
	svc := service.NewAgentService(store, agents)

	updated, err := svc.UpdateAgent(context.Background(), "qa", service.UpdateAgentInput{
		Description:  "Finds regressions and release risk.",
		SystemPrompt: "   ",
	})
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}
	if updated.SystemPrompt != "Keep replies brief." {
		t.Fatalf("expected blank update to preserve existing role template, got %q", updated.SystemPrompt)
	}
	if updated.Description != "Finds regressions and release risk." {
		t.Fatalf("expected non-prompt fields to update, got %#v", updated)
	}
}

func TestAgentServiceFailedUpdateDoesNotMutateResolvedAgentState(t *testing.T) {
	agents := []model.Agent{
		{
			ID:           "qa",
			Name:         "QA",
			Mention:      "@QA",
			Role:         "QA Engineer",
			Description:  "Finds risk.",
			SystemPrompt: "Keep replies brief.",
			Enabled:      true,
		},
	}
	backingStore := &teststore.Store{
		Agents:         append([]model.Agent(nil), agents...),
		UpdateAgentErr: fmt.Errorf("update failed: %w", store.ErrAgentNotFound),
	}
	svc := service.NewAgentService(backingStore, agents)

	_, err := svc.UpdateAgent(context.Background(), "qa", service.UpdateAgentInput{Name: "Reviewer"})
	if !errors.Is(err, service.ErrAgentNotFound) {
		t.Fatalf("expected ErrAgentNotFound, got %v", err)
	}

	resolved := svc.ResolveForRoom([]string{"qa"})
	if len(resolved) != 1 {
		t.Fatalf("expected original agent to remain resolvable, got %#v", resolved)
	}
	if resolved[0].Name != "QA" || resolved[0].Mention != "@QA" {
		t.Fatalf("expected in-memory agent state to remain unchanged, got %#v", resolved[0])
	}
}

func TestSeedAgentsAddsMissingPredefinedAgentsWithoutOverwritingExistingOnes(t *testing.T) {
	backingStore := &teststore.Store{
		Agents: []model.Agent{
			{
				ID:           "pm",
				Name:         "Custom PM",
				Mention:      "@CustomPM",
				Role:         "Product Manager",
				Runtime:      model.AgentRuntimeLLM,
				Description:  "User customized PM.",
				SystemPrompt: "Keep this custom prompt.",
				Enabled:      true,
			},
		},
	}

	err := backingStore.SeedAgents(context.Background(), []model.Agent{
		{
			ID:           "pm",
			Name:         "产品经理",
			Mention:      "@产品经理",
			Role:         "Product Manager",
			Runtime:      model.AgentRuntimeLLM,
			Description:  "Default PM.",
			SystemPrompt: "Default PM prompt.",
			Enabled:      true,
		},
		{
			ID:           "research",
			Name:         "Research",
			Mention:      "@Research",
			Role:         "Deep Research",
			Runtime:      model.AgentRuntimeDeepAgent,
			Description:  "Runs DeepAgent research.",
			SystemPrompt: "Use DeepAgent.",
			Enabled:      true,
		},
	})
	if err != nil {
		t.Fatalf("seed agents: %v", err)
	}

	agents, err := backingStore.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("list agents: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("expected existing agent plus missing registry agent, got %#v", agents)
	}
	if agents[0].ID != "pm" || agents[0].Name != "Custom PM" || agents[0].SystemPrompt != "Keep this custom prompt." {
		t.Fatalf("seed should not overwrite existing customized agent, got %#v", agents[0])
	}
	if agents[1].ID != "research" || agents[1].Runtime != model.AgentRuntimeDeepAgent {
		t.Fatalf("seed should append missing deepagent agent, got %#v", agents[1])
	}
}

func TestSeedAgentsSkipsRegistryAgentWhenMentionAlreadyExists(t *testing.T) {
	backingStore := &teststore.Store{
		Agents: []model.Agent{
			{
				ID:      "manual_research",
				Name:    "Research",
				Mention: "@Research",
				Runtime: model.AgentRuntimeDeepAgent,
				Enabled: true,
			},
		},
	}

	err := backingStore.SeedAgents(context.Background(), []model.Agent{
		{
			ID:      "research",
			Name:    "Research",
			Mention: "@Research",
			Runtime: model.AgentRuntimeDeepAgent,
			Enabled: true,
		},
	})
	if err != nil {
		t.Fatalf("seed agents: %v", err)
	}

	agents, err := backingStore.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("list agents: %v", err)
	}
	if len(agents) != 1 || agents[0].ID != "manual_research" {
		t.Fatalf("seed should not duplicate an existing mention, got %#v", agents)
	}
}
