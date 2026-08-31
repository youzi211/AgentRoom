package collaborationgrpc

import (
	"fmt"

	"agentroom/backend/internal/collaboration"
	collaborationruntimev1 "agentroom/backend/internal/collaborationproto/v1"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func mapRequest(request collaboration.Request) (*collaborationruntimev1.ExecuteConversationRequest, error) {
	engine, err := mapEngineToProto(request.Engine)
	if err != nil {
		return nil, err
	}
	policyEngine, err := mapEngineToProto(request.Snapshot.Policy.Engine)
	if err != nil {
		return nil, err
	}
	triggerMode, err := mapTriggerModeToProto(request.Snapshot.Policy.TriggerMode)
	if err != nil {
		return nil, err
	}

	mapped := &collaborationruntimev1.ExecuteConversationRequest{
		ProtocolVersion:    request.ProtocolVersion,
		CollaborationRunId: request.CollaborationRunID,
		TraceId:            request.TraceID,
		Engine:             engine,
		Snapshot: &collaborationruntimev1.ConversationSnapshot{
			Room: &collaborationruntimev1.RoomSnapshot{
				Id: request.Snapshot.Room.ID, Name: request.Snapshot.Room.Name, Status: request.Snapshot.Room.Status,
			},
			Trigger: mapMessageToProto(request.Snapshot.Trigger),
			Policy: &collaborationruntimev1.CollaborationPolicySnapshot{
				Version: request.Snapshot.Policy.Version, Engine: policyEngine, TriggerMode: triggerMode,
				MaxTurns: request.Snapshot.Policy.MaxTurns, MaxTurnsPerAgent: request.Snapshot.Policy.MaxTurnsPerAgent,
				AllowAgentHandoff:    request.Snapshot.Policy.AllowAgentHandoff,
				AllowSelfFollowup:    request.Snapshot.Policy.AllowSelfFollowup,
				Cooldown:             durationpb.New(request.Snapshot.Policy.Cooldown),
				StopOnEmptyOutput:    request.Snapshot.Policy.StopOnEmptyOutput,
				StopOnRepeatedOutput: request.Snapshot.Policy.StopOnRepeatedOutput,
			},
			Limits: &collaborationruntimev1.ExecutionLimits{
				Timeout:            durationpb.New(request.Snapshot.Limits.Timeout),
				MaxOutputBytes:     request.Snapshot.Limits.MaxOutputBytes,
				MaxArtifactBytes:   request.Snapshot.Limits.MaxArtifactBytes,
				MaxToolSteps:       request.Snapshot.Limits.MaxToolSteps,
				MaxRequestBytes:    request.Snapshot.Limits.MaxRequestBytes,
				MaxEventBytes:      request.Snapshot.Limits.MaxEventBytes,
				MaxCheckpointBytes: request.Snapshot.Limits.MaxCheckpointBytes,
			},
			InitialCandidateAgentIds: append([]string(nil), request.Snapshot.InitialCandidateAgentIDs...),
		},
	}
	for _, agent := range request.Snapshot.Agents {
		mapped.Snapshot.Agents = append(mapped.Snapshot.Agents, &collaborationruntimev1.AgentSnapshot{
			Id: agent.ID, Name: agent.Name, Mention: agent.Mention, Role: agent.Role,
			Description: agent.Description, SystemPrompt: agent.SystemPrompt, Runtime: agent.Runtime,
			ModelReferenceId: agent.ModelReferenceID, ToolNames: append([]string(nil), agent.ToolNames...),
		})
	}
	for _, message := range request.Snapshot.Transcript {
		mapped.Snapshot.Transcript = append(mapped.Snapshot.Transcript, mapMessageToProto(message))
	}
	for _, chunk := range request.Snapshot.KnowledgeChunks {
		mapped.Snapshot.KnowledgeChunks = append(mapped.Snapshot.KnowledgeChunks, &collaborationruntimev1.KnowledgeChunk{
			Id: chunk.ID, DocumentId: chunk.DocumentID, DocumentName: chunk.DocumentName,
			Scope: chunk.Scope, ScopeId: chunk.ScopeID, ChunkIndex: chunk.ChunkIndex, Content: chunk.Content,
		})
	}
	for _, reference := range request.Snapshot.ModelReferences {
		mapped.Snapshot.ModelReferences = append(mapped.Snapshot.ModelReferences, &collaborationruntimev1.ModelReference{
			Id: reference.ID, ProfileId: reference.ProfileID, Source: reference.Source,
			Protocol: reference.Protocol, ModelName: reference.ModelName, RuntimeScope: reference.RuntimeScope,
		})
	}
	if request.Checkpoint != nil {
		mapped.Checkpoint, err = mapCheckpointToProto(*request.Checkpoint)
		if err != nil {
			return nil, err
		}
	}
	return mapped, nil
}

func mapMessageToProto(message collaboration.MessageSnapshot) *collaborationruntimev1.MessageSnapshot {
	mapped := &collaborationruntimev1.MessageSnapshot{
		Id: message.ID, SenderId: message.SenderID, SenderName: message.SenderName,
		SenderType: mapSenderTypeToProto(message.SenderType), Content: message.Content,
		CollaborationRunId: message.CollaborationRunID, TurnIndex: message.TurnIndex,
		ParentMessageId: message.ParentMessageID,
	}
	if !message.CreatedAt.IsZero() {
		mapped.CreatedAt = timestamppb.New(message.CreatedAt)
	}
	return mapped
}

func mapCheckpointToProto(checkpoint collaboration.Checkpoint) (*collaborationruntimev1.OpaqueCheckpoint, error) {
	engine, err := mapEngineToProto(checkpoint.Engine)
	if err != nil {
		return nil, err
	}
	return &collaborationruntimev1.OpaqueCheckpoint{
		Engine: engine, EngineVersion: checkpoint.EngineVersion, FormatVersion: checkpoint.FormatVersion,
		Sha256: checkpoint.SHA256, SizeBytes: checkpoint.SizeBytes, Payload: append([]byte(nil), checkpoint.Payload...),
	}, nil
}

func mapEvent(event *collaborationruntimev1.CollaborationEvent) (collaboration.Event, error) {
	if event == nil || event.GetPayload() == nil {
		return collaboration.Event{}, ErrProtocol
	}
	mapped := collaboration.Event{
		ProtocolVersion: event.GetProtocolVersion(), CollaborationRunID: event.GetCollaborationRunId(),
		Sequence: event.GetSequence(), TurnID: event.GetTurnId(), AgentID: event.GetAgentId(),
	}
	if event.GetOccurredAt() != nil {
		mapped.OccurredAt = event.GetOccurredAt().AsTime()
	}
	switch payload := event.GetPayload().(type) {
	case *collaborationruntimev1.CollaborationEvent_Accepted:
		mapped.Kind = collaboration.EventAccepted
	case *collaborationruntimev1.CollaborationEvent_CollaborationStarted:
		mapped.Kind = collaboration.EventCollaborationStarted
	case *collaborationruntimev1.CollaborationEvent_SpeakerSelected:
		mapped.Kind, mapped.ReasonCategory = collaboration.EventSpeakerSelected, payload.SpeakerSelected.GetReasonCategory()
	case *collaborationruntimev1.CollaborationEvent_AgentTurnStarted:
		mapped.Kind = collaboration.EventAgentTurnStarted
	case *collaborationruntimev1.CollaborationEvent_ModelStarted:
		mapped.Kind, mapped.ModelReferenceID = collaboration.EventModelStarted, payload.ModelStarted.GetModelReferenceId()
	case *collaborationruntimev1.CollaborationEvent_ModelCompleted:
		mapped.Kind, mapped.ModelReferenceID = collaboration.EventModelCompleted, payload.ModelCompleted.GetModelReferenceId()
		mapped.Usage = mapUsage(payload.ModelCompleted.GetUsage())
	case *collaborationruntimev1.CollaborationEvent_ToolStarted:
		mapped.Kind = collaboration.EventToolStarted
		mapped.Tool = &collaboration.ToolActivity{CallID: payload.ToolStarted.GetToolCallId(), Name: payload.ToolStarted.GetToolName(), InputSummary: payload.ToolStarted.GetInputSummary()}
	case *collaborationruntimev1.CollaborationEvent_ToolCompleted:
		mapped.Kind = collaboration.EventToolCompleted
		mapped.Tool = &collaboration.ToolActivity{CallID: payload.ToolCompleted.GetToolCallId(), Name: payload.ToolCompleted.GetToolName(), OutputSummary: payload.ToolCompleted.GetOutputSummary()}
	case *collaborationruntimev1.CollaborationEvent_ToolFailed:
		mapped.Kind = collaboration.EventToolFailed
		mapped.Tool = &collaboration.ToolActivity{CallID: payload.ToolFailed.GetToolCallId(), Name: payload.ToolFailed.GetToolName(), Failure: mapFailure(payload.ToolFailed.GetFailure())}
	case *collaborationruntimev1.CollaborationEvent_OutputDelta:
		mapped.Kind, mapped.OutputDelta = collaboration.EventOutputDelta, payload.OutputDelta.GetText()
	case *collaborationruntimev1.CollaborationEvent_ArtifactReady:
		mapped.Kind, mapped.Artifact = collaboration.EventArtifactReady, mapArtifact(payload.ArtifactReady.GetArtifact())
	case *collaborationruntimev1.CollaborationEvent_HandoffRequested:
		mapped.Kind = collaboration.EventHandoffRequested
		mapped.Handoff = &collaboration.Handoff{TargetAgentID: payload.HandoffRequested.GetTargetAgentId(), ReasonCategory: payload.HandoffRequested.GetReasonCategory()}
	case *collaborationruntimev1.CollaborationEvent_AgentMessageCompleted:
		mapped.Kind, mapped.Message = collaboration.EventAgentMessageCompleted, mapAgentMessage(payload.AgentMessageCompleted)
	case *collaborationruntimev1.CollaborationEvent_Checkpoint:
		mapped.Kind = collaboration.EventCheckpoint
		checkpoint, err := mapCheckpointFromProto(payload.Checkpoint.GetCheckpoint())
		if err != nil {
			return collaboration.Event{}, err
		}
		mapped.Checkpoint = checkpoint
	case *collaborationruntimev1.CollaborationEvent_Completed:
		terminal, err := mapTerminal(payload.Completed.GetTurnCount(), payload.Completed.GetReason(), nil)
		if err != nil {
			return collaboration.Event{}, err
		}
		mapped.Kind, mapped.Terminal = collaboration.EventCompleted, terminal
	case *collaborationruntimev1.CollaborationEvent_Stopped:
		terminal, err := mapTerminal(payload.Stopped.GetTurnCount(), payload.Stopped.GetReason(), nil)
		if err != nil {
			return collaboration.Event{}, err
		}
		mapped.Kind, mapped.Terminal = collaboration.EventStopped, terminal
	case *collaborationruntimev1.CollaborationEvent_Cancelled:
		terminal, err := mapTerminal(payload.Cancelled.GetTurnCount(), payload.Cancelled.GetReason(), nil)
		if err != nil {
			return collaboration.Event{}, err
		}
		mapped.Kind, mapped.Terminal = collaboration.EventCancelled, terminal
	case *collaborationruntimev1.CollaborationEvent_Failed:
		terminal, err := mapTerminal(payload.Failed.GetTurnCount(), payload.Failed.GetReason(), payload.Failed.GetFailure())
		if err != nil {
			return collaboration.Event{}, err
		}
		mapped.Kind, mapped.Terminal = collaboration.EventFailed, terminal
	default:
		return collaboration.Event{}, ErrProtocol
	}
	return mapped, nil
}

func mapAgentMessage(message *collaborationruntimev1.AgentMessageCompletedEvent) *collaboration.AgentMessage {
	if message == nil {
		return nil
	}
	mapped := &collaboration.AgentMessage{
		Content: message.GetContent(), Model: mapModelAudit(message.GetModel()), Usage: mapUsage(message.GetUsage()),
	}
	for _, artifact := range message.GetArtifacts() {
		if value := mapArtifact(artifact); value != nil {
			mapped.Artifacts = append(mapped.Artifacts, *value)
		}
	}
	for _, source := range message.GetKnowledgeSources() {
		mapped.KnowledgeSources = append(mapped.KnowledgeSources, collaboration.KnowledgeSource{
			DocumentID: source.GetDocumentId(), DocumentName: source.GetDocumentName(), Scope: source.GetScope(),
		})
	}
	return mapped
}

func mapArtifact(artifact *collaborationruntimev1.Artifact) *collaboration.Artifact {
	if artifact == nil {
		return nil
	}
	return &collaboration.Artifact{
		ID: artifact.GetId(), Type: artifact.GetType(), Title: artifact.GetTitle(), FileName: artifact.GetFileName(),
		MIMEType: artifact.GetMimeType(), Content: append([]byte(nil), artifact.GetContent()...), ExternalURI: artifact.GetExternalUri(),
	}
}

func mapModelAudit(audit *collaborationruntimev1.ModelAudit) collaboration.ModelAudit {
	if audit == nil {
		return collaboration.ModelAudit{}
	}
	return collaboration.ModelAudit{
		ModelReferenceID: audit.GetModelReferenceId(), ProfileID: audit.GetProfileId(),
		Source: audit.GetSource(), ModelName: audit.GetModelName(),
	}
}

func mapUsage(usage *collaborationruntimev1.Usage) collaboration.Usage {
	if usage == nil {
		return collaboration.Usage{}
	}
	return collaboration.Usage{InputTokens: usage.GetInputTokens(), OutputTokens: usage.GetOutputTokens(), TotalTokens: usage.GetTotalTokens()}
}

func mapFailure(failure *collaborationruntimev1.CollaborationFailure) *collaboration.Failure {
	if failure == nil {
		return nil
	}
	code := mapErrorCode(failure.GetCode())
	return &collaboration.Failure{Code: code, Message: safeFailureMessage(code), Retryable: failure.GetRetryable()}
}

func safeFailureMessage(code collaboration.ErrorCode) string {
	messages := map[collaboration.ErrorCode]string{
		collaboration.ErrorInvalidRequest:            "Collaboration request is invalid",
		collaboration.ErrorUnsupportedVersion:        "Collaboration protocol version is unsupported",
		collaboration.ErrorEngineUnavailable:         "Collaboration Engine is unavailable",
		collaboration.ErrorResourceExhausted:         "Collaboration resource limit exceeded",
		collaboration.ErrorDuplicateRun:              "Collaboration run is already active",
		collaboration.ErrorRoomBusy:                  "Collaboration room already has an active run",
		collaboration.ErrorModelNotConfigured:        "Collaboration model is not configured",
		collaboration.ErrorModelAuthenticationFailed: "Collaboration model authentication failed",
		collaboration.ErrorModelRateLimited:          "Collaboration model rate limit exceeded",
		collaboration.ErrorModelTimeout:              "Collaboration model timed out",
		collaboration.ErrorToolFailed:                "Collaboration tool failed",
		collaboration.ErrorOutputInvalid:             "Collaboration output is invalid",
		collaboration.ErrorCheckpointInvalid:         "Collaboration checkpoint is invalid",
		collaboration.ErrorProtocol:                  "Collaboration Engine violated the event protocol",
		collaboration.ErrorCancelled:                 "Collaboration was cancelled",
		collaboration.ErrorDeadlineExceeded:          "Collaboration deadline exceeded",
	}
	if message, ok := messages[code]; ok {
		return message
	}
	return "Collaboration Engine failed"
}

func mapTerminal(turnCount uint32, reason collaborationruntimev1.CollaborationStopReason, failure *collaborationruntimev1.CollaborationFailure) (*collaboration.Terminal, error) {
	mappedReason, ok := mapStopReason(reason)
	if !ok {
		return nil, ErrProtocol
	}
	return &collaboration.Terminal{TurnCount: turnCount, Reason: mappedReason, Failure: mapFailure(failure)}, nil
}

func mapCheckpointFromProto(checkpoint *collaborationruntimev1.OpaqueCheckpoint) (*collaboration.Checkpoint, error) {
	if checkpoint == nil {
		return nil, ErrProtocol
	}
	engine, ok := mapEngineFromProto(checkpoint.GetEngine())
	if !ok {
		return nil, ErrProtocol
	}
	return &collaboration.Checkpoint{
		Engine: engine, EngineVersion: checkpoint.GetEngineVersion(), FormatVersion: checkpoint.GetFormatVersion(),
		SHA256: checkpoint.GetSha256(), SizeBytes: checkpoint.GetSizeBytes(), Payload: append([]byte(nil), checkpoint.GetPayload()...),
	}, nil
}

func mapEngineToProto(engine collaboration.Engine) (collaborationruntimev1.CollaborationEngine, error) {
	switch engine {
	case collaboration.EngineNative:
		return collaborationruntimev1.CollaborationEngine_COLLABORATION_ENGINE_NATIVE, nil
	case collaboration.EngineAutoGen:
		return collaborationruntimev1.CollaborationEngine_COLLABORATION_ENGINE_AUTOGEN, nil
	default:
		return collaborationruntimev1.CollaborationEngine_COLLABORATION_ENGINE_UNSPECIFIED, fmt.Errorf("%w: unsupported Engine", ErrInvalidRequest)
	}
}

func mapEngineFromProto(engine collaborationruntimev1.CollaborationEngine) (collaboration.Engine, bool) {
	switch engine {
	case collaborationruntimev1.CollaborationEngine_COLLABORATION_ENGINE_NATIVE:
		return collaboration.EngineNative, true
	case collaborationruntimev1.CollaborationEngine_COLLABORATION_ENGINE_AUTOGEN:
		return collaboration.EngineAutoGen, true
	default:
		return "", false
	}
}

func mapTriggerModeToProto(mode collaboration.TriggerMode) (collaborationruntimev1.TriggerMode, error) {
	switch mode {
	case collaboration.TriggerMentionOnly:
		return collaborationruntimev1.TriggerMode_TRIGGER_MODE_MENTION_ONLY, nil
	case collaboration.TriggerAutomatic:
		return collaborationruntimev1.TriggerMode_TRIGGER_MODE_AUTOMATIC, nil
	default:
		return collaborationruntimev1.TriggerMode_TRIGGER_MODE_UNSPECIFIED, fmt.Errorf("%w: unsupported trigger mode", ErrInvalidRequest)
	}
}

func mapSenderTypeToProto(senderType collaboration.SenderType) collaborationruntimev1.SenderType {
	switch senderType {
	case collaboration.SenderHuman:
		return collaborationruntimev1.SenderType_SENDER_TYPE_HUMAN
	case collaboration.SenderAgent:
		return collaborationruntimev1.SenderType_SENDER_TYPE_AGENT
	case collaboration.SenderSystem:
		return collaborationruntimev1.SenderType_SENDER_TYPE_SYSTEM
	default:
		return collaborationruntimev1.SenderType_SENDER_TYPE_UNSPECIFIED
	}
}

func mapStopReason(reason collaborationruntimev1.CollaborationStopReason) (collaboration.StopReason, bool) {
	values := map[collaborationruntimev1.CollaborationStopReason]collaboration.StopReason{
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_COMPLETED:           collaboration.StopReasonCompleted,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_MAX_TURNS:           collaboration.StopReasonMaxTurns,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_MAX_TURNS_PER_AGENT: collaboration.StopReasonMaxTurnsPerAgent,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_EMPTY_OUTPUT:        collaboration.StopReasonEmptyOutput,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_DUPLICATE_OUTPUT:    collaboration.StopReasonDuplicateOutput,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_NO_ELIGIBLE_AGENT:   collaboration.StopReasonNoEligibleAgent,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_CANCELLED:           collaboration.StopReasonCancelled,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_DEADLINE_EXCEEDED:   collaboration.StopReasonDeadlineExceeded,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_INTERRUPTED:         collaboration.StopReasonInterrupted,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_ENGINE_FAILURE:      collaboration.StopReasonEngineFailure,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_PROTOCOL_ERROR:      collaboration.StopReasonProtocolError,
	}
	value, ok := values[reason]
	return value, ok
}

func mapErrorCode(code collaborationruntimev1.CollaborationErrorCode) collaboration.ErrorCode {
	values := map[collaborationruntimev1.CollaborationErrorCode]collaboration.ErrorCode{
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_INVALID_REQUEST:             collaboration.ErrorInvalidRequest,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_UNSUPPORTED_VERSION:         collaboration.ErrorUnsupportedVersion,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_ENGINE_UNAVAILABLE:          collaboration.ErrorEngineUnavailable,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_RESOURCE_EXHAUSTED:          collaboration.ErrorResourceExhausted,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_DUPLICATE_RUN:               collaboration.ErrorDuplicateRun,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_ROOM_BUSY:                   collaboration.ErrorRoomBusy,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_MODEL_NOT_CONFIGURED:        collaboration.ErrorModelNotConfigured,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_MODEL_AUTHENTICATION_FAILED: collaboration.ErrorModelAuthenticationFailed,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_MODEL_RATE_LIMITED:          collaboration.ErrorModelRateLimited,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_MODEL_TIMEOUT:               collaboration.ErrorModelTimeout,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_TOOL_FAILED:                 collaboration.ErrorToolFailed,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_OUTPUT_INVALID:              collaboration.ErrorOutputInvalid,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_CHECKPOINT_INVALID:          collaboration.ErrorCheckpointInvalid,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_PROTOCOL_ERROR:              collaboration.ErrorProtocol,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_CANCELLED:                   collaboration.ErrorCancelled,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_DEADLINE_EXCEEDED:           collaboration.ErrorDeadlineExceeded,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_INTERNAL:                    collaboration.ErrorInternal,
	}
	if value, ok := values[code]; ok {
		return value
	}
	return collaboration.ErrorInternal
}
