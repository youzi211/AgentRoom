package contracts

import "agentroom/backend/internal/model"

type UpdateAgentRequest struct {
	Name         string `json:"name"`
	Role         string `json:"role"`
	Description  string `json:"description"`
	SystemPrompt string `json:"systemPrompt"`
	Enabled      *bool  `json:"enabled"`
}

type CreateAgentRequest struct {
	Name         string `json:"name"`
	Role         string `json:"role"`
	Description  string `json:"description"`
	SystemPrompt string `json:"systemPrompt"`
	Enabled      *bool  `json:"enabled"`
}

type AgentsResponse struct {
	Agents []model.AgentConfig `json:"agents"`
}
