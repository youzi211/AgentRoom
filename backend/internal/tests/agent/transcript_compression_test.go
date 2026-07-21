package agent_test

import (
	"strings"
	"testing"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/model"
)

func renderTranscriptOnly(t *testing.T, transcript []model.Message) string {
	t.Helper()
	messages, err := agent.ComposePromptForRuntime(model.Agent{}, agent.PromptContext{
		RoomName:     "Planning",
		DialogueMode: model.DialogueModeMentionFanout,
		Transcript:   transcript,
	})
	if err != nil {
		t.Fatalf("ComposePromptForRuntime returned error: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected system and user messages, got %#v", messages)
	}
	return messages[1].Content
}

func TestTranscriptShortMessageRendersInFull(t *testing.T) {
	transcript := []model.Message{
		{SenderName: "Alice", SenderType: model.SenderTypeHuman, Content: "A short message."},
	}
	userPrompt := renderTranscriptOnly(t, transcript)
	if !strings.Contains(userPrompt, "Alice (human): A short message.") {
		t.Fatalf("expected short message to render in full, got %q", userPrompt)
	}
}

func TestTranscriptOverlongRecentMessageIsTruncatedWithMarker(t *testing.T) {
	longContent := strings.Repeat("x", 801)
	transcript := []model.Message{
		{SenderName: "Alice", SenderType: model.SenderTypeHuman, Content: longContent},
	}
	userPrompt := renderTranscriptOnly(t, transcript)
	if strings.Contains(userPrompt, longContent) {
		t.Fatalf("expected overlong message to be truncated, got full content in %q", userPrompt)
	}
	if !strings.Contains(userPrompt, "…[消息过长，已截断]") {
		t.Fatalf("expected truncated message to carry the truncation marker, got %q", userPrompt)
	}
	if !strings.Contains(userPrompt, strings.Repeat("x", 800)) {
		t.Fatalf("expected truncated message to keep exactly the char limit worth of content, got %q", userPrompt)
	}
}

func TestTranscriptMessageAtExactCharLimitIsNotTruncated(t *testing.T) {
	exactContent := strings.Repeat("y", 800)
	transcript := []model.Message{
		{SenderName: "Alice", SenderType: model.SenderTypeHuman, Content: exactContent},
	}
	userPrompt := renderTranscriptOnly(t, transcript)
	if strings.Contains(userPrompt, "已截断") {
		t.Fatalf("expected message at exact limit to render untruncated, got %q", userPrompt)
	}
	if !strings.Contains(userPrompt, exactContent) {
		t.Fatalf("expected exact-limit message content to be preserved in full, got %q", userPrompt)
	}
}

func TestTranscriptOlderMessagesDowngradeToSummaryLine(t *testing.T) {
	transcript := make([]model.Message, 0, 12)
	for i := 0; i < 12; i++ {
		transcript = append(transcript, model.Message{
			SenderName: "Alice",
			SenderType: model.SenderTypeHuman,
			Content:    "Full detail content for message number that is reasonably long to test summarization behavior " + string(rune('A'+i)),
		})
	}
	userPrompt := renderTranscriptOnly(t, transcript)

	// The oldest 2 messages (12 - 10 full-layer count) should be summarized
	// down to an 80-char prefix plus an ellipsis marker.
	if !strings.Contains(userPrompt, "Full detail content for message number that is reasonably long to test summariza…") {
		t.Fatalf("expected oldest messages to be downgraded to a short summary line, got %q", userPrompt)
	}
	// The most recent message should still render close to full content.
	if !strings.Contains(userPrompt, "Full detail content for message number that is reasonably long to test summarization behavior L") {
		t.Fatalf("expected most recent message to keep full-text rendering, got %q", userPrompt)
	}
}

func TestTranscriptEmptyProducesNoneMarker(t *testing.T) {
	userPrompt := renderTranscriptOnly(t, nil)
	if !strings.Contains(userPrompt, "Visible room transcript:\n- none") {
		t.Fatalf("expected empty transcript to render as none, got %q", userPrompt)
	}
}

func TestTranscriptBudgetDropsOldestMessagesWhenOverBudget(t *testing.T) {
	// 30 messages each ~800 chars in the full layer range would vastly exceed
	// the 6000 char budget; only the newest ones (and a minimum retained
	// count) should survive.
	transcript := make([]model.Message, 0, 30)
	for i := 0; i < 30; i++ {
		transcript = append(transcript, model.Message{
			SenderName: "Alice",
			SenderType: model.SenderTypeHuman,
			Content:    strings.Repeat("z", 800),
		})
	}
	userPrompt := renderTranscriptOnly(t, transcript)

	lineCount := strings.Count(userPrompt, "Alice (human):")
	if lineCount >= 30 {
		t.Fatalf("expected budget enforcement to drop oldest messages, got %d rendered lines", lineCount)
	}
	if lineCount < 5 {
		t.Fatalf("expected at least the minimum retained message count to survive, got %d rendered lines", lineCount)
	}
}

func TestTranscriptBudgetStopsAtMinimumRetainedMessages(t *testing.T) {
	// Even if the minimum retained messages alone exceed the budget, budget
	// enforcement must not drop below the floor.
	transcript := make([]model.Message, 0, 5)
	for i := 0; i < 5; i++ {
		transcript = append(transcript, model.Message{
			SenderName: "Alice",
			SenderType: model.SenderTypeHuman,
			Content:    strings.Repeat("w", 800),
		})
	}
	userPrompt := renderTranscriptOnly(t, transcript)

	lineCount := strings.Count(userPrompt, "Alice (human):")
	if lineCount != 5 {
		t.Fatalf("expected minimum retained message floor to keep all 5 messages, got %d", lineCount)
	}
}
