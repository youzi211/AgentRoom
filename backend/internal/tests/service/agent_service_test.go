package service_test

import (
	"context"
	"errors"
	"testing"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/service"
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
