package collaboration

import (
	"fmt"
	collaborationruntimev1 "agentroom/backend/internal/collaborationproto/v1"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func mapRequest(request Request) (*collaborationruntimev1.ExecuteConversationRequest, error) {
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
			ModelSelectionId: agent.ModelSelectionID, ToolNames: append([]string(nil), agent.ToolNames...),
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
	for _, selection := range request.Snapshot.ModelSelections {
		mapped.Snapshot.ModelSelections = append(mapped.Snapshot.ModelSelections, &collaborationruntimev1.ModelSelection{
			Id: selection.ID, ProfileId: selection.ProfileID, Source: selection.Source,
			Protocol: selection.Protocol, ModelName: selection.ModelName, RuntimeScope: selection.RuntimeScope,
			CredentialRef: selection.CredentialRef, Purpose: collaborationruntimev1.ModelSelectionPurpose(collaborationruntimev1.ModelSelectionPurpose_value[string(selection.Purpose)]),
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

func mapMessageToProto(message MessageSnapshot) *collaborationruntimev1.MessageSnapshot {
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

func mapCheckpointToProto(checkpoint Checkpoint) (*collaborationruntimev1.OpaqueCheckpoint, error) {
	engine, err := mapEngineToProto(checkpoint.Engine)
	if err != nil {
		return nil, err
	}
	return &collaborationruntimev1.OpaqueCheckpoint{
		Engine: engine, EngineVersion: checkpoint.EngineVersion, FormatVersion: checkpoint.FormatVersion,
		Sha256: checkpoint.SHA256, SizeBytes: checkpoint.SizeBytes, Payload: append([]byte(nil), checkpoint.Payload...),
	}, nil
}

func mapEvent(event *collaborationruntimev1.CollaborationEvent) (Event, error) {
	if event == nil || event.GetPayload() == nil {
		return Event{}, ErrProtocol
	}
	mapped := Event{
		ProtocolVersion: event.GetProtocolVersion(), CollaborationRunID: event.GetCollaborationRunId(),
		Sequence: event.GetSequence(), TurnID: event.GetTurnId(), AgentID: event.GetAgentId(),
	}
	if event.GetOccurredAt() != nil {
		mapped.OccurredAt = event.GetOccurredAt().AsTime()
	}
	switch payload := event.GetPayload().(type) {
	case *collaborationruntimev1.CollaborationEvent_Accepted:
		mapped.Kind = EventAccepted
	case *collaborationruntimev1.CollaborationEvent_CollaborationStarted:
		mapped.Kind = EventCollaborationStarted
	case *collaborationruntimev1.CollaborationEvent_SpeakerSelected:
		mapped.Kind, mapped.ReasonCategory = EventSpeakerSelected, payload.SpeakerSelected.GetReasonCategory()
	case *collaborationruntimev1.CollaborationEvent_AgentTurnStarted:
		mapped.Kind = EventAgentTurnStarted
	case *collaborationruntimev1.CollaborationEvent_ModelStarted:
		mapped.Kind, mapped.ModelSelectionID = EventModelStarted, payload.ModelStarted.GetModelSelectionId()
	case *collaborationruntimev1.CollaborationEvent_ModelCompleted:
		mapped.Kind, mapped.ModelSelectionID = EventModelCompleted, payload.ModelCompleted.GetModelSelectionId()
		mapped.Usage = mapUsage(payload.ModelCompleted.GetUsage())
	case *collaborationruntimev1.CollaborationEvent_ToolStarted:
		mapped.Kind = EventToolStarted
		mapped.Tool = &ToolActivity{CallID: payload.ToolStarted.GetToolCallId(), Name: payload.ToolStarted.GetToolName(), InputSummary: payload.ToolStarted.GetInputSummary()}
	case *collaborationruntimev1.CollaborationEvent_ToolCompleted:
		mapped.Kind = EventToolCompleted
		mapped.Tool = &ToolActivity{CallID: payload.ToolCompleted.GetToolCallId(), Name: payload.ToolCompleted.GetToolName(), OutputSummary: payload.ToolCompleted.GetOutputSummary()}
	case *collaborationruntimev1.CollaborationEvent_ToolFailed:
		mapped.Kind = EventToolFailed
		mapped.Tool = &ToolActivity{CallID: payload.ToolFailed.GetToolCallId(), Name: payload.ToolFailed.GetToolName(), Failure: mapFailure(payload.ToolFailed.GetFailure())}
	case *collaborationruntimev1.CollaborationEvent_OutputDelta:
		mapped.Kind, mapped.OutputDelta = EventOutputDelta, payload.OutputDelta.GetText()
	case *collaborationruntimev1.CollaborationEvent_ArtifactReady:
		mapped.Kind, mapped.Artifact = EventArtifactReady, mapArtifact(payload.ArtifactReady.GetArtifact())
	case *collaborationruntimev1.CollaborationEvent_HandoffRequested:
		mapped.Kind = EventHandoffRequested
		mapped.Handoff = &Handoff{TargetAgentID: payload.HandoffRequested.GetTargetAgentId(), ReasonCategory: payload.HandoffRequested.GetReasonCategory()}
	case *collaborationruntimev1.CollaborationEvent_AgentMessageCompleted:
		mapped.Kind, mapped.Message = EventAgentMessageCompleted, mapAgentMessage(payload.AgentMessageCompleted)
	case *collaborationruntimev1.CollaborationEvent_Checkpoint:
		mapped.Kind = EventCheckpoint
		checkpoint, err := mapCheckpointFromProto(payload.Checkpoint.GetCheckpoint())
		if err != nil {
			return Event{}, err
		}
		mapped.Checkpoint = checkpoint
	case *collaborationruntimev1.CollaborationEvent_Completed:
		terminal, err := mapTerminal(payload.Completed.GetTurnCount(), payload.Completed.GetReason(), nil)
		if err != nil {
			return Event{}, err
		}
		mapped.Kind, mapped.Terminal = EventCompleted, terminal
	case *collaborationruntimev1.CollaborationEvent_Stopped:
		terminal, err := mapTerminal(payload.Stopped.GetTurnCount(), payload.Stopped.GetReason(), nil)
		if err != nil {
			return Event{}, err
		}
		mapped.Kind, mapped.Terminal = EventStopped, terminal
	case *collaborationruntimev1.CollaborationEvent_Cancelled:
		terminal, err := mapTerminal(payload.Cancelled.GetTurnCount(), payload.Cancelled.GetReason(), nil)
		if err != nil {
			return Event{}, err
		}
		mapped.Kind, mapped.Terminal = EventCancelled, terminal
	case *collaborationruntimev1.CollaborationEvent_Failed:
		terminal, err := mapTerminal(payload.Failed.GetTurnCount(), payload.Failed.GetReason(), payload.Failed.GetFailure())
		if err != nil {
			return Event{}, err
		}
		mapped.Kind, mapped.Terminal = EventFailed, terminal
	default:
		return Event{}, ErrProtocol
	}
	return mapped, nil
}

func mapAgentMessage(message *collaborationruntimev1.AgentMessageCompletedEvent) *AgentMessage {
	if message == nil {
		return nil
	}
	mapped := &AgentMessage{
		Content: message.GetContent(), Model: mapModelAudit(message.GetModel()), Usage: mapUsage(message.GetUsage()),
	}
	for _, artifact := range message.GetArtifacts() {
		if value := mapArtifact(artifact); value != nil {
			mapped.Artifacts = append(mapped.Artifacts, *value)
		}
	}
	for _, source := range message.GetKnowledgeSources() {
		mapped.KnowledgeSources = append(mapped.KnowledgeSources, KnowledgeSource{
			DocumentID: source.GetDocumentId(), DocumentName: source.GetDocumentName(), Scope: source.GetScope(),
		})
	}
	return mapped
}

func mapArtifact(artifact *collaborationruntimev1.Artifact) *Artifact {
	if artifact == nil {
		return nil
	}
	return &Artifact{
		ID: artifact.GetId(), Type: artifact.GetType(), Title: artifact.GetTitle(), FileName: artifact.GetFileName(),
		MIMEType: artifact.GetMimeType(), Content: append([]byte(nil), artifact.GetContent()...), ExternalURI: artifact.GetExternalUri(),
	}
}

func mapModelAudit(audit *collaborationruntimev1.ModelAudit) ModelAudit {
	if audit == nil {
		return ModelAudit{}
	}
	return ModelAudit{
		ModelSelectionID: audit.GetModelSelectionId(), ProfileID: audit.GetProfileId(),
		Source: audit.GetSource(), ModelName: audit.GetModelName(),
	}
}

func mapUsage(usage *collaborationruntimev1.Usage) Usage {
	if usage == nil {
		return Usage{}
	}
	return Usage{InputTokens: usage.GetInputTokens(), OutputTokens: usage.GetOutputTokens(), TotalTokens: usage.GetTotalTokens()}
}

func mapFailure(failure *collaborationruntimev1.CollaborationFailure) *Failure {
	if failure == nil {
		return nil
	}
	code := mapErrorCode(failure.GetCode())
	return &Failure{Code: code, Message: safeFailureMessage(code), Retryable: failure.GetRetryable()}
}

func safeFailureMessage(code ErrorCode) string {
	messages := map[ErrorCode]string{
		ErrorInvalidRequest:            "Collaboration request is invalid",
		ErrorUnsupportedVersion:        "Collaboration protocol version is unsupported",
		ErrorEngineUnavailable:         "Collaboration Engine is unavailable",
		ErrorResourceExhausted:         "Collaboration resource limit exceeded",
		ErrorDuplicateRun:              "Collaboration run is already active",
		ErrorRoomBusy:                  "Collaboration room already has an active run",
		ErrorModelNotConfigured:        "Collaboration model is not configured",
		ErrorModelAuthenticationFailed: "Collaboration model authentication failed",
		ErrorModelRateLimited:          "Collaboration model rate limit exceeded",
		ErrorModelTimeout:              "Collaboration model timed out",
		ErrorToolFailed:                "Collaboration tool failed",
		ErrorOutputInvalid:             "Collaboration output is invalid",
		ErrorCheckpointInvalid:         "Collaboration checkpoint is invalid",
		ErrorProtocol:                  "Collaboration Engine violated the event protocol",
		ErrorCancelled:                 "Collaboration was cancelled",
		ErrorDeadlineExceeded:          "Collaboration deadline exceeded",
	}
	if message, ok := messages[code]; ok {
		return message
	}
	return "Collaboration Engine failed"
}

func mapTerminal(turnCount uint32, reason collaborationruntimev1.CollaborationStopReason, failure *collaborationruntimev1.CollaborationFailure) (*Terminal, error) {
	mappedReason, ok := mapStopReason(reason)
	if !ok {
		return nil, ErrProtocol
	}
	return &Terminal{TurnCount: turnCount, Reason: mappedReason, Failure: mapFailure(failure)}, nil
}

func mapCheckpointFromProto(checkpoint *collaborationruntimev1.OpaqueCheckpoint) (*Checkpoint, error) {
	if checkpoint == nil {
		return nil, ErrProtocol
	}
	engine, ok := mapEngineFromProto(checkpoint.GetEngine())
	if !ok {
		return nil, ErrProtocol
	}
	return &Checkpoint{
		Engine: engine, EngineVersion: checkpoint.GetEngineVersion(), FormatVersion: checkpoint.GetFormatVersion(),
		SHA256: checkpoint.GetSha256(), SizeBytes: checkpoint.GetSizeBytes(), Payload: append([]byte(nil), checkpoint.GetPayload()...),
	}, nil
}

func mapEngineToProto(engine Engine) (collaborationruntimev1.CollaborationEngine, error) {
	switch engine {
	case EngineNative:
		return collaborationruntimev1.CollaborationEngine_COLLABORATION_ENGINE_NATIVE, nil
	case EngineAutoGen:
		return collaborationruntimev1.CollaborationEngine_COLLABORATION_ENGINE_AUTOGEN, nil
	default:
		return collaborationruntimev1.CollaborationEngine_COLLABORATION_ENGINE_UNSPECIFIED, fmt.Errorf("%w: unsupported Engine", ErrInvalidRequest)
	}
}

func mapEngineFromProto(engine collaborationruntimev1.CollaborationEngine) (Engine, bool) {
	switch engine {
	case collaborationruntimev1.CollaborationEngine_COLLABORATION_ENGINE_NATIVE:
		return EngineNative, true
	case collaborationruntimev1.CollaborationEngine_COLLABORATION_ENGINE_AUTOGEN:
		return EngineAutoGen, true
	default:
		return "", false
	}
}

func mapTriggerModeToProto(mode TriggerMode) (collaborationruntimev1.TriggerMode, error) {
	switch mode {
	case TriggerMentionOnly:
		return collaborationruntimev1.TriggerMode_TRIGGER_MODE_MENTION_ONLY, nil
	case TriggerAutomatic:
		return collaborationruntimev1.TriggerMode_TRIGGER_MODE_AUTOMATIC, nil
	default:
		return collaborationruntimev1.TriggerMode_TRIGGER_MODE_UNSPECIFIED, fmt.Errorf("%w: unsupported trigger mode", ErrInvalidRequest)
	}
}

func mapSenderTypeToProto(senderType SenderType) collaborationruntimev1.SenderType {
	switch senderType {
	case SenderHuman:
		return collaborationruntimev1.SenderType_SENDER_TYPE_HUMAN
	case SenderAgent:
		return collaborationruntimev1.SenderType_SENDER_TYPE_AGENT
	case SenderSystem:
		return collaborationruntimev1.SenderType_SENDER_TYPE_SYSTEM
	default:
		return collaborationruntimev1.SenderType_SENDER_TYPE_UNSPECIFIED
	}
}

func mapStopReason(reason collaborationruntimev1.CollaborationStopReason) (StopReason, bool) {
	values := map[collaborationruntimev1.CollaborationStopReason]StopReason{
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_COMPLETED:           StopReasonCompleted,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_MAX_TURNS:           StopReasonMaxTurns,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_MAX_TURNS_PER_AGENT: StopReasonMaxTurnsPerAgent,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_EMPTY_OUTPUT:        StopReasonEmptyOutput,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_DUPLICATE_OUTPUT:    StopReasonDuplicateOutput,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_NO_ELIGIBLE_AGENT:   StopReasonNoEligibleAgent,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_CANCELLED:           StopReasonCancelled,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_DEADLINE_EXCEEDED:   StopReasonDeadlineExceeded,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_INTERRUPTED:         StopReasonInterrupted,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_ENGINE_FAILURE:      StopReasonEngineFailure,
		collaborationruntimev1.CollaborationStopReason_COLLABORATION_STOP_REASON_PROTOCOL_ERROR:      StopReasonProtocolError,
	}
	value, ok := values[reason]
	return value, ok
}

func mapErrorCode(code collaborationruntimev1.CollaborationErrorCode) ErrorCode {
	values := map[collaborationruntimev1.CollaborationErrorCode]ErrorCode{
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_INVALID_REQUEST:             ErrorInvalidRequest,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_UNSUPPORTED_VERSION:         ErrorUnsupportedVersion,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_ENGINE_UNAVAILABLE:          ErrorEngineUnavailable,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_RESOURCE_EXHAUSTED:          ErrorResourceExhausted,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_DUPLICATE_RUN:               ErrorDuplicateRun,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_ROOM_BUSY:                   ErrorRoomBusy,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_MODEL_NOT_CONFIGURED:        ErrorModelNotConfigured,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_MODEL_AUTHENTICATION_FAILED: ErrorModelAuthenticationFailed,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_MODEL_RATE_LIMITED:          ErrorModelRateLimited,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_MODEL_TIMEOUT:               ErrorModelTimeout,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_TOOL_FAILED:                 ErrorToolFailed,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_OUTPUT_INVALID:              ErrorOutputInvalid,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_CHECKPOINT_INVALID:          ErrorCheckpointInvalid,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_PROTOCOL_ERROR:              ErrorProtocol,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_CANCELLED:                   ErrorCancelled,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_DEADLINE_EXCEEDED:           ErrorDeadlineExceeded,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_INTERNAL:                    ErrorInternal,
	}
	if value, ok := values[code]; ok {
		return value
	}
	return ErrorInternal
}
