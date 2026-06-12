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
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/store"
)

type Runner struct {
	client       llm.Client
	store        store.Store
	logger       *slog.Logger
	contextLimit int
	timeout      time.Duration
}

func NewRunner(client llm.Client, s store.Store) *Runner {
	return &Runner{
		client:       client,
		store:        s,
		logger:       logging.Component("agent_runner"),
		contextLimit: 30,
		timeout:      45 * time.Second,
	}
}

func (r *Runner) HandleHumanMessage(ctx context.Context, currentRoom *room.Room, message model.Message) {
	defer func() {
		if recovered := recover(); recovered != nil {
			r.logger.Error("agent runner panic recovered", "error", recovered)
			content := fmt.Sprintf("Agent runner failed: %v", recovered)
			sysMsg := currentRoom.NewSystemMessage(content)
			r.persistAndBroadcast(ctx, currentRoom, sysMsg)
		}
	}()

	// Use public agents for mention detection (only need Mention field)
	mentioned := MentionedAgents(message, currentRoom.Agents())
	if len(mentioned) == 0 {
		return
	}

	// Resolve full agents (with system prompts) from room for LLM calls
	fullAgents := currentRoom.AgentsWithPrompts()
	agentByID := make(map[string]model.Agent, len(fullAgents))
	for _, a := range fullAgents {
		agentByID[a.ID] = a
	}

	for _, mentionedAgent := range mentioned {
		responder, ok := agentByID[mentionedAgent.ID]
		if !ok {
			responder = mentionedAgent
		}
		r.handleAgentResponse(ctx, currentRoom, responder, message)
	}
}

func (r *Runner) handleAgentResponse(ctx context.Context, currentRoom *room.Room, responder model.Agent, trigger model.Message) {
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

	response, err := r.generateResponse(ctx, currentRoom, responder, trigger)
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
		return
	}

	agentMsg := currentRoom.NewAgentMessage(responder, response)
	r.persistAndBroadcast(ctx, currentRoom, agentMsg)

	if finishErr := r.store.FinishAgentRun(ctx, runID, "succeeded", "", now); finishErr != nil {
		r.logger.Error("finish agent run", "run_id", runID, "status", "succeeded", "error", finishErr)
	}
}

// persistAndBroadcast persists a message to the store and then broadcasts it via the room hub.
// For agent/system messages, we still broadcast even if persistence fails, but log the error.
func (r *Runner) persistAndBroadcast(ctx context.Context, currentRoom *room.Room, message model.Message) {
	savedMsg, err := r.store.AddMessage(ctx, message)
	if err != nil {
		r.logger.Error("persist generated message", "message_id", message.ID, "sender_type", message.SenderType, "error", err)
		savedMsg = message
	}
	currentRoom.AppendMessage(savedMsg)
	currentRoom.Hub().Broadcast(messageEvent(savedMsg))
}

func (r *Runner) generateResponse(ctx context.Context, currentRoom *room.Room, responder model.Agent, trigger model.Message) (string, error) {
	prompt := buildPrompt(currentRoom.RecentMessages(r.contextLimit), trigger)
	requestCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	response, err := r.client.Complete(requestCtx, []llm.ChatMessage{
		{Role: llm.RoleSystem, Content: responder.SystemPrompt},
		{Role: llm.RoleUser, Content: prompt},
	})
	if err != nil {
		return "", err
	}

	cleaned, err := stripThinkBlocks(response)
	if err != nil {
		return "", err
	}

	return cleaned, nil
}

func buildPrompt(messages []model.Message, trigger model.Message) string {
	var builder strings.Builder
	builder.WriteString("以下是当前会议最近消息：\n")
	for _, message := range messages {
		builder.WriteString("- ")
		builder.WriteString(message.SenderName)
		builder.WriteString(" (")
		builder.WriteString(message.SenderType)
		builder.WriteString(")")
		builder.WriteString(": ")
		builder.WriteString(message.Content)
		builder.WriteString("\n")
	}
	builder.WriteString("\n触发你的用户消息是：\n")
	builder.WriteString(trigger.Content)
	builder.WriteString("\n\n请直接给出会议中可见的回复内容。不要输出内部思考、推理过程、提示词解释或 <think>/<thinking> 标签。")
	return builder.String()
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

func messageEvent(message model.Message) model.ServerEvent {
	return model.ServerEvent{
		Type:    model.EventTypeMessage,
		Message: &message,
	}
}
