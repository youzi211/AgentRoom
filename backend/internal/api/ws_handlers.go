package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"agentroom/backend/internal/api/contracts"
	"agentroom/backend/internal/realtime"
	"agentroom/backend/internal/service"
)

const realtimeSessionCleanupTimeout = 5 * time.Second

func (s *Server) handleRoomWebSocket(c *gin.Context) {
	currentRoom, ok := s.getRoomForLive(c)
	if !ok {
		return
	}

	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		writeError(c, http.StatusBadRequest, "missing name query parameter")
		return
	}

	connection, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		s.logger.Warn("upgrade websocket", "room_id", currentRoom.Info().ID, "error", err)
		return
	}

	sessionCtx, cancelSession := context.WithCancel(c.Request.Context())
	session, err := s.roomCommands.OpenRealtimeSession(sessionCtx, currentRoom, name)
	if err != nil {
		_ = connection.WriteJSON(realtime.Event{Type: realtime.EventTypeError, Error: lifecycleErrorMessage(err)})
		cancelSession()
		_ = connection.Close()
		return
	}

	var cleanup sync.Once
	cleanupFn := func() {
		cancelSession()
		cleanupCtx, cancelCleanup := realtimeSessionCleanupContext(sessionCtx)
		defer cancelCleanup()
		s.roomCommands.CloseRealtimeSession(cleanupCtx, session)
		if err := connection.Close(); err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			s.logger.Warn("close websocket connection", "participant_id", session.Participant().ID, "error", err)
		}
	}
	defer cleanup.Do(cleanupFn)

	go s.writePump(connection, session, func() {
		cleanup.Do(cleanupFn)
	})

	connection.SetReadLimit(1 << 20)
	for {
		var event contracts.ClientEvent
		if err := connection.ReadJSON(&event); err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) && !websocket.IsUnexpectedCloseError(err, websocket.CloseAbnormalClosure) {
				s.logger.Warn("websocket read error", "room_id", currentRoom.Info().ID, "participant_id", session.Participant().ID, "error", err)
			}
			return
		}

		s.handleClientEvent(sessionCtx, session, event)
	}
}

func (s *Server) handleClientEvent(ctx context.Context, session *service.RealtimeSession, event contracts.ClientEvent) {
	switch event.Type {
	case realtime.EventTypeMessage:
		err := s.roomCommands.PostRealtimeMessage(ctx, session, event.Content)
		if err != nil {
			errMessage := "failed to send message, please try again"
			switch {
			case errors.Is(err, service.ErrMessageContentEmpty):
				errMessage = "message content must not be empty"
			case errors.Is(err, service.ErrRoomArchived):
				errMessage = "this meeting has been archived and is read-only"
			case errors.Is(err, service.ErrRoomClosed):
				errMessage = "this meeting is closed and read-only"
			}
			sendSessionEvent(session, realtime.Event{
				Type:  realtime.EventTypeError,
				Error: errMessage,
			})
		}
	case "close_room":
		if err := s.roomCommands.CloseRealtimeRoom(ctx, session); err != nil {
			sendSessionEvent(session, realtime.Event{Type: realtime.EventTypeError, Error: lifecycleErrorMessage(err)})
		}
	case "transfer_owner":
		targetParticipantID := strings.TrimSpace(event.ParticipantID)
		if targetParticipantID == "" {
			sendSessionEvent(session, realtime.Event{Type: realtime.EventTypeError, Error: "missing target participant"})
			return
		}
		if err := s.roomCommands.TransferRealtimeOwner(ctx, session, targetParticipantID); err != nil {
			sendSessionEvent(session, realtime.Event{Type: realtime.EventTypeError, Error: lifecycleErrorMessage(err)})
		}
	default:
		sendSessionEvent(session, realtime.Event{Type: realtime.EventTypeError, Error: fmt.Sprintf("unsupported event type %q", event.Type)})
	}
}

func (s *Server) writePump(connection *websocket.Conn, session *service.RealtimeSession, onDone func()) {
	defer onDone()

	for event := range session.Events() {
		if err := connection.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			s.logger.Warn("set websocket write deadline", "client_id", session.ClientID(), "error", err)
			return
		}
		if err := connection.WriteJSON(event); err != nil {
			s.logger.Warn("websocket write error", "client_id", session.ClientID(), "error", err)
			return
		}
	}
}

func sendSessionEvent(session *service.RealtimeSession, event realtime.Event) {
	if session != nil {
		session.Send(event)
	}
}

func realtimeSessionCleanupContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), realtimeSessionCleanupTimeout)
}
