package agent

import (
	"context"
	"errors"
	"fmt"

	"agentroom/backend/internal/model"
)

var ErrRuntimeNotConfigured = errors.New("agent runtime is not configured")

type AgentRuntime interface {
	Name() string
	Respond(ctx context.Context, request AgentRuntimeRequest) (AgentRuntimeResponse, error)
}

type AgentRuntimeRequest struct {
	RunID           string
	Room            RuntimeRoom
	Agent           model.Agent
	Trigger         model.Message
	RecentMessages  []model.Message
	KnowledgeChunks []model.KnowledgeChunk
	PromptContext   PromptContext
}

type AgentRuntimeResponse struct {
	Content   string
	Artifacts []AgentRuntimeArtifact
	Metadata  map[string]string
}

type AgentRuntimeArtifact struct {
	ID       string
	Type     string
	Path     string
	MIMEType string
	Title    string
	FileName string
	Content  string
}

type RuntimeRegistry struct {
	runtimes map[string]AgentRuntime
}

func NewRuntimeRegistry(runtimes ...AgentRuntime) *RuntimeRegistry {
	registry := &RuntimeRegistry{runtimes: make(map[string]AgentRuntime, len(runtimes))}
	for _, runtime := range runtimes {
		if runtime == nil {
			continue
		}
		registry.runtimes[model.NormalizeAgentRuntime(runtime.Name())] = runtime
	}
	return registry
}

func (r *RuntimeRegistry) Resolve(agent model.Agent) (AgentRuntime, error) {
	name := model.NormalizeAgentRuntime(agent.Runtime)
	if runtime, ok := r.runtimes[name]; ok {
		return runtime, nil
	}
	return nil, fmt.Errorf("%w: runtime %s is not configured", ErrRuntimeNotConfigured, name)
}
