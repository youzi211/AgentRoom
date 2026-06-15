package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/logging"
	"agentroom/backend/internal/model"
	langllms "github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/prompts"
)

type FocusService struct {
	llmClient llm.Client
	logger    *slog.Logger
	mu        sync.RWMutex
	rooms     map[string]*roomFocusState
}

type roomFocusState struct {
	messages     []model.Message
	focusPoints  []model.FocusPoint
	lastAnalyzed int
}

var focusPromptTemplate = prompts.NewChatPromptTemplate([]prompts.MessageFormatter{
	prompts.NewSystemMessagePromptTemplate(
		"你是一个会议分析助手，负责提取会议对话的关键焦点话题。只返回 JSON 数组。",
		nil,
	),
	prompts.NewHumanMessagePromptTemplate(
		`分析以下会议对话，提取关键焦点话题。返回 JSON 数组，每个焦点包含：
- content: 焦点描述（简洁，20字以内）
- category: 类别（如"需求"、"技术"、"决策"、"问题"、"计划"）
只返回 JSON，不要其他文字。示例：[{"content":"讨论用户登录功能","category":"需求"},{"content":"决定使用React框架","category":"决策"}]

对话内容：
{{.conversation}}`,
		[]string{"conversation"},
	),
})

func NewFocusService(llmClient llm.Client) *FocusService {
	return &FocusService{
		llmClient: llmClient,
		logger:    logging.Component("focus_service"),
		rooms:     make(map[string]*roomFocusState),
	}
}

func (s *FocusService) AddMessage(ctx context.Context, roomID string, message model.Message) []model.FocusPoint {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.rooms[roomID]
	if !exists {
		state = &roomFocusState{}
		s.rooms[roomID] = state
	}

	state.messages = append(state.messages, message)
	messageCount := len(state.messages)
	threshold := 3

	s.logger.Info("focus: message added",
		"room_id", roomID,
		"total_messages", messageCount,
		"last_analyzed", state.lastAnalyzed,
		"diff", messageCount-state.lastAnalyzed,
		"threshold", threshold)

	if messageCount-state.lastAnalyzed >= threshold {
		s.logger.Info("focus: triggering analysis", "room_id", roomID)
		focusPoints := s.analyzeMessages(ctx, roomID, state.messages)
		if len(focusPoints) > 0 {
			state.focusPoints = focusPoints
			state.lastAnalyzed = messageCount
			s.logger.Info("focus: analysis complete", "room_id", roomID, "points_count", len(focusPoints))
			return focusPoints
		}
		s.logger.Warn("focus: analysis returned no points", "room_id", roomID)
	}

	return state.focusPoints
}

func (s *FocusService) GetFocusPoints(roomID string) []model.FocusPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, exists := s.rooms[roomID]
	if !exists {
		return nil
	}
	return state.focusPoints
}

func (s *FocusService) analyzeMessages(ctx context.Context, roomID string, messages []model.Message) []model.FocusPoint {
	if len(messages) == 0 {
		return nil
	}

	recentMessages := messages
	if len(recentMessages) > 20 {
		recentMessages = recentMessages[len(recentMessages)-20:]
	}

	var conversationBuilder strings.Builder
	for _, msg := range recentMessages {
		conversationBuilder.WriteString(fmt.Sprintf("[%s] %s: %s\n",
			msg.CreatedAt.Format("15:04"),
			msg.SenderName,
			msg.Content))
	}

	promptMessages, err := renderChatMessages(focusPromptTemplate, map[string]any{
		"conversation": conversationBuilder.String(),
	})
	if err != nil {
		s.logger.Error("render focus prompt", "room_id", roomID, "error", err)
		return nil
	}

	response, err := completeJSONIfSupported(ctx, s.llmClient, promptMessages)
	if err != nil {
		s.logger.Error("LLM focus analysis failed", "room_id", roomID, "error", err)
		return nil
	}

	s.logger.Info("focus: LLM response received", "room_id", roomID, "response_length", len(response))

	var focusItems []struct {
		Content  string `json:"content"`
		Category string `json:"category"`
	}

	cleaned := strings.TrimSpace(response)
	if err := json.Unmarshal([]byte(cleaned), &focusItems); err != nil {
		s.logger.Warn("failed to parse focus analysis", "room_id", roomID, "error", err, "cleaned", cleaned)
		return nil
	}

	s.logger.Info("focus: parsed items", "room_id", roomID, "items_count", len(focusItems))

	now := time.Now()
	var focusPoints []model.FocusPoint
	for i, item := range focusItems {
		if item.Content == "" {
			continue
		}
		focusPoints = append(focusPoints, model.FocusPoint{
			ID:        fmt.Sprintf("focus_%s_%d", roomID, now.UnixNano()+int64(i)),
			Content:   item.Content,
			Timestamp: now,
			Category:  item.Category,
		})
	}

	return focusPoints
}

func completeJSONIfSupported(ctx context.Context, client llm.Client, messages []llm.ChatMessage) (string, error) {
	if jsonClient, ok := client.(llm.JSONClient); ok {
		return jsonClient.CompleteJSON(ctx, messages)
	}
	return client.Complete(ctx, messages)
}

func renderChatMessages(template prompts.ChatPromptTemplate, values map[string]any) ([]llm.ChatMessage, error) {
	formatted, err := template.FormatMessages(values)
	if err != nil {
		return nil, err
	}

	result := make([]llm.ChatMessage, 0, len(formatted))
	for _, message := range formatted {
		role := llm.RoleUser
		switch message.GetType() {
		case langllms.ChatMessageTypeSystem:
			role = llm.RoleSystem
		case langllms.ChatMessageTypeAI:
			role = llm.RoleAssistant
		}

		result = append(result, llm.ChatMessage{
			Role:    role,
			Content: message.GetContent(),
		})
	}

	return result, nil
}
