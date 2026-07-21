package llm

import (
	"agentroom/backend/internal/model"
	"context"
)

type ModelResolver interface {
	Resolve(context.Context, string, string) (model.ResolvedModelConfig, error)
}
type ResolvingClient struct {
	resolver ModelResolver
	scope    string
}

func NewResolvingClient(resolver ModelResolver, scope string) *ResolvingClient {
	return &ResolvingClient{resolver: resolver, scope: scope}
}
func (c *ResolvingClient) client(ctx context.Context) (*OpenAIClient, error) {
	resolved, err := c.resolver.Resolve(ctx, c.scope, "")
	if err != nil {
		return nil, err
	}
	return NewClient(Config{BaseURL: resolved.BaseURL, APIKey: resolved.APIKey, Model: resolved.ModelName}), nil
}
func (c *ResolvingClient) Complete(ctx context.Context, messages []ChatMessage) (string, error) {
	client, err := c.client(ctx)
	if err != nil {
		return "", err
	}
	return client.Complete(ctx, messages)
}
func (c *ResolvingClient) CompleteJSON(ctx context.Context, messages []ChatMessage) (string, error) {
	client, err := c.client(ctx)
	if err != nil {
		return "", err
	}
	return client.CompleteJSON(ctx, messages)
}
