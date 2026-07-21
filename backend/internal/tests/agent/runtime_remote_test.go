package agent_test

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"agentroom/backend/internal/agent"
	agentruntimev1 "agentroom/backend/internal/agentproto/v1"
	"agentroom/backend/internal/config"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/realtime"
	"agentroom/backend/internal/room"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeRuntimeServer struct {
	agentruntimev1.UnimplementedAgentRuntimeServiceServer
	handler func(*agentruntimev1.ExecuteAgentRequest, grpc.ServerStreamingServer[agentruntimev1.AgentEvent]) error
}

func TestRemoteRuntimeReadinessUsesStandardHealthService(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	client, err := agent.NewRemoteRuntimeClient(config.AgentRuntimeConfig{
		Transport: config.AgentRuntimeTransportGRPC, GRPCAddress: listener.Addr().String(), GRPCInsecure: true,
		LLMTimeout: time.Second, DeepTimeout: time.Second, MaxRequestBytes: 1024, MaxEventBytes: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	healthServer.SetServingStatus("agentroom.runtime.v1.AgentRuntimeService", grpc_health_v1.HealthCheckResponse_SERVING)
	if err := client.Ready(context.Background()); err != nil {
		t.Fatalf("expected serving runtime: %v", err)
	}
	healthServer.SetServingStatus("agentroom.runtime.v1.AgentRuntimeService", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	if err := client.Ready(context.Background()); err == nil {
		t.Fatal("expected not-serving runtime readiness failure")
	}
}

func (s fakeRuntimeServer) ExecuteAgent(request *agentruntimev1.ExecuteAgentRequest, stream grpc.ServerStreamingServer[agentruntimev1.AgentEvent]) error {
	return s.handler(request, stream)
}

type remoteTestRoom struct{ meta model.RoomMeta }

func (r remoteTestRoom) Info() model.RoomMeta                            { return r.meta }
func (remoteTestRoom) Participants() []model.Participant                 { return nil }
func (remoteTestRoom) Agents() []model.Agent                             { return nil }
func (remoteTestRoom) AgentsWithPrompts() []model.Agent                  { return nil }
func (remoteTestRoom) RecentMessages(int) []model.Message                { return nil }
func (remoteTestRoom) NewSystemMessage(string) model.Message             { return model.Message{} }
func (remoteTestRoom) NewAgentMessage(model.Agent, string) model.Message { return model.Message{} }
func (remoteTestRoom) AppendMessage(model.Message)                       {}
func (remoteTestRoom) Broadcaster() room.MessageBroadcaster              { return remoteNoopBroadcaster{} }

type remoteNoopBroadcaster struct{}

func (remoteNoopBroadcaster) BroadcastMessage(model.Message) {}
func (remoteNoopBroadcaster) BroadcastEvent(realtime.Event)  {}

type remoteModelResolver struct{ resolved model.ResolvedModelConfig }

func (r remoteModelResolver) Resolve(context.Context, string, string) (model.ResolvedModelConfig, error) {
	return r.resolved, nil
}

func startRemoteRuntime(
	t *testing.T,
	handler func(*agentruntimev1.ExecuteAgentRequest, grpc.ServerStreamingServer[agentruntimev1.AgentEvent]) error,
	timeout time.Duration,
	maxEventBytes int,
) (*agent.RemoteRuntimeClient, *agent.RemotePythonRuntime) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	agentruntimev1.RegisterAgentRuntimeServiceServer(server, fakeRuntimeServer{handler: handler})
	go func() { _ = server.Serve(listener) }()

	runtimeConfig := config.AgentRuntimeConfig{
		Transport: config.AgentRuntimeTransportGRPC, GRPCAddress: listener.Addr().String(),
		GRPCInsecure: true, LLMTimeout: timeout, DeepTimeout: time.Second,
		MaxRequestBytes: 1024 * 1024, MaxEventBytes: maxEventBytes,
	}
	client, err := agent.NewRemoteRuntimeClient(runtimeConfig)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := agent.NewRemotePythonRuntime(
		model.AgentRuntimeLLM,
		client,
		timeout,
		remoteModelResolver{resolved: model.ResolvedModelConfig{
			ProfileID: "profile_1", Source: "database", BaseURL: "https://model.invalid/v1",
			ModelName: "test-model", APIKey: "secret",
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = client.Close()
		server.Stop()
		_ = listener.Close()
	})
	return client, runtime
}

func remoteRequest(runID string) agent.AgentRuntimeRequest {
	createdAt := time.Now().UTC()
	return agent.AgentRuntimeRequest{
		RunID: runID, TraceID: "trace_1",
		Room: remoteTestRoom{meta: model.RoomMeta{
			ID: "room_1", Name: "Planning",
			DialoguePolicy: model.DialoguePolicy{Mode: model.DialogueModeGuided},
		}},
		Agent: model.Agent{
			ID: "agent_1", Name: "Planner", Mention: "@Planner", Role: "planner",
			SystemPrompt: "Plan safely", Runtime: model.AgentRuntimeLLM, ModelProfileID: "profile_1",
		},
		Trigger: model.Message{
			ID: "message_1", SenderID: "human_1", SenderName: "Alice",
			SenderType: model.SenderTypeHuman, Content: "Make a plan", CreatedAt: createdAt,
			DialogueRunID: "dialogue_1",
		},
		RecentMessages: []model.Message{{
			ID: "message_0", SenderType: model.SenderTypeSystem, Content: "Context", CreatedAt: createdAt,
		}},
		KnowledgeChunks: []model.KnowledgeChunk{{
			ID: "chunk_1", DocumentID: "document_1", DocumentName: "Plan.md",
			Scope: model.KnowledgeScopeRoom, ScopeID: "room_1", ChunkIndex: 2, Content: "Known fact",
		}},
		PromptContext: agent.PromptContext{
			RoomName: "Planning", DialogueMode: model.DialogueModeGuided,
			OnlineHumanParticipants: []model.Participant{{ID: "human_1", Name: "Alice"}},
			RoomAgents:              []model.Agent{{ID: "agent_1", Name: "Planner", Mention: "@Planner"}},
			TriggerSender:           "Alice", TriggerSenderType: model.SenderTypeHuman,
			TriggerContent: "Make a plan", LatestVisibleSpeaker: "Alice",
			LatestVisibleSpeakerType: model.SenderTypeHuman,
			RootHumanTriggerSender:   "Alice", RootHumanTriggerType: model.SenderTypeHuman,
			RootHumanTriggerContent: "Make a plan", AutonomousTurnIndex: 1,
			MaxAutonomousTurns: 3, AllowAgentToAgentMentions: true, MaxTurnsPerAgent: 1,
			EligiblePeers: []model.Agent{{ID: "reviewer", Name: "Reviewer", Mention: "@Reviewer"}},
		},
	}
}

func runtimeEvent(runID string, sequence uint64, payload any) *agentruntimev1.AgentEvent {
	event := &agentruntimev1.AgentEvent{
		ProtocolVersion: agentruntimev1.ProtocolVersion,
		RunId:           runID,
		Sequence:        sequence,
		OccurredAt:      timestamppb.Now(),
	}
	switch typed := payload.(type) {
	case *agentruntimev1.AgentEvent_Accepted:
		event.Payload = typed
	case *agentruntimev1.AgentEvent_ModelStarted:
		event.Payload = typed
	case *agentruntimev1.AgentEvent_ModelCompleted:
		event.Payload = typed
	case *agentruntimev1.AgentEvent_ToolStarted:
		event.Payload = typed
	case *agentruntimev1.AgentEvent_ToolCompleted:
		event.Payload = typed
	case *agentruntimev1.AgentEvent_ToolFailed:
		event.Payload = typed
	case *agentruntimev1.AgentEvent_OutputDelta:
		event.Payload = typed
	case *agentruntimev1.AgentEvent_ArtifactReady:
		event.Payload = typed
	case *agentruntimev1.AgentEvent_Completed:
		event.Payload = typed
	case *agentruntimev1.AgentEvent_Failed:
		event.Payload = typed
	default:
		panic("unsupported test event payload")
	}
	return event
}

func TestRemotePythonRuntimeStreamsSuccessAndMapsRequest(t *testing.T) {
	captured := make(chan *agentruntimev1.ExecuteAgentRequest, 1)
	_, runtime := startRemoteRuntime(t, func(request *agentruntimev1.ExecuteAgentRequest, stream grpc.ServerStreamingServer[agentruntimev1.AgentEvent]) error {
		captured <- request
		events := []*agentruntimev1.AgentEvent{
			runtimeEvent(request.GetRunId(), 1, &agentruntimev1.AgentEvent_Accepted{Accepted: &agentruntimev1.AcceptedEvent{}}),
			runtimeEvent(request.GetRunId(), 2, &agentruntimev1.AgentEvent_ModelStarted{ModelStarted: &agentruntimev1.ModelStartedEvent{ModelName: "test-model"}}),
			runtimeEvent(request.GetRunId(), 3, &agentruntimev1.AgentEvent_ArtifactReady{ArtifactReady: &agentruntimev1.ArtifactReadyEvent{Artifact: &agentruntimev1.Artifact{Id: "report", FileName: "report.md", Content: []byte("report")}}}),
			runtimeEvent(request.GetRunId(), 4, &agentruntimev1.AgentEvent_Completed{Completed: &agentruntimev1.CompletedEvent{
				Content: "done", Model: &agentruntimev1.ModelAudit{ProfileId: "profile_1", Source: "database", ModelName: "test-model"},
			}}),
		}
		for _, event := range events {
			if err := stream.Send(event); err != nil {
				return err
			}
		}
		return nil
	}, time.Second, 1024*1024)

	var observed []string
	response, err := runtime.Respond(context.Background(), remoteRequest("run_success"), agent.AgentEventObserverFunc(func(_ context.Context, event agent.AgentRuntimeEvent) {
		observed = append(observed, event.Kind)
	}))
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != "done" || len(response.Artifacts) != 1 || response.Artifacts[0].Content != "report" {
		t.Fatalf("unexpected response: %#v", response)
	}
	if got := response.Metadata["model_name"]; got != "test-model" {
		t.Fatalf("expected model audit, got %#v", response.Metadata)
	}
	if len(observed) != 4 || observed[0] != "accepted" || observed[3] != "completed" {
		t.Fatalf("unexpected observed events: %#v", observed)
	}

	mapped := <-captured
	if mapped.GetTraceId() != "trace_1" || mapped.GetDialogueRunId() != "dialogue_1" || mapped.GetRoom().GetDialogueMode() != model.DialogueModeGuided {
		t.Fatalf("request context was not mapped: %#v", mapped)
	}
	if mapped.GetModel().GetApiKey() != "secret" || mapped.GetModel().GetProfileId() != "profile_1" {
		t.Fatalf("resolved model context was not mapped: %#v", mapped.GetModel())
	}
	if len(mapped.GetRecentMessages()) != 1 || len(mapped.GetKnowledgeChunks()) != 1 {
		t.Fatalf("history or knowledge was not mapped: %#v", mapped)
	}
	if mapped.GetPromptContext().GetRootHumanTriggerContent() != "Make a plan" || len(mapped.GetPromptContext().GetEligiblePeers()) != 1 {
		t.Fatalf("structured PromptContext was not mapped: %#v", mapped.GetPromptContext())
	}
}

func TestRemotePythonRuntimeReturnsApplicationFailure(t *testing.T) {
	_, runtime := startRemoteRuntime(t, func(request *agentruntimev1.ExecuteAgentRequest, stream grpc.ServerStreamingServer[agentruntimev1.AgentEvent]) error {
		_ = stream.Send(runtimeEvent(request.GetRunId(), 1, &agentruntimev1.AgentEvent_Accepted{Accepted: &agentruntimev1.AcceptedEvent{}}))
		return stream.Send(runtimeEvent(request.GetRunId(), 2, &agentruntimev1.AgentEvent_Failed{Failed: &agentruntimev1.FailedEvent{Failure: &agentruntimev1.RunFailure{Code: agentruntimev1.RunErrorCode_RUN_ERROR_CODE_TOOL_FAILED, Message: "tool failed"}}}))
	}, time.Second, 1024*1024)

	_, err := runtime.Respond(context.Background(), remoteRequest("run_failed"))
	if !errors.Is(err, agent.ErrRuntimeApplication) {
		t.Fatalf("expected application failure, got %v", err)
	}
}

func TestRemotePythonRuntimeRejectsInvalidStreams(t *testing.T) {
	tests := []struct {
		name    string
		handler func(*agentruntimev1.ExecuteAgentRequest, grpc.ServerStreamingServer[agentruntimev1.AgentEvent]) error
	}{
		{
			name: "EOF without terminal",
			handler: func(request *agentruntimev1.ExecuteAgentRequest, stream grpc.ServerStreamingServer[agentruntimev1.AgentEvent]) error {
				return stream.Send(runtimeEvent(request.GetRunId(), 1, &agentruntimev1.AgentEvent_Accepted{Accepted: &agentruntimev1.AcceptedEvent{}}))
			},
		},
		{
			name: "wrong run id",
			handler: func(_ *agentruntimev1.ExecuteAgentRequest, stream grpc.ServerStreamingServer[agentruntimev1.AgentEvent]) error {
				return stream.Send(runtimeEvent("wrong", 1, &agentruntimev1.AgentEvent_Accepted{Accepted: &agentruntimev1.AcceptedEvent{}}))
			},
		},
		{
			name: "out of order",
			handler: func(request *agentruntimev1.ExecuteAgentRequest, stream grpc.ServerStreamingServer[agentruntimev1.AgentEvent]) error {
				return stream.Send(runtimeEvent(request.GetRunId(), 2, &agentruntimev1.AgentEvent_Accepted{Accepted: &agentruntimev1.AcceptedEvent{}}))
			},
		},
		{
			name: "event after terminal",
			handler: func(request *agentruntimev1.ExecuteAgentRequest, stream grpc.ServerStreamingServer[agentruntimev1.AgentEvent]) error {
				_ = stream.Send(runtimeEvent(request.GetRunId(), 1, &agentruntimev1.AgentEvent_Accepted{Accepted: &agentruntimev1.AcceptedEvent{}}))
				_ = stream.Send(runtimeEvent(request.GetRunId(), 2, &agentruntimev1.AgentEvent_Completed{Completed: &agentruntimev1.CompletedEvent{Content: "done"}}))
				return stream.Send(runtimeEvent(request.GetRunId(), 3, &agentruntimev1.AgentEvent_Failed{Failed: &agentruntimev1.FailedEvent{}}))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, runtime := startRemoteRuntime(t, test.handler, time.Second, 1024*1024)
			_, err := runtime.Respond(context.Background(), remoteRequest("run_protocol"))
			if !errors.Is(err, agent.ErrRuntimeProtocol) {
				t.Fatalf("expected protocol error, got %v", err)
			}
		})
	}
}

func TestRemotePythonRuntimeMapsGRPCStatus(t *testing.T) {
	tests := []struct {
		name string
		code codes.Code
		want error
	}{
		{name: "capacity", code: codes.ResourceExhausted, want: agent.ErrRuntimeCapacity},
		{name: "unavailable", code: codes.Unavailable, want: agent.ErrRuntimeUnavailable},
		{name: "authentication", code: codes.Unauthenticated, want: agent.ErrRuntimeAuthentication},
		{name: "invalid", code: codes.InvalidArgument, want: agent.ErrRuntimeInvalidArgument},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, runtime := startRemoteRuntime(t, func(*agentruntimev1.ExecuteAgentRequest, grpc.ServerStreamingServer[agentruntimev1.AgentEvent]) error {
				return status.Error(test.code, test.name)
			}, time.Second, 1024*1024)
			_, err := runtime.Respond(context.Background(), remoteRequest("run_status"))
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func TestRemotePythonRuntimeDeadlineCancellationAndDuplicateRun(t *testing.T) {
	started := make(chan struct{}, 3)
	client, runtime := startRemoteRuntime(t, func(_ *agentruntimev1.ExecuteAgentRequest, stream grpc.ServerStreamingServer[agentruntimev1.AgentEvent]) error {
		started <- struct{}{}
		<-stream.Context().Done()
		return stream.Context().Err()
	}, 40*time.Millisecond, 1024*1024)

	_, err := runtime.Respond(context.Background(), remoteRequest("run_deadline"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
	<-started

	result := make(chan error, 1)
	go func() {
		_, callErr := runtime.Respond(context.Background(), remoteRequest("run_cancel"))
		result <- callErr
	}()
	<-started
	if !client.Cancel("run_cancel") {
		t.Fatal("expected active run cancellation")
	}
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}

	blocking := make(chan error, 1)
	go func() {
		_, callErr := runtime.Respond(context.Background(), remoteRequest("run_duplicate"))
		blocking <- callErr
	}()
	<-started
	_, duplicateErr := runtime.Respond(context.Background(), remoteRequest("run_duplicate"))
	if !errors.Is(duplicateErr, agent.ErrRuntimeDuplicateRun) {
		t.Fatalf("expected duplicate run error, got %v", duplicateErr)
	}
	client.Cancel("run_duplicate")
	<-blocking
}

func TestRemotePythonRuntimeEnforcesEventReceiveLimit(t *testing.T) {
	_, runtime := startRemoteRuntime(t, func(request *agentruntimev1.ExecuteAgentRequest, stream grpc.ServerStreamingServer[agentruntimev1.AgentEvent]) error {
		_ = stream.Send(runtimeEvent(request.GetRunId(), 1, &agentruntimev1.AgentEvent_Accepted{Accepted: &agentruntimev1.AcceptedEvent{}}))
		return stream.Send(runtimeEvent(request.GetRunId(), 2, &agentruntimev1.AgentEvent_Completed{Completed: &agentruntimev1.CompletedEvent{Content: string(make([]byte, 2048))}}))
	}, time.Second, 256)

	_, err := runtime.Respond(context.Background(), remoteRequest("run_size"))
	if !errors.Is(err, agent.ErrRuntimeCapacity) {
		t.Fatalf("expected receive limit error, got %v", err)
	}
}

func TestRemoteRuntimeClientCancelRoomStopsActiveCalls(t *testing.T) {
	started := make(chan struct{}, 2)
	client, runtime := startRemoteRuntime(t, func(_ *agentruntimev1.ExecuteAgentRequest, stream grpc.ServerStreamingServer[agentruntimev1.AgentEvent]) error {
		started <- struct{}{}
		<-stream.Context().Done()
		return stream.Context().Err()
	}, time.Second, 1024*1024)

	var wait sync.WaitGroup
	errorsSeen := make(chan error, 2)
	for _, runID := range []string{"run_all_1", "run_all_2"} {
		wait.Add(1)
		go func(id string) {
			defer wait.Done()
			_, err := runtime.Respond(context.Background(), remoteRequest(id))
			errorsSeen <- err
		}(runID)
	}
	<-started
	<-started
	if cancelled := client.CancelRoom("other_room"); cancelled != 0 {
		t.Fatalf("expected no cancellation for another room, got %d", cancelled)
	}
	if cancelled := client.CancelRoom("room_1"); cancelled != 2 {
		t.Fatalf("expected two room calls to be cancelled, got %d", cancelled)
	}
	wait.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
	}
}
