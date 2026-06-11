package llm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const (
	defaultBaseURL = "https://api.openai.com"
	defaultModel   = "gpt-4o-mini"

	RoleSystem = "system"
	RoleUser   = "user"
)

var ErrNotConfigured = errors.New("llm api key is not configured")

type ChatMessage struct {
	Role    string
	Content string
}

type Client interface {
	Complete(ctx context.Context, messages []ChatMessage) (string, error)
}

type OpenAIClient struct {
	client openai.Client
	apiKey string
	model  string
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
	model := normalizeModel(config.Model)
	baseURL := normalizeBaseURL(config.BaseURL)

	return &OpenAIClient{
		client: openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithBaseURL(baseURL),
		),
		apiKey: apiKey,
		model:  model,
	}
}

func (c *OpenAIClient) Complete(ctx context.Context, messages []ChatMessage) (string, error) {
	if c == nil || c.apiKey == "" {
		return "", ErrNotConfigured
	}

	chatCompletion, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: toOpenAIMessages(messages),
		Model:    c.model,
	})
	if err != nil {
		return "", fmt.Errorf("chat completion request failed: %w", err)
	}

	if len(chatCompletion.Choices) == 0 {
		return "", errors.New("chat completion returned no choices")
	}

	return strings.TrimSpace(chatCompletion.Choices[0].Message.Content), nil
}

func toOpenAIMessages(messages []ChatMessage) []openai.ChatCompletionMessageParamUnion {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case RoleSystem:
			result = append(result, openai.SystemMessage(message.Content))
		default:
			result = append(result, openai.UserMessage(message.Content))
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
