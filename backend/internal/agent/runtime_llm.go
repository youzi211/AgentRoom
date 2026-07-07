package agent

import (
	"context"
	"time"

	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/model"
)

type LLMAgentRuntime struct {
	client  llm.Client
	timeout time.Duration
}

func NewLLMAgentRuntime(client llm.Client, timeout time.Duration) *LLMAgentRuntime {
	return &LLMAgentRuntime{client: client, timeout: timeout}
}

func (r *LLMAgentRuntime) Name() string {
	return model.AgentRuntimeLLM
}

func (r *LLMAgentRuntime) Respond(ctx context.Context, request AgentRuntimeRequest) (AgentRuntimeResponse, error) {
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

	response, err := r.client.Complete(requestCtx, promptMessages)
	if err != nil {
		return AgentRuntimeResponse{}, err
	}

	cleaned, err := StripThinkBlocks(response)
	if err != nil {
		return AgentRuntimeResponse{}, err
	}
	return AgentRuntimeResponse{Content: cleaned}, nil
}
