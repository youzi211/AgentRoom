package agent_test

import (
	"testing"

	"agentroom/backend/internal/agent"
)

func TestStripThinkBlocksRemovesPrivateReasoning(t *testing.T) {
	got, err := agent.StripThinkBlocks("<think>private chain</think>\nVisible answer")
	if err != nil {
		t.Fatalf("stripThinkBlocks returned error: %v", err)
	}
	if got != "Visible answer" {
		t.Fatalf("unexpected cleaned answer: %q", got)
	}
}

func TestStripThinkBlocksRejectsThinkOnlyResponses(t *testing.T) {
	if _, err := agent.StripThinkBlocks("<thinking>private chain</thinking>"); err == nil {
		t.Fatal("expected error for response that only contains private reasoning")
	}
}

func TestStripThinkBlocksHandlesUnclosedTags(t *testing.T) {
	got, err := agent.StripThinkBlocks("Visible answer\n<think>private tail")
	if err != nil {
		t.Fatalf("stripThinkBlocks returned error: %v", err)
	}
	if got != "Visible answer" {
		t.Fatalf("unexpected cleaned answer: %q", got)
	}
}
