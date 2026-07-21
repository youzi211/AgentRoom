package agent

import (
	"context"
	"errors"
	"time"

	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/model"
)

type LLMAgentRuntime struct {
	client   llm.Client
	timeout  time.Duration
	resolver ModelConfigResolver
}

func (r *LLMAgentRuntime) WithModelResolver(resolver ModelConfigResolver) *LLMAgentRuntime {
	r.resolver = resolver
	return r
}

func NewLLMAgentRuntime(client llm.Client, timeout time.Duration) *LLMAgentRuntime {
	return &LLMAgentRuntime{client: client, timeout: timeout}
}

func (r *LLMAgentRuntime) Name() string {
	return model.AgentRuntimeLLM
}

func (r *LLMAgentRuntime) Respond(ctx context.Context, request AgentRuntimeRequest, observers ...AgentEventObserver) (_ AgentRuntimeResponse, err error) {
	observeRuntimeEvent(ctx, observers, AgentRuntimeEvent{RunID: request.RunID, Kind: "accepted"})
	defer func() {
		kind := "completed"
		failure := ""
		if err != nil {
			kind = "failed"
			failure = "local runtime failed"
		}
		observeRuntimeEvent(ctx, observers, AgentRuntimeEvent{RunID: request.RunID, Kind: kind, Failure: failure})
	}()
	promptContext := request.PromptContext
	if promptContext.RoomName == "" {
		promptContext = NewMentionPromptContext(request.Room, request.RecentMessages, request.Trigger, request.KnowledgeChunks)
	}
	promptMessages, err := composePromptMessages(request.Agent, promptContext)
	if err != nil {
		return AgentRuntimeResponse{}, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	client := r.client
	resolved := model.ResolvedModelConfig{}
	if r.resolver != nil {
		resolved, err = r.resolver.Resolve(requestCtx, model.ModelRuntimeGo, request.Agent.ModelProfileID)
		if err != nil {
			return AgentRuntimeResponse{}, err
		}
		client = llm.NewClient(llm.Config{BaseURL: resolved.BaseURL, APIKey: resolved.APIKey, Model: resolved.ModelName})
	}
	metadata := modelAuditMetadata(resolved)
	observeRuntimeEvent(ctx, observers, AgentRuntimeEvent{RunID: request.RunID, Kind: "model_started", ModelName: resolved.ModelName})
	response, err := client.Complete(requestCtx, promptMessages)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return AgentRuntimeResponse{Metadata: metadata}, err
		}
		return AgentRuntimeResponse{Metadata: metadata}, errors.New("model request failed")
	}

	cleaned, err := StripThinkBlocks(response)
	if err != nil {
		return AgentRuntimeResponse{}, err
	}
	observeRuntimeEvent(ctx, observers, AgentRuntimeEvent{RunID: request.RunID, Kind: "model_completed", ModelName: resolved.ModelName})
	return AgentRuntimeResponse{Content: cleaned, Metadata: metadata}, nil
}
