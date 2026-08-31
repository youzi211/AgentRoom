package api_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/api"
	"agentroom/backend/internal/collaboration"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/tests/teststore"
)

type readinessProbe struct{ err error }

func (probe readinessProbe) Ready(context.Context) error { return probe.err }

type capabilityProbe struct {
	capabilities collaboration.RuntimeCapabilities
	err          error
}

func (probe capabilityProbe) Capabilities(context.Context) (collaboration.RuntimeCapabilities, error) {
	return probe.capabilities, probe.err
}

func TestHealthIsCoreLivenessAndReadyReportsDependencies(t *testing.T) {
	store := &teststore.Store{PingErr: errors.New("database unavailable")}
	agents := agent.PredefinedAgents()
	agentService := service.NewAgentService(store, agents)
	knowledgeService := service.NewKnowledgeService(store)
	manager := room.NewManager(store, agentService.ResolveForRoom)
	llmClient := stubLLM{response: "unused"}
	runner := agent.NewRunner(llmClient, store)
	roomService := service.NewRoomService(
		manager, agentService, knowledgeService, runner,
		service.NewFocusService(llmClient), store,
	)
	server := api.NewServer(api.Dependencies{
		Queries: roomService.Queries(), Commands: roomService.Commands(), Access: roomService.Access(),
		AgentRuntime: readinessProbe{err: errors.New("runtime unavailable")},
	})

	health := httptest.NewRecorder()
	server.Routes().ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if health.Code != http.StatusOK || !strings.Contains(health.Body.String(), `"ok":true`) {
		t.Fatalf("core liveness must remain healthy: %d %s", health.Code, health.Body.String())
	}

	ready := httptest.NewRecorder()
	server.Routes().ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/api/ready", nil))
	if ready.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected dependency readiness failure, got %d %s", ready.Code, ready.Body.String())
	}
	for _, expected := range []string{`"database":{"ok":false}`, `"agentRuntime":{"ok":false}`} {
		if !strings.Contains(ready.Body.String(), expected) {
			t.Fatalf("missing readiness detail %s in %s", expected, ready.Body.String())
		}
	}
}

func TestReadyTreatsLocalRuntimeAsReady(t *testing.T) {
	server := newTestServer(t, api.Config{})
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/ready", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"agentRuntime":{"ok":true}`) {
		t.Fatalf("unexpected local readiness: %d %s", response.Code, response.Body.String())
	}
}

func TestAdminCollaborationRuntimeCapabilitiesComeFromProvider(t *testing.T) {
	server := newRuntimeCapabilitiesServer(t, api.Config{
		AdminAPIKey: "secret", CollaborationRuntimeMode: "remote",
	}, capabilityProbe{capabilities: collaboration.RuntimeCapabilities{
		Ready:                     true,
		SupportedProtocolVersions: []string{"v1", "v2-test"},
		Engines: []collaboration.EngineCapability{{
			Engine: collaboration.EngineAutoGen, Version: "test-runtime-v9", Enabled: true, Ready: true,
		}},
		SupportedTriggerModes: []collaboration.TriggerMode{collaboration.TriggerAutomatic},
	}})

	unauthorized := httptest.NewRecorder()
	server.Routes().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/admin/collaboration-runtime", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected admin authentication, got %d %s", unauthorized.Code, unauthorized.Body.String())
	}

	request := httptest.NewRequest(http.MethodGet, "/api/admin/collaboration-runtime", nil)
	request.Header.Set("X-Admin-Key", "secret")
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected capabilities status: %d %s", response.Code, response.Body.String())
	}
	for _, expected := range []string{
		`"mode":"remote"`, `"ready":true`, `"v2-test"`, `"engine":"autogen"`,
		`"version":"test-runtime-v9"`, `"supportedTriggerModes":["automatic"]`,
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("missing provider capability %s in %s", expected, response.Body.String())
		}
	}
}

func TestAdminCollaborationRuntimeReportsRemoteUnavailableWithoutLeakingError(t *testing.T) {
	server := newRuntimeCapabilitiesServer(t, api.Config{CollaborationRuntimeMode: "remote"}, capabilityProbe{
		err: errors.New("Authorization: secret-runtime-token"),
	})
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/admin/collaboration-runtime", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected capabilities status: %d %s", response.Code, response.Body.String())
	}
	for _, expected := range []string{`"ready":false`, `"engines":[]`, `"supportedTriggerModes":[]`} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("missing unavailable state %s in %s", expected, response.Body.String())
		}
	}
	if strings.Contains(strings.ToLower(response.Body.String()), "secret") || strings.Contains(strings.ToLower(response.Body.String()), "authorization") {
		t.Fatalf("runtime error leaked in response: %s", response.Body.String())
	}
}

func TestAdminCollaborationRuntimeReportsLegacyCompatibility(t *testing.T) {
	server := newTestServer(t, api.Config{})
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/admin/collaboration-runtime", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected capabilities status: %d %s", response.Code, response.Body.String())
	}
	for _, expected := range []string{
		`"mode":"legacy"`, `"ready":true`, `"engine":"native"`,
		`"supportedTriggerModes":["mention_only"]`,
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("missing legacy capability %s in %s", expected, response.Body.String())
		}
	}
}

func newRuntimeCapabilitiesServer(t *testing.T, config api.Config, provider collaboration.CapabilityProvider) *api.Server {
	t.Helper()
	store := &teststore.Store{}
	agents := agent.PredefinedAgents()
	agentService := service.NewAgentService(store, agents)
	knowledgeService := service.NewKnowledgeService(store)
	manager := room.NewManager(store, agentService.ResolveForRoom)
	llmClient := stubLLM{response: "unused"}
	roomService := service.NewRoomService(
		manager, agentService, knowledgeService, agent.NewRunner(llmClient, store),
		service.NewFocusService(llmClient), store,
	)
	return api.NewServerWithConfig(api.Dependencies{
		Queries: roomService.Queries(), Commands: roomService.Commands(), Access: roomService.Access(),
		CollaborationRuntime: provider,
	}, config)
}
