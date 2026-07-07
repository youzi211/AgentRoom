package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"agentroom/backend/internal/model"
)

type deepAgentRegistry struct {
	Agents []deepAgentRegistryAgent `json:"agents"`
}

type deepAgentRegistryAgent struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Mention      string `json:"mention"`
	Role         string `json:"role"`
	Description  string `json:"description"`
	SystemPrompt string `json:"systemPrompt"`
	Enabled      *bool  `json:"enabled"`
}

func LoadDeepAgentRegistryAgents(path string) ([]model.Agent, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return nil, nil
	}
	data, err := os.ReadFile(trimmedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read deepagent registry: %w", err)
	}

	var registry deepAgentRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("parse deepagent registry: %w", err)
	}

	agents := make([]model.Agent, 0, len(registry.Agents))
	for _, entry := range registry.Agents {
		agent := deepAgentRegistryEntryToAgent(entry)
		if agent.ID == "" || agent.Name == "" {
			continue
		}
		agents = append(agents, agent)
	}
	return agents, nil
}

func deepAgentRegistryEntryToAgent(entry deepAgentRegistryAgent) model.Agent {
	enabled := true
	if entry.Enabled != nil {
		enabled = *entry.Enabled
	}
	name := strings.TrimSpace(entry.Name)
	mention := strings.TrimSpace(entry.Mention)
	if mention == "" && name != "" {
		mention = "@" + name
	}
	return model.Agent{
		ID:           strings.TrimSpace(entry.ID),
		Name:         name,
		Mention:      mention,
		Role:         strings.TrimSpace(entry.Role),
		Runtime:      model.AgentRuntimeDeepAgent,
		Source:       model.AgentSourceDeepAgent,
		Description:  strings.TrimSpace(entry.Description),
		SystemPrompt: strings.TrimSpace(entry.SystemPrompt),
		Enabled:      enabled,
	}
}

func MergeAgentDefinitions(groups ...[]model.Agent) []model.Agent {
	merged := make([]model.Agent, 0)
	seenIDs := make(map[string]struct{})
	seenMentions := make(map[string]struct{})
	for _, group := range groups {
		for _, candidate := range group {
			id := strings.TrimSpace(candidate.ID)
			mention := strings.ToLower(strings.TrimSpace(candidate.Mention))
			if id == "" {
				continue
			}
			if _, ok := seenIDs[id]; ok {
				continue
			}
			if mention != "" {
				if _, ok := seenMentions[mention]; ok {
					continue
				}
			}
			candidate.ID = id
			candidate.Runtime = model.NormalizeAgentRuntime(candidate.Runtime)
			candidate.Source = model.NormalizeAgentSource(candidate.Source)
			merged = append(merged, candidate)
			seenIDs[id] = struct{}{}
			if mention != "" {
				seenMentions[mention] = struct{}{}
			}
		}
	}
	return merged
}
