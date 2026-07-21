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
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/tests/teststore"
)

type readinessProbe struct{ err error }

func (probe readinessProbe) Ready(context.Context) error { return probe.err }

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
