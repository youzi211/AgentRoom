package collaboration_test

import (
	"context"
	"errors"
	"net"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	collaborationruntimev1 "agentroom/backend/internal/collaborationproto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"agentroom/backend/internal/collaboration"
)

type fakeServer struct {
	collaborationruntimev1.UnimplementedCollaborationRuntimeServiceServer
	handler func(*collaborationruntimev1.ExecuteConversationRequest, grpc.ServerStreamingServer[collaborationruntimev1.CollaborationEvent]) error
}

func (s fakeServer) ExecuteConversation(request *collaborationruntimev1.ExecuteConversationRequest, stream grpc.ServerStreamingServer[collaborationruntimev1.CollaborationEvent]) error {
	return s.handler(request, stream)
}

func (s fakeServer) GetCapabilities(context.Context, *collaborationruntimev1.GetCapabilitiesRequest) (*collaborationruntimev1.GetCapabilitiesResponse, error) {
	return &collaborationruntimev1.GetCapabilitiesResponse{
		Ready:                     true,
		SupportedProtocolVersions: []string{"v1"},
		Engines: []*collaborationruntimev1.CollaborationEngineCapability{{
			Engine: "native", Version: "native-v1", Enabled: true, Ready: true,
		}},
		SupportedTriggerModes: []string{"mention_only", "automatic"},
	}, nil
}

func startClient(
	t *testing.T,
	handler func(*collaborationruntimev1.ExecuteConversationRequest, grpc.ServerStreamingServer[collaborationruntimev1.CollaborationEvent]) error,
	configure func(*collaboration.ClientConfig),
) (*collaboration.Client, *health.Server) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	collaborationruntimev1.RegisterCollaborationRuntimeServiceServer(server, fakeServer{handler: handler})
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	go func() { _ = server.Serve(listener) }()

	clientConfig := collaboration.ClientConfig{
		Address: listener.Addr().String(), Insecure: true, Timeout: time.Second,
		MaxRequestBytes: 1024 * 1024, MaxEventBytes: 1024 * 1024,
	}
	if configure != nil {
		configure(&clientConfig)
	}
	client, err := collaboration.NewClient(clientConfig)
	if err != nil {
		server.Stop()
		_ = listener.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = client.Close()
		server.Stop()
		_ = listener.Close()
	})
	return client, healthServer
}

func TestClientMapsRequestAndOrderedEvents(t *testing.T) {
	received := make(chan *collaborationruntimev1.ExecuteConversationRequest, 1)
	deadlineSeen := make(chan bool, 1)
	client, _ := startClient(t, func(request *collaborationruntimev1.ExecuteConversationRequest, stream grpc.ServerStreamingServer[collaborationruntimev1.CollaborationEvent]) error {
		received <- request
		_, ok := stream.Context().Deadline()
		deadlineSeen <- ok
		events := []*collaborationruntimev1.CollaborationEvent{
			protoEvent(1, &collaborationruntimev1.CollaborationEvent_Accepted{Accepted: &collaborationruntimev1.AcceptedEvent{}}),
			turnProtoEvent(2, &collaborationruntimev1.CollaborationEvent_SpeakerSelected{SpeakerSelected: &collaborationruntimev1.SpeakerSelectedEvent{ReasonCategory: "mention"}}),
			turnProtoEvent(3, &collaborationruntimev1.CollaborationEvent_ModelCompleted{ModelCompleted: &collaborationruntimev1.ModelCompletedEvent{
				ModelSelectionId: "model_1", Usage: &collaborationruntimev1.Usage{InputTokens: 3, OutputTokens: 5, TotalTokens: 8},
			}}),
			turnProtoEvent(4, &collaborationruntimev1.CollaborationEvent_ToolFailed{ToolFailed: &collaborationruntimev1.ToolFailedEvent{
				ToolCallId: "tool_1", ToolName: "search", Failure: &collaborationruntimev1.CollaborationFailure{
					Code:    collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_TOOL_FAILED,
					Message: "Authorization: secret-token",
				},
			}}),
			turnProtoEvent(5, &collaborationruntimev1.CollaborationEvent_ArtifactReady{ArtifactReady: &collaborationruntimev1.ArtifactReadyEvent{
				Artifact: &collaborationruntimev1.Artifact{Id: "artifact_1", Type: "report", Content: []byte("data")},
			}}),
			turnProtoEvent(6, &collaborationruntimev1.CollaborationEvent_HandoffRequested{HandoffRequested: &collaborationruntimev1.HandoffRequestedEvent{
				TargetAgentId: "agent_2", ReasonCategory: "mention",
			}}),
			turnProtoEvent(7, &collaborationruntimev1.CollaborationEvent_AgentMessageCompleted{AgentMessageCompleted: &collaborationruntimev1.AgentMessageCompletedEvent{
				Content: "final", Artifacts: []*collaborationruntimev1.Artifact{{Id: "artifact_1", Content: []byte("data")}},
				KnowledgeSources: []*collaborationruntimev1.KnowledgeSource{{DocumentId: "doc_1", DocumentName: "Plan", Scope: "room"}},
				Model:            &collaborationruntimev1.ModelAudit{ModelSelectionId: "model_1", ProfileId: "profile_1", Source: "database", ModelName: "test-model"},
				Usage:            &collaborationruntimev1.Usage{InputTokens: 3, OutputTokens: 5, TotalTokens: 8},
			}}),
			protoEvent(8, &collaborationruntimev1.CollaborationEvent_Checkpoint{Checkpoint: &collaborationruntimev1.CheckpointEvent{
				Checkpoint: &collaborationruntimev1.OpaqueCheckpoint{
					Engine:        collaborationruntimev1.CollaborationEngine_COLLABORATION_ENGINE_NATIVE,
					EngineVersion: "native-v1", FormatVersion: "1", Sha256: "hash", SizeBytes: 3, Payload: []byte("raw"),
				},
			}}),
			protoEvent(9, &collaborationruntimev1.CollaborationEvent_Completed{Completed: &collaborationruntimev1.CompletedEvent{
				TurnCount: 1, Reason: collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_COMPLETED,
			}}),
		}
		for _, event := range events {
			if err := stream.Send(event); err != nil {
				return err
			}
		}
		return nil
	}, nil)

	stream, err := client.ExecuteConversation(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	var events []collaboration.Event
	for len(events) < 9 {
		event, err := stream.Recv()
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}

	request := <-received
	if request.GetEngine() != collaborationruntimev1.CollaborationEngine_COLLABORATION_ENGINE_NATIVE || request.GetSnapshot().GetRoom().GetId() != "room_1" {
		t.Fatalf("unexpected mapped request: %#v", request)
	}
	if got := request.GetSnapshot().GetAgents()[0]; got.GetModelSelectionId() != "model_1" || len(got.GetToolNames()) != 1 {
		t.Fatalf("unexpected Agent mapping: %#v", got)
	}
	if request.GetSnapshot().GetTrigger().GetCreatedAt() == nil || request.GetCheckpoint().GetEngineVersion() != "native-v1" {
		t.Fatal("request timestamp or checkpoint was not mapped")
	}
	if !<-deadlineSeen {
		t.Fatal("gRPC call did not carry a deadline")
	}
	if events[1].Kind != collaboration.EventSpeakerSelected || events[1].ReasonCategory != "mention" {
		t.Fatalf("unexpected speaker event: %#v", events[1])
	}
	if events[2].ModelSelectionID != "model_1" || events[2].Usage.TotalTokens != 8 {
		t.Fatalf("unexpected model event: %#v", events[2])
	}
	if events[3].Tool == nil || events[3].Tool.Failure == nil || events[3].Tool.Failure.Code != collaboration.ErrorToolFailed {
		t.Fatalf("unexpected tool event: %#v", events[3])
	}
	if strings.Contains(strings.ToLower(events[3].Tool.Failure.Message), "secret") {
		t.Fatalf("remote failure text leaked: %#v", events[3].Tool.Failure)
	}
	if events[4].Artifact == nil || string(events[4].Artifact.Content) != "data" {
		t.Fatalf("unexpected artifact event: %#v", events[4])
	}
	if events[5].Handoff == nil || events[5].Handoff.TargetAgentID != "agent_2" {
		t.Fatalf("unexpected handoff event: %#v", events[5])
	}
	if events[6].Message == nil || events[6].Message.Model.ProfileID != "profile_1" || len(events[6].Message.KnowledgeSources) != 1 {
		t.Fatalf("unexpected message event: %#v", events[6])
	}
	if events[7].Checkpoint == nil || events[7].Checkpoint.Engine != collaboration.EngineNative {
		t.Fatalf("unexpected checkpoint event: %#v", events[7])
	}
	if events[8].Terminal == nil || events[8].Terminal.Reason != collaboration.StopReasonCompleted {
		t.Fatalf("unexpected terminal event: %#v", events[8])
	}
}

func TestClientUsesCollaborationHealthService(t *testing.T) {
	client, healthServer := startClient(t, func(*collaborationruntimev1.ExecuteConversationRequest, grpc.ServerStreamingServer[collaborationruntimev1.CollaborationEvent]) error {
		return nil
	}, nil)
	healthServer.SetServingStatus("agentroom.collaboration.v1.CollaborationRuntimeService", grpc_health_v1.HealthCheckResponse_SERVING)
	if err := client.Ready(context.Background()); err != nil {
		t.Fatalf("expected serving client: %v", err)
	}
	healthServer.SetServingStatus("agentroom.collaboration.v1.CollaborationRuntimeService", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	if err := client.Ready(context.Background()); !errors.Is(err, collaboration.ErrUnavailable) {
		t.Fatalf("expected unavailable health status, got %v", err)
	}
}

func TestClientGetsRuntimeCapabilities(t *testing.T) {
	client, _ := startClient(t, func(*collaborationruntimev1.ExecuteConversationRequest, grpc.ServerStreamingServer[collaborationruntimev1.CollaborationEvent]) error {
		return nil
	}, nil)

	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !capabilities.Ready || len(capabilities.Engines) != 1 {
		t.Fatalf("unexpected capabilities: %#v", capabilities)
	}
	engine := capabilities.Engines[0]
	if engine.Engine != collaboration.EngineNative || engine.Version != "native-v1" || !engine.Enabled || !engine.Ready {
		t.Fatalf("unexpected engine capability: %#v", engine)
	}
	wantModes := []collaboration.TriggerMode{collaboration.TriggerMentionOnly, collaboration.TriggerAutomatic}
	if !slices.Equal(capabilities.SupportedTriggerModes, wantModes) {
		t.Fatalf("unexpected trigger modes: %#v", capabilities.SupportedTriggerModes)
	}
}

func TestClientPropagatesCancellation(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	client, _ := startClient(t, func(_ *collaborationruntimev1.ExecuteConversationRequest, stream grpc.ServerStreamingServer[collaborationruntimev1.CollaborationEvent]) error {
		close(started)
		<-stream.Context().Done()
		close(cancelled)
		return stream.Context().Err()
	}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := client.ExecuteConversation(ctx, validRequest())
	if err != nil {
		t.Fatal(err)
	}
	<-started
	cancel()
	if _, err := stream.Recv(); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("server did not observe cancellation")
	}
}

func TestClientAppliesDefaultDeadline(t *testing.T) {
	cancelled := make(chan struct{})
	client, _ := startClient(t, func(_ *collaborationruntimev1.ExecuteConversationRequest, stream grpc.ServerStreamingServer[collaborationruntimev1.CollaborationEvent]) error {
		<-stream.Context().Done()
		close(cancelled)
		return stream.Context().Err()
	}, func(config *collaboration.ClientConfig) {
		config.Timeout = 30 * time.Millisecond
	})
	request := validRequest()
	request.Snapshot.Limits.Timeout = 0
	stream, err := client.ExecuteConversation(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected default deadline, got %v", err)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("server did not observe deadline cancellation")
	}
}

func TestClientCloseCancelsActiveStream(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	client, _ := startClient(t, func(_ *collaborationruntimev1.ExecuteConversationRequest, stream grpc.ServerStreamingServer[collaborationruntimev1.CollaborationEvent]) error {
		close(started)
		<-stream.Context().Done()
		close(cancelled)
		return stream.Context().Err()
	}, nil)
	stream, err := client.ExecuteConversation(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); err == nil {
		t.Fatal("expected closed connection to end stream")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("server did not observe connection close")
	}
}

func TestClientEnforcesMessageLimits(t *testing.T) {
	client, _ := startClient(t, func(_ *collaborationruntimev1.ExecuteConversationRequest, stream grpc.ServerStreamingServer[collaborationruntimev1.CollaborationEvent]) error {
		return stream.Send(turnProtoEvent(1, &collaborationruntimev1.CollaborationEvent_OutputDelta{OutputDelta: &collaborationruntimev1.OutputDeltaEvent{
			Text: strings.Repeat("x", 2048),
		}}))
	}, func(config *collaboration.ClientConfig) {
		config.MaxRequestBytes = 1024
		config.MaxEventBytes = 512
	})

	oversized := validRequest()
	oversized.Snapshot.Trigger.Content = strings.Repeat("x", 4096)
	if _, err := client.ExecuteConversation(context.Background(), oversized); !errors.Is(err, collaboration.ErrInvalidRequest) {
		t.Fatalf("expected request limit error, got %v", err)
	}

	stream, err := client.ExecuteConversation(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); !errors.Is(err, collaboration.ErrCapacity) {
		t.Fatalf("expected receive limit error, got %v", err)
	}
}

func TestClientDoesNotExposeRemoteErrorText(t *testing.T) {
	client, _ := startClient(t, func(*collaborationruntimev1.ExecuteConversationRequest, grpc.ServerStreamingServer[collaborationruntimev1.CollaborationEvent]) error {
		return status.Error(codes.Internal, "Authorization: secret-token")
	}, nil)
	stream, err := client.ExecuteConversation(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	_, err = stream.Recv()
	if !errors.Is(err, collaboration.ErrProtocol) {
		t.Fatalf("expected protocol error, got %v", err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "secret") || strings.Contains(strings.ToLower(err.Error()), "authorization") {
		t.Fatalf("remote error text leaked: %v", err)
	}
}

func TestClientRejectsInvalidTLSConfiguration(t *testing.T) {
	config := collaboration.ClientConfig{
		Address: "runtime.invalid:443", Timeout: time.Second, MaxRequestBytes: 1024, MaxEventBytes: 1024,
	}
	if _, err := collaboration.NewClient(config); !errors.Is(err, collaboration.ErrInvalidTransport) {
		t.Fatalf("expected missing CA error, got %v", err)
	}

	caFile := t.TempDir() + string(os.PathSeparator) + "ca.pem"
	if err := os.WriteFile(caFile, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	config.CAFile = caFile
	if _, err := collaboration.NewClient(config); !errors.Is(err, collaboration.ErrInvalidTransport) {
		t.Fatalf("expected invalid CA error, got %v", err)
	}

	config.ClientCertFile = "client.pem"
	if _, err := collaboration.NewClient(config); !errors.Is(err, collaboration.ErrInvalidTransport) {
		t.Fatalf("expected incomplete mTLS identity error, got %v", err)
	}
}

func validRequest() collaboration.Request {
	createdAt := time.Now().UTC().Truncate(time.Second)
	return collaboration.Request{
		ProtocolVersion: "v1", CollaborationRunID: "collab_1", TraceID: "trace_1", Engine: collaboration.EngineNative,
		Snapshot: collaboration.ConversationSnapshot{
			Room: collaboration.RoomSnapshot{ID: "room_1", Name: "Planning", Status: "active"},
			Agents: []collaboration.AgentSnapshot{{
				ID: "agent_1", Name: "Architect", Mention: "Architect", Runtime: "llm",
				ModelSelectionID: "model_1", ToolNames: []string{"search"},
			}},
			Trigger: collaboration.MessageSnapshot{
				ID: "message_1", SenderID: "user_1", SenderName: "Alice", SenderType: collaboration.SenderHuman,
				Content: "Plan this", CreatedAt: createdAt,
			},
			Transcript: []collaboration.MessageSnapshot{{
				ID: "message_0", SenderID: "user_1", SenderType: collaboration.SenderHuman, Content: "Context", CreatedAt: createdAt,
			}},
			KnowledgeChunks: []collaboration.KnowledgeChunk{{ID: "chunk_1", DocumentID: "doc_1", DocumentName: "Plan", Scope: "room", ScopeID: "room_1", Content: "Knowledge"}},
			ModelSelections: []collaboration.ModelSelection{{ID: "model_1", ProfileID: "profile_1", Source: "database", Protocol: "openai_chat_completions", ModelName: "test-model", RuntimeScope: "llm"}},
			Policy: collaboration.PolicySnapshot{
				Version: "v1", Engine: collaboration.EngineNative, TriggerMode: collaboration.TriggerMentionOnly,
				MaxTurns: 3, MaxTurnsPerAgent: 1, AllowAgentHandoff: true, Cooldown: time.Millisecond,
				StopOnEmptyOutput: true, StopOnRepeatedOutput: true,
			},
			Limits: collaboration.ExecutionLimits{
				Timeout: time.Second, MaxOutputBytes: 4096, MaxArtifactBytes: 4096, MaxToolSteps: 4,
				MaxRequestBytes: 1024 * 1024, MaxEventBytes: 1024 * 1024, MaxCheckpointBytes: 1024,
			},
			InitialCandidateAgentIDs: []string{"agent_1"},
		},
		Checkpoint: &collaboration.Checkpoint{
			Engine: collaboration.EngineNative, EngineVersion: "native-v1", FormatVersion: "1",
			SHA256: "hash", SizeBytes: 3, Payload: []byte("raw"),
		},
	}
}

func protoEvent(sequence uint64, payload any) *collaborationruntimev1.CollaborationEvent {
	event := &collaborationruntimev1.CollaborationEvent{
		ProtocolVersion: "v1", CollaborationRunId: "collab_1", Sequence: sequence,
		OccurredAt: timestamppb.Now(),
	}
	switch value := payload.(type) {
	case *collaborationruntimev1.CollaborationEvent_Accepted:
		event.Payload = value
	case *collaborationruntimev1.CollaborationEvent_SpeakerSelected:
		event.Payload = value
	case *collaborationruntimev1.CollaborationEvent_ModelCompleted:
		event.Payload = value
	case *collaborationruntimev1.CollaborationEvent_ToolFailed:
		event.Payload = value
	case *collaborationruntimev1.CollaborationEvent_OutputDelta:
		event.Payload = value
	case *collaborationruntimev1.CollaborationEvent_ArtifactReady:
		event.Payload = value
	case *collaborationruntimev1.CollaborationEvent_HandoffRequested:
		event.Payload = value
	case *collaborationruntimev1.CollaborationEvent_AgentMessageCompleted:
		event.Payload = value
	case *collaborationruntimev1.CollaborationEvent_Checkpoint:
		event.Payload = value
	case *collaborationruntimev1.CollaborationEvent_Completed:
		event.Payload = value
	default:
		panic("unsupported test payload")
	}
	return event
}

func turnProtoEvent(sequence uint64, payload any) *collaborationruntimev1.CollaborationEvent {
	event := protoEvent(sequence, payload)
	event.TurnId, event.AgentId = "turn_1", "agent_1"
	return event
}
