package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/logging"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/store"
)

const (
	defaultAgentResponseWorkers = 4
	defaultAgentResponseQueue   = 64
)

// RoomService coordinates room use cases across runtime room state, persistence, and agents.
// During the governance refactor it remains a thin facade, while its methods are split across
// focused files for reads, writes, and access policy.
type RoomService struct {
	manager       *room.Manager
	agents        *AgentService
	knowledge     *KnowledgeService
	runner        *agent.Runner
	focus         *FocusService
	minutes       *MinutesService
	store         roomStore
	logger        *slog.Logger
	lifecycle     *MeetingLifecycle
	responseJobs  chan agentResponseJob
	responseStart sync.Once
}

type roomStore interface {
	Ping(ctx context.Context) error
	ListRooms(ctx context.Context, query store.ListRoomsQuery) ([]model.RoomSummary, error)
	AddParticipant(ctx context.Context, input store.AddParticipantInput) (model.Participant, error)
	MarkParticipantLeft(ctx context.Context, participantID string, leftAt time.Time) error
	AddMessage(ctx context.Context, message model.Message) (model.Message, error)
	ListMessages(ctx context.Context, query store.ListMessagesQuery) ([]model.Message, error)
	ListMessagesPage(ctx context.Context, query store.ListMessagesQuery) (store.MessagePage, error)
	ListAgentRuns(ctx context.Context, query store.ListRunsQuery) ([]store.AgentRun, error)
	ListDialogueRuns(ctx context.Context, query store.ListRunsQuery) ([]store.DialogueRun, error)
	CreateMinutes(ctx context.Context, minutes model.MeetingMinutes) (model.MeetingMinutes, error)
	ListMinutes(ctx context.Context, roomID string) ([]model.MeetingMinutes, error)
	LatestMinutes(ctx context.Context, roomID string) (model.MeetingMinutes, bool, error)
	UpdateRoomLifecycle(ctx context.Context, input store.UpdateRoomLifecycleInput) error
}

type agentResponseJob struct {
	ctx     context.Context
	room    *room.Room
	message model.Message
}

func NewRoomService(manager *room.Manager, agents *AgentService, knowledge *KnowledgeService, runner *agent.Runner, focus *FocusService, s roomStore) *RoomService {
	return &RoomService{
		manager:   manager,
		agents:    agents,
		knowledge: knowledge,
		runner:    runner,
		focus:     focus,
		store:     s,
		logger:    logging.Component("room_service"),
		lifecycle: NewMeetingLifecycle(s),
	}
}

func (s *RoomService) WithMinutes(minutes *MinutesService) *RoomService {
	s.minutes = minutes
	return s
}
