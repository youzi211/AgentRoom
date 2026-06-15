package llm

import (
	"context"
	"testing"

	langllms "github.com/tmc/langchaingo/llms"
)

type fakeModel struct {
	response *langllms.ContentResponse
	err      error
	messages []langllms.MessageContent
	options  langllms.CallOptions
}

func (f *fakeModel) GenerateContent(_ context.Context, messages []langllms.MessageContent, opts ...langllms.CallOption) (*langllms.ContentResponse, error) {
	f.messages = append([]langllms.MessageContent(nil), messages...)

	var options langllms.CallOptions
	for _, opt := range opts {
		opt(&options)
	}
	f.options = options

	return f.response, f.err
}

func (f *fakeModel) Call(context.Context, string, ...langllms.CallOption) (string, error) {
	panic("unexpected Call invocation in test")
}

func TestToMessageContentsPreservesConversationRoles(t *testing.T) {
	messages := []ChatMessage{
		{Role: RoleSystem, Content: "system"},
		{Role: RoleUser, Content: "user"},
		{Role: RoleAssistant, Content: "assistant"},
	}

	got := toMessageContents(messages)
	if len(got) != len(messages) {
		t.Fatalf("expected %d messages, got %d", len(messages), len(got))
	}

	if got[0].Role != langllms.ChatMessageTypeSystem {
		t.Fatalf("expected first role to be system, got %q", got[0].Role)
	}
	if got[1].Role != langllms.ChatMessageTypeHuman {
		t.Fatalf("expected second role to be human, got %q", got[1].Role)
	}
	if got[2].Role != langllms.ChatMessageTypeAI {
		t.Fatalf("expected third role to be ai, got %q", got[2].Role)
	}
}

func TestOpenAIClientCompleteJSONUsesJSONMode(t *testing.T) {
	model := &fakeModel{
		response: &langllms.ContentResponse{
			Choices: []*langllms.ContentChoice{{Content: `{"ok":true}`}},
		},
	}

	client := &OpenAIClient{
		modelClient: model,
		apiKey:      "test-key",
		modelName:   "gpt-4o-mini",
		baseURL:     "https://example.com/v1",
	}

	response, err := client.CompleteJSON(context.Background(), []ChatMessage{
		{Role: RoleSystem, Content: "You output JSON."},
		{Role: RoleUser, Content: "Return an object."},
	})
	if err != nil {
		t.Fatalf("CompleteJSON returned error: %v", err)
	}
	if response != `{"ok":true}` {
		t.Fatalf("expected JSON response, got %q", response)
	}
	if !model.options.JSONMode {
		t.Fatal("expected CompleteJSON to enable JSON mode")
	}
	if len(model.messages) != 2 {
		t.Fatalf("expected 2 prompt messages, got %d", len(model.messages))
	}
	if model.messages[1].Role != langllms.ChatMessageTypeHuman {
		t.Fatalf("expected user message to map to human role, got %q", model.messages[1].Role)
	}
}
