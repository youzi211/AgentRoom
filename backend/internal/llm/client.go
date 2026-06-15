package llm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	langllms "github.com/tmc/langchaingo/llms"
	langopenai "github.com/tmc/langchaingo/llms/openai"
)

const (
	defaultBaseURL = "https://api.openai.com"
	defaultModel   = "gpt-4o-mini"

	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

var ErrNotConfigured = errors.New("llm api key is not configured")

type ChatMessage struct {
	Role    string
	Content string
}

type Client interface {
	Complete(ctx context.Context, messages []ChatMessage) (string, error)
}

type JSONClient interface {
	CompleteJSON(ctx context.Context, messages []ChatMessage) (string, error)
}

type OpenAIClient struct {
	modelClient langllms.Model
	apiKey      string
	modelName   string
	baseURL     string
	initErr     error
}

type Config struct {
	BaseURL string
	APIKey  string
	Model   string
}

func NewClientFromEnv() *OpenAIClient {
	return NewClient(Config{
		BaseURL: os.Getenv("LLM_BASE_URL"),
		APIKey:  os.Getenv("LLM_API_KEY"),
		Model:   os.Getenv("LLM_MODEL"),
	})
}

func NewClient(config Config) *OpenAIClient {
	apiKey := strings.TrimSpace(config.APIKey)
	modelName := normalizeModel(config.Model)
	baseURL := normalizeBaseURL(config.BaseURL)

	client := &OpenAIClient{
		apiKey:    apiKey,
		modelName: modelName,
		baseURL:   baseURL,
	}

	if apiKey == "" {
		return client
	}

	modelClient, err := langopenai.New(
		langopenai.WithToken(apiKey),
		langopenai.WithModel(modelName),
		langopenai.WithBaseURL(baseURL),
	)
	if err != nil {
		client.initErr = fmt.Errorf("initialize langchaingo openai client: %w", err)
		return client
	}

	client.modelClient = modelClient
	return client
}

func (c *OpenAIClient) Complete(ctx context.Context, messages []ChatMessage) (string, error) {
	return c.complete(ctx, messages)
}

func (c *OpenAIClient) CompleteJSON(ctx context.Context, messages []ChatMessage) (string, error) {
	return c.complete(ctx, messages, langllms.WithJSONMode())
}

func (c *OpenAIClient) complete(ctx context.Context, messages []ChatMessage, options ...langllms.CallOption) (string, error) {
	if c == nil || c.apiKey == "" {
		return "", ErrNotConfigured
	}
	if c.initErr != nil {
		return "", c.initErr
	}

	response, err := c.modelClient.GenerateContent(ctx, toMessageContents(messages), options...)
	if err != nil {
		return "", fmt.Errorf("chat completion request failed: %w", err)
	}
	if len(response.Choices) == 0 {
		return "", errors.New("chat completion returned no choices")
	}

	return strings.TrimSpace(response.Choices[0].Content), nil
}

func toMessageContents(messages []ChatMessage) []langllms.MessageContent {
	result := make([]langllms.MessageContent, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case RoleSystem:
			result = append(result, langllms.TextParts(langllms.ChatMessageTypeSystem, message.Content))
		case RoleAssistant:
			result = append(result, langllms.TextParts(langllms.ChatMessageTypeAI, message.Content))
		case RoleUser:
			result = append(result, langllms.TextParts(langllms.ChatMessageTypeHuman, message.Content))
		default:
			result = append(result, langllms.TextParts(langllms.ChatMessageTypeHuman, message.Content))
		}
	}
	return result
}

func normalizeBaseURL(value string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(value), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL
	}
	return baseURL + "/v1"
}

func normalizeModel(value string) string {
	model := strings.TrimSpace(value)
	if model == "" {
		return defaultModel
	}
	return model
}
