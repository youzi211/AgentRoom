package contracts

import "agentroom/backend/internal/model"

type ModelProfilesResponse struct {
	Profiles []model.ModelProfile `json:"profiles"`
}
type CreateModelProfileRequest struct {
	Name         string `json:"name"`
	RuntimeScope string `json:"runtimeScope"`
	Protocol     string `json:"protocol"`
	BaseURL      string `json:"baseURL"`
	ModelName    string `json:"modelName"`
	APIKey       string `json:"apiKey"`
	Enabled      bool   `json:"enabled"`
	IsDefault    bool   `json:"isDefault"`
}
type UpdateModelProfileRequest struct {
	Name        string  `json:"name"`
	BaseURL     string  `json:"baseURL"`
	ModelName   string  `json:"modelName"`
	APIKey      *string `json:"apiKey"`
	ClearAPIKey bool    `json:"clearAPIKey"`
	Enabled     *bool   `json:"enabled"`
}
type TestModelProfileRequest struct {
	BaseURL   string `json:"baseURL"`
	ModelName string `json:"modelName"`
	APIKey    string `json:"apiKey"`
}
