package agent

import (
	"testing"

	"agentroom/backend/internal/model"
)

func TestMentionedAgentsDetectsMultipleMentionsInMessageOrder(t *testing.T) {
	agents := PredefinedAgents()
	message := model.Message{
		SenderType: model.SenderTypeHuman,
		Content:    "@产品经理 @测试工程师 第一版验收标准怎么定？",
	}

	mentioned := MentionedAgents(message, agents)
	if len(mentioned) != 2 {
		t.Fatalf("expected 2 mentioned agents, got %d", len(mentioned))
	}
	if mentioned[0].ID != "pm" {
		t.Fatalf("expected first agent to be pm, got %q", mentioned[0].ID)
	}
	if mentioned[1].ID != "qa" {
		t.Fatalf("expected second agent to be qa, got %q", mentioned[1].ID)
	}
}

func TestMentionedAgentsReturnsNoAgentsForNormalMessage(t *testing.T) {
	agents := PredefinedAgents()
	message := model.Message{
		SenderType: model.SenderTypeHuman,
		Content:    "我们先做一个文字会议室",
	}

	mentioned := MentionedAgents(message, agents)
	if len(mentioned) != 0 {
		t.Fatalf("expected no mentioned agents, got %d", len(mentioned))
	}
}

func TestMentionedAgentsIgnoresAgentMessages(t *testing.T) {
	agents := PredefinedAgents()
	message := model.Message{
		SenderType: model.SenderTypeAgent,
		Content:    "@产品经理 这个页面怎么布局？",
	}

	mentioned := MentionedAgents(message, agents)
	if len(mentioned) != 0 {
		t.Fatalf("expected no mentioned agents for agent message, got %d", len(mentioned))
	}
}
