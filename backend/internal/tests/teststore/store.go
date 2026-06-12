package teststore

import (
	"context"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

type Store struct {
	Agents    []model.Agent
	Rooms     map[string]model.RoomMeta
	Documents []model.KnowledgeDocument
	Chunks    []model.KnowledgeChunk
}

func (s *Store) Ping(context.Context) error { return nil }
func (s *Store) Close() error               { return nil }
func (s *Store) SeedAgents(_ context.Context, agents []model.Agent) error {
	s.Agents = append([]model.Agent(nil), agents...)
	return nil
}
func (s *Store) ListAgents(context.Context) ([]model.Agent, error) {
	return append([]model.Agent(nil), s.Agents...), nil
}
func (s *Store) CreateAgent(_ context.Context, agent model.Agent) (model.Agent, error) {
	s.Agents = append(s.Agents, agent)
	return agent, nil
}
func (s *Store) UpdateAgent(_ context.Context, agent model.Agent) (model.Agent, error) {
	for i := range s.Agents {
		if s.Agents[i].ID == agent.ID {
			s.Agents[i] = agent
			return agent, nil
		}
	}
	s.Agents = append(s.Agents, agent)
	return agent, nil
}
func (s *Store) DeleteAgent(_ context.Context, agentID string) error {
	next := make([]model.Agent, 0, len(s.Agents))
	for _, agent := range s.Agents {
		if agent.ID != agentID {
			next = append(next, agent)
		}
	}
	s.Agents = next
	return nil
}
func (s *Store) CreateRoom(_ context.Context, input store.CreateRoomInput) (model.RoomMeta, []model.Agent, error) {
	if s.Rooms == nil {
		s.Rooms = make(map[string]model.RoomMeta)
	}
	meta := model.RoomMeta{ID: input.ID, Name: input.Name, CreatedAt: input.CreatedAt}
	s.Rooms[input.ID] = meta
	return meta, input.Agents, nil
}
func (s *Store) GetRoom(_ context.Context, roomID string) (model.RoomMeta, error) {
	return s.Rooms[roomID], nil
}
func (s *Store) ListRoomAgents(context.Context, string) ([]model.Agent, error) {
	return nil, nil
}
func (s *Store) AddParticipant(_ context.Context, input store.AddParticipantInput) (model.Participant, error) {
	return model.Participant{ID: input.ID, Name: input.DisplayName, JoinedAt: input.JoinedAt}, nil
}
func (s *Store) MarkParticipantLeft(context.Context, string, time.Time) error {
	return nil
}
func (s *Store) ListActiveParticipants(context.Context, string) ([]model.Participant, error) {
	return nil, nil
}
func (s *Store) MarkAllActiveParticipantsLeft(context.Context, time.Time) error {
	return nil
}
func (s *Store) AddMessage(_ context.Context, message model.Message) (model.Message, error) {
	return message, nil
}
func (s *Store) ListMessages(context.Context, store.ListMessagesQuery) ([]model.Message, error) {
	return nil, nil
}
func (s *Store) CreateAgentRun(context.Context, store.AgentRun) error { return nil }
func (s *Store) FinishAgentRun(context.Context, string, string, string, time.Time) error {
	return nil
}
func (s *Store) CreateKnowledgeDocument(_ context.Context, document model.KnowledgeDocument, chunks []model.KnowledgeChunk) (model.KnowledgeDocument, error) {
	s.Documents = append(s.Documents, document)
	s.Chunks = append(s.Chunks, chunks...)
	return document, nil
}
func (s *Store) ListKnowledgeDocuments(_ context.Context, query store.ListKnowledgeDocumentsQuery) ([]model.KnowledgeDocument, error) {
	result := make([]model.KnowledgeDocument, 0)
	for _, document := range s.Documents {
		if document.Scope == query.Scope && document.ScopeID == query.ScopeID {
			result = append(result, document)
		}
	}
	return result, nil
}
func (s *Store) DeleteKnowledgeDocument(_ context.Context, documentID string) error {
	nextDocuments := make([]model.KnowledgeDocument, 0, len(s.Documents))
	for _, document := range s.Documents {
		if document.ID != documentID {
			nextDocuments = append(nextDocuments, document)
		}
	}
	nextChunks := make([]model.KnowledgeChunk, 0, len(s.Chunks))
	for _, chunk := range s.Chunks {
		if chunk.DocumentID != documentID {
			nextChunks = append(nextChunks, chunk)
		}
	}
	s.Documents = nextDocuments
	s.Chunks = nextChunks
	return nil
}
func (s *Store) SearchKnowledgeChunks(_ context.Context, query store.SearchKnowledgeChunksQuery) ([]model.KnowledgeChunk, error) {
	result := make([]model.KnowledgeChunk, 0)
	for _, chunk := range s.Chunks {
		if chunk.Scope == query.Scope && chunk.ScopeID == query.ScopeID {
			result = append(result, chunk)
		}
	}
	if query.Limit > 0 && len(result) > query.Limit {
		return result[:query.Limit], nil
	}
	return result, nil
}
