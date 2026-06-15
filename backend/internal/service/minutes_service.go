package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/model"
	"github.com/tmc/langchaingo/prompts"
)

type MinutesService struct {
	llmClient llm.Client
}

var minutesPromptTemplate = prompts.NewChatPromptTemplate([]prompts.MessageFormatter{
	prompts.NewSystemMessagePromptTemplate(
		"You generate concise Chinese meeting minutes in Markdown. Do not invent facts.",
		nil,
	),
	prompts.NewHumanMessagePromptTemplate(
		`Generate Markdown meeting minutes for this AgentRoom meeting.
Include: summary, decisions, action items, risks, open questions.
Only use facts present in the transcript.

Room: {{.roomName}}
Transcript:
{{.transcript}}`,
		[]string{"roomName", "transcript"},
	),
})

func NewMinutesService(llmClient llm.Client) *MinutesService {
	return &MinutesService{llmClient: llmClient}
}

func (s *MinutesService) Generate(ctx context.Context, room model.RoomMeta, messages []model.Message) (string, error) {
	if s == nil || s.llmClient == nil {
		return fallbackMinutes(room, messages), nil
	}

	promptMessages, err := renderChatMessages(minutesPromptTemplate, map[string]any{
		"roomName":   room.Name,
		"transcript": buildMinutesTranscript(messages),
	})
	if err != nil {
		return fallbackMinutes(room, messages), nil
	}

	response, err := s.llmClient.Complete(ctx, promptMessages)
	if err != nil || strings.TrimSpace(response) == "" {
		return fallbackMinutes(room, messages), nil
	}
	return normalizeMinutesMarkdown(response), nil
}

func buildMinutesTranscript(messages []model.Message) string {
	var builder strings.Builder
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
