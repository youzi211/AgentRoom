package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agentroom/backend/internal/api"
)

func TestAgentRoleSetsEndpointReturnsSelectionShortcuts(t *testing.T) {
	server, _, _ := newActivityTestServer(t, api.Config{})

	request := httptest.NewRequest(http.MethodGet, "/api/agent-role-sets", nil)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected role sets response 200, got %d body=%s", response.Code, response.Body.String())
	}

	var payload struct {
		RoleSets []struct {
			ID          string   `json:"id"`
			Name        string   `json:"name"`
			Description string   `json:"description"`
			TemplateIDs []string `json:"templateIDs"`
		} `json:"roleSets"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode role sets response: %v", err)
	}
	if len(payload.RoleSets) == 0 {
		t.Fatalf("expected role sets, got %#v", payload.RoleSets)
	}
	first := payload.RoleSets[0]
	if first.ID != "product_review" || first.Name == "" || len(first.TemplateIDs) < 3 {
		t.Fatalf("unexpected role set payload: %#v", first)
	}
}
