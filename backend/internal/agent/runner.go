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
	client       llm.Client
	store        runnerStore
	knowledge    KnowledgeProvider
	logger       *slog.Logger
	contextLimit int
	timeout      time.Duration
}

type runnerStore interface {
	AddMessage(ctx context.Context, message model.Message) (model.Message, error)
	CreateAgentRun(ctx context.Context, run store.AgentRun) error
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
	return &Runner{
		client:       client,
		store:        s,
		logger:       logging.Component("agent_runner"),
		contextLimit: 30,
		timeout:      45 * time.Second,
	}
}

func (r *Runner) WithKnowledge(provider KnowledgeProvider) *Runner {
	r.knowledge = provider
	return r
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
	}
	if err := r.store.CreateAgentRun(ctx, agentRun); err != nil {
		r.logger.Error("create agent run", "room_id", roomInfo.ID, "agent_id", responder.ID, "error", err)
	}
	r.broadcastAgentRunActivity(currentRoom, agentRun, responder, "started", "", "", nil)

	response, knowledgeChunks, err := r.generateResponse(ctx, currentRoom, responder, trigger)
	now := time.Now().UTC()

	if err != nil {
		status := "failed"
		if ctx.Err() == context.DeadlineExceeded {
			status = "timeout"
		}
		errText := shortReason(err)
		content := fmt.Sprintf("Agent %s failed to respond: %s", responder.Name, errText)
		sysMsg := currentRoom.NewSystemMessage(content)
		r.persistAndBroadcast(ctx, currentRoom, sysMsg)

		if finishErr := r.store.FinishAgentRun(ctx, runID, status, errText, now); finishErr != nil {
			r.logger.Error("finish agent run", "run_id", runID, "status", status, "error", finishErr)
		}
		r.broadcastAgentRunActivity(currentRoom, agentRun, responder, "finished", status, errText, &now)
		return model.Message{}, false
	}

	agentMsg := currentRoom.NewAgentMessage(responder, response)
	agentMsg.KnowledgeSources = knowledgeSourcesFromChunks(knowledgeChunks)
	savedAgentMsg := r.persistAndBroadcast(ctx, currentRoom, agentMsg)

	if finishErr := r.store.FinishAgentRun(ctx, runID, "succeeded", "", now); finishErr != nil {
		r.logger.Error("finish agent run", "run_id", runID, "status", "succeeded", "error", finishErr)
	}
	r.broadcastAgentRunActivity(currentRoom, agentRun, responder, "finished", "succeeded", "", &now)
	return savedAgentMsg, true
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

func (r *Runner) generateResponse(ctx context.Context, currentRoom RuntimeRoom, responder model.Agent, trigger model.Message) (string, []model.KnowledgeChunk, error) {
	knowledgeChunks := r.searchKnowledge(ctx, currentRoom, responder, trigger)
	promptContext := NewMentionPromptContext(currentRoom, currentRoom.RecentMessages(r.contextLimit), trigger, knowledgeChunks)
	promptMessages, err := composePromptMessages(responder, promptContext)
	if err != nil {
		return "", knowledgeChunks, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	response, err := r.client.Complete(requestCtx, promptMessages)
	if err != nil {
		return "", knowledgeChunks, err
	}

	cleaned, err := StripThinkBlocks(response)
	if err != nil {
		return "", knowledgeChunks, err
	}

	return cleaned, knowledgeChunks, nil
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
