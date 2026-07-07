package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agentroom/backend/internal/api"
	"agentroom/backend/internal/model"
)

func TestAgentsEndpointReturnsRuntime(t *testing.T) {
	server, _, _ := newActivityTestServer(t, api.Config{})

	request := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected agents response 200, got %d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Agents []struct {
			ID      string `json:"id"`
			Runtime string `json:"runtime"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode agents response: %v", err)
	}
	if len(payload.Agents) == 0 || payload.Agents[0].Runtime != model.AgentRuntimeLLM {
		t.Fatalf("expected default llm runtime in agents response, got %#v", payload.Agents)
	}
}

func TestCreateAgentAcceptsRuntime(t *testing.T) {
	server, _, _ := newActivityTestServer(t, api.Config{AdminAPIKey: "secret"})

	request := httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewBufferString(`{"name":"Deep Runtime Tester","role":"Researcher","description":"Finds sources.","runtime":"deepagent","enabled":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Admin-Key", "secret")
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected create response 201, got %d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Runtime string `json:"runtime"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if payload.Runtime != model.AgentRuntimeDeepAgent {
		t.Fatalf("expected deepagent runtime, got %#v", payload)
	}
}

func TestCreateAgentRejectsInvalidRuntime(t *testing.T) {
	server, _, _ := newActivityTestServer(t, api.Config{AdminAPIKey: "secret"})

	request := httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewBufferString(`{"name":"Broken","runtime":"unknown"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Admin-Key", "secret")
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid runtime 400, got %d body=%s", response.Code, response.Body.String())
	}
}
