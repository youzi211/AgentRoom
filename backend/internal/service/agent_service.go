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

var (
	ErrAgentNotFound      = errors.New("agent not found")
	ErrAgentMentionExists = errors.New("agent mention already exists")
)

type UpdateAgentInput struct {
	Name         string
	Role         string
	Description  string
	SystemPrompt string
	Enabled      *bool
}

type AgentService struct {
	mu     sync.RWMutex
	store  store.Store
	agents []model.Agent
	logger *slog.Logger
}

func NewAgentService(s store.Store, agents []model.Agent) *AgentService {
	copiedAgents := make([]model.Agent, len(agents))
	copy(copiedAgents, agents)

	return &AgentService{
		store:  s,
		agents: copiedAgents,
		logger: logging.Component("agent_service"),
	}
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
		return enabledAgents(s.agents)
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

	return selected
}

func (s *AgentService) UpdateAgent(ctx context.Context, agentID string, input UpdateAgentInput) (model.Agent, error) {
	s.mu.Lock()

	var current *model.Agent
	for i := range s.agents {
		if s.agents[i].ID == agentID {
			current = &s.agents[i]
			break
		}
	}
	if current == nil {
		s.mu.Unlock()
		return model.Agent{}, ErrAgentNotFound
	}

	updated := applyAgentUpdate(*current, input)
	if hasAgentMentionConflict(s.agents, updated.ID, updated.Mention) {
		s.mu.Unlock()
		return model.Agent{}, ErrAgentMentionExists
	}
	s.agents = replaceAgentInSlice(s.agents, updated)
	s.mu.Unlock()

	result, err := s.store.UpdateAgent(ctx, updated)
	if err != nil {
		s.logger.Error("persist agent update", "agent_id", agentID, "error", err)
		return model.Agent{}, fmt.Errorf("persist agent update: %w", err)
	}

	return result, nil
}

func (s *AgentService) CreateAgent(ctx context.Context, name, role, description, systemPrompt string, enabled bool) (model.Agent, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return model.Agent{}, fmt.Errorf("agent name is required")
	}
	mention := "@" + trimmedName

	s.mu.RLock()
	if hasAgentMentionConflict(s.agents, "", mention) {
		s.mu.RUnlock()
		return model.Agent{}, ErrAgentMentionExists
	}
	s.mu.RUnlock()

	a := model.Agent{
		ID:           model.NewID("agent"),
		Name:         trimmedName,
		Mention:      mention,
		Role:         strings.TrimSpace(role),
		Description:  strings.TrimSpace(description),
		SystemPrompt: strings.TrimSpace(systemPrompt),
		Enabled:      enabled,
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

func (s *AgentService) DeleteAgent(ctx context.Context, agentID string) error {
	if err := s.store.DeleteAgent(ctx, agentID); err != nil {
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
	if description := strings.TrimSpace(input.Description); description != "" {
		current.Description = description
	}
	if systemPrompt := strings.TrimSpace(input.SystemPrompt); systemPrompt != "" {
		current.SystemPrompt = systemPrompt
	}
	if input.Enabled != nil {
		current.Enabled = *input.Enabled
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
