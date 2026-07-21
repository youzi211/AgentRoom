package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/logging"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/realtime"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/store"
)

type RuntimeRoom interface {
	Info() model.RoomMeta
	Participants() []model.Participant
	Agents() []model.Agent
	AgentsWithPrompts() []model.Agent
	RecentMessages(limit int) []model.Message
	NewSystemMessage(content string) model.Message
	NewAgentMessage(agent model.Agent, content string) model.Message
	AppendMessage(message model.Message)
	Broadcaster() room.MessageBroadcaster
}

type Runner struct {
	store        runnerStore
	knowledge    KnowledgeProvider
	runtimes     *RuntimeRegistry
	logger       *slog.Logger
	contextLimit int
	timeout      time.Duration
}

type runnerStore interface {
	AddMessage(ctx context.Context, message model.Message) (model.Message, error)
	CreateAgentRun(ctx context.Context, run store.AgentRun) error
	CommitAgentRunSuccess(ctx context.Context, input store.CommitAgentRunSuccessInput) (model.Message, error)
	FinishAgentRun(ctx context.Context, runID string, status string, errText string, completedAt time.Time) error
	ListAgentRuns(ctx context.Context, query store.ListRunsQuery) ([]store.AgentRun, error)
	CreateDialogueRun(ctx context.Context, run store.DialogueRun) error
	FinishDialogueRun(ctx context.Context, runID string, status string, turnCount int, completedAt time.Time) error
	ListDialogueRuns(ctx context.Context, query store.ListRunsQuery) ([]store.DialogueRun, error)
}

type KnowledgeProvider interface {
	SearchForAgent(ctx context.Context, roomID string, agentID string, query string) ([]model.KnowledgeChunk, error)
}

func NewRunner(client llm.Client, s runnerStore) *Runner {
	timeout := 45 * time.Second
	return &Runner{
		store:        s,
		runtimes:     NewRuntimeRegistry(NewLLMAgentRuntime(client, timeout)),
		logger:       logging.Component("agent_runner"),
		contextLimit: 30,
		timeout:      timeout,
	}
}

func (r *Runner) WithKnowledge(provider KnowledgeProvider) *Runner {
	r.knowledge = provider
	return r
}

func (r *Runner) WithRuntimeRegistry(registry *RuntimeRegistry) *Runner {
	r.runtimes = registry
	return r
}

func (r *Runner) CancelRoom(roomID string) int {
	if r == nil || r.runtimes == nil {
		return 0
	}
	return r.runtimes.CancelRoom(roomID)
}

func (r *Runner) HandleHumanMessage(ctx context.Context, currentRoom RuntimeRoom, message model.Message) {
	defer func() {
		if recovered := recover(); recovered != nil {
			r.logger.Error("agent runner panic recovered", "error", recovered)
			content := fmt.Sprintf("Agent runner failed: %v", recovered)
			sysMsg := currentRoom.NewSystemMessage(content)
			r.persistAndBroadcast(ctx, currentRoom, sysMsg)
		}
	}()

	policy := currentRoom.Info().DialoguePolicy.WithDefaults()
	if policy.IsGuided() {
		r.handleGuidedDialogue(ctx, currentRoom, message, policy)
		return
	}

	r.handleMentionFanout(ctx, currentRoom, message, policy)
}

func (r *Runner) handleMentionFanout(ctx context.Context, currentRoom RuntimeRoom, trigger model.Message, policy model.DialoguePolicy) {
	fullAgents := currentRoom.AgentsWithPrompts()
	agentByID := make(map[string]model.Agent, len(fullAgents))
	for _, candidate := range fullAgents {
		agentByID[candidate.ID] = candidate
	}

	pending := []model.Message{trigger}
	turnsByAgent := make(map[string]int, len(fullAgents))
	autonomousTurns := 0

	for len(pending) > 0 {
		currentTrigger := pending[0]
		pending = pending[1:]

		if currentTrigger.SenderType == model.SenderTypeAgent && autonomousTurns >= policy.MaxAutonomousTurns {
			return
		}

		responders := resolveMentionFanoutResponders(
			detectMentionFanoutResponders(currentTrigger, currentRoom.Agents(), policy),
			currentTrigger,
			turnsByAgent,
			policy,
			agentByID,
		)
		if len(responders) == 0 {
			continue
		}

		for _, responder := range responders {
			if currentTrigger.SenderType == model.SenderTypeAgent && autonomousTurns >= policy.MaxAutonomousTurns {
				return
			}

			agentMessage, ok := r.handleAgentResponse(ctx, currentRoom, responder, currentTrigger)
			if !ok {
				continue
			}

			turnsByAgent[responder.ID]++

			if currentTrigger.SenderType == model.SenderTypeAgent {
				autonomousTurns++
			}

			if !policy.AllowAgentToAgentMentions {
				continue
			}
			if currentTrigger.SenderType == model.SenderTypeAgent && autonomousTurns >= policy.MaxAutonomousTurns {
				continue
			}

			pending = append(pending, agentMessage)
		}
	}
}

func detectMentionFanoutResponders(trigger model.Message, agents []model.Agent, policy model.DialoguePolicy) []model.Agent {
	switch trigger.SenderType {
	case model.SenderTypeHuman:
		return MentionedAgents(trigger, agents)
	case model.SenderTypeAgent:
		if !policy.AllowAgentToAgentMentions {
			return nil
		}
		return DetectMentions(trigger.Content, agents)
	default:
		return nil
	}
}

func resolveMentionFanoutResponders(candidates []model.Agent, trigger model.Message, turnsByAgent map[string]int, policy model.DialoguePolicy, fullAgentByID map[string]model.Agent) []model.Agent {
	if len(candidates) == 0 {
		return nil
	}

	result := make([]model.Agent, 0, len(candidates))
	for _, candidate := range candidates {
		responder, ok := fullAgentByID[candidate.ID]
		if !ok {
			responder = candidate
		}
		if !policy.AllowSelfFollowup && trigger.SenderType == model.SenderTypeAgent && responder.ID == trigger.SenderID {
			continue
		}
		if turnsByAgent[responder.ID] >= policy.MaxTurnsPerAgent {
			continue
		}
		result = append(result, responder)
	}
	return result
}

func (r *Runner) handleAgentResponse(ctx context.Context, currentRoom RuntimeRoom, responder model.Agent, trigger model.Message) (model.Message, bool) {
	runID := model.NewID("run")
	roomInfo := currentRoom.Info()

	agentRun := store.AgentRun{
		ID:               runID,
		RoomID:           roomInfo.ID,
		AgentID:          responder.ID,
		TriggerMessageID: trigger.ID,
		Status:           "running",
		StartedAt:        time.Now().UTC(),
		ModelProfileID:   responder.ModelProfileID,
		ModelSource:      modelSourceForAgent(responder),
	}
	if err := r.store.CreateAgentRun(ctx, agentRun); err != nil {
		r.logger.Error("create agent run", "room_id", roomInfo.ID, "agent_id", responder.ID, "error", err)
		content := fmt.Sprintf("Agent %s could not start: run persistence failed", responder.Name)
		r.persistAndBroadcast(ctx, currentRoom, currentRoom.NewSystemMessage(content))
		return model.Message{}, false
	}
	r.broadcastAgentRunActivity(currentRoom, agentRun, responder, "started", "", "", nil)

	response, knowledgeChunks, err := r.generateResponse(ctx, currentRoom, responder, trigger, runID)
	now := time.Now().UTC()

	if err != nil {
		terminalCtx, cancelTerminal := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancelTerminal()
		r.recordModelAudit(terminalCtx, runID, response.Metadata)
		status := agentRunFailureStatus(ctx, err)
		errText := shortReason(err)
		content := fmt.Sprintf("Agent %s failed to respond: %s", responder.Name, errText)
		sysMsg := currentRoom.NewSystemMessage(content)
		r.persistAndBroadcast(terminalCtx, currentRoom, sysMsg)

		if finishErr := r.store.FinishAgentRun(terminalCtx, runID, status, errText, now); finishErr != nil {
			r.logger.Error("finish agent run", "run_id", runID, "status", status, "error", finishErr)
		}
		r.broadcastAgentRunActivity(currentRoom, agentRun, responder, "finished", status, errText, &now)
		return model.Message{}, false
	}

	agentMsg := currentRoom.NewAgentMessage(responder, response.Content)
	agentMsg.KnowledgeSources = knowledgeSourcesForResponse(response, knowledgeChunks)
	agentMsg.Artifacts = messageArtifactsFromRuntime(response.Artifacts)
	savedAgentMsg, commitErr := r.store.CommitAgentRunSuccess(ctx, commitAgentRunSuccessInput(runID, agentMsg, response.Metadata, now))
	if commitErr != nil {
		terminalCtx, cancelTerminal := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancelTerminal()
		errText := shortReason(commitErr)
		r.logger.Error("commit successful agent run", "run_id", runID, "error", commitErr)
		r.persistAndBroadcast(terminalCtx, currentRoom, currentRoom.NewSystemMessage(fmt.Sprintf("Agent %s response could not be saved", responder.Name)))
		if finishErr := r.store.FinishAgentRun(terminalCtx, runID, "failed", errText, now); finishErr != nil && !errors.Is(finishErr, store.ErrAgentRunAlreadyFinished) {
			r.logger.Error("finish agent run after commit failure", "run_id", runID, "error", finishErr)
		}
		r.broadcastAgentRunActivity(currentRoom, agentRun, responder, "finished", "failed", errText, &now)
		return model.Message{}, false
	}
	currentRoom.AppendMessage(savedAgentMsg)
	currentRoom.Broadcaster().BroadcastMessage(savedAgentMsg)
	r.broadcastAgentRunActivity(currentRoom, agentRun, responder, "finished", "succeeded", "", &now)
	return savedAgentMsg, true
}

func commitAgentRunSuccessInput(runID string, message model.Message, metadata map[string]string, completedAt time.Time) store.CommitAgentRunSuccessInput {
	return store.CommitAgentRunSuccessInput{
		RunID: runID, Message: message, CompletedAt: completedAt,
		ModelProfileID: metadata["model_profile_id"], ModelSource: metadata["model_source"], ModelName: metadata["model_name"],
	}
}

func agentRunFailureStatus(ctx context.Context, err error) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return "canceled"
	}
	return "failed"
}

func modelSourceForAgent(agent model.Agent) string {
	if agent.ModelProfileID != "" {
		return "database"
	}
	return ""
}

func (r *Runner) recordModelAudit(ctx context.Context, runID string, metadata map[string]string) {
	if len(metadata) == 0 || metadata["model_source"] == "" {
		return
	}
	auditStore, ok := r.store.(interface {
		UpdateAgentRunModel(context.Context, string, string, string, string) error
	})
	if !ok {
		return
	}
	if err := auditStore.UpdateAgentRunModel(ctx, runID, metadata["model_profile_id"], metadata["model_source"], metadata["model_name"]); err != nil {
		r.logger.Error("update agent run model audit", "run_id", runID, "error", err)
	}
}

func (r *Runner) broadcastAgentRunActivity(currentRoom RuntimeRoom, run store.AgentRun, responder model.Agent, phase string, status string, errText string, completedAt *time.Time) {
	currentRoom.Broadcaster().BroadcastEvent(realtime.Event{
		Type: realtime.EventTypeAgentActivity,
		Activity: &realtime.Activity{
			Kind:             "agent_run",
			Phase:            phase,
			ID:               run.ID,
			RoomID:           run.RoomID,
			AgentID:          responder.ID,
			AgentName:        responder.Name,
			TriggerMessageID: run.TriggerMessageID,
			Status:           status,
			ErrorText:        errText,
			CreatedAt:        run.StartedAt,
			CompletedAt:      completedAt,
		},
	})
}

func (r *Runner) runtimeEventObserver(currentRoom RuntimeRoom, responder model.Agent, runID string) AgentEventObserver {
	return AgentEventObserverFunc(func(_ context.Context, event AgentRuntimeEvent) {
		currentRoom.Broadcaster().BroadcastEvent(realtime.Event{
			Type: realtime.EventTypeAgentActivity,
			Activity: &realtime.Activity{
				Kind:         "agent_runtime",
				Phase:        event.Kind,
				RuntimeEvent: event.Kind,
				ID:           runID,
				RoomID:       currentRoom.Info().ID,
				AgentID:      responder.ID,
				AgentName:    responder.Name,
				ModelName:    event.ModelName,
				ToolName:     event.ToolName,
				ErrorText:    event.Failure,
				CreatedAt:    event.OccurredAt,
			},
		})
	})
}

func (r *Runner) broadcastDialogueRunActivity(currentRoom RuntimeRoom, run store.DialogueRun, phase string, status string, turnCount int, completedAt *time.Time) {
	currentRoom.Broadcaster().BroadcastEvent(realtime.Event{
		Type: realtime.EventTypeAgentActivity,
		Activity: &realtime.Activity{
			Kind:             "dialogue_run",
			Phase:            phase,
			ID:               run.ID,
			RoomID:           run.RoomID,
			TriggerMessageID: run.TriggerMessageID,
			Status:           status,
			TurnCount:        turnCount,
			CreatedAt:        run.StartedAt,
			CompletedAt:      completedAt,
		},
	})
}

func (r *Runner) persistAndBroadcast(ctx context.Context, currentRoom RuntimeRoom, message model.Message) model.Message {
	savedMsg, err := r.store.AddMessage(ctx, message)
	if err != nil {
		r.logger.Error("persist generated message", "message_id", message.ID, "sender_type", message.SenderType, "error", err)
		savedMsg = message
	}
	currentRoom.AppendMessage(savedMsg)
	currentRoom.Broadcaster().BroadcastMessage(savedMsg)
	return savedMsg
}

func (r *Runner) generateResponse(ctx context.Context, currentRoom RuntimeRoom, responder model.Agent, trigger model.Message, runID string) (AgentRuntimeResponse, []model.KnowledgeChunk, error) {
	knowledgeChunks := r.searchKnowledge(ctx, currentRoom, responder, trigger)
	recentMessages := currentRoom.RecentMessages(r.contextLimit)
	promptContext := NewMentionPromptContext(currentRoom, recentMessages, trigger, knowledgeChunks)
	runtime, err := r.runtimes.Resolve(responder)
	if err != nil {
		return AgentRuntimeResponse{}, knowledgeChunks, err
	}
	response, err := runtime.Respond(ctx, AgentRuntimeRequest{
		RunID:           runID,
		TraceID:         runID,
		Room:            currentRoom,
		Agent:           responder,
		Trigger:         trigger,
		RecentMessages:  recentMessages,
		KnowledgeChunks: knowledgeChunks,
		PromptContext:   promptContext,
	}, r.runtimeEventObserver(currentRoom, responder, runID))
	if err != nil {
		return AgentRuntimeResponse{}, knowledgeChunks, err
	}
	return response, knowledgeChunks, nil
}

func (r *Runner) searchKnowledge(ctx context.Context, currentRoom RuntimeRoom, responder model.Agent, trigger model.Message) []model.KnowledgeChunk {
	if r.knowledge == nil {
		return nil
	}

	chunks, err := r.knowledge.SearchForAgent(ctx, currentRoom.Info().ID, responder.ID, trigger.Content)
	if err != nil {
		r.logger.Warn("search knowledge chunks failed", "room_id", currentRoom.Info().ID, "agent_id", responder.ID, "error", err)
		return nil
	}
	return chunks
}

func shortReason(err error) string {
	if errors.Is(err, llm.ErrNotConfigured) {
		return "LLM_API_KEY is not configured"
	}

	message := strings.TrimSpace(err.Error())
	message = strings.ReplaceAll(message, "\n", " ")
	if len(message) > 160 {
		return message[:157] + "..."
	}
	if message == "" {
		return "unknown error"
	}
	return message
}

func knowledgeSourcesFromChunks(chunks []model.KnowledgeChunk) []model.MessageKnowledgeSource {
	if len(chunks) == 0 {
		return nil
	}

	sources := make([]model.MessageKnowledgeSource, 0, len(chunks))
	seen := make(map[string]struct{}, len(chunks))
	for _, chunk := range chunks {
		if chunk.DocumentID == "" && chunk.DocumentName == "" {
			continue
		}
		key := chunk.Scope + "\x00" + chunk.DocumentID + "\x00" + chunk.DocumentName
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		sources = append(sources, model.MessageKnowledgeSource{
			DocumentID:   chunk.DocumentID,
			DocumentName: chunk.DocumentName,
			Scope:        chunk.Scope,
		})
	}
	return sources
}

func knowledgeSourcesForResponse(response AgentRuntimeResponse, fallback []model.KnowledgeChunk) []model.MessageKnowledgeSource {
	if response.KnowledgeSources != nil {
		return append([]model.MessageKnowledgeSource(nil), response.KnowledgeSources...)
	}
	return knowledgeSourcesFromChunks(fallback)
}

func messageArtifactsFromRuntime(artifacts []AgentRuntimeArtifact) []model.MessageArtifact {
	if len(artifacts) == 0 {
		return nil
	}

	result := make([]model.MessageArtifact, 0, len(artifacts))
	for i, artifact := range artifacts {
		id := strings.TrimSpace(artifact.ID)
		if id == "" {
			id = fmt.Sprintf("artifact_%d", i+1)
		}
		fileName := strings.TrimSpace(artifact.FileName)
		if fileName == "" {
			fileName = id
		}
		result = append(result, model.MessageArtifact{
			ID:       id,
			Type:     strings.TrimSpace(artifact.Type),
			Title:    strings.TrimSpace(artifact.Title),
			FileName: fileName,
			MIMEType: strings.TrimSpace(artifact.MIMEType),
			Content:  artifact.Content,
		})
	}
	return result
}
