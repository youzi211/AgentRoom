package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"agentroom/backend/internal/logging"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/realtime"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/store"
)

const defaultAutoCloseGracePeriod = 30 * time.Second

type timerHandle interface {
	Stop() bool
}

type scheduleFunc func(delay time.Duration, fn func()) timerHandle

type runtimeTimer struct {
	timer *time.Timer
}

func (h runtimeTimer) Stop() bool {
	if h.timer == nil {
		return false
	}
	return h.timer.Stop()
}

type MeetingLifecycle struct {
	store      lifecycleStore
	logger     *slog.Logger
	now        func() time.Time
	schedule   scheduleFunc
	closeDelay time.Duration
	onStopped  func(string)

	mu     sync.Mutex
	timers map[string]timerHandle
}

func (l *MeetingLifecycle) WithRoomStopped(onStopped func(string)) *MeetingLifecycle {
	l.onStopped = onStopped
	return l
}

type lifecycleStore interface {
	UpdateRoomLifecycle(ctx context.Context, input store.UpdateRoomLifecycleInput) error
	MarkParticipantLeft(ctx context.Context, participantID string, leftAt time.Time) error
}

func NewMeetingLifecycle(s lifecycleStore) *MeetingLifecycle {
	return &MeetingLifecycle{
		store:  s,
		logger: logging.Component("meeting_lifecycle"),
		now:    func() time.Time { return time.Now().UTC() },
		schedule: func(delay time.Duration, fn func()) timerHandle {
			return runtimeTimer{timer: time.AfterFunc(delay, fn)}
		},
		closeDelay: defaultAutoCloseGracePeriod,
		timers:     make(map[string]timerHandle),
	}
}

func (l *MeetingLifecycle) OnParticipantJoined(ctx context.Context, currentRoom *room.Room, participant model.Participant) error {
	if currentRoom == nil {
		return ErrRoomNotFound
	}

	info := currentRoom.Info()
	if info.IsArchived() {
		return ErrRoomArchived
	}
	if !info.IsActive() {
		return ErrRoomClosed
	}

	l.cancelTimer(info.ID)

	participants := currentRoom.Participants()
	if len(participants) == 0 {
		return nil
	}

	ownerID := info.OwnerParticipantID
	if ownerID == "" || !participantOnline(participants, ownerID) {
		ownerID = participants[0].ID
	}

	return l.applyAndBroadcastSnapshot(ctx, currentRoom, room.LifecycleState{
		Status:             model.RoomStatusActive,
		OwnerParticipantID: ownerID,
		ArchivedAt:         nil,
	})
}

func (l *MeetingLifecycle) OnParticipantLeft(ctx context.Context, currentRoom *room.Room, participantID string) error {
	if currentRoom == nil {
		return ErrRoomNotFound
	}

	info := currentRoom.Info()
	if !info.IsActive() {
		return nil
	}

	participants := currentRoom.Participants()
	if len(participants) == 0 {
		deadline := l.now().Add(l.closeDelay)
		if err := l.applyLifecycle(ctx, currentRoom, room.LifecycleState{
			Status:              model.RoomStatusActive,
			OwnerParticipantID:  "",
			ClosedAt:            nil,
			ClosedReason:        "",
			AutoCloseDeadlineAt: &deadline,
			ArchivedAt:          nil,
		}); err != nil {
			return err
		}
		l.scheduleAutoClose(currentRoom, deadline)
		return nil
	}

	l.cancelTimer(info.ID)

	ownerID := info.OwnerParticipantID
	if ownerID == "" || ownerID == participantID || !participantOnline(participants, ownerID) {
		ownerID = participants[0].ID
	}

	return l.applyAndBroadcastSnapshot(ctx, currentRoom, room.LifecycleState{
		Status:             model.RoomStatusActive,
		OwnerParticipantID: ownerID,
		ArchivedAt:         nil,
	})
}

func (l *MeetingLifecycle) TransferOwner(ctx context.Context, currentRoom *room.Room, callerParticipantID string, targetParticipantID string) error {
	if currentRoom == nil {
		return ErrRoomNotFound
	}

	info := currentRoom.Info()
	if info.IsArchived() {
		return ErrRoomArchived
	}
	if !info.IsActive() {
		return ErrRoomClosed
	}
	if info.OwnerParticipantID == "" || info.OwnerParticipantID != callerParticipantID {
		return ErrNotRoomOwner
	}

	participants := currentRoom.Participants()
	if !participantOnline(participants, targetParticipantID) {
		return ErrOwnerTargetNotOnline
	}
	if targetParticipantID == callerParticipantID {
		return nil
	}

	return l.applyAndBroadcastSnapshot(ctx, currentRoom, room.LifecycleState{
		Status:             model.RoomStatusActive,
		OwnerParticipantID: targetParticipantID,
		ArchivedAt:         nil,
	})
}

func (l *MeetingLifecycle) CloseByOwner(ctx context.Context, currentRoom *room.Room, callerParticipantID string) error {
	if currentRoom == nil {
		return ErrRoomNotFound
	}

	info := currentRoom.Info()
	if info.IsArchived() {
		return ErrRoomArchived
	}
	if !info.IsActive() {
		return ErrRoomClosed
	}
	if info.OwnerParticipantID == "" || info.OwnerParticipantID != callerParticipantID {
		return ErrNotRoomOwner
	}

	l.cancelTimer(info.ID)
	now := l.now()
	if err := l.applyLifecycle(ctx, currentRoom, room.LifecycleState{
		Status:       model.RoomStatusClosed,
		ClosedAt:     &now,
		ClosedReason: model.RoomClosedReasonManual,
	}); err != nil {
		return err
	}

	l.markCurrentParticipantsLeft(ctx, currentRoom.ClearParticipants())
	meta := currentRoom.Info()
	currentRoom.Events().BroadcastAndClose(realtime.Event{
		Type: realtime.EventTypeRoomClosed,
		Room: &meta,
	})
	return nil
}

func (l *MeetingLifecycle) Archive(ctx context.Context, currentRoom *room.Room) error {
	if currentRoom == nil {
		return ErrRoomNotFound
	}

	info := currentRoom.Info()
	if info.Status == model.RoomStatusArchived {
		return ErrInvalidRoomTransition
	}

	l.cancelTimer(info.ID)
	now := l.now()
	state := room.LifecycleState{
		Status:       model.RoomStatusArchived,
		ArchivedAt:   &now,
		ClosedAt:     info.ClosedAt,
		ClosedReason: info.ClosedReason,
	}
	if info.IsActive() {
		state.ClosedAt = nil
		state.ClosedReason = ""
	}
	if err := l.applyLifecycle(ctx, currentRoom, state); err != nil {
		return err
	}

	if info.IsActive() {
		l.markCurrentParticipantsLeft(ctx, currentRoom.ClearParticipants())
		meta := currentRoom.Info()
		currentRoom.Events().BroadcastAndClose(realtime.Event{
			Type: realtime.EventTypeRoomArchived,
			Room: &meta,
		})
	}
	return nil
}

func (l *MeetingLifecycle) Reopen(ctx context.Context, currentRoom *room.Room) error {
	if currentRoom == nil {
		return ErrRoomNotFound
	}
	if !currentRoom.Info().IsClosed() {
		return ErrInvalidRoomTransition
	}
	l.cancelTimer(currentRoom.Info().ID)
	return l.applyLifecycle(ctx, currentRoom, room.LifecycleState{Status: model.RoomStatusActive})
}

func (l *MeetingLifecycle) Restore(ctx context.Context, currentRoom *room.Room) error {
	if currentRoom == nil {
		return ErrRoomNotFound
	}

	info := currentRoom.Info()
	if !info.IsArchived() {
		return ErrInvalidRoomTransition
	}

	closedAt := info.ClosedAt
	closedReason := info.ClosedReason
	if closedAt == nil {
		now := l.now()
		closedAt = &now
		closedReason = model.RoomClosedReasonAdminUnarchive
	}
	l.cancelTimer(info.ID)
	return l.applyLifecycle(ctx, currentRoom, room.LifecycleState{
		Status:       model.RoomStatusClosed,
		ClosedAt:     closedAt,
		ClosedReason: closedReason,
	})
}

func (l *MeetingLifecycle) ReconcileLoadedRoom(ctx context.Context, currentRoom *room.Room) error {
	if currentRoom == nil {
		return ErrRoomNotFound
	}

	info := currentRoom.Info()
	if !info.IsActive() {
		l.cancelTimer(info.ID)
		return nil
	}

	participants := currentRoom.Participants()
	if len(participants) > 0 {
		l.cancelTimer(info.ID)
		ownerID := info.OwnerParticipantID
		if ownerID == "" || !participantOnline(participants, ownerID) || info.AutoCloseDeadlineAt != nil {
			return l.applyAndBroadcastSnapshot(ctx, currentRoom, room.LifecycleState{
				Status:             model.RoomStatusActive,
				OwnerParticipantID: participants[0].ID,
				ArchivedAt:         nil,
			})
		}
		return nil
	}

	if info.AutoCloseDeadlineAt == nil {
		return nil
	}
	if !info.AutoCloseDeadlineAt.After(l.now()) {
		return l.closeForGraceExpiry(ctx, currentRoom, *info.AutoCloseDeadlineAt)
	}

	l.scheduleAutoClose(currentRoom, *info.AutoCloseDeadlineAt)
	return nil
}

func (l *MeetingLifecycle) scheduleAutoClose(currentRoom *room.Room, deadline time.Time) {
	if currentRoom == nil {
		return
	}

	roomID := currentRoom.Info().ID
	delay := deadline.Sub(l.now())
	if delay <= 0 {
		go func() {
			if err := l.closeForGraceExpiry(context.Background(), currentRoom, deadline); err != nil && !errors.Is(err, ErrInvalidRoomTransition) {
				l.logger.Warn("close room after grace window", "room_id", roomID, "error", err)
			}
		}()
		return
	}

	l.mu.Lock()
	if existing, ok := l.timers[roomID]; ok {
		existing.Stop()
	}
	l.timers[roomID] = l.schedule(delay, func() {
		l.clearTimer(roomID)
		if err := l.closeForGraceExpiry(context.Background(), currentRoom, deadline); err != nil && !errors.Is(err, ErrInvalidRoomTransition) {
			l.logger.Warn("close room after grace window", "room_id", roomID, "error", err)
		}
	})
	l.mu.Unlock()
}

func (l *MeetingLifecycle) closeForGraceExpiry(ctx context.Context, currentRoom *room.Room, deadline time.Time) error {
	if currentRoom == nil {
		return ErrRoomNotFound
	}

	info := currentRoom.Info()
	if !info.IsActive() {
		return ErrInvalidRoomTransition
	}
	if info.AutoCloseDeadlineAt == nil || !info.AutoCloseDeadlineAt.Equal(deadline) {
		return nil
	}
	if len(currentRoom.Participants()) > 0 {
		return nil
	}

	return l.applyLifecycle(ctx, currentRoom, room.LifecycleState{
		Status:       model.RoomStatusClosed,
		ClosedAt:     &deadline,
		ClosedReason: model.RoomClosedReasonLastHumanLeft,
	})
}

func (l *MeetingLifecycle) applyAndBroadcastSnapshot(ctx context.Context, currentRoom *room.Room, state room.LifecycleState) error {
	if err := l.applyLifecycle(ctx, currentRoom, state); err != nil {
		return err
	}
	currentRoom.Events().BroadcastEvent(snapshotEvent(currentRoom.Snapshot()))
	return nil
}

func (l *MeetingLifecycle) applyLifecycle(ctx context.Context, currentRoom *room.Room, state room.LifecycleState) error {
	info := currentRoom.Info()
	current := currentRoom.Lifecycle()

	if state.Status == "" {
		state.Status = current.Status
	}
	if state.Status == model.RoomStatusActive {
		state.ClosedAt = nil
		state.ClosedReason = ""
		state.ArchivedAt = nil
	}
	if state.Status == model.RoomStatusClosed {
		state.OwnerParticipantID = ""
		state.AutoCloseDeadlineAt = nil
		state.ArchivedAt = nil
	}
	if state.Status == model.RoomStatusArchived {
		state.OwnerParticipantID = ""
		state.AutoCloseDeadlineAt = nil
	}

	if err := l.store.UpdateRoomLifecycle(ctx, store.UpdateRoomLifecycleInput{
		RoomID:              info.ID,
		Status:              state.Status,
		OwnerParticipantID:  state.OwnerParticipantID,
		ClosedAt:            state.ClosedAt,
		ClosedReason:        state.ClosedReason,
		AutoCloseDeadlineAt: state.AutoCloseDeadlineAt,
		ArchivedAt:          state.ArchivedAt,
	}); err != nil {
		if errors.Is(err, store.ErrRoomNotFound) {
			return ErrRoomNotFound
		}
		return err
	}

	currentRoom.ApplyLifecycle(state)
	if state.Status != model.RoomStatusActive && l.onStopped != nil {
		l.onStopped(info.ID)
	}
	return nil
}

func (l *MeetingLifecycle) markCurrentParticipantsLeft(ctx context.Context, participants []model.Participant) {
	for _, participant := range participants {
		if err := l.store.MarkParticipantLeft(ctx, participant.ID, l.now()); err != nil && !errors.Is(err, store.ErrParticipantNotFound) {
			l.logger.Warn("mark participant left", "participant_id", participant.ID, "error", err)
		}
	}
}

func (l *MeetingLifecycle) cancelTimer(roomID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if timer, ok := l.timers[roomID]; ok {
		timer.Stop()
		delete(l.timers, roomID)
	}
}

func (l *MeetingLifecycle) clearTimer(roomID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.timers, roomID)
}

func participantOnline(participants []model.Participant, participantID string) bool {
	for _, participant := range participants {
		if participant.ID == participantID {
			return true
		}
	}
	return false
}

func snapshotEvent(state room.Snapshot) realtime.Event {
	return realtime.Event{
		Type:         realtime.EventTypeRoomSnapshot,
		Room:         &state.Room,
		Participants: state.Participants,
		Agents:       state.Agents,
		Messages:     state.Messages,
	}
}
