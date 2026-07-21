package contracts

import "agentroom/backend/internal/model"

type UpdateAgentRequest struct {
	Name           string  `json:"name"`
	Role           string  `json:"role"`
	Runtime        string  `json:"runtime"`
	Description    string  `json:"description"`
	SystemPrompt   string  `json:"systemPrompt"`
	Enabled        *bool   `json:"enabled"`
	ModelProfileID *string `json:"modelProfileID"`
}

type CreateAgentRequest struct {
	Name           string `json:"name"`
	Role           string `json:"role"`
	Runtime        string `json:"runtime"`
	Description    string `json:"description"`
	SystemPrompt   string `json:"systemPrompt"`
	Enabled        *bool  `json:"enabled"`
	ModelProfileID string `json:"modelProfileID"`
}

type AgentsResponse struct {
	Agents []model.AgentConfig `json:"agents"`
}

type RoleTemplate struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	Description  string `json:"description"`
	SystemPrompt string `json:"systemPrompt"`
}

type AgentTemplatesResponse struct {
	Templates []RoleTemplate `json:"templates"`
}

type RoleSet struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	TemplateIDs []string `json:"templateIDs"`
}

type AgentRoleSetsResponse struct {
	RoleSets []RoleSet `json:"roleSets"`
}
