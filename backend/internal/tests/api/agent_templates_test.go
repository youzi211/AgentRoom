package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agentroom/backend/internal/api"
)

func TestAgentTemplatesEndpointReturnsProductDefaults(t *testing.T) {
	server, _, _ := newActivityTestServer(t, api.Config{})

	request := httptest.NewRequest(http.MethodGet, "/api/agent-templates", nil)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected templates response 200, got %d body=%s", response.Code, response.Body.String())
	}

	var payload struct {
		Templates []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Role         string `json:"role"`
			Description  string `json:"description"`
			SystemPrompt string `json:"systemPrompt"`
		} `json:"templates"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode templates response: %v", err)
	}
	if len(payload.Templates) < 5 {
		t.Fatalf("expected role templates, got %#v", payload.Templates)
	}
	if payload.Templates[0].ID != "product_manager" || payload.Templates[0].SystemPrompt == "" {
		t.Fatalf("expected product manager template first, got %#v", payload.Templates[0])
	}
}
