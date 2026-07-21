package model

import "time"

const (
	ModelRuntimeGo        = "go"
	ModelRuntimeDeepAgent = "deepagent"

	ModelProtocolOpenAIChatCompletions = "openai_chat_completions"
)

type ModelProfile struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	RuntimeScope        string    `json:"runtimeScope"`
	Protocol            string    `json:"protocol"`
	BaseURL             string    `json:"baseURL"`
	ModelName           string    `json:"modelName"`
	APIKeyCiphertext    string    `json:"-"`
	APIKeyHint          string    `json:"apiKeyHint,omitempty"`
	HasAPIKey           bool      `json:"hasAPIKey"`
	Enabled             bool      `json:"enabled"`
	IsDefault           bool      `json:"isDefault"`
	EnvironmentFallback bool      `json:"environmentFallback,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type ResolvedModelConfig struct{ ProfileID, Source, BaseURL, ModelName, APIKey string }

func RuntimeScopeForAgent(runtime string) string {
	if NormalizeAgentRuntime(runtime) == AgentRuntimeDeepAgent {
		return ModelRuntimeDeepAgent
	}
	return ModelRuntimeGo
}
