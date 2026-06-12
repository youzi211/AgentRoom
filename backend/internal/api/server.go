package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"agentroom/backend/internal/logging"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/service"
)

type Server struct {
	rooms    *service.RoomService
	logger   *slog.Logger
	upgrader websocket.Upgrader
}

func NewServer(rooms *service.RoomService) *Server {
	return &Server{
		rooms:  rooms,
		logger: logging.Component("api"),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (s *Server) Routes() http.Handler {
	router := gin.New()
	router.Use(logging.RequestLogger(s.logger))
	router.Use(logging.Recovery(s.logger))

	s.registerAPIRoutes(router.Group("/api"))

	// Keep legacy routes during the transition; the frontend uses /api/* so
	// application pages can safely own paths such as /agents and /rooms/:id.
	s.registerAPIRoutes(router.Group(""))

	return router
}

func (s *Server) registerAPIRoutes(routes gin.IRoutes) {
	routes.GET("/health", s.handleHealth)
	routes.GET("/agents", s.handleAgents)
	routes.POST("/agents", s.handleCreateAgent)
	routes.PUT("/agents/:agentID", s.handleUpdateAgent)
	routes.DELETE("/agents/:agentID", s.handleDeleteAgent)
	routes.POST("/rooms", s.handleCreateRoom)
	routes.GET("/rooms/:roomID", s.handleGetRoom)
	routes.GET("/rooms/:roomID/messages", s.handleGetMessages)
	routes.GET("/rooms/:roomID/ws", s.handleRoomWebSocket)
}

func (s *Server) handleHealth(c *gin.Context) {
	dbOK := true
	if err := s.rooms.Ping(c.Request.Context()); err != nil {
		dbOK = false
	}

	c.JSON(http.StatusOK, gin.H{
		"ok": true,
		"database": gin.H{
			"ok": dbOK,
		},
	})
}

func (s *Server) handleAgents(c *gin.Context) {
	c.JSON(http.StatusOK, model.AgentsResponse{Agents: s.rooms.Agents()})
}

func (s *Server) handleUpdateAgent(c *gin.Context) {
	agentID := strings.TrimSpace(c.Param("agentID"))
	if agentID == "" {
		writeError(c, http.StatusBadRequest, "missing agent id")
		return
	}

	var request model.UpdateAgentRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&request); err != nil && !errors.Is(err, context.Canceled) {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	updated, err := s.rooms.UpdateAgent(c.Request.Context(), agentID, service.UpdateAgentInput{
		Name:         request.Name,
		Role:         request.Role,
		Description:  request.Description,
		SystemPrompt: request.SystemPrompt,
		Enabled:      request.Enabled,
	})
	if err != nil {
		if errors.Is(err, service.ErrAgentNotFound) {
			writeError(c, http.StatusNotFound, "agent not found")
			return
		}
		if errors.Is(err, service.ErrAgentMentionExists) {
			writeError(c, http.StatusConflict, "agent name already exists")
			return
		}
		s.logger.Error("update agent", "agent_id", agentID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to update agent")
		return
	}

	if updated.ID == "" {
		writeError(c, http.StatusNotFound, "agent not found")
		return
	}

	c.JSON(http.StatusOK, updated.Config())
}

func (s *Server) handleCreateAgent(c *gin.Context) {
	var request model.CreateAgentRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&request); err != nil && !errors.Is(err, context.Canceled) {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(request.Name)
	if name == "" {
		writeError(c, http.StatusBadRequest, "agent name is required")
		return
	}

	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}

	created, err := s.rooms.CreateAgent(c.Request.Context(), name, request.Role, request.Description, request.SystemPrompt, enabled)
	if err != nil {
		if errors.Is(err, service.ErrAgentMentionExists) {
			writeError(c, http.StatusConflict, "agent name already exists")
			return
		}
		s.logger.Error("create agent", "agent_name", name, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to create agent")
		return
	}

	c.JSON(http.StatusCreated, created.Config())
}

func (s *Server) handleDeleteAgent(c *gin.Context) {
	agentID := strings.TrimSpace(c.Param("agentID"))
	if agentID == "" {
		writeError(c, http.StatusBadRequest, "missing agent id")
		return
	}

	if err := s.rooms.DeleteAgent(c.Request.Context(), agentID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(c, http.StatusNotFound, "agent not found")
			return
		}
		s.logger.Error("delete agent", "agent_id", agentID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to delete agent")
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleCreateRoom(c *gin.Context) {
	var request model.CreateRoomRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&request); err != nil && !errors.Is(err, context.Canceled) {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	currentRoom, err := s.rooms.CreateRoom(c.Request.Context(), request.Name, request.AgentIDs)
	if err != nil {
		s.logger.Error("create room", "room_name", request.Name, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to create room")
		return
	}

	c.JSON(http.StatusCreated, model.CreateRoomResponse{Room: currentRoom.Info()})
}

func (s *Server) handleGetRoom(c *gin.Context) {
	currentRoom, ok := s.getRoomFromRequest(c)
	if !ok {
		return
	}

	c.JSON(http.StatusOK, model.GetRoomResponse{
		Room:         currentRoom.Info(),
		Participants: currentRoom.Participants(),
		Agents:       currentRoom.Agents(),
	})
}

func (s *Server) handleGetMessages(c *gin.Context) {
	currentRoom, ok := s.getRoomFromRequest(c)
	if !ok {
		return
	}

	// Support limit query parameter
	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	messages := s.rooms.ListMessages(c.Request.Context(), currentRoom, limit)

	c.JSON(http.StatusOK, model.GetMessagesResponse{Messages: messages})
}

func (s *Server) handleRoomWebSocket(c *gin.Context) {
	currentRoom, ok := s.getRoomFromRequest(c)
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

	savedParticipant := s.rooms.JoinParticipant(c.Request.Context(), currentRoom, name)

	client := &room.Client{
		ID:            model.NewID("client"),
		ParticipantID: savedParticipant.ID,
		Send:          make(chan model.ServerEvent, 16),
	}
	currentRoom.Hub().Register(client)

	var cleanup sync.Once
	cleanupFn := func() {
		currentRoom.Hub().Unregister(client)
		if s.rooms.LeaveParticipant(context.Background(), currentRoom, savedParticipant.ID) {
			currentRoom.Hub().Broadcast(model.ServerEvent{
				Type:          model.EventTypeParticipantLeft,
				ParticipantID: savedParticipant.ID,
			})
		}
		if err := connection.Close(); err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			s.logger.Warn("close websocket connection", "participant_id", savedParticipant.ID, "error", err)
		}
	}
	defer cleanup.Do(cleanupFn)

	go s.writePump(connection, client, func() {
		cleanup.Do(cleanupFn)
	})

	currentRoom.Hub().BroadcastExcept(model.ServerEvent{
		Type:        model.EventTypeParticipantJoined,
		Participant: &savedParticipant,
	}, client)

	client.Send <- snapshotEvent(currentRoom.Snapshot())

	connection.SetReadLimit(1 << 20)
	for {
		var event model.ClientEvent
		if err := connection.ReadJSON(&event); err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) && !websocket.IsUnexpectedCloseError(err, websocket.CloseAbnormalClosure) {
				s.logger.Warn("websocket read error", "room_id", currentRoom.Info().ID, "participant_id", savedParticipant.ID, "error", err)
			}
			return
		}

		s.handleClientEvent(currentRoom, savedParticipant, client, event)
	}
}

func (s *Server) handleClientEvent(currentRoom *room.Room, participant model.Participant, client *room.Client, event model.ClientEvent) {
	switch event.Type {
	case model.EventTypeMessage:
		content := strings.TrimSpace(event.Content)
		if content == "" {
			sendClientEvent(client, model.ServerEvent{Type: model.EventTypeError, Error: "message content must not be empty"})
			return
		}

		savedMessage, err := s.rooms.HandleHumanMessage(context.Background(), currentRoom, participant, content)
		if err != nil {
			sendClientEvent(client, model.ServerEvent{
				Type:  model.EventTypeError,
				Error: "failed to send message, please try again",
			})
			return
		}

		currentRoom.Hub().Broadcast(model.ServerEvent{Type: model.EventTypeMessage, Message: &savedMessage})
	default:
		sendClientEvent(client, model.ServerEvent{Type: model.EventTypeError, Error: fmt.Sprintf("unsupported event type %q", event.Type)})
	}
}

func (s *Server) writePump(connection *websocket.Conn, client *room.Client, onDone func()) {
	defer onDone()

	for event := range client.Send {
		if err := connection.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			s.logger.Warn("set websocket write deadline", "client_id", client.ID, "error", err)
			return
		}
		if err := connection.WriteJSON(event); err != nil {
			s.logger.Warn("websocket write error", "client_id", client.ID, "error", err)
			return
		}
	}
}

func (s *Server) getRoomFromRequest(c *gin.Context) (*room.Room, bool) {
	roomID := c.Param("roomID")
	currentRoom, ok := s.rooms.GetRoom(c.Request.Context(), roomID)
	if !ok {
		writeError(c, http.StatusNotFound, "room not found")
		return nil, false
	}
	return currentRoom, true
}

func sendClientEvent(client *room.Client, event model.ServerEvent) {
	select {
	case client.Send <- event:
	default:
	}
}

func snapshotEvent(state model.RoomState) model.ServerEvent {
	return model.ServerEvent{
		Type:         model.EventTypeRoomSnapshot,
		Room:         &state.Room,
		Participants: state.Participants,
		Agents:       state.Agents,
		Messages:     state.Messages,
	}
}

func writeError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, model.ErrorResponse{Error: message})
}
