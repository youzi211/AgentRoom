package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/model"
)

type MinutesService struct {
	llmClient llm.Client
}

func NewMinutesService(llmClient llm.Client) *MinutesService {
	return &MinutesService{llmClient: llmClient}
}

func (s *MinutesService) Generate(ctx context.Context, room model.RoomMeta, messages []model.Message) (string, error) {
	if s == nil || s.llmClient == nil {
		return fallbackMinutes(room, messages), nil
	}

	prompt := buildMinutesPrompt(room, messages)
	response, err := s.llmClient.Complete(ctx, []llm.ChatMessage{
		{Role: llm.RoleSystem, Content: "You generate concise Chinese meeting minutes in Markdown. Do not invent facts."},
		{Role: llm.RoleUser, Content: prompt},
	})
	if err != nil || strings.TrimSpace(response) == "" {
		return fallbackMinutes(room, messages), nil
	}
	return normalizeMinutesMarkdown(response), nil
}

func buildMinutesPrompt(room model.RoomMeta, messages []model.Message) string {
	var builder strings.Builder
	builder.WriteString("Generate Markdown meeting minutes for this AgentRoom meeting.\n")
	builder.WriteString("Include: summary, decisions, action items, risks, open questions.\n")
	builder.WriteString("Only use facts present in the transcript.\n\n")
	builder.WriteString("Room: ")
	builder.WriteString(room.Name)
	builder.WriteString("\nTranscript:\n")
	for _, message := range messages {
		builder.WriteString("- ")
		builder.WriteString(message.CreatedAt.Format(time.RFC3339))
		builder.WriteString(" ")
		builder.WriteString(message.SenderName)
		builder.WriteString(" (")
		builder.WriteString(message.SenderType)
		builder.WriteString("): ")
		builder.WriteString(message.Content)
		builder.WriteString("\n")
	}
	return builder.String()
}

func normalizeMinutesMarkdown(markdown string) string {
	trimmed := strings.TrimSpace(markdown)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "#") {
		return "## Meeting Minutes\n\n" + trimmed
	}
	return trimmed
}

func fallbackMinutes(room model.RoomMeta, messages []model.Message) string {
	var builder strings.Builder
	builder.WriteString("## Meeting Minutes\n\n")
	builder.WriteString("### Summary\n")
	if len(messages) == 0 {
		builder.WriteString("- No meeting messages have been recorded yet.\n")
	} else {
		builder.WriteString(fmt.Sprintf("- %s has %d recorded messages.\n", room.Name, len(messages)))
	}
	builder.WriteString("\n### Decisions\n- Pending confirmation.\n")
	builder.WriteString("\n### Action Items\n- Pending assignment.\n")
	builder.WriteString("\n### Risks\n- No explicit risks captured.\n")
	builder.WriteString("\n### Open Questions\n- No explicit open questions captured.\n")
	return builder.String()
}
