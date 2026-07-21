package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"agentroom/backend/internal/logging"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

type UpdateAgentInput struct {
	Name           string
	Role           string
	Runtime        string
	Description    string
	SystemPrompt   string
	Enabled        *bool
	ModelProfileID *string
}

var ErrInvalidAgentModelBinding = errors.New("invalid agent model profile binding")

type AgentService struct {
	mu            sync.RWMutex
	store         agentStore
	agents        []model.Agent
	logger        *slog.Logger
	modelProfiles agentModelProfileStore
}

type agentModelProfileStore interface {
	GetModelProfile(context.Context, string) (model.ModelProfile, error)
	GetDefaultModelProfile(context.Context, string) (model.ModelProfile, error)
}

type agentStore interface {
	CreateAgent(ctx context.Context, agent model.Agent) (model.Agent, error)
	UpdateAgent(ctx context.Context, agent model.Agent) (model.Agent, error)
	DeleteAgent(ctx context.Context, agentID string) error
}

func NewAgentService(s agentStore, agents []model.Agent) *AgentService {
	copiedAgents := make([]model.Agent, len(agents))
	copy(copiedAgents, agents)

	return &AgentService{
		store:  s,
		agents: copiedAgents,
		logger: logging.Component("agent_service"),
	}
}

func (s *AgentService) WithModelProfiles(profiles agentModelProfileStore) *AgentService {
	s.modelProfiles = profiles
	return s
}

func (s *AgentService) Agents() []model.AgentConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]model.AgentConfig, 0, len(s.agents))
	for _, configuredAgent := range s.agents {
		agents = append(agents, configuredAgent.Config())
	}
	return agents
}

func (s *AgentService) ResolveForRoom(agentIDs []string) []model.Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if agentIDs == nil {
		return s.snapshotModelProfiles(enabledAgents(s.agents))
	}
	if len(agentIDs) == 0 {
		return []model.Agent{}
	}

	agentSet := make(map[string]struct{}, len(agentIDs))
	for _, id := range agentIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			agentSet[trimmed] = struct{}{}
		}
	}

	selected := make([]model.Agent, 0, len(agentSet))
	for _, a := range s.agents {
		if !a.Enabled {
			continue
		}
		if _, ok := agentSet[a.ID]; ok {
			selected = append(selected, a)
		}
	}

	return s.snapshotModelProfiles(selected)
}

func (s *AgentService) snapshotModelProfiles(agents []model.Agent) []model.Agent {
	result := append([]model.Agent(nil), agents...)
	if s.modelProfiles == nil {
		return result
	}
	for i := range result {
		if result[i].ModelProfileID != "" {
			continue
		}
		if p, err := s.modelProfiles.GetDefaultModelProfile(context.Background(), model.RuntimeScopeForAgent(result[i].Runtime)); err == nil && p.Enabled {
			result[i].ModelProfileID = p.ID
		}
	}
	return result
}

func (s *AgentService) UpdateAgent(ctx context.Context, agentID string, input UpdateAgentInput) (model.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var current *model.Agent
	for i := range s.agents {
		if s.agents[i].ID == agentID {
			current = &s.agents[i]
			break
		}
	}
	if current == nil {
		return model.Agent{}, ErrAgentNotFound
	}
	if runtime := strings.TrimSpace(input.Runtime); runtime != "" && !model.IsValidAgentRuntime(runtime) {
		return model.Agent{}, ErrInvalidAgentRuntime
	}

	updated := applyAgentUpdate(*current, input)
	if err := s.validateModelBinding(ctx, updated.Runtime, updated.ModelProfileID); err != nil {
		return model.Agent{}, err
	}
	if hasAgentMentionConflict(s.agents, updated.ID, updated.Mention) {
		return model.Agent{}, ErrAgentMentionExists
	}

	result, err := s.store.UpdateAgent(ctx, updated)
	if err != nil {
		s.logger.Error("persist agent update", "agent_id", agentID, "error", err)
		if errors.Is(err, store.ErrAgentNotFound) {
			return model.Agent{}, ErrAgentNotFound
		}
		return model.Agent{}, fmt.Errorf("persist agent update: %w", err)
	}

	s.agents = replaceAgentInSlice(s.agents, result)
	return result, nil
}

func (s *AgentService) CreateAgent(ctx context.Context, name, role, description, systemPrompt string, enabled bool, runtime string) (model.Agent, error) {
	return s.CreateAgentWithModel(ctx, name, role, description, systemPrompt, enabled, runtime, "")
}

func (s *AgentService) CreateAgentWithModel(ctx context.Context, name, role, description, systemPrompt string, enabled bool, runtime string, modelProfileID string) (model.Agent, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return model.Agent{}, fmt.Errorf("agent name is required")
	}
	normalizedRuntime := model.NormalizeAgentRuntime(runtime)
	if !model.IsValidAgentRuntime(normalizedRuntime) {
		return model.Agent{}, ErrInvalidAgentRuntime
	}
	mention := "@" + trimmedName

	s.mu.RLock()
	if hasAgentMentionConflict(s.agents, "", mention) {
		s.mu.RUnlock()
		return model.Agent{}, ErrAgentMentionExists
	}
	s.mu.RUnlock()

	a := model.Agent{
		ID:             model.NewID("agent"),
		Name:           trimmedName,
		Mention:        mention,
		Role:           strings.TrimSpace(role),
		Runtime:        normalizedRuntime,
		Source:         model.AgentSourceBuiltin,
		Description:    strings.TrimSpace(description),
		SystemPrompt:   strings.TrimSpace(systemPrompt),
		Enabled:        enabled,
		ModelProfileID: strings.TrimSpace(modelProfileID),
	}
	if err := s.validateModelBinding(ctx, a.Runtime, a.ModelProfileID); err != nil {
		return model.Agent{}, err
	}

	result, err := s.store.CreateAgent(ctx, a)
	if err != nil {
		return model.Agent{}, fmt.Errorf("persist new agent: %w", err)
	}

	s.mu.Lock()
	s.agents = append(s.agents, result)
	s.mu.Unlock()

	return result, nil
}

func (s *AgentService) validateModelBinding(ctx context.Context, runtime, profileID string) error {
	if strings.TrimSpace(profileID) == "" || s.modelProfiles == nil {
		return nil
	}
	p, err := s.modelProfiles.GetModelProfile(ctx, profileID)
	if err != nil {
		return fmt.Errorf("%w: model profile not found", ErrInvalidAgentModelBinding)
	}
	if !p.Enabled {
		return fmt.Errorf("%w: model profile is disabled", ErrInvalidAgentModelBinding)
	}
	if p.RuntimeScope != model.RuntimeScopeForAgent(runtime) {
		return fmt.Errorf("%w: runtime scope mismatch", ErrInvalidAgentModelBinding)
	}
	return nil
}

func (s *AgentService) DeleteAgent(ctx context.Context, agentID string) error {
	if err := s.store.DeleteAgent(ctx, agentID); err != nil {
		if errors.Is(err, store.ErrAgentNotFound) {
			return ErrAgentNotFound
		}
		return fmt.Errorf("delete agent: %w", err)
	}

	s.mu.Lock()
	s.agents = removeAgentFromSlice(s.agents, agentID)
	s.mu.Unlock()

	return nil
}

func applyAgentUpdate(current model.Agent, input UpdateAgentInput) model.Agent {
	if name := strings.TrimSpace(input.Name); name != "" {
		current.Name = name
		current.Mention = "@" + name
	}
	if role := strings.TrimSpace(input.Role); role != "" {
		current.Role = role
	}
	if runtime := strings.TrimSpace(input.Runtime); runtime != "" {
		current.Runtime = model.NormalizeAgentRuntime(runtime)
	}
	if description := strings.TrimSpace(input.Description); description != "" {
		current.Description = description
	}
	if systemPrompt := strings.TrimSpace(input.SystemPrompt); systemPrompt != "" {
		current.SystemPrompt = systemPrompt
	}
	if input.Enabled != nil {
		current.Enabled = *input.Enabled
	}
	if input.ModelProfileID != nil {
		current.ModelProfileID = strings.TrimSpace(*input.ModelProfileID)
	}
	return current
}

func enabledAgents(agents []model.Agent) []model.Agent {
	enabled := make([]model.Agent, 0, len(agents))
	for _, configuredAgent := range agents {
		if !configuredAgent.Enabled {
			continue
		}
		enabled = append(enabled, configuredAgent)
	}
	return enabled
}

func replaceAgentInSlice(agents []model.Agent, updated model.Agent) []model.Agent {
	result := make([]model.Agent, len(agents))
	copy(result, agents)
	for i, a := range result {
		if a.ID == updated.ID {
			result[i] = updated
			break
		}
	}
	return result
}

func removeAgentFromSlice(agents []model.Agent, agentID string) []model.Agent {
	result := make([]model.Agent, 0, len(agents))
	for _, a := range agents {
		if a.ID != agentID {
			result = append(result, a)
		}
	}
	return result
}

func hasAgentMentionConflict(agents []model.Agent, currentAgentID string, mention string) bool {
	for _, a := range agents {
		if a.ID == currentAgentID {
			continue
		}
		if a.Mention == mention {
			return true
		}
	}
	return false
}
