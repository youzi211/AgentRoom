package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/collaboration"
	"agentroom/backend/internal/logging"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/realtime"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/store"
)

var ErrInvalidRemoteCollaboration = errors.New("invalid remote collaboration configuration")

type RemoteCollaborationConfig struct {
	Limits          collaboration.ExecutionLimits
	TranscriptLimit int
	EngineVersions  map[string]string
}

type collaborationExecutor interface {
	Execute(context.Context, collaboration.Request, collaboration.EventHandler) error
}

type remoteCollaborationStore interface {
	CreateCollaborationRun(context.Context, store.CollaborationRun) error
	StartCollaborationRun(context.Context, string, string, time.Time) error
	FinishCollaborationRun(context.Context, store.FinishCollaborationRunInput) error
	CreateAgentRun(context.Context, store.AgentRun) error
	CommitAgentRunSuccess(context.Context, store.CommitAgentRunSuccessInput) (model.Message, error)
	FinishAgentRun(context.Context, string, string, string, time.Time) error
}

type collaborationModelResolver interface {
	Resolve(context.Context, string, string) (ResolvedModelConfig, error)
}

type collaborationKnowledgeProvider interface {
	SearchForAgent(context.Context, string, string, string) ([]model.KnowledgeChunk, error)
}

type CollaborationScheduler interface {
	HandleHumanMessage(context.Context, *room.Room, model.Message) error
}

type RemoteCollaborationScheduler struct {
	coordinator     collaborationExecutor
	store           remoteCollaborationStore
	modelResolver   collaborationModelResolver
	knowledge       collaborationKnowledgeProvider
	limits          collaboration.ExecutionLimits
	transcriptLimit int
	engineVersions  map[string]string
	logger          *slog.Logger
}

func NewRemoteCollaborationScheduler(
	coordinator collaborationExecutor,
	store remoteCollaborationStore,
	modelResolver collaborationModelResolver,
	knowledge collaborationKnowledgeProvider,
	config RemoteCollaborationConfig,
) (*RemoteCollaborationScheduler, error) {
	if coordinator == nil || store == nil || modelResolver == nil {
		return nil, fmt.Errorf("%w: coordinator, store, and model resolver are required", ErrInvalidRemoteCollaboration)
	}
	if config.TranscriptLimit <= 0 {
		return nil, fmt.Errorf("%w: transcript limit must be positive", ErrInvalidRemoteCollaboration)
	}
	if err := validateCollaborationLimits(config.Limits); err != nil {
		return nil, err
	}
	engineVersions := make(map[string]string, len(config.EngineVersions))
	for engine, version := range config.EngineVersions {
		engine = strings.ToLower(strings.TrimSpace(engine))
		version = strings.TrimSpace(version)
		if !model.IsValidCollaborationEngine(engine) || version == "" {
			return nil, fmt.Errorf("%w: every Engine requires a version", ErrInvalidRemoteCollaboration)
		}
		engineVersions[engine] = version
	}
	return &RemoteCollaborationScheduler{
		coordinator: coordinator, store: store, modelResolver: modelResolver, knowledge: knowledge,
		limits: config.Limits, transcriptLimit: config.TranscriptLimit, engineVersions: engineVersions,
		logger: logging.Component("collaboration_scheduler"),
	}, nil
}

func (s *RemoteCollaborationScheduler) HandleHumanMessage(ctx context.Context, currentRoom *room.Room, trigger model.Message) error {
	if currentRoom == nil || trigger.SenderType != model.SenderTypeHuman {
		return fmt.Errorf("%w: room and human trigger are required", ErrInvalidRemoteCollaboration)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	roomInfo := currentRoom.Info()
	policy := roomInfo.CollaborationPolicy.WithDefaults()
	if err := policy.Validate(); err != nil {
		return err
	}
	engineVersion := strings.TrimSpace(s.engineVersions[policy.Engine])
	if engineVersion == "" {
		return fmt.Errorf("%w: Engine %q is not configured", ErrInvalidRemoteCollaboration, policy.Engine)
	}

	agents := eligibleCollaborationAgents(currentRoom.AgentsWithPrompts())
	mentioned := agent.MentionedAgents(trigger, agents)
	if policy.TriggerMode == model.CollaborationTriggerMentionOnly && len(mentioned) == 0 {
		return nil
	}

	runID := model.NewID("collaboration")
	createdAt := time.Now().UTC()
	if err := s.store.CreateCollaborationRun(ctx, store.CollaborationRun{
		ID: runID, RoomID: roomInfo.ID, RootMessageID: trigger.ID, Engine: policy.Engine,
		PolicyVersion: model.CollaborationPolicyVersion, Status: model.CollaborationRunStatusCreated, CreatedAt: createdAt,
	}); err != nil {
		return err
	}

	request, err := s.buildRequest(ctx, currentRoom, trigger, runID, policy, agents, mentioned)
	if err != nil {
		return errors.Join(err, s.failPreparation(ctx, runID, createdAt))
	}
	lifecycle, err := collaboration.NewLifecycle(s.store, request, engineVersion)
	if err != nil {
		return errors.Join(err, s.failPreparation(ctx, runID, createdAt))
	}
	turns, err := collaboration.NewTurnHandler(s.store, request)
	if err != nil {
		return errors.Join(err, s.failPreparation(ctx, runID, createdAt))
	}

	executionErr := s.coordinator.Execute(ctx, request, func(eventCtx context.Context, event collaboration.Event) error {
		message, committed, err := turns.Handle(eventCtx, event)
		if err != nil {
			return err
		}
		if err := lifecycle.Handle(eventCtx, event); err != nil {
			return err
		}
		if activity, ok := collaborationActivityForEvent(request, event); ok {
			currentRoom.Broadcaster().BroadcastEvent(realtime.Event{
				Type:          realtime.EventTypeCollaborationActivity,
				Collaboration: &activity,
			})
		}
		if committed {
			currentRoom.AppendMessage(message)
			currentRoom.Broadcaster().BroadcastMessage(message)
		}
		return nil
	})
	convergeErr := lifecycle.Converge(ctx, executionErr)
	return errors.Join(executionErr, convergeErr)
}

func collaborationActivityForEvent(request collaboration.Request, event collaboration.Event) (realtime.CollaborationActivity, bool) {
	switch event.Kind {
	case collaboration.EventCollaborationStarted,
		collaboration.EventSpeakerSelected,
		collaboration.EventAgentTurnStarted,
		collaboration.EventModelStarted,
		collaboration.EventModelCompleted,
		collaboration.EventToolStarted,
		collaboration.EventToolCompleted,
		collaboration.EventToolFailed,
		collaboration.EventArtifactReady,
		collaboration.EventHandoffRequested,
		collaboration.EventAgentMessageCompleted,
		collaboration.EventCompleted,
		collaboration.EventStopped,
		collaboration.EventCancelled,
		collaboration.EventFailed:
	default:
		return realtime.CollaborationActivity{}, false
	}

	agentNames := make(map[string]string, len(request.Snapshot.Agents))
	for _, currentAgent := range request.Snapshot.Agents {
		agentNames[currentAgent.ID] = currentAgent.Name
	}
	activity := realtime.CollaborationActivity{
		Kind:               string(event.Kind),
		CollaborationRunID: event.CollaborationRunID,
		Sequence:           event.Sequence,
		RoomID:             request.Snapshot.Room.ID,
		TriggerMessageID:   request.Snapshot.Trigger.ID,
		TurnID:             event.TurnID,
		AgentID:            event.AgentID,
		AgentName:          agentNames[event.AgentID],
		ReasonCategory:     event.ReasonCategory,
		OccurredAt:         event.OccurredAt,
	}
	if event.Tool != nil {
		activity.ToolName = event.Tool.Name
		if event.Tool.Failure != nil {
			activity.ErrorCode = string(event.Tool.Failure.Code)
		}
	}
	if event.Artifact != nil {
		activity.ArtifactID = event.Artifact.ID
	}
	if event.Handoff != nil {
		activity.TargetAgentID = event.Handoff.TargetAgentID
		activity.TargetAgentName = agentNames[event.Handoff.TargetAgentID]
		activity.ReasonCategory = event.Handoff.ReasonCategory
	}
	if event.Terminal != nil {
		activity.StopReason = string(event.Terminal.Reason)
		activity.TurnCount = event.Terminal.TurnCount
		if event.Terminal.Failure != nil {
			activity.ErrorCode = string(event.Terminal.Failure.Code)
		}
	}
	return activity, true
}

func (s *RemoteCollaborationScheduler) buildRequest(
	ctx context.Context,
	currentRoom *room.Room,
	trigger model.Message,
	runID string,
	policy model.CollaborationPolicy,
	agents []model.Agent,
	mentioned []model.Agent,
) (collaboration.Request, error) {
	bindings := make([]collaboration.AgentBinding, 0, len(agents))
	for _, currentAgent := range agents {
		scope := model.RuntimeScopeForAgent(currentAgent.Runtime)
		resolved, err := s.modelResolver.Resolve(ctx, scope, currentAgent.ModelProfileID)
		if err != nil {
			return collaboration.Request{}, err
		}
		profileID := strings.TrimSpace(resolved.ProfileID)
		if profileID == "" {
			profileID = "environment:" + scope
		}
		bindings = append(bindings, collaboration.AgentBinding{
			Agent: currentAgent,
			ModelReference: collaboration.ModelReference{
				ID: "model_ref_" + currentAgent.ID, ProfileID: profileID, Source: resolved.Source,
				Protocol: model.ModelProtocolOpenAIChatCompletions, ModelName: resolved.ModelName, RuntimeScope: scope,
			},
		})
	}
	candidateIDs := make([]string, 0, len(mentioned))
	for _, candidate := range mentioned {
		candidateIDs = append(candidateIDs, candidate.ID)
	}
	return collaboration.Build(collaboration.Input{
		CollaborationRunID:       runID,
		TraceID:                  runID,
		Room:                     currentRoom.Info(),
		Agents:                   bindings,
		Trigger:                  trigger,
		Transcript:               currentRoom.RecentMessages(s.transcriptLimit),
		KnowledgeChunks:          s.loadKnowledge(ctx, currentRoom.Info().ID, agents, trigger.Content),
		Policy:                   policy,
		Limits:                   s.limits,
		InitialCandidateAgentIDs: candidateIDs,
	})
}

func (s *RemoteCollaborationScheduler) loadKnowledge(ctx context.Context, roomID string, agents []model.Agent, query string) []model.KnowledgeChunk {
	if s.knowledge == nil {
		return nil
	}
	chunks := make([]model.KnowledgeChunk, 0)
	seen := make(map[string]struct{})
	for _, currentAgent := range agents {
		found, err := s.knowledge.SearchForAgent(ctx, roomID, currentAgent.ID, query)
		if err != nil {
			s.logger.Warn("search collaboration knowledge", "room_id", roomID, "agent_id", currentAgent.ID, "error", err)
			continue
		}
		for _, chunk := range found {
			key := chunk.ID
			if key == "" {
				key = chunk.Scope + "\x00" + chunk.ScopeID + "\x00" + chunk.DocumentID + "\x00" + fmt.Sprint(chunk.ChunkIndex)
			}
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}
			chunks = append(chunks, chunk)
		}
	}
	return chunks
}

func (s *RemoteCollaborationScheduler) failPreparation(ctx context.Context, runID string, completedAt time.Time) error {
	return s.store.FinishCollaborationRun(context.WithoutCancel(ctx), store.FinishCollaborationRunInput{
		RunID: runID, Status: model.CollaborationRunStatusFailed,
		StopReason: model.CollaborationStopReasonEngineFailure, Error: "collaboration request preparation failed",
		CompletedAt: completedAt,
	})
}

func eligibleCollaborationAgents(agents []model.Agent) []model.Agent {
	result := make([]model.Agent, 0, len(agents))
	for _, currentAgent := range agents {
		if currentAgent.Enabled && model.IsValidAgentRuntime(currentAgent.Runtime) {
			result = append(result, currentAgent)
		}
	}
	return result
}

func validateCollaborationLimits(limits collaboration.ExecutionLimits) error {
	if limits.Timeout <= 0 || limits.MaxOutputBytes == 0 || limits.MaxArtifactBytes == 0 || limits.MaxToolSteps == 0 ||
		limits.MaxRequestBytes == 0 || limits.MaxEventBytes == 0 || limits.MaxCheckpointBytes == 0 {
		return fmt.Errorf("%w: execution limits must be positive", ErrInvalidRemoteCollaboration)
	}
	return nil
}
