package agent_test

import (
	"testing"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/model"
)

func TestMentionedAgentsOnlyRespondsToHumanMessages(t *testing.T) {
	agents := []model.Agent{
		{ID: "pm", Name: "Product", Mention: "@Product"},
	}

	agentMessage := model.Message{
		SenderType: model.SenderTypeAgent,
		Content:    "@Product please respond",
	}
	if got := agent.MentionedAgents(agentMessage, agents); len(got) != 0 {
		t.Fatalf("agent messages must not trigger agents, got %d mentions", len(got))
	}

	humanMessage := model.Message{
		SenderType: model.SenderTypeHuman,
		Content:    "@Product please respond",
	}
	got := agent.MentionedAgents(humanMessage, agents)
	if len(got) != 1 || got[0].ID != "pm" {
		t.Fatalf("human mention should trigger pm, got %#v", got)
	}
}

func TestDetectMentionsReturnsTextOrderAndDeduplicates(t *testing.T) {
	agents := []model.Agent{
		{ID: "pm", Name: "Product", Mention: "@Product"},
		{ID: "qa", Name: "QA", Mention: "@QA"},
	}

	got := agent.DetectMentions("Need @QA first, then @Product and @QA again", agents)
	if len(got) != 2 {
		t.Fatalf("expected 2 unique mentions, got %d", len(got))
	}
	if got[0].ID != "qa" || got[1].ID != "pm" {
		t.Fatalf("expected text order qa, pm; got %#v", got)
	}
}
