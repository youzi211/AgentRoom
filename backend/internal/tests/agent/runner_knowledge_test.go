package agent_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/tests/teststore"
)

type recordingLLM struct {
	messages []llm.ChatMessage
}

func (c *recordingLLM) Complete(_ context.Context, messages []llm.ChatMessage) (string, error) {
	c.messages = append([]llm.ChatMessage(nil), messages...)
	return "基于知识库回答。", nil
}

type runtimeRoom struct {
	meta         model.RoomMeta
	participants []model.Participant
	agents       []model.Agent
	messages     []model.Message
}

func (r *runtimeRoom) Info() model.RoomMeta { return r.meta }
func (r *runtimeRoom) Participants() []model.Participant {
	result := make([]model.Participant, len(r.participants))
	copy(result, r.participants)
	return result
}
func (r *runtimeRoom) Agents() []model.Agent {
	public := make([]model.Agent, 0, len(r.agents))
	for _, a := range r.agents {
		public = append(public, a.Public())
	}
	return public
}
func (r *runtimeRoom) AgentsWithPrompts() []model.Agent {
	result := make([]model.Agent, len(r.agents))
	copy(result, r.agents)
	return result
}
func (r *runtimeRoom) RecentMessages(int) []model.Message {
	result := make([]model.Message, len(r.messages))
	copy(result, r.messages)
	return result
}
func (r *runtimeRoom) NewSystemMessage(content string) model.Message {
	return model.Message{ID: "system", RoomID: r.meta.ID, SenderID: "system", SenderName: "System", SenderType: model.SenderTypeSystem, Content: content, CreatedAt: time.Now().UTC()}
}
func (r *runtimeRoom) NewAgentMessage(a model.Agent, content string) model.Message {
	return model.Message{ID: "agent_msg", RoomID: r.meta.ID, SenderID: a.ID, SenderName: a.Name, SenderType: model.SenderTypeAgent, Content: content, CreatedAt: time.Now().UTC()}
}
func (r *runtimeRoom) AppendMessage(message model.Message) {
	r.messages = append(r.messages, message)
}
func (r *runtimeRoom) Broadcast(model.Message)          {}
func (r *runtimeRoom) BroadcastEvent(model.ServerEvent) {}

func TestRunnerIncludesRoomAndAgentKnowledgeInPrompt(t *testing.T) {
	llmClient := &recordingLLM{}
	store := &teststore.Store{
		Chunks: []model.KnowledgeChunk{
			{ID: "room_chunk", Scope: model.KnowledgeScopeRoom, ScopeID: "room_1", Content: "Room launch date is July."},
			{ID: "agent_chunk", Scope: model.KnowledgeScopeAgent, ScopeID: "agent_1", Content: "Product agent should discuss roadmap risk."},
		},
	}
	runner := agent.NewRunner(llmClient, store).WithKnowledge(testKnowledgeProvider{store: store})
	room := &runtimeRoom{
		meta: model.RoomMeta{ID: "room_1", Name: "Planning", CreatedAt: time.Now().UTC()},
		agents: []model.Agent{
			{ID: "agent_1", Name: "产品经理", Mention: "@产品经理", Role: "Product Manager", SystemPrompt: "You are PM.", Enabled: true},
		},
		messages: []model.Message{
			{ID: "msg_1", RoomID: "room_1", SenderID: "human_1", SenderName: "Alice", SenderType: model.SenderTypeHuman, Content: "@产品经理 这次发布有什么风险？", CreatedAt: time.Now().UTC()},
		},
	}

	runner.HandleHumanMessage(context.Background(), room, room.messages[0])

	if len(llmClient.messages) != 2 {
		t.Fatalf("expected LLM request with system and user messages, got %#v", llmClient.messages)
	}
	userPrompt := llmClient.messages[1].Content
	if !strings.Contains(userPrompt, "Room launch date is July.") {
		t.Fatalf("expected room knowledge in prompt, got %q", userPrompt)
	}
	if !strings.Contains(userPrompt, "Product agent should discuss roadmap risk.") {
		t.Fatalf("expected agent knowledge in prompt, got %q", userPrompt)
	}
}

type testKnowledgeProvider struct {
	store *teststore.Store
}

func (p testKnowledgeProvider) SearchForAgent(_ context.Context, roomID string, agentID string, _ string) ([]model.KnowledgeChunk, error) {
	result := make([]model.KnowledgeChunk, 0)
	for _, chunk := range p.store.Chunks {
		if (chunk.Scope == model.KnowledgeScopeRoom && chunk.ScopeID == roomID) ||
			(chunk.Scope == model.KnowledgeScopeAgent && chunk.ScopeID == agentID) {
			result = append(result, chunk)
		}
	}
	return result, nil
}
