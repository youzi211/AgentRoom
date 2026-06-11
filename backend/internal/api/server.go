package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/store"
)

type Server struct {
	manager  *room.Manager
	runner   *agent.Runner
	store    store.Store
	upgrader websocket.Upgrader
}

func NewServer(manager *room.Manager, runner *agent.Runner) *Server {
	return &Server{
		manager: manager,
		runner:  runner,
		store:   manager.Store(),
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
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	s.registerAPIRoutes(router.Group("/api"))

	// Keep legacy routes during the transition; the frontend uses /api/* so
	// application pages can safely own paths such as /agents and /rooms/:id.
	s.registerAPIRoutes(router.Group(""))

	return router
}

func (s *Server) registerAPIRoutes(routes gin.IRoutes) {
	routes.GET("/health", s.handleHealth)
	routes.GET("/agents", s.handleAgents)
	routes.PUT("/agents/:agentID", s.handleUpdateAgent)
	routes.POST("/rooms", s.handleCreateRoom)
	routes.GET("/rooms/:roomID", s.handleGetRoom)
	routes.GET("/rooms/:roomID/messages", s.handleGetMessages)
	routes.GET("/rooms/:roomID/ws", s.handleRoomWebSocket)
}

func (s *Server) handleHealth(c *gin.Context) {
	dbOK := true
	if err := s.store.Ping(c.Request.Context()); err != nil {
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
	c.JSON(http.StatusOK, model.AgentsResponse{Agents: s.manager.Agents()})
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

	updated, ok := s.manager.UpdateAgent(c.Request.Context(), agentID, room.UpdateAgentInput{
		Name:         request.Name,
		Role:         request.Role,
		Description:  request.Description,
		SystemPrompt: request.SystemPrompt,
		Enabled:      request.Enabled,
	})
	if !ok {
		writeError(c, http.StatusNotFound, "agent not found")
		return
	}

	c.JSON(http.StatusOK, updated.Config())
}

func (s *Server) handleCreateRoom(c *gin.Context) {
	var request model.CreateRoomRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&request); err != nil && !errors.Is(err, context.Canceled) {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	currentRoom, err := s.manager.CreateRoom(c.Request.Context(), request.Name)
	if err != nil {
		log.Printf("create room: %v", err)
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

	// Try loading from store first for consistency
	messages, err := s.store.ListMessages(c.Request.Context(), store.ListMessagesQuery{
		RoomID: currentRoom.Info().ID,
		Limit:  limit,
	})
	if err != nil {
		log.Printf("list messages from store: %v", err)
		// Fallback to in-memory messages
		messages = currentRoom.Messages()
	}

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
		log.Printf("upgrade websocket: %v", err)
		return
	}

	// Create and persist participant
	participant := currentRoom.NewParticipant(name)
	savedParticipant, err := s.store.AddParticipant(c.Request.Context(), store.AddParticipantInput{
		ID:          participant.ID,
		RoomID:      currentRoom.Info().ID,
		DisplayName: participant.Name,
		JoinedAt:    participant.JoinedAt,
	})
	if err != nil {
		log.Printf("persist participant: %v", err)
		// Still continue; persistence failure should not block the user
		savedParticipant = participant
	}
	currentRoom.AddParticipantFromStore(savedParticipant)

	client := &room.Client{
		ID:            model.NewID("client"),
		ParticipantID: savedParticipant.ID,
		Send:          make(chan model.ServerEvent, 16),
	}
	currentRoom.Hub().Register(client)

	var cleanup sync.Once
	cleanupFn := func() {
		currentRoom.Hub().Unregister(client)
		if currentRoom.RemoveParticipant(savedParticipant.ID) {
			currentRoom.Hub().Broadcast(model.ServerEvent{
				Type:          model.EventTypeParticipantLeft,
				ParticipantID: savedParticipant.ID,
			})
		}
		// Persist participant left
		if err := s.store.MarkParticipantLeft(context.Background(), savedParticipant.ID, time.Now().UTC()); err != nil {
			log.Printf("mark participant left: %v", err)
		}
		if err := connection.Close(); err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			log.Printf("close websocket connection: %v", err)
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
				log.Printf("websocket read error: %v", err)
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

		// Create message model
		message := currentRoom.NewHumanMessage(participant, content)

		// Persist to store first (design: write to DB before broadcast)
		savedMessage, err := s.store.AddMessage(context.Background(), message)
		if err != nil {
			log.Printf("persist message: %v", err)
			sendClientEvent(client, model.ServerEvent{
				Type:  model.EventTypeError,
				Error: "failed to send message, please try again",
			})
			return
		}

		// Add to in-memory and broadcast
		currentRoom.AppendMessage(savedMessage)
		currentRoom.Hub().Broadcast(model.ServerEvent{Type: model.EventTypeMessage, Message: &savedMessage})
		go s.runner.HandleHumanMessage(context.Background(), currentRoom, savedMessage)
	default:
		sendClientEvent(client, model.ServerEvent{Type: model.EventTypeError, Error: fmt.Sprintf("unsupported event type %q", event.Type)})
	}
}

func (s *Server) writePump(connection *websocket.Conn, client *room.Client, onDone func()) {
	defer onDone()

	for event := range client.Send {
		if err := connection.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			log.Printf("set websocket write deadline: %v", err)
			return
		}
		if err := connection.WriteJSON(event); err != nil {
			log.Printf("websocket write error: %v", err)
			return
		}
	}
}

func (s *Server) getRoomFromRequest(c *gin.Context) (*room.Room, bool) {
	roomID := c.Param("roomID")
	currentRoom, ok := s.manager.GetRoom(c.Request.Context(), roomID)
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
