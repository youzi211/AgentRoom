package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.openai.com"
	defaultModel   = "gpt-4o-mini"

	RoleSystem = "system"
	RoleUser   = "user"
)

var ErrNotConfigured = errors.New("llm api key is not configured")

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client interface {
	Complete(ctx context.Context, messages []ChatMessage) (string, error)
}

type OpenAIClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

type Config struct {
	BaseURL string
	APIKey  string
	Model   string
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

type apiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func NewClientFromEnv() *OpenAIClient {
	return NewClient(Config{
		BaseURL: os.Getenv("LLM_BASE_URL"),
		APIKey:  os.Getenv("LLM_API_KEY"),
		Model:   os.Getenv("LLM_MODEL"),
	})
}

func NewClient(config Config) *OpenAIClient {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	model := strings.TrimSpace(config.Model)
	if model == "" {
		model = defaultModel
	}

	return &OpenAIClient{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(config.APIKey),
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *OpenAIClient) Complete(ctx context.Context, messages []ChatMessage) (string, error) {
	if c == nil || c.apiKey == "" {
		return "", ErrNotConfigured
	}

	payload, err := json.Marshal(chatCompletionRequest{
		Model:    c.model,
		Messages: messages,
	})
	if err != nil {
		return "", fmt.Errorf("marshal chat completion request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build chat completion request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("send chat completion request: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read chat completion response: %w", err)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", parseAPIError(response.StatusCode, body)
	}

	var parsed chatCompletionResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decode chat completion response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("chat completion returned no choices")
	}

	return parsed.Choices[0].Message.Content, nil
}

func parseAPIError(statusCode int, body []byte) error {
	var parsed apiErrorResponse
	if err := json.Unmarshal(body, &parsed); err == nil {
		message := strings.TrimSpace(parsed.Error.Message)
		if message != "" {
			return fmt.Errorf("llm request failed with status %d: %s", statusCode, message)
		}
	}

	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(statusCode)
	}
	return fmt.Errorf("llm request failed with status %d: %s", statusCode, message)
}
