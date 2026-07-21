package api_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/api"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/tests/teststore"
)

func TestModelProfileManagementRoutesRequireAdmin(t *testing.T) {
	server, _, _ := newModelProfileAPIServer(t, nil)
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/model-profiles", ""},
		{http.MethodPost, "/api/model-profiles", `{}`},
		{http.MethodPut, "/api/model-profiles/profile-1", `{}`},
		{http.MethodPost, "/api/model-profiles/profile-1/default", ""},
		{http.MethodPost, "/api/model-profiles/profile-1/test", ""},
		{http.MethodPost, "/api/model-profiles/test", `{}`},
		{http.MethodDelete, "/api/model-profiles/profile-1", ""},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			response := httptest.NewRecorder()
			server.Routes().ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestModelProfileAPIAlwaysRedactsCredentials(t *testing.T) {
	server, _, backingStore := newModelProfileAPIServer(t, nil)
	secret := "sk-api-must-never-return"
	response := performAdminJSON(t, server, http.MethodPost, "/api/model-profiles", `{
		"name":"Primary","runtimeScope":"go","protocol":"openai_chat_completions",
		"baseURL":"https://provider.example/v1","modelName":"model-a","apiKey":"`+secret+`","enabled":true
	}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("create profile: %d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), secret) || strings.Contains(response.Body.String(), "apiKeyCiphertext") || strings.Contains(response.Body.String(), "v1:") {
		t.Fatalf("create response leaked credential material: %s", response.Body.String())
	}
	if len(backingStore.ModelProfiles) != 1 || backingStore.ModelProfiles[0].APIKeyCiphertext == "" || strings.Contains(backingStore.ModelProfiles[0].APIKeyCiphertext, secret) {
		t.Fatalf("credential was not encrypted at rest: %+v", backingStore.ModelProfiles)
	}

	response = performAdminJSON(t, server, http.MethodGet, "/api/model-profiles", "")
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), secret) || strings.Contains(response.Body.String(), backingStore.ModelProfiles[0].APIKeyCiphertext) {
		t.Fatalf("list response leaked credential material: %d body=%s", response.Code, response.Body.String())
	}
}

func TestModelProfileAPIExplainsMissingEncryptionKey(t *testing.T) {
	backingStore := &teststore.Store{}
	agents := agent.PredefinedAgents()
	if err := backingStore.SeedAgents(context.Background(), agents); err != nil {
		t.Fatal(err)
	}
	agentService := service.NewAgentService(backingStore, agents).WithModelProfiles(backingStore)
	manager := room.NewManager(backingStore, agentService.ResolveForRoom)
	llmClient := activityStubLLM{response: "ok"}
	knowledge := service.NewKnowledgeService(backingStore)
	runner := agent.NewRunner(llmClient, backingStore).WithKnowledge(knowledge)
	roomService := service.NewRoomService(manager, agentService, knowledge, runner, service.NewFocusService(llmClient), backingStore)
	profiles := service.NewModelProfileService(backingStore, nil, nil)
	server := api.NewServerWithConfig(api.Dependencies{
		Queries: roomService.Queries(), Commands: roomService.Commands(), Access: roomService.Access(), ModelProfiles: profiles,
	}, api.Config{AdminAPIKey: "admin-secret"})

	response := performAdminJSON(t, server, http.MethodPost, "/api/model-profiles", `{
		"name":"Primary","runtimeScope":"go","protocol":"openai_chat_completions",
		"baseURL":"https://provider.example/v1","modelName":"model-a","apiKey":"sk-test","enabled":true
	}`)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "MODEL_CONFIG_ENCRYPTION_KEY") {
		t.Fatalf("expected actionable encryption configuration error, got %d body=%s", response.Code, response.Body.String())
	}
}

func TestAgentAPIValidatesAndPersistsModelProfileBinding(t *testing.T) {
	server, profiles, backingStore := newModelProfileAPIServer(t, nil)
	goProfile := createAPIProfile(t, profiles, "Go", model.ModelRuntimeGo, true, true)
	deepProfile := createAPIProfile(t, profiles, "Deep", model.ModelRuntimeDeepAgent, true, true)

	rejected := performAdminJSON(t, server, http.MethodPost, "/api/agents", `{
		"name":"Wrong Scope","runtime":"llm","modelProfileID":"`+deepProfile.ID+`","enabled":true
	}`)
	if rejected.Code != http.StatusBadRequest || !strings.Contains(rejected.Body.String(), "incompatible") {
		t.Fatalf("expected incompatible binding 400, got %d body=%s", rejected.Code, rejected.Body.String())
	}

	accepted := performAdminJSON(t, server, http.MethodPost, "/api/agents", `{
		"name":"Bound Go","runtime":"llm","modelProfileID":"`+goProfile.ID+`","enabled":true
	}`)
	if accepted.Code != http.StatusCreated {
		t.Fatalf("expected compatible binding 201, got %d body=%s", accepted.Code, accepted.Body.String())
	}
	var payload model.AgentConfig
	if err := json.Unmarshal(accepted.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ModelProfileID != goProfile.ID || backingStore.Agents[len(backingStore.Agents)-1].ModelProfileID != goProfile.ID {
		t.Fatalf("binding was not persisted: response=%+v stored=%+v", payload, backingStore.Agents[len(backingStore.Agents)-1])
	}
}

func TestModelProfileDeleteConflictReturns409(t *testing.T) {
	server, profiles, backingStore := newModelProfileAPIServer(t, nil)
	profile := createAPIProfile(t, profiles, "Referenced", model.ModelRuntimeGo, false, false)
	backingStore.RoomAgents = map[string][]model.Agent{"room-1": {{ID: "a", ModelProfileID: profile.ID}}}

	response := performAdminJSON(t, server, http.MethodDelete, "/api/model-profiles/"+profile.ID, "")
	if response.Code != http.StatusConflict {
		t.Fatalf("expected delete conflict 409, got %d body=%s", response.Code, response.Body.String())
	}
}

func TestDraftConnectionAPIUsesBackendRequestAndReturnsSanitizedError(t *testing.T) {
	secret := "draft-api-secret"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+secret {
			t.Errorf("draft key was not sent in bearer auth")
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(secret))
	}))
	defer upstream.Close()
	server, _, _ := newModelProfileAPIServer(t, upstream.Client())

	response := performAdminJSON(t, server, http.MethodPost, "/api/model-profiles/test", `{
		"baseURL":"`+upstream.URL+`","modelName":"draft-model","apiKey":"`+secret+`"
	}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "authentication failed") || strings.Contains(response.Body.String(), secret) {
		t.Fatalf("unexpected draft test response: %d body=%s", response.Code, response.Body.String())
	}
}

func newModelProfileAPIServer(t *testing.T, client *http.Client) (*api.Server, *service.ModelProfileService, *teststore.Store) {
	t.Helper()
	backingStore := &teststore.Store{}
	agents := agent.PredefinedAgents()
	if err := backingStore.SeedAgents(context.Background(), agents); err != nil {
		t.Fatal(err)
	}
	agentService := service.NewAgentService(backingStore, agents).WithModelProfiles(backingStore)
	manager := room.NewManager(backingStore, agentService.ResolveForRoom)
	llmClient := activityStubLLM{response: "ok"}
	knowledge := service.NewKnowledgeService(backingStore)
	runner := agent.NewRunner(llmClient, backingStore).WithKnowledge(knowledge)
	roomService := service.NewRoomService(manager, agentService, knowledge, runner, service.NewFocusService(llmClient), backingStore)
	cipher, err := service.NewSecretCipher(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	profiles := service.NewModelProfileService(backingStore, cipher, client)
	server := api.NewServerWithConfig(api.Dependencies{
		Queries: roomService.Queries(), Commands: roomService.Commands(), Access: roomService.Access(), ModelProfiles: profiles,
	}, api.Config{AdminAPIKey: "admin-secret"})
	return server, profiles, backingStore
}

func createAPIProfile(t *testing.T, profiles *service.ModelProfileService, name, scope string, enabled, isDefault bool) model.ModelProfile {
	t.Helper()
	profile, err := profiles.Create(context.Background(), service.CreateModelProfileInput{
		Name: name, RuntimeScope: scope, Protocol: model.ModelProtocolOpenAIChatCompletions,
		BaseURL: "https://provider.example", ModelName: "model-" + strings.ToLower(name), Enabled: enabled, IsDefault: isDefault,
	})
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func performAdminJSON(t *testing.T, server *api.Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Admin-Key", "admin-secret")
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	return response
}
