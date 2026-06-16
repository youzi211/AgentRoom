package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/api"
	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/tests/teststore"
)

func TestAdminMutationsRequireAdminKeyWhenConfigured(t *testing.T) {
	server := newTestServer(t, api.Config{AdminAPIKey: "secret"})

	requestBody := bytes.NewBufferString(`{"name":"Reviewer","role":"Review","description":"Reviews work","systemPrompt":"Be concise"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/agents", requestBody)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected missing admin key to return 401, got %d body=%s", response.Code, response.Body.String())
	}

	requestBody = bytes.NewBufferString(`{"name":"Reviewer","role":"Review","description":"Reviews work","systemPrompt":"Be concise"}`)
	request = httptest.NewRequest(http.MethodPost, "/api/agents", requestBody)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Admin-Key", "secret")
	response = httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected valid admin key to create agent, got %d body=%s", response.Code, response.Body.String())
	}
}

func TestKnowledgeMutationsRequireAdminKeyWhenConfigured(t *testing.T) {
	server := newTestServer(t, api.Config{AdminAPIKey: "secret"})

	createRoomBody := bytes.NewBufferString(`{"name":"Protected knowledge room"}`)
	createRoomRequest := httptest.NewRequest(http.MethodPost, "/api/rooms", createRoomBody)
	createRoomRequest.Header.Set("Content-Type", "application/json")
	createRoomResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(createRoomResponse, createRoomRequest)
	if createRoomResponse.Code != http.StatusCreated {
		t.Fatalf("create room failed: %d body=%s", createRoomResponse.Code, createRoomResponse.Body.String())
	}

	var createdRoom struct {
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
	}
	if err := json.Unmarshal(createRoomResponse.Body.Bytes(), &createdRoom); err != nil {
		t.Fatalf("decode create room response: %v", err)
	}

	agentsResponse := httptest.NewRecorder()
	agentsRequest := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	server.Routes().ServeHTTP(agentsResponse, agentsRequest)
	if agentsResponse.Code != http.StatusOK {
		t.Fatalf("list agents failed: %d body=%s", agentsResponse.Code, agentsResponse.Body.String())
	}

	var agentsPayload struct {
		Agents []struct {
			ID string `json:"id"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(agentsResponse.Body.Bytes(), &agentsPayload); err != nil {
		t.Fatalf("decode agents response: %v", err)
	}
	if len(agentsPayload.Agents) == 0 {
		t.Fatal("expected seeded agents")
	}
	agentID := agentsPayload.Agents[0].ID

	t.Run("agent knowledge upload", func(t *testing.T) {
		request := multipartRequest(t, http.MethodPost, "/api/agents/"+agentID+"/knowledge", "notes.md", "# agent notes")
		response := httptest.NewRecorder()
		server.Routes().ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("expected missing admin key to return 401, got %d body=%s", response.Code, response.Body.String())
		}

		request = multipartRequest(t, http.MethodPost, "/api/agents/"+agentID+"/knowledge", "notes.md", "# agent notes")
		request.Header.Set("X-Admin-Key", "secret")
		response = httptest.NewRecorder()
		server.Routes().ServeHTTP(response, request)
		if response.Code != http.StatusCreated {
			t.Fatalf("expected valid admin key to upload agent knowledge, got %d body=%s", response.Code, response.Body.String())
		}
	})

	var createdDocument struct {
		Document struct {
			ID string `json:"id"`
		} `json:"document"`
	}

	t.Run("room knowledge upload and delete", func(t *testing.T) {
		request := multipartRequest(t, http.MethodPost, "/api/rooms/"+createdRoom.Room.ID+"/knowledge", "room-notes.md", "# room notes")
		response := httptest.NewRecorder()
		server.Routes().ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("expected missing admin key to return 401, got %d body=%s", response.Code, response.Body.String())
		}

		request = multipartRequest(t, http.MethodPost, "/api/rooms/"+createdRoom.Room.ID+"/knowledge", "room-notes.md", "# room notes")
		request.Header.Set("X-Admin-Key", "secret")
		response = httptest.NewRecorder()
		server.Routes().ServeHTTP(response, request)
		if response.Code != http.StatusCreated {
			t.Fatalf("expected valid admin key to upload room knowledge, got %d body=%s", response.Code, response.Body.String())
		}
		if err := json.Unmarshal(response.Body.Bytes(), &createdDocument); err != nil {
			t.Fatalf("decode room knowledge response: %v", err)
		}
		if createdDocument.Document.ID == "" {
			t.Fatal("expected created knowledge document id")
		}

		deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/knowledge/"+createdDocument.Document.ID, nil)
		deleteResponse := httptest.NewRecorder()
		server.Routes().ServeHTTP(deleteResponse, deleteRequest)
		if deleteResponse.Code != http.StatusUnauthorized {
			t.Fatalf("expected delete without admin key to return 401, got %d body=%s", deleteResponse.Code, deleteResponse.Body.String())
		}

		deleteRequest = httptest.NewRequest(http.MethodDelete, "/api/knowledge/"+createdDocument.Document.ID, nil)
		deleteRequest.Header.Set("X-Admin-Key", "secret")
		deleteResponse = httptest.NewRecorder()
		server.Routes().ServeHTTP(deleteResponse, deleteRequest)
		if deleteResponse.Code != http.StatusOK {
			t.Fatalf("expected delete with admin key to return 200, got %d body=%s", deleteResponse.Code, deleteResponse.Body.String())
		}
	})
}

func TestWebSocketOriginIsRejectedOutsideAllowlist(t *testing.T) {
	server := newTestServer(t, api.Config{AllowedOrigins: []string{"https://agentroom.example.com"}})

	if server.AllowsOriginForTest("https://agentroom.example.com") != true {
		t.Fatal("expected configured origin to be allowed")
	}
	if server.AllowsOriginForTest("") != true {
		t.Fatal("expected blank origin to stay allowed for non-browser clients")
	}
	if server.AllowsOriginForTest("https://evil.example.com") != false {
		t.Fatal("expected unconfigured origin to be rejected")
	}
}

func TestRoomPasscodeIsRequiredForProtectedRoomAccess(t *testing.T) {
	server := newTestServer(t, api.Config{})

	createBody := bytes.NewBufferString(`{"name":"Protected","passcode":"open-sesame"}`)
	createRequest := httptest.NewRequest(http.MethodPost, "/api/rooms", createBody)
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create room failed: %d body=%s", createResponse.Code, createResponse.Body.String())
	}

	var created struct {
		Room struct {
			ID          string `json:"id"`
			HasPasscode bool   `json:"hasPasscode"`
		} `json:"room"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if !created.Room.HasPasscode {
		t.Fatal("expected created room response to reveal that passcode is enabled")
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/api/rooms/"+created.Room.ID, nil)
	getResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusForbidden {
		t.Fatalf("expected missing passcode to return 403, got %d body=%s", getResponse.Code, getResponse.Body.String())
	}

	getRequest = httptest.NewRequest(http.MethodGet, "/api/rooms/"+created.Room.ID+"?passcode=open-sesame", nil)
	getResponse = httptest.NewRecorder()
	server.Routes().ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("expected correct passcode to return 200, got %d body=%s", getResponse.Code, getResponse.Body.String())
	}
}

func TestOpenRoomAccessDoesNotRequirePasscode(t *testing.T) {
	server := newTestServer(t, api.Config{})

	createBody := bytes.NewBufferString(`{"name":"Open room"}`)
	createRequest := httptest.NewRequest(http.MethodPost, "/api/rooms", createBody)
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create room failed: %d body=%s", createResponse.Code, createResponse.Body.String())
	}

	var created struct {
		Room struct {
			ID          string `json:"id"`
			HasPasscode bool   `json:"hasPasscode"`
		} `json:"room"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Room.HasPasscode {
		t.Fatal("expected open room to report no passcode")
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/api/rooms/"+created.Room.ID, nil)
	getResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("expected open room metadata without passcode to return 200, got %d body=%s", getResponse.Code, getResponse.Body.String())
	}

	messagesRequest := httptest.NewRequest(http.MethodGet, "/api/rooms/"+created.Room.ID+"/messages", nil)
	messagesResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(messagesResponse, messagesRequest)
	if messagesResponse.Code != http.StatusOK {
		t.Fatalf("expected open room messages without passcode to return 200, got %d body=%s", messagesResponse.Code, messagesResponse.Body.String())
	}
}

func TestCreateRoomPersistsDialoguePolicy(t *testing.T) {
	server := newTestServer(t, api.Config{})

	createBody := bytes.NewBufferString(`{
		"name":"Guided room",
		"dialoguePolicy":{
			"mode":"guided_dialogue",
			"maxAutonomousTurns":4,
			"maxTurnsPerAgent":2,
			"allowSelfFollowup":false,
			"allowAgentToAgentMentions":true,
			"responseStrategy":"mentioned_first",
			"cooldownMs":25
		}
	}`)
	createRequest := httptest.NewRequest(http.MethodPost, "/api/rooms", createBody)
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create room failed: %d body=%s", createResponse.Code, createResponse.Body.String())
	}

	var created struct {
		Room struct {
			ID             string               `json:"id"`
			DialoguePolicy model.DialoguePolicy `json:"dialoguePolicy"`
		} `json:"room"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Room.DialoguePolicy.Mode != model.DialogueModeGuided {
		t.Fatalf("expected created room to return guided dialogue mode, got %#v", created.Room.DialoguePolicy)
	}
	if created.Room.DialoguePolicy.MaxAutonomousTurns != 4 || created.Room.DialoguePolicy.CooldownMS != 25 {
		t.Fatalf("expected custom dialogue policy values to round-trip, got %#v", created.Room.DialoguePolicy)
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/api/rooms/"+created.Room.ID, nil)
	getResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get room failed: %d body=%s", getResponse.Code, getResponse.Body.String())
	}

	var fetched struct {
		Room struct {
			DialoguePolicy model.DialoguePolicy `json:"dialoguePolicy"`
		} `json:"room"`
	}
	if err := json.Unmarshal(getResponse.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if fetched.Room.DialoguePolicy.Mode != model.DialogueModeGuided {
		t.Fatalf("expected stored room to keep guided dialogue policy, got %#v", fetched.Room.DialoguePolicy)
	}
}

func TestMeetingMinutesCanBeGeneratedAsMarkdown(t *testing.T) {
	server := newTestServer(t, api.Config{})

	createBody := bytes.NewBufferString(`{"name":"Planning"}`)
	createRequest := httptest.NewRequest(http.MethodPost, "/api/rooms", createBody)
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create room failed: %d body=%s", createResponse.Code, createResponse.Body.String())
	}

	var created struct {
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	roomValue, ok := server.RoomsForTest().GetRoom(context.Background(), created.Room.ID)
	if !ok {
		t.Fatal("created room not found")
	}
	participant := roomValue.NewParticipant("Alice")
	if _, _, err := server.RoomsForTest().HandleHumanMessage(context.Background(), roomValue, participant, "We decided to launch v0.2 after auth is finished."); err != nil {
		t.Fatalf("add message: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/rooms/"+created.Room.ID+"/minutes", nil)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected minutes response 200, got %d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "## Meeting Minutes") {
		t.Fatalf("expected markdown minutes, got %s", response.Body.String())
	}
}

func TestMeetingMinutesMarkdownCanBeDownloaded(t *testing.T) {
	server := newTestServer(t, api.Config{})

	createBody := bytes.NewBufferString(`{"name":"Planning"}`)
	createRequest := httptest.NewRequest(http.MethodPost, "/api/rooms", createBody)
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create room failed: %d body=%s", createResponse.Code, createResponse.Body.String())
	}

	var created struct {
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	roomValue, ok := server.RoomsForTest().GetRoom(context.Background(), created.Room.ID)
	if !ok {
		t.Fatal("created room not found")
	}
	participant := roomValue.NewParticipant("Alice")
	if _, _, err := server.RoomsForTest().HandleHumanMessage(context.Background(), roomValue, participant, "Please export these meeting notes."); err != nil {
		t.Fatalf("add message: %v", err)
	}

	generateRequest := httptest.NewRequest(http.MethodPost, "/api/rooms/"+created.Room.ID+"/minutes", nil)
	generateResponse := httptest.NewRecorder()
	server.Routes().ServeHTTP(generateResponse, generateRequest)
	if generateResponse.Code != http.StatusOK {
		t.Fatalf("generate minutes failed: %d body=%s", generateResponse.Code, generateResponse.Body.String())
	}

	request := httptest.NewRequest(http.MethodGet, "/api/rooms/"+created.Room.ID+"/minutes.md", nil)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected markdown download response 200, got %d body=%s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); !strings.Contains(contentType, "text/markdown") {
		t.Fatalf("expected markdown content type, got %q", contentType)
	}
	if contentDisposition := response.Header().Get("Content-Disposition"); !strings.Contains(contentDisposition, "minutes.md") {
		t.Fatalf("expected attachment filename to end with minutes.md, got %q", contentDisposition)
	}
	if !strings.Contains(response.Body.String(), "## Meeting Minutes") {
		t.Fatalf("expected markdown minutes body, got %s", response.Body.String())
	}
}

func newTestServer(t *testing.T, config api.Config) *api.Server {
	t.Helper()

	store := &teststore.Store{}
	agents := agent.PredefinedAgents()
	if err := store.SeedAgents(context.Background(), agents); err != nil {
		t.Fatalf("seed agents: %v", err)
	}

	agentService := service.NewAgentService(store, agents)
	knowledgeService := service.NewKnowledgeService(store)
	manager := room.NewManager(store, agentService.ResolveForRoom)
	llmClient := stubLLM{response: "## Meeting Minutes\n\n- Decision: launch v0.2 after auth is finished."}
	runner := agent.NewRunner(llmClient, store).WithKnowledge(knowledgeService)
	focusService := service.NewFocusService(llmClient)
	roomService := service.NewRoomService(manager, agentService, knowledgeService, runner, focusService, store)
	return api.NewServerWithConfig(roomService, config)
}

type stubLLM struct {
	response string
}

func (s stubLLM) Complete(context.Context, []llm.ChatMessage) (string, error) {
	return s.response, nil
}

func multipartRequest(t *testing.T, method string, target string, fileName string, content string) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := file.Write([]byte(content)); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(method, target, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}
