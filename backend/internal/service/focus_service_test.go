package service

import (
	"context"
	"testing"
	"time"

	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/model"
)

type jsonAwareLLM struct {
	completeCalls int
	jsonCalls     int
	textResponse  string
	jsonResponse  string
	err           error
}

func (c *jsonAwareLLM) Complete(context.Context, []llm.ChatMessage) (string, error) {
	c.completeCalls++
	return c.textResponse, c.err
}

func (c *jsonAwareLLM) CompleteJSON(context.Context, []llm.ChatMessage) (string, error) {
	c.jsonCalls++
	return c.jsonResponse, c.err
}

func TestFocusServicePrefersStructuredJSONCompletionWhenAvailable(t *testing.T) {
	client := &jsonAwareLLM{
		textResponse: "this should not be used",
		jsonResponse: `[{"content":"排期风险","category":"风险"}]`,
	}
	service := NewFocusService(client)
	now := time.Now().UTC()

	var got []model.FocusPoint
	for i := 0; i < 3; i++ {
		got = service.AddMessage(context.Background(), "room-1", model.Message{
			ID:         "msg",
			RoomID:     "room-1",
			SenderID:   "human-1",
			SenderName: "Alice",
			SenderType: model.SenderTypeHuman,
			Content:    "讨论发布时间和风险",
			CreatedAt:  now.Add(time.Duration(i) * time.Minute),
		})
	}

	if client.jsonCalls != 1 {
		t.Fatalf("expected structured JSON completion to be used once, got %d", client.jsonCalls)
	}
	if client.completeCalls != 0 {
		t.Fatalf("expected plain completion not to be used when JSON completion exists, got %d calls", client.completeCalls)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 focus point, got %d", len(got))
	}
	if got[0].Content != "排期风险" {
		t.Fatalf("expected focus point content to come from JSON response, got %q", got[0].Content)
	}
}
