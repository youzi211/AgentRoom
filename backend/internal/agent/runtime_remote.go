package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	agentruntimev1 "agentroom/backend/internal/agentproto/v1"
	"agentroom/backend/internal/config"
	"agentroom/backend/internal/logging"
	"agentroom/backend/internal/model"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ErrRuntimeInvalidArgument = errors.New("agent runtime request is invalid")
	ErrRuntimeAuthentication  = errors.New("agent runtime authentication failed")
	ErrRuntimeCapacity        = errors.New("agent runtime capacity exhausted")
	ErrRuntimeUnavailable     = errors.New("agent runtime is unavailable")
	ErrRuntimeProtocol        = errors.New("agent runtime protocol violation")
	ErrRuntimeApplication     = errors.New("agent runtime application failed")
	ErrRuntimeDuplicateRun    = errors.New("agent runtime run is already active")
)

type RemoteRuntimeClient struct {
	conn   *grpc.ClientConn
	client agentruntimev1.AgentRuntimeServiceClient

	mu     sync.Mutex
	active map[string]activeRemoteCall
	logger *slog.Logger
}

type activeRemoteCall struct {
	roomID string
	cancel context.CancelFunc
}

func NewRemoteRuntimeClient(runtimeConfig config.AgentRuntimeConfig) (*RemoteRuntimeClient, error) {
	if runtimeConfig.Transport != config.AgentRuntimeTransportGRPC {
		return nil, errors.New("remote runtime client requires grpc transport")
	}
	if err := runtimeConfig.Validate(); err != nil {
		return nil, err
	}
	transportCredentials, err := runtimeTransportCredentials(runtimeConfig)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(
		runtimeConfig.GRPCAddress,
		grpc.WithTransportCredentials(transportCredentials),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(runtimeConfig.MaxRequestBytes),
			grpc.MaxCallRecvMsgSize(runtimeConfig.MaxEventBytes),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create Agent Runtime gRPC client: %w", err)
	}
	return &RemoteRuntimeClient{
		conn:   conn,
		client: agentruntimev1.NewAgentRuntimeServiceClient(conn),
		active: make(map[string]activeRemoteCall),
		logger: logging.Component("agent_runtime_grpc"),
	}, nil
}

func (c *RemoteRuntimeClient) Ready(ctx context.Context) error {
	response, err := grpc_health_v1.NewHealthClient(c.conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{
		Service: "agentroom.runtime.v1.AgentRuntimeService",
	})
	if err != nil {
		return fmt.Errorf("Agent Runtime health check: %w", err)
	}
	if response.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("Agent Runtime is not serving: %s", response.GetStatus())
	}
	return nil
}

func (c *RemoteRuntimeClient) Close() error {
	c.CancelAll()
	return c.conn.Close()
}

func (c *RemoteRuntimeClient) Cancel(runID string) bool {
	c.mu.Lock()
	call, ok := c.active[runID]
	c.mu.Unlock()
	if ok {
		call.cancel()
	}
	return ok
}

func (c *RemoteRuntimeClient) CancelAll() {
	c.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(c.active))
	for _, call := range c.active {
		cancels = append(cancels, call.cancel)
	}
	c.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (c *RemoteRuntimeClient) execute(
	ctx context.Context,
	request *agentruntimev1.ExecuteAgentRequest,
	observer AgentEventObserver,
) (response AgentRuntimeResponse, resultErr error) {
	callCtx, cancel := context.WithCancel(ctx)
	if !c.register(request.GetRunId(), request.GetRoom().GetId(), cancel) {
		cancel()
		return AgentRuntimeResponse{}, ErrRuntimeDuplicateRun
	}
	defer func() {
		cancel()
		c.unregister(request.GetRunId())
	}()
	startedAt := time.Now()
	c.logger.Info("remote Agent Runtime call started",
		"run_id", request.GetRunId(), "room_id", request.GetRoom().GetId(),
		"agent_id", request.GetAgent().GetId(), "dialogue_run_id", request.GetDialogueRunId(),
		"trace_id", request.GetTraceId(), "executor_kind", request.GetExecutorKind().String(),
	)
	defer func() {
		outcome := "succeeded"
		if resultErr != nil {
			outcome = remoteOutcome(resultErr)
		}
		c.logger.Info("remote Agent Runtime call finished",
			"run_id", request.GetRunId(), "room_id", request.GetRoom().GetId(),
			"agent_id", request.GetAgent().GetId(), "dialogue_run_id", request.GetDialogueRunId(),
			"trace_id", request.GetTraceId(), "executor_kind", request.GetExecutorKind().String(),
			"outcome", outcome, "duration_ms", time.Since(startedAt).Milliseconds(),
		)
	}()

	stream, err := c.client.ExecuteAgent(callCtx, request)
	if err != nil {
		return AgentRuntimeResponse{}, mapGRPCError(callCtx, err)
	}

	artifacts := make(map[string]AgentRuntimeArtifact)
	expectedSequence := uint64(1)
	accepted := false
	terminal := false
	var applicationErr error

	for {
		event, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			if !terminal {
				return AgentRuntimeResponse{}, fmt.Errorf("%w: stream ended without terminal event", ErrRuntimeProtocol)
			}
			if applicationErr != nil {
				return response, applicationErr
			}
			return response, nil
		}
		if recvErr != nil {
			if terminal {
				return AgentRuntimeResponse{}, fmt.Errorf("%w: stream failed after terminal event: %v", ErrRuntimeProtocol, recvErr)
			}
			return AgentRuntimeResponse{}, mapGRPCError(callCtx, recvErr)
		}
		if terminal {
			return AgentRuntimeResponse{}, fmt.Errorf("%w: event received after terminal event", ErrRuntimeProtocol)
		}
		if err := validateRemoteEvent(event, request.GetRunId(), expectedSequence, accepted); err != nil {
			return AgentRuntimeResponse{}, err
		}
		expectedSequence++

		runtimeEvent, isTerminal, eventErr := consumeRemoteEvent(event, &response, artifacts)
		if event.GetAccepted() != nil {
			accepted = true
		}
		if isTerminal {
			terminal = true
			applicationErr = eventErr
		}
		if observer != nil {
			observer.ObserveAgentEvent(callCtx, runtimeEvent)
		}
	}
}

func remoteOutcome(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, ErrRuntimeUnavailable):
		return "unavailable"
	case errors.Is(err, ErrRuntimeProtocol):
		return "protocol_error"
	default:
		return "failed"
	}
}

func (c *RemoteRuntimeClient) CancelRoom(roomID string) int {
	c.mu.Lock()
	cancels := make([]context.CancelFunc, 0)
	for _, call := range c.active {
		if call.roomID == roomID {
			cancels = append(cancels, call.cancel)
		}
	}
	c.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	return len(cancels)
}

func (c *RemoteRuntimeClient) register(runID string, roomID string, cancel context.CancelFunc) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.active[runID]; exists {
		return false
	}
	c.active[runID] = activeRemoteCall{roomID: roomID, cancel: cancel}
	return true
}

func (c *RemoteRuntimeClient) unregister(runID string) {
	c.mu.Lock()
	delete(c.active, runID)
	c.mu.Unlock()
}

type RemotePythonRuntime struct {
	name     string
	kind     agentruntimev1.ExecutorKind
	timeout  time.Duration
	client   *RemoteRuntimeClient
	resolver ModelConfigResolver
}

func NewRemotePythonRuntime(
	name string,
	client *RemoteRuntimeClient,
	timeout time.Duration,
	resolver ModelConfigResolver,
) (*RemotePythonRuntime, error) {
	normalized := model.NormalizeAgentRuntime(name)
	kind := agentruntimev1.ExecutorKind_EXECUTOR_KIND_LLM
	if normalized == model.AgentRuntimeDeepAgent {
		kind = agentruntimev1.ExecutorKind_EXECUTOR_KIND_DEEPAGENT
	} else if normalized != model.AgentRuntimeLLM {
		return nil, fmt.Errorf("unsupported remote Agent Runtime %q", name)
	}
	if timeout <= 0 {
		return nil, errors.New("remote Agent Runtime timeout must be positive")
	}
	return &RemotePythonRuntime{name: normalized, kind: kind, timeout: timeout, client: client, resolver: resolver}, nil
}

func (r *RemotePythonRuntime) Name() string { return r.name }

func (r *RemotePythonRuntime) CancelRoom(roomID string) int {
	return r.client.CancelRoom(roomID)
}

func (r *RemotePythonRuntime) CancelRun(runID string) bool {
	return r.client.Cancel(runID)
}

func (r *RemotePythonRuntime) Respond(
	ctx context.Context,
	request AgentRuntimeRequest,
	observers ...AgentEventObserver,
) (AgentRuntimeResponse, error) {
	requestCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	resolved := model.ResolvedModelConfig{}
	var err error
	if r.resolver != nil {
		resolved, err = r.resolver.Resolve(requestCtx, model.RuntimeScopeForAgent(r.name), request.Agent.ModelProfileID)
		if err != nil {
			return AgentRuntimeResponse{}, err
		}
	}
	var observer AgentEventObserver
	if len(observers) > 0 {
		observer = observers[0]
	}
	mapped := mapRemoteRequest(request, r.kind, resolved, r.timeout)
	defer func() { mapped.Model.ApiKey = "" }()
	return r.client.execute(requestCtx, mapped, observer)
}

func mapRemoteRequest(
	request AgentRuntimeRequest,
	kind agentruntimev1.ExecutorKind,
	resolved model.ResolvedModelConfig,
	timeout time.Duration,
) *agentruntimev1.ExecuteAgentRequest {
	roomInfo := request.Room.Info()
	mapped := &agentruntimev1.ExecuteAgentRequest{
		ProtocolVersion: agentruntimev1.ProtocolVersion,
		RunId:           request.RunID,
		TraceId:         request.TraceID,
		DialogueRunId:   request.Trigger.DialogueRunID,
		ExecutorKind:    kind,
		Room: &agentruntimev1.RoomSnapshot{
			Id:           roomInfo.ID,
			Name:         roomInfo.Name,
			DialogueMode: roomInfo.DialoguePolicy.WithDefaults().Mode,
		},
		Agent: &agentruntimev1.AgentSnapshot{
			Id:             request.Agent.ID,
			Name:           request.Agent.Name,
			Mention:        request.Agent.Mention,
			Role:           request.Agent.Role,
			Description:    request.Agent.Description,
			SystemPrompt:   request.Agent.SystemPrompt,
			Runtime:        rtrimRuntime(request.Agent.Runtime),
			ModelProfileId: request.Agent.ModelProfileID,
		},
		Trigger: mapRemoteMessage(request.Trigger),
		Model: &agentruntimev1.ModelConnection{
			Protocol:  model.ModelProtocolOpenAIChatCompletions,
			BaseUrl:   resolved.BaseURL,
			ModelName: resolved.ModelName,
			ApiKey:    resolved.APIKey,
			ProfileId: resolved.ProfileID,
			Source:    resolved.Source,
		},
		Limits: &agentruntimev1.ExecutionLimits{
			Timeout:          durationpb.New(timeout),
			MaxOutputBytes:   1024 * 1024,
			MaxArtifactBytes: 2 * 1024 * 1024,
		},
		PromptContext: mapRemotePromptContext(request.PromptContext),
	}
	for _, message := range request.RecentMessages {
		mapped.RecentMessages = append(mapped.RecentMessages, mapRemoteMessage(message))
	}
	for _, chunk := range request.KnowledgeChunks {
		mapped.KnowledgeChunks = append(mapped.KnowledgeChunks, mapRemoteKnowledgeChunk(chunk))
	}
	return mapped
}

func mapRemotePromptContext(prompt PromptContext) *agentruntimev1.PromptContextSnapshot {
	mapped := &agentruntimev1.PromptContextSnapshot{
		RoomName: prompt.RoomName, DialogueMode: prompt.DialogueMode,
		TriggerSender: prompt.TriggerSender, TriggerSenderType: mapRemoteSenderType(prompt.TriggerSenderType),
		TriggerContent: prompt.TriggerContent, LatestVisibleSpeaker: prompt.LatestVisibleSpeaker,
		LatestVisibleSpeakerType: mapRemoteSenderType(prompt.LatestVisibleSpeakerType),
		CurrentSpeaker:           mapRemoteAgentSnapshot(prompt.CurrentSpeaker),
		RootHumanTriggerSender:   prompt.RootHumanTriggerSender,
		RootHumanTriggerType:     mapRemoteSenderType(prompt.RootHumanTriggerType),
		RootHumanTriggerContent:  prompt.RootHumanTriggerContent,
		AutonomousTurnIndex:      uint32(prompt.AutonomousTurnIndex), MaxAutonomousTurns: uint32(prompt.MaxAutonomousTurns),
		AllowSelfFollowup: prompt.AllowSelfFollowup, AllowAgentToAgentMentions: prompt.AllowAgentToAgentMentions,
		MaxTurnsPerAgent: uint32(prompt.MaxTurnsPerAgent), ResponseStrategy: prompt.ResponseStrategy,
	}
	for _, participant := range prompt.OnlineHumanParticipants {
		mapped.OnlineHumanParticipants = append(mapped.OnlineHumanParticipants, &agentruntimev1.ParticipantSnapshot{Id: participant.ID, Name: participant.Name})
	}
	for _, candidate := range prompt.RoomAgents {
		mapped.RoomAgents = append(mapped.RoomAgents, mapRemoteAgentSnapshot(candidate))
	}
	for _, message := range prompt.Transcript {
		mapped.Transcript = append(mapped.Transcript, mapRemoteMessage(message))
	}
	for _, chunk := range prompt.KnowledgeChunks {
		mapped.KnowledgeChunks = append(mapped.KnowledgeChunks, mapRemoteKnowledgeChunk(chunk))
	}
	for _, peer := range prompt.EligiblePeers {
		mapped.EligiblePeers = append(mapped.EligiblePeers, mapRemoteAgentSnapshot(peer))
	}
	return mapped
}

func mapRemoteAgentSnapshot(candidate model.Agent) *agentruntimev1.AgentSnapshot {
	return &agentruntimev1.AgentSnapshot{
		Id: candidate.ID, Name: candidate.Name, Mention: candidate.Mention, Role: candidate.Role,
		Description: candidate.Description, SystemPrompt: candidate.SystemPrompt,
		Runtime: model.NormalizeAgentRuntime(candidate.Runtime), ModelProfileId: candidate.ModelProfileID,
	}
}

func mapRemoteKnowledgeChunk(chunk model.KnowledgeChunk) *agentruntimev1.KnowledgeChunk {
	return &agentruntimev1.KnowledgeChunk{
		Id: chunk.ID, DocumentId: chunk.DocumentID, DocumentName: chunk.DocumentName,
		Scope: chunk.Scope, ScopeId: chunk.ScopeID, ChunkIndex: int32(chunk.ChunkIndex), Content: chunk.Content,
	}
}

func mapRemoteMessage(message model.Message) *agentruntimev1.MessageSnapshot {
	return &agentruntimev1.MessageSnapshot{
		Id: message.ID, SenderId: message.SenderID, SenderName: message.SenderName,
		SenderType: mapRemoteSenderType(message.SenderType), Content: message.Content,
		CreatedAt: timestamppb.New(message.CreatedAt), DialogueRunId: message.DialogueRunID,
		TurnIndex: int32(message.TurnIndex), ParentMessageId: message.ParentMessageID,
	}
}

func mapRemoteSenderType(senderType string) agentruntimev1.SenderType {
	switch senderType {
	case model.SenderTypeHuman:
		return agentruntimev1.SenderType_SENDER_TYPE_HUMAN
	case model.SenderTypeAgent:
		return agentruntimev1.SenderType_SENDER_TYPE_AGENT
	case model.SenderTypeSystem:
		return agentruntimev1.SenderType_SENDER_TYPE_SYSTEM
	default:
		return agentruntimev1.SenderType_SENDER_TYPE_UNSPECIFIED
	}
}

func rtrimRuntime(runtime string) string {
	return model.NormalizeAgentRuntime(runtime)
}

func validateRemoteEvent(event *agentruntimev1.AgentEvent, runID string, expected uint64, accepted bool) error {
	if event == nil {
		return fmt.Errorf("%w: nil event", ErrRuntimeProtocol)
	}
	if err := agentruntimev1.ValidateProtocolVersion(event.GetProtocolVersion()); err != nil {
		return fmt.Errorf("%w: %v", ErrRuntimeProtocol, err)
	}
	if event.GetRunId() != runID {
		return fmt.Errorf("%w: event run_id %q does not match %q", ErrRuntimeProtocol, event.GetRunId(), runID)
	}
	if event.GetSequence() != expected {
		return fmt.Errorf("%w: event sequence %d, expected %d", ErrRuntimeProtocol, event.GetSequence(), expected)
	}
	if !accepted && event.GetAccepted() == nil {
		return fmt.Errorf("%w: accepted must be the first event", ErrRuntimeProtocol)
	}
	if accepted && event.GetAccepted() != nil {
		return fmt.Errorf("%w: duplicate accepted event", ErrRuntimeProtocol)
	}
	if event.GetPayload() == nil {
		return fmt.Errorf("%w: event payload is required", ErrRuntimeProtocol)
	}
	return nil
}

func consumeRemoteEvent(
	event *agentruntimev1.AgentEvent,
	response *AgentRuntimeResponse,
	artifacts map[string]AgentRuntimeArtifact,
) (AgentRuntimeEvent, bool, error) {
	observed := AgentRuntimeEvent{RunID: event.GetRunId(), OccurredAt: event.GetOccurredAt().AsTime()}
	switch payload := event.GetPayload().(type) {
	case *agentruntimev1.AgentEvent_Accepted:
		observed.Kind = "accepted"
	case *agentruntimev1.AgentEvent_ModelStarted:
		observed.Kind, observed.ModelName = "model_started", payload.ModelStarted.GetModelName()
	case *agentruntimev1.AgentEvent_ModelCompleted:
		observed.Kind, observed.ModelName = "model_completed", payload.ModelCompleted.GetModelName()
	case *agentruntimev1.AgentEvent_ToolStarted:
		observed.Kind, observed.ToolName = "tool_started", payload.ToolStarted.GetToolName()
	case *agentruntimev1.AgentEvent_ToolCompleted:
		observed.Kind, observed.ToolName = "tool_completed", payload.ToolCompleted.GetToolName()
	case *agentruntimev1.AgentEvent_ToolFailed:
		observed.Kind, observed.ToolName = "tool_failed", payload.ToolFailed.GetToolName()
		observed.Failure = safeRemoteFailure(payload.ToolFailed.GetFailure())
	case *agentruntimev1.AgentEvent_OutputDelta:
		observed.Kind = "output_delta"
	case *agentruntimev1.AgentEvent_ArtifactReady:
		observed.Kind = "artifact_ready"
		artifact := mapRemoteArtifact(payload.ArtifactReady.GetArtifact())
		artifacts[artifact.ID] = artifact
	case *agentruntimev1.AgentEvent_Completed:
		observed.Kind = "completed"
		completed := payload.Completed
		response.Content = completed.GetContent()
		for _, artifact := range completed.GetArtifacts() {
			mapped := mapRemoteArtifact(artifact)
			artifacts[mapped.ID] = mapped
		}
		response.Artifacts = make([]AgentRuntimeArtifact, 0, len(artifacts))
		for _, artifact := range artifacts {
			response.Artifacts = append(response.Artifacts, artifact)
		}
		response.Metadata = map[string]string{
			"model_profile_id": completed.GetModel().GetProfileId(),
			"model_source":     completed.GetModel().GetSource(),
			"model_name":       completed.GetModel().GetModelName(),
		}
		response.KnowledgeSources = make([]model.MessageKnowledgeSource, 0, len(completed.GetKnowledgeSources()))
		for _, source := range completed.GetKnowledgeSources() {
			response.KnowledgeSources = append(response.KnowledgeSources, model.MessageKnowledgeSource{
				DocumentID: source.GetDocumentId(), DocumentName: source.GetDocumentName(), Scope: source.GetScope(),
			})
		}
		return observed, true, nil
	case *agentruntimev1.AgentEvent_Failed:
		observed.Kind = "failed"
		failure := payload.Failed.GetFailure()
		observed.Failure = safeRemoteFailure(failure)
		return observed, true, fmt.Errorf("%w: %s", ErrRuntimeApplication, observed.Failure)
	default:
		return observed, false, fmt.Errorf("%w: unknown event payload", ErrRuntimeProtocol)
	}
	return observed, false, nil
}

func mapRemoteArtifact(artifact *agentruntimev1.Artifact) AgentRuntimeArtifact {
	if artifact == nil {
		return AgentRuntimeArtifact{}
	}
	return AgentRuntimeArtifact{
		ID: artifact.GetId(), Type: artifact.GetType(), Path: artifact.GetExternalUri(),
		MIMEType: artifact.GetMimeType(), Title: artifact.GetTitle(), FileName: artifact.GetFileName(),
		Content: string(artifact.GetContent()),
	}
}

func safeRemoteFailure(failure *agentruntimev1.RunFailure) string {
	if failure == nil || strings.TrimSpace(failure.GetMessage()) == "" {
		return "Agent Runtime failed"
	}
	return shortReason(errors.New(failure.GetMessage()))
}

func mapGRPCError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	grpcStatus, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("%w: %v", ErrRuntimeUnavailable, err)
	}
	message := strings.TrimSpace(grpcStatus.Message())
	switch grpcStatus.Code() {
	case codes.InvalidArgument, codes.AlreadyExists:
		return fmt.Errorf("%w: %s", ErrRuntimeInvalidArgument, message)
	case codes.Unauthenticated, codes.PermissionDenied:
		return fmt.Errorf("%w: %s", ErrRuntimeAuthentication, message)
	case codes.ResourceExhausted:
		return fmt.Errorf("%w: %s", ErrRuntimeCapacity, message)
	case codes.Canceled:
		return context.Canceled
	case codes.DeadlineExceeded:
		return context.DeadlineExceeded
	case codes.Unimplemented, codes.DataLoss, codes.Internal:
		return fmt.Errorf("%w: %s", ErrRuntimeProtocol, message)
	case codes.Unavailable:
		return fmt.Errorf("%w: %s", ErrRuntimeUnavailable, message)
	default:
		return fmt.Errorf("%w: grpc %s: %s", ErrRuntimeUnavailable, grpcStatus.Code(), message)
	}
}

func runtimeTransportCredentials(runtimeConfig config.AgentRuntimeConfig) (credentials.TransportCredentials, error) {
	if runtimeConfig.GRPCInsecure {
		return insecure.NewCredentials(), nil
	}
	caBytes, err := os.ReadFile(runtimeConfig.CAFile)
	if err != nil {
		return nil, fmt.Errorf("read Agent Runtime CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caBytes) {
		return nil, errors.New("Agent Runtime CA file contains no certificates")
	}
	tlsConfig := &tls.Config{RootCAs: roots, ServerName: runtimeConfig.ServerName, MinVersion: tls.VersionTLS12}
	if runtimeConfig.ClientCertFile != "" {
		certificate, err := tls.LoadX509KeyPair(runtimeConfig.ClientCertFile, runtimeConfig.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load Agent Runtime client identity: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	return credentials.NewTLS(tlsConfig), nil
}
