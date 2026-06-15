package teststore

import (
	"context"
	"sort"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

type Store struct {
	Agents       []model.Agent
	Rooms        map[string]model.RoomMeta
	AgentRuns    []store.AgentRun
	DialogueRuns []store.DialogueRun
	Documents    []model.KnowledgeDocument
	Chunks       []model.KnowledgeChunk
	Minutes      []model.MeetingMinutes
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
	meta := model.RoomMeta{
		ID:             input.ID,
		Name:           input.Name,
		CreatedAt:      input.CreatedAt,
		HasPasscode:    input.PasscodeHash != "",
		PasscodeHash:   input.PasscodeHash,
		DialoguePolicy: input.DialoguePolicy.WithDefaults(),
	}
	s.Rooms[input.ID] = meta
	return meta, input.Agents, nil
}
func (s *Store) GetRoom(_ context.Context, roomID string) (model.RoomMeta, error) {
	return s.Rooms[roomID], nil
}
func (s *Store) ListRoomAgents(context.Context, string) ([]model.Agent, error) {
	return nil, nil
}
func (s *Store) ListRooms(_ context.Context, query store.ListRoomsQuery) ([]model.RoomSummary, error) {
	result := make([]model.RoomSummary, 0, len(s.Rooms))
	for _, meta := range s.Rooms {
		status := meta.Status
		if status == "" {
			status = model.RoomStatusActive
		}
		if query.Status == model.RoomStatusActive || query.Status == model.RoomStatusArchived {
			if status != query.Status {
				continue
			}
		}
		result = append(result, model.RoomSummary{
			ID:          meta.ID,
			Name:        meta.Name,
			Status:      status,
			HasPasscode: meta.PasscodeHash != "",
			CreatedAt:   meta.CreatedAt,
			ArchivedAt:  meta.ArchivedAt,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}
func (s *Store) SetRoomStatus(_ context.Context, roomID string, status string, archivedAt *time.Time) error {
	if s.Rooms == nil {
		return nil
	}
	meta, ok := s.Rooms[roomID]
	if !ok {
		return nil
	}
	meta.Status = status
	meta.ArchivedAt = archivedAt
	s.Rooms[roomID] = meta
	return nil
}
func (s *Store) CreateMinutes(_ context.Context, minutes model.MeetingMinutes) (model.MeetingMinutes, error) {
	maxVersion := 0
	for _, existing := range s.Minutes {
		if existing.RoomID == minutes.RoomID && existing.Version > maxVersion {
			maxVersion = existing.Version
		}
	}
	minutes.Version = maxVersion + 1
	s.Minutes = append(s.Minutes, minutes)
	return minutes, nil
}
func (s *Store) ListMinutes(_ context.Context, roomID string) ([]model.MeetingMinutes, error) {
	result := make([]model.MeetingMinutes, 0)
	for _, minutes := range s.Minutes {
		if minutes.RoomID == roomID {
			result = append(result, minutes)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version > result[j].Version
	})
	return result, nil
}
func (s *Store) LatestMinutes(_ context.Context, roomID string) (model.MeetingMinutes, bool, error) {
	var latest model.MeetingMinutes
	found := false
	for _, minutes := range s.Minutes {
		if minutes.RoomID == roomID && (!found || minutes.Version > latest.Version) {
			latest = minutes
			found = true
		}
	}
	return latest, found, nil
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
func (s *Store) CreateAgentRun(_ context.Context, run store.AgentRun) error {
	s.AgentRuns = append(s.AgentRuns, run)
	return nil
}
func (s *Store) FinishAgentRun(_ context.Context, runID string, status string, errText string, completedAt time.Time) error {
	for i := range s.AgentRuns {
		if s.AgentRuns[i].ID == runID {
			s.AgentRuns[i].Status = status
			s.AgentRuns[i].Error = errText
			s.AgentRuns[i].CompletedAt = &completedAt
			return nil
		}
	}
	return nil
}
func (s *Store) ListAgentRuns(_ context.Context, query store.ListRunsQuery) ([]store.AgentRun, error) {
	result := make([]store.AgentRun, 0)
	for _, run := range s.AgentRuns {
		if run.RoomID == query.RoomID {
			result = append(result, run)
		}
	}
	sortRuns(result, query.Limit, func(i int) time.Time { return result[i].StartedAt })
	if limit := normalizedTestRunLimit(query.Limit, len(result)); limit < len(result) {
		return result[:limit], nil
	}
	return result, nil
}
func (s *Store) CreateDialogueRun(_ context.Context, run store.DialogueRun) error {
	s.DialogueRuns = append(s.DialogueRuns, run)
	return nil
}
func (s *Store) FinishDialogueRun(_ context.Context, runID string, status string, turnCount int, completedAt time.Time) error {
	for i := range s.DialogueRuns {
		if s.DialogueRuns[i].ID == runID {
			s.DialogueRuns[i].Status = status
			s.DialogueRuns[i].TurnCount = turnCount
			s.DialogueRuns[i].CompletedAt = &completedAt
			return nil
		}
	}
	return nil
}
func (s *Store) ListDialogueRuns(_ context.Context, query store.ListRunsQuery) ([]store.DialogueRun, error) {
	result := make([]store.DialogueRun, 0)
	for _, run := range s.DialogueRuns {
		if run.RoomID == query.RoomID {
			result = append(result, run)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})
	if limit := normalizedTestRunLimit(query.Limit, len(result)); limit < len(result) {
		return result[:limit], nil
	}
	return result, nil
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

func sortRuns[T any](runs []T, _ int, startedAt func(int) time.Time) {
	sort.Slice(runs, func(i, j int) bool {
		return startedAt(i).After(startedAt(j))
	})
}

func normalizedTestRunLimit(limit int, total int) int {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if limit > total {
		return total
	}
	return limit
}
