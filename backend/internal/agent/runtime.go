package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"agentroom/backend/internal/model"
)

var ErrRuntimeNotConfigured = errors.New("agent runtime is not configured")

type AgentRuntime interface {
	Name() string
	Respond(ctx context.Context, request AgentRuntimeRequest, observers ...AgentEventObserver) (AgentRuntimeResponse, error)
}

type AgentRuntimeEvent struct {
	RunID      string
	Kind       string
	ModelName  string
	ToolName   string
	Failure    string
	OccurredAt time.Time
}

type AgentEventObserver interface {
	ObserveAgentEvent(context.Context, AgentRuntimeEvent)
}

type AgentEventObserverFunc func(context.Context, AgentRuntimeEvent)

func (f AgentEventObserverFunc) ObserveAgentEvent(ctx context.Context, event AgentRuntimeEvent) {
	f(ctx, event)
}

type ModelConfigResolver interface {
	Resolve(context.Context, string, string) (model.ResolvedModelConfig, error)
}

type AgentRuntimeRequest struct {
	RunID           string
	TraceID         string
	Room            RuntimeRoom
	Agent           model.Agent
	Trigger         model.Message
	RecentMessages  []model.Message
	KnowledgeChunks []model.KnowledgeChunk
	PromptContext   PromptContext
}

func observeRuntimeEvent(ctx context.Context, observers []AgentEventObserver, event AgentRuntimeEvent) {
	if len(observers) == 0 || observers[0] == nil {
		return
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	observers[0].ObserveAgentEvent(ctx, event)
}

type AgentRuntimeResponse struct {
	Content          string
	Artifacts        []AgentRuntimeArtifact
	KnowledgeSources []model.MessageKnowledgeSource
	Metadata         map[string]string
}

func modelAuditMetadata(config model.ResolvedModelConfig) map[string]string {
	if config.Source == "" {
		return nil
	}
	return map[string]string{
		"model_profile_id": config.ProfileID,
		"model_source":     config.Source,
		"model_name":       config.ModelName,
	}
}

func redactSecret(value, secret string) string {
	if secret == "" {
		return value
	}
	return strings.ReplaceAll(value, secret, "[REDACTED]")
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

func (r *RuntimeRegistry) CancelRoom(roomID string) int {
	cancelled := 0
	seenRemoteClients := make(map[*RemoteRuntimeClient]struct{})
	for _, runtime := range r.runtimes {
		if remote, ok := runtime.(*RemotePythonRuntime); ok {
			if _, duplicate := seenRemoteClients[remote.client]; duplicate {
				continue
			}
			seenRemoteClients[remote.client] = struct{}{}
			cancelled += remote.client.CancelRoom(roomID)
			continue
		}
		canceler, ok := runtime.(interface{ CancelRoom(string) int })
		if !ok {
			continue
		}
		cancelled += canceler.CancelRoom(roomID)
	}
	return cancelled
}
