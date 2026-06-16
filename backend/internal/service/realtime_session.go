package service

import (
	"context"
	"strings"
	"sync"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/realtime"
	"agentroom/backend/internal/room"
)

type RealtimeSession struct {
	currentRoom  *room.Room
	participant  model.Participant
	client       *room.Client
	cleanupOnce  sync.Once
}

func (s *RealtimeSession) Events() <-chan realtime.Event {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Send
}

func (s *RealtimeSession) Send(event realtime.Event) {
	if s == nil || s.client == nil {
		return
	}
	select {
	case s.client.Send <- event:
	default:
	}
}

func (s *RealtimeSession) ClientID() string {
	if s == nil || s.client == nil {
		return ""
	}
	return s.client.ID
}

func (s *RealtimeSession) Participant() model.Participant {
	if s == nil {
		return model.Participant{}
	}
	return s.participant
}

func (s *RealtimeSession) Room() *room.Room {
	if s == nil {
		return nil
	}
	return s.currentRoom
}

func (s *RoomService) OpenRealtimeSession(ctx context.Context, currentRoom *room.Room, name string) (*RealtimeSession, error) {
	savedParticipant, err := s.JoinParticipant(ctx, currentRoom, name)
	if err != nil {
		return nil, err
	}

	client := &room.Client{
		ID:            model.NewID("client"),
		ParticipantID: savedParticipant.ID,
		Send:          make(chan realtime.Event, 16),
	}
	currentRoom.Events().Register(client)

	session := &RealtimeSession{
		currentRoom: currentRoom,
		participant: savedParticipant,
		client:      client,
	}

	currentRoom.Events().BroadcastExcept(realtime.Event{
		Type:        realtime.EventTypeParticipantJoined,
		Participant: &savedParticipant,
	}, client)

	initialSnapshot := snapshotEvent(currentRoom.Snapshot())
	initialSnapshot.ParticipantID = savedParticipant.ID
	client.Send <- initialSnapshot
	currentRoom.Events().BroadcastExcept(snapshotEvent(currentRoom.Snapshot()), client)

	return session, nil
}

func (s *RoomService) CloseRealtimeSession(ctx context.Context, session *RealtimeSession) {
	if session == nil || session.currentRoom == nil || session.client == nil {
		return
	}

	session.cleanupOnce.Do(func() {
		session.currentRoom.Events().Unregister(session.client)
		if s.LeaveParticipant(ctx, session.currentRoom, session.participant.ID) {
			session.currentRoom.Events().BroadcastEvent(realtime.Event{
				Type:          realtime.EventTypeParticipantLeft,
				ParticipantID: session.participant.ID,
			})
		}
	})
}

func (s *RoomService) PostRealtimeMessage(ctx context.Context, session *RealtimeSession, content string) error {
	if session == nil || session.currentRoom == nil {
		return ErrRoomNotFound
	}

	savedMessage, focusPoints, err := s.HandleHumanMessage(ctx, session.currentRoom, session.participant, strings.TrimSpace(content))
	if err != nil {
		return err
	}

	session.currentRoom.Events().BroadcastEvent(realtime.Event{
		Type:    realtime.EventTypeMessage,
		Message: &savedMessage,
	})
	if len(focusPoints) > 0 {
		session.currentRoom.Events().BroadcastEvent(realtime.Event{
			Type:        realtime.EventTypeFocusUpdate,
			FocusPoints: focusPoints,
		})
	}

	triggerCtx := context.Background()
	if ctx != nil {
		triggerCtx = context.WithoutCancel(ctx)
	}
	s.TriggerAgentResponses(triggerCtx, session.currentRoom, savedMessage)
	return nil
}

func (s *RoomService) TransferRealtimeOwner(ctx context.Context, session *RealtimeSession, targetParticipantID string) error {
	if session == nil || session.currentRoom == nil {
		return ErrRoomNotFound
	}
	return s.TransferRoomOwner(ctx, session.currentRoom, session.participant.ID, targetParticipantID)
}

func (s *RoomService) CloseRealtimeRoom(ctx context.Context, session *RealtimeSession) error {
	if session == nil || session.currentRoom == nil {
		return ErrRoomNotFound
	}
	return s.CloseRoomByOwner(ctx, session.currentRoom, session.participant.ID)
}
