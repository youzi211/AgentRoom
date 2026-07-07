package teststore

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

type Store struct {
	Agents              []model.Agent
	Rooms               map[string]model.RoomMeta
	RoomAgents          map[string][]model.Agent
	RoomMessages        map[string][]model.Message
	ActiveParticipants  map[string][]model.Participant
	AgentRuns           []store.AgentRun
	DialogueRuns        []store.DialogueRun
	Documents           []model.KnowledgeDocument
	Chunks              []model.KnowledgeChunk
	Minutes             []model.MeetingMinutes
	UpdateAgentErr      error
	DeleteAgentErr      error
	ListRoomAgentsErr   error
	ListMessagesErr     error
	ListParticipantsErr error
	DeleteDocumentErr   error
}

func (s *Store) Ping(context.Context) error { return nil }
func (s *Store) Close() error               { return nil }

func (s *Store) SeedAgents(_ context.Context, agents []model.Agent) error {
	existingIDs := make(map[string]struct{}, len(s.Agents))
	existingMentions := make(map[string]struct{}, len(s.Agents))
	for _, agent := range s.Agents {
		existingIDs[agent.ID] = struct{}{}
		if mention := strings.ToLower(strings.TrimSpace(agent.Mention)); mention != "" {
			existingMentions[mention] = struct{}{}
		}
	}
	for _, agent := range agents {
		if _, ok := existingIDs[agent.ID]; ok {
			continue
		}
		mention := strings.ToLower(strings.TrimSpace(agent.Mention))
		if mention != "" {
			if _, ok := existingMentions[mention]; ok {
				continue
			}
		}
		s.Agents = append(s.Agents, agent)
		existingIDs[agent.ID] = struct{}{}
		if mention != "" {
			existingMentions[mention] = struct{}{}
		}
	}
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
	if s.UpdateAgentErr != nil {
		return model.Agent{}, s.UpdateAgentErr
	}
	for i := range s.Agents {
		if s.Agents[i].ID == agent.ID {
			s.Agents[i] = agent
			return agent, nil
		}
	}
	return model.Agent{}, fmt.Errorf("%w: %s", store.ErrAgentNotFound, agent.ID)
}

func (s *Store) DeleteAgent(_ context.Context, agentID string) error {
	if s.DeleteAgentErr != nil {
		return s.DeleteAgentErr
	}
	next := make([]model.Agent, 0, len(s.Agents))
	found := false
	for _, agent := range s.Agents {
		if agent.ID != agentID {
			next = append(next, agent)
			continue
		}
		found = true
	}
	if !found {
		return fmt.Errorf("%w: %s", store.ErrAgentNotFound, agentID)
	}
	s.Agents = next
	return nil
}

func (s *Store) CreateRoom(_ context.Context, input store.CreateRoomInput) (model.RoomMeta, []model.Agent, error) {
	s.ensureMaps()
	meta := model.RoomMeta{
		ID:             input.ID,
		Name:           input.Name,
		CreatedAt:      input.CreatedAt,
		HasPasscode:    input.PasscodeHash != "",
		PasscodeHash:   input.PasscodeHash,
		DialoguePolicy: input.DialoguePolicy.WithDefaults(),
		Status:         model.RoomStatusActive,
	}
	s.Rooms[input.ID] = meta
	s.RoomAgents[input.ID] = append([]model.Agent(nil), input.Agents...)
	return meta, append([]model.Agent(nil), input.Agents...), nil
}

func (s *Store) GetRoom(_ context.Context, roomID string) (model.RoomMeta, error) {
	s.ensureMaps()
	meta, ok := s.Rooms[roomID]
	if !ok {
		return model.RoomMeta{}, fmt.Errorf("%w: %s", store.ErrRoomNotFound, roomID)
	}
	return meta, nil
}

func (s *Store) LoadRoomSnapshot(_ context.Context, roomID string, messageLimit int) (store.RoomSnapshot, error) {
	s.ensureMaps()
	meta, ok := s.Rooms[roomID]
	if !ok {
		return store.RoomSnapshot{}, fmt.Errorf("%w: %s", store.ErrRoomNotFound, roomID)
	}
	if s.ListRoomAgentsErr != nil {
		return store.RoomSnapshot{}, s.ListRoomAgentsErr
	}
	if s.ListMessagesErr != nil {
		return store.RoomSnapshot{}, s.ListMessagesErr
	}
	if s.ListParticipantsErr != nil {
		return store.RoomSnapshot{}, s.ListParticipantsErr
	}

	messages, err := s.listMessages(store.ListMessagesQuery{
		RoomID: roomID,
		Limit:  messageLimit,
	}, false)
	if err != nil {
		return store.RoomSnapshot{}, err
	}
	participants := append([]model.Participant(nil), s.ActiveParticipants[roomID]...)
	sortParticipants(participants)

	return store.RoomSnapshot{
		Meta:         meta,
		Agents:       append([]model.Agent(nil), s.RoomAgents[roomID]...),
		Messages:     messages,
		Participants: participants,
	}, nil
}

func (s *Store) ListRoomAgents(_ context.Context, roomID string) ([]model.Agent, error) {
	s.ensureMaps()
	if s.ListRoomAgentsErr != nil {
		return nil, s.ListRoomAgentsErr
	}
	return append([]model.Agent(nil), s.RoomAgents[roomID]...), nil
}

func (s *Store) ListRooms(_ context.Context, query store.ListRoomsQuery) ([]model.RoomSummary, error) {
	s.ensureMaps()
	result := make([]model.RoomSummary, 0, len(s.Rooms))
	for _, meta := range s.Rooms {
		status := normalizeRoomStatus(meta.Status)
		if query.Status == model.RoomStatusActive || query.Status == model.RoomStatusClosed || query.Status == model.RoomStatusArchived {
			if status != query.Status {
				continue
			}
		}
		messages := append([]model.Message(nil), s.RoomMessages[meta.ID]...)
		var lastMessageAt *time.Time
		if len(messages) > 0 {
			lastMessageAt = cloneTimePtr(&messages[len(messages)-1].CreatedAt)
		}
		result = append(result, model.RoomSummary{
			ID:                  meta.ID,
			Name:                meta.Name,
			Status:              status,
			HasPasscode:         meta.PasscodeHash != "",
			CreatedAt:           meta.CreatedAt,
			OwnerParticipantID:  meta.OwnerParticipantID,
			ClosedAt:            cloneTimePtr(meta.ClosedAt),
			ClosedReason:        meta.ClosedReason,
			AutoCloseDeadlineAt: cloneTimePtr(meta.AutoCloseDeadlineAt),
			ArchivedAt:          cloneTimePtr(meta.ArchivedAt),
			MessageCount:        len(messages),
			LastMessageAt:       lastMessageAt,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID > result[j].ID
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (s *Store) UpdateRoomLifecycle(_ context.Context, input store.UpdateRoomLifecycleInput) error {
	s.ensureMaps()
	meta, ok := s.Rooms[input.RoomID]
	if !ok {
		return fmt.Errorf("%w: %s", store.ErrRoomNotFound, input.RoomID)
	}
	meta.Status = normalizeRoomStatus(input.Status)
	meta.OwnerParticipantID = input.OwnerParticipantID
	meta.ClosedAt = cloneTimePtr(input.ClosedAt)
	meta.ClosedReason = input.ClosedReason
	meta.AutoCloseDeadlineAt = cloneTimePtr(input.AutoCloseDeadlineAt)
	meta.ArchivedAt = cloneTimePtr(input.ArchivedAt)
	s.Rooms[input.RoomID] = meta
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
	s.ensureMaps()
	participant := model.Participant{ID: input.ID, Name: input.DisplayName, JoinedAt: input.JoinedAt}
	s.ActiveParticipants[input.RoomID] = append(s.ActiveParticipants[input.RoomID], participant)
	sortParticipants(s.ActiveParticipants[input.RoomID])
	return participant, nil
}

func (s *Store) MarkParticipantLeft(_ context.Context, participantID string, _ time.Time) error {
	s.ensureMaps()
	for roomID, participants := range s.ActiveParticipants {
		next := make([]model.Participant, 0, len(participants))
		found := false
		for _, participant := range participants {
			if participant.ID == participantID {
				found = true
				continue
			}
			next = append(next, participant)
		}
		if found {
			s.ActiveParticipants[roomID] = next
			return nil
		}
	}
	return fmt.Errorf("%w: %s", store.ErrParticipantNotFound, participantID)
}

func (s *Store) ListActiveParticipants(_ context.Context, roomID string) ([]model.Participant, error) {
	s.ensureMaps()
	if s.ListParticipantsErr != nil {
		return nil, s.ListParticipantsErr
	}
	participants := append([]model.Participant(nil), s.ActiveParticipants[roomID]...)
	sortParticipants(participants)
	return participants, nil
}

func (s *Store) MarkAllActiveParticipantsLeft(context.Context, time.Time) error {
	s.ensureMaps()
	s.ActiveParticipants = make(map[string][]model.Participant)
	return nil
}

func (s *Store) AddMessage(_ context.Context, message model.Message) (model.Message, error) {
	s.ensureMaps()
	s.RoomMessages[message.RoomID] = append(s.RoomMessages[message.RoomID], message)
	sortMessages(s.RoomMessages[message.RoomID])
	return message, nil
}

func (s *Store) GetMessage(_ context.Context, roomID string, messageID string) (model.Message, error) {
	s.ensureMaps()
	for _, message := range s.RoomMessages[roomID] {
		if message.ID == messageID {
			return message, nil
		}
	}
	return model.Message{}, fmt.Errorf("%w: %s", store.ErrMessageNotFound, messageID)
}

func (s *Store) ListMessages(_ context.Context, query store.ListMessagesQuery) ([]model.Message, error) {
	s.ensureMaps()
	if s.ListMessagesErr != nil {
		return nil, s.ListMessagesErr
	}
	return s.listMessages(query, false)
}

func (s *Store) ListMessagesPage(_ context.Context, query store.ListMessagesQuery) (store.MessagePage, error) {
	s.ensureMaps()
	if s.ListMessagesErr != nil {
		return store.MessagePage{}, s.ListMessagesErr
	}
	messages, err := s.listMessages(query, true)
	if err != nil {
		return store.MessagePage{}, err
	}

	limit := normalizeMessageLimit(query.Limit)
	total := len(messages)
	if total <= limit {
		return store.MessagePage{Messages: messages}, nil
	}
	page := messages[total-limit:]
	return store.MessagePage{
		Messages:   append([]model.Message(nil), page...),
		HasMore:    true,
		NextBefore: page[0].ID,
	}, nil
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
	sortRuns(result, func(i int) time.Time { return result[i].StartedAt })
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
	sortRuns(result, func(i int) time.Time { return result[i].StartedAt })
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
	if s.DeleteDocumentErr != nil {
		return s.DeleteDocumentErr
	}
	nextDocuments := make([]model.KnowledgeDocument, 0, len(s.Documents))
	found := false
	for _, document := range s.Documents {
		if document.ID != documentID {
			nextDocuments = append(nextDocuments, document)
			continue
		}
		found = true
	}
	if !found {
		return fmt.Errorf("%w: %s", store.ErrKnowledgeDocumentNotFound, documentID)
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
			result = append(result, s.withKnowledgeDocumentName(chunk))
		}
	}
	if query.Limit > 0 && len(result) > query.Limit {
		return result[:query.Limit], nil
	}
	return result, nil
}

func (s *Store) withKnowledgeDocumentName(chunk model.KnowledgeChunk) model.KnowledgeChunk {
	if chunk.DocumentName != "" {
		return chunk
	}
	for _, document := range s.Documents {
		if document.ID == chunk.DocumentID {
			chunk.DocumentName = document.FileName
			return chunk
		}
	}
	return chunk
}

func (s *Store) listMessages(query store.ListMessagesQuery, strictCursor bool) ([]model.Message, error) {
	messages := append([]model.Message(nil), s.RoomMessages[query.RoomID]...)
	sortMessages(messages)

	if query.Before == "" {
		if strictCursor {
			return messages, nil
		}
		limit := normalizeMessageLimit(query.Limit)
		if len(messages) > limit {
			return append([]model.Message(nil), messages[:limit]...), nil
		}
		return messages, nil
	}

	index := -1
	for i, message := range messages {
		if message.ID == query.Before {
			index = i
			break
		}
	}
	if index == -1 {
		if strictCursor {
			return nil, store.ErrInvalidMessageCursor
		}
		limit := normalizeMessageLimit(query.Limit)
		if len(messages) > limit {
			return append([]model.Message(nil), messages[:limit]...), nil
		}
		return messages, nil
	}

	filtered := append([]model.Message(nil), messages[:index]...)
	if strictCursor {
		return filtered, nil
	}

	limit := normalizeMessageLimit(query.Limit)
	if len(filtered) > limit {
		return append([]model.Message(nil), filtered[:limit]...), nil
	}
	return filtered, nil
}

func (s *Store) ensureMaps() {
	if s.Rooms == nil {
		s.Rooms = make(map[string]model.RoomMeta)
	}
	if s.RoomAgents == nil {
		s.RoomAgents = make(map[string][]model.Agent)
	}
	if s.RoomMessages == nil {
		s.RoomMessages = make(map[string][]model.Message)
	}
	if s.ActiveParticipants == nil {
		s.ActiveParticipants = make(map[string][]model.Participant)
	}
}

func sortParticipants(participants []model.Participant) {
	sort.Slice(participants, func(i, j int) bool {
		if participants[i].JoinedAt.Equal(participants[j].JoinedAt) {
			return participants[i].ID < participants[j].ID
		}
		return participants[i].JoinedAt.Before(participants[j].JoinedAt)
	})
}

func sortMessages(messages []model.Message) {
	sort.Slice(messages, func(i, j int) bool {
		if messages[i].CreatedAt.Equal(messages[j].CreatedAt) {
			return messages[i].ID < messages[j].ID
		}
		return messages[i].CreatedAt.Before(messages[j].CreatedAt)
	})
}

func sortRuns[T any](runs []T, startedAt func(int) time.Time) {
	sort.Slice(runs, func(i, j int) bool {
		if startedAt(i).Equal(startedAt(j)) {
			return i < j
		}
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

func normalizeMessageLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func normalizeRoomStatus(status string) string {
	switch status {
	case "", model.RoomStatusActive:
		return model.RoomStatusActive
	case model.RoomStatusClosed:
		return model.RoomStatusClosed
	case model.RoomStatusArchived:
		return model.RoomStatusArchived
	default:
		return model.RoomStatusActive
	}
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}
