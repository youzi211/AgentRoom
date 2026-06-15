package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"agentroom/backend/internal/llm"
)

func TestClientReturnsErrNotConfiguredWithoutAPIKey(t *testing.T) {
	client := llm.NewClient(llm.Config{})

	_, err := client.Complete(context.Background(), []llm.ChatMessage{
		{Role: llm.RoleUser, Content: "hello"},
	})
	if !errors.Is(err, llm.ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestClientUsesJSONModeAndPreservesChatRoles(t *testing.T) {
	var captured struct {
		Model          string `json:"model"`
		ResponseFormat struct {
			Type string `json:"type"`
		} `json:"response_format"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("expected request path /v1/chat/completions, got %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Fatalf("expected bearer auth header, got %q", auth)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": `{"ok":true}`,
					},
					"finish_reason": "stop",
				},
			},
		})
	}))
	defer server.Close()

	client := llm.NewClient(llm.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "gpt-4o-mini",
	})

	jsonClient, ok := interface{}(client).(llm.JSONClient)
	if !ok {
		t.Fatal("expected OpenAI client to implement JSONClient")
	}

	response, err := jsonClient.CompleteJSON(context.Background(), []llm.ChatMessage{
		{Role: llm.RoleSystem, Content: "system"},
		{Role: llm.RoleUser, Content: "user"},
		{Role: llm.RoleAssistant, Content: "assistant"},
	})
	if err != nil {
		t.Fatalf("CompleteJSON returned error: %v", err)
	}
	if response != `{"ok":true}` {
		t.Fatalf("expected JSON response, got %q", response)
	}
	if captured.Model != "gpt-4o-mini" {
		t.Fatalf("expected model name to be forwarded, got %q", captured.Model)
	}
	if captured.ResponseFormat.Type != "json_object" {
		t.Fatalf("expected JSON mode request, got response format %q", captured.ResponseFormat.Type)
	}
	if len(captured.Messages) != 3 {
		t.Fatalf("expected 3 chat messages, got %d", len(captured.Messages))
	}
	if captured.Messages[0].Role != "system" || captured.Messages[1].Role != "user" || captured.Messages[2].Role != "assistant" {
		t.Fatalf("expected system/user/assistant roles, got %#v", captured.Messages)
	}
}
