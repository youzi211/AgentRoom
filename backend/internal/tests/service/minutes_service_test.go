package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/service"
)

type stubMinutesLLM struct {
	response string
	err      error
	messages []llm.ChatMessage
}

func (s *stubMinutesLLM) Complete(_ context.Context, messages []llm.ChatMessage) (string, error) {
	s.messages = append([]llm.ChatMessage(nil), messages...)
	return s.response, s.err
}

func TestMinutesServiceFallsBackWhenLLMReturnsError(t *testing.T) {
	minutesService := service.NewMinutesService(&stubMinutesLLM{err: errors.New("boom")})

	markdown, err := minutesService.Generate(context.Background(), model.RoomMeta{Name: "规划会"}, []model.Message{
		{
			ID:         "msg-1",
			RoomID:     "room-1",
			SenderID:   "human-1",
			SenderName: "Alice",
			SenderType: model.SenderTypeHuman,
			Content:    "讨论发布时间",
			CreatedAt:  time.Now().UTC(),
		},
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if markdown == "" {
		t.Fatal("expected fallback markdown when LLM errors")
	}
	if !strings.HasPrefix(markdown, "##") {
		t.Fatalf("expected fallback markdown heading, got %q", markdown)
	}
}

func TestMinutesServiceNormalizesMarkdownHeadings(t *testing.T) {
	minutesService := service.NewMinutesService(&stubMinutesLLM{response: "Summary only"})

	markdown, err := minutesService.Generate(context.Background(), model.RoomMeta{Name: "规划会"}, nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	expectedPrefix := "## Meeting Minutes\n\n"
	if !strings.HasPrefix(markdown, expectedPrefix) {
		t.Fatalf("expected normalized markdown prefix %q, got %q", expectedPrefix, markdown)
	}
}

func TestMinutesServiceIncludesRoomAndTranscriptInPrompt(t *testing.T) {
	client := &stubMinutesLLM{response: "# Minutes"}
	minutesService := service.NewMinutesService(client)

	_, err := minutesService.Generate(context.Background(), model.RoomMeta{Name: "规划会"}, []model.Message{
		{
			ID:         "msg-1",
			RoomID:     "room-1",
			SenderID:   "human-1",
			SenderName: "Alice",
			SenderType: model.SenderTypeHuman,
			Content:    "讨论发布时间",
			CreatedAt:  time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if len(client.messages) != 2 {
		t.Fatalf("expected 2 prompt messages, got %d", len(client.messages))
	}
	if client.messages[0].Role != llm.RoleSystem {
		t.Fatalf("expected system message first, got %q", client.messages[0].Role)
	}
	if client.messages[1].Role != llm.RoleUser {
		t.Fatalf("expected user message second, got %q", client.messages[1].Role)
	}
	if got := client.messages[1].Content; got == "" || !containsAll(got, "规划会", "Alice", "讨论发布时间") {
		t.Fatalf("expected prompt to include room and transcript, got %q", got)
	}
}

func containsAll(text string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(text, part) {
			return false
		}
	}
	return true
}
