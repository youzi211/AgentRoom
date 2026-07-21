package service

import (
	"context"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
)

// RoomQueries is the focused read-side application service used by the API layer.
type RoomQueries struct {
	root *RoomService
}

// RoomCommands is the focused write-side application service used by the API layer.
type RoomCommands struct {
	root *RoomService
}

// RoomAccess is the focused access-policy application service used by the API layer.
type RoomAccess struct {
	root *RoomService
}

func (s *RoomService) Queries() *RoomQueries {
	return &RoomQueries{root: s}
}

func (s *RoomService) Commands() *RoomCommands {
	return &RoomCommands{root: s}
}

func (s *RoomService) Access() *RoomAccess {
	return &RoomAccess{root: s}
}

func (q *RoomQueries) Ping(ctx context.Context) error {
	return q.root.Ping(ctx)
}

func (q *RoomQueries) Agents() []model.AgentConfig {
	return q.root.Agents()
}

func (q *RoomQueries) GetRoom(ctx context.Context, roomID string) (*room.Room, bool) {
	return q.root.GetRoom(ctx, roomID)
}

func (q *RoomQueries) ListRooms(ctx context.Context, query ListRoomsInput) ([]model.RoomSummary, error) {
	return q.root.ListRooms(ctx, query)
}

func (q *RoomQueries) EntrySummary(ctx context.Context) (EntrySummary, error) {
	return q.root.EntrySummary(ctx, EntrySummaryInput{Now: time.Now()})
}

func (q *RoomQueries) ListAgentKnowledge(ctx context.Context, agentID string) ([]model.KnowledgeDocument, error) {
	return q.root.ListAgentKnowledge(ctx, agentID)
}

func (q *RoomQueries) ListMessagesPage(ctx context.Context, currentRoom *room.Room, limit int, before string) (MessagePage, error) {
	return q.root.ListMessagesPage(ctx, currentRoom, limit, before)
}

func (q *RoomQueries) GetMessageArtifact(ctx context.Context, currentRoom *room.Room, messageID string, artifactID string) (model.MessageArtifact, error) {
	return q.root.GetMessageArtifact(ctx, currentRoom, messageID, artifactID)
}

func (q *RoomQueries) ListRoomActivity(ctx context.Context, currentRoom *room.Room, limit int) (RoomActivity, error) {
	return q.root.ListRoomActivity(ctx, currentRoom, limit)
}

func (q *RoomQueries) LatestPersistedMinutesMarkdown(ctx context.Context, currentRoom *room.Room) (string, bool, error) {
	return q.root.LatestPersistedMinutesMarkdown(ctx, currentRoom)
}

func (q *RoomQueries) ListMinutes(ctx context.Context, currentRoom *room.Room) ([]model.MeetingMinutes, error) {
	return q.root.ListMinutes(ctx, currentRoom)
}

func (q *RoomQueries) ListRoomKnowledge(ctx context.Context, roomID string) ([]model.KnowledgeDocument, error) {
	return q.root.ListRoomKnowledge(ctx, roomID)
}

func (c *RoomCommands) UpdateAgent(ctx context.Context, agentID string, input UpdateAgentInput) (model.Agent, error) {
	return c.root.UpdateAgent(ctx, agentID, input)
}

func (c *RoomCommands) CreateAgent(ctx context.Context, name, role, description, systemPrompt string, enabled bool, runtime string) (model.Agent, error) {
	return c.root.CreateAgent(ctx, name, role, description, systemPrompt, enabled, runtime)
}

func (c *RoomCommands) CreateAgentWithModel(ctx context.Context, name, role, description, systemPrompt string, enabled bool, runtime string, modelProfileID string) (model.Agent, error) {
	return c.root.agents.CreateAgentWithModel(ctx, name, role, description, systemPrompt, enabled, runtime, modelProfileID)
}

func (c *RoomCommands) DeleteAgent(ctx context.Context, agentID string) error {
	return c.root.DeleteAgent(ctx, agentID)
}

func (c *RoomCommands) UploadAgentKnowledge(ctx context.Context, agentID string, fileName string, content []byte) (model.KnowledgeDocument, error) {
	return c.root.UploadAgentKnowledge(ctx, agentID, fileName, content)
}

func (c *RoomCommands) CreateRoom(ctx context.Context, name string, agentIDs []string, passcode string, dialoguePolicy model.DialoguePolicy) (*room.Room, error) {
	return c.root.CreateRoom(ctx, name, agentIDs, passcode, dialoguePolicy)
}

func (c *RoomCommands) GenerateMinutes(ctx context.Context, currentRoom *room.Room) (string, model.MeetingMinutes, error) {
	return c.root.GenerateMinutes(ctx, currentRoom)
}

func (c *RoomCommands) ReopenRoom(ctx context.Context, roomID string) error {
	return c.root.ReopenRoom(ctx, roomID)
}

func (c *RoomCommands) ArchiveRoom(ctx context.Context, roomID string) error {
	return c.root.ArchiveRoom(ctx, roomID)
}

func (c *RoomCommands) RestoreRoom(ctx context.Context, roomID string) error {
	return c.root.RestoreRoom(ctx, roomID)
}

func (c *RoomCommands) SaveManualMinutes(ctx context.Context, currentRoom *room.Room, content string) (model.MeetingMinutes, error) {
	return c.root.SaveManualMinutes(ctx, currentRoom, content)
}

func (c *RoomCommands) UploadRoomKnowledge(ctx context.Context, roomID string, fileName string, content []byte) (model.KnowledgeDocument, error) {
	return c.root.UploadRoomKnowledge(ctx, roomID, fileName, content)
}

func (c *RoomCommands) DeleteKnowledgeDocument(ctx context.Context, documentID string) error {
	return c.root.DeleteKnowledgeDocument(ctx, documentID)
}

func (c *RoomCommands) OpenRealtimeSession(ctx context.Context, currentRoom *room.Room, name string) (*RealtimeSession, error) {
	return c.root.OpenRealtimeSession(ctx, currentRoom, name)
}

func (c *RoomCommands) CloseRealtimeSession(ctx context.Context, session *RealtimeSession) {
	c.root.CloseRealtimeSession(ctx, session)
}

func (c *RoomCommands) PostRealtimeMessage(ctx context.Context, session *RealtimeSession, content string) error {
	return c.root.PostRealtimeMessage(ctx, session, content)
}

func (c *RoomCommands) CloseRealtimeRoom(ctx context.Context, session *RealtimeSession) error {
	return c.root.CloseRealtimeRoom(ctx, session)
}

func (c *RoomCommands) TransferRealtimeOwner(ctx context.Context, session *RealtimeSession, targetParticipantID string) error {
	return c.root.TransferRealtimeOwner(ctx, session, targetParticipantID)
}

func (a *RoomAccess) CanAccessRoom(currentRoom *room.Room, passcode string) bool {
	return a.root.CanAccessRoom(currentRoom, passcode)
}
