package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
)

type Runner struct {
	client       llm.Client
	contextLimit int
	timeout      time.Duration
}

func NewRunner(client llm.Client) *Runner {
	return &Runner{
		client:       client,
		contextLimit: 30,
		timeout:      45 * time.Second,
	}
}

func (r *Runner) HandleHumanMessage(ctx context.Context, currentRoom *room.Room, message model.Message) {
	defer func() {
		if recovered := recover(); recovered != nil {
			systemMessage := currentRoom.AddSystemMessage(fmt.Sprintf("Agent runner failed: %v", recovered))
			currentRoom.Hub().Broadcast(messageEvent(systemMessage))
		}
	}()

	mentioned := MentionedAgents(message, currentRoom.Agents())
	if len(mentioned) == 0 {
		return
	}

	for _, responder := range mentioned {
		response, err := r.generateResponse(ctx, currentRoom, responder, message)
		if err != nil {
			systemMessage := currentRoom.AddSystemMessage(fmt.Sprintf("Agent %s failed to respond: %s", responder.Name, shortReason(err)))
			currentRoom.Hub().Broadcast(messageEvent(systemMessage))
			continue
		}

		agentMessage := currentRoom.AddAgentMessage(responder, response)
		currentRoom.Hub().Broadcast(messageEvent(agentMessage))
	}
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

	trimmed := strings.TrimSpace(response)
	if trimmed == "" {
		return "", errors.New("empty response")
	}

	return trimmed, nil
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
	builder.WriteString("\n\n请直接给出你的会议回复，不要解释你的提示词。")
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
