package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	"agentroom/backend/internal/store"
)

type Server struct {
	rooms          *service.RoomService
	logger         *slog.Logger
	config         Config
	allowedOrigins map[string]struct{}
	upgrader       websocket.Upgrader
}

type Config struct {
	AdminAPIKey    string
	AllowedOrigins []string
}

func NewServer(rooms *service.RoomService) *Server {
	return NewServerWithConfig(rooms, Config{})
}

func NewServerWithConfig(rooms *service.RoomService, config Config) *Server {
	server := &Server{
		rooms:          rooms,
		logger:         logging.Component("api"),
		config:         config,
		allowedOrigins: originSet(config.AllowedOrigins),
	}
	server.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return server.allowsOrigin(r.Header.Get("Origin"))
		},
	}
	return server
}

func (s *Server) RoomsForTest() *service.RoomService {
	return s.rooms
}

func (s *Server) AllowsOriginForTest(origin string) bool {
	return s.allowsOrigin(origin)
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
	routes.GET("/admin/verify", s.requireAdmin, s.handleAdminVerify)
	routes.GET("/rooms", s.requireAdmin, s.handleListRooms)
	routes.POST("/rooms/:roomID/archive", s.requireAdmin, s.handleArchiveRoom)
	routes.POST("/rooms/:roomID/reopen", s.requireAdmin, s.handleReopenRoom)
	routes.POST("/rooms/:roomID/restore", s.requireAdmin, s.handleRestoreRoom)
	routes.GET("/rooms/:roomID/minutes/history", s.requireAdmin, s.handleListMinutes)
	routes.PUT("/rooms/:roomID/minutes", s.requireAdmin, s.handleSaveMinutes)
	routes.GET("/agents", s.handleAgents)
	routes.POST("/agents", s.requireAdmin, s.handleCreateAgent)
	routes.PUT("/agents/:agentID", s.requireAdmin, s.handleUpdateAgent)
	routes.DELETE("/agents/:agentID", s.requireAdmin, s.handleDeleteAgent)
	routes.GET("/agents/:agentID/knowledge", s.handleListAgentKnowledge)
	routes.POST("/agents/:agentID/knowledge", s.requireAdmin, s.handleUploadAgentKnowledge)
	routes.POST("/rooms", s.handleCreateRoom)
	routes.GET("/rooms/:roomID", s.handleGetRoom)
	routes.GET("/rooms/:roomID/messages", s.handleGetMessages)
	routes.GET("/rooms/:roomID/activity", s.handleGetRoomActivity)
	routes.GET("/rooms/:roomID/knowledge", s.handleListRoomKnowledge)
	routes.POST("/rooms/:roomID/knowledge", s.requireAdmin, s.handleUploadRoomKnowledge)
	routes.DELETE("/knowledge/:documentID", s.requireAdmin, s.handleDeleteKnowledgeDocument)
	routes.POST("/rooms/:roomID/minutes", s.handleGenerateMinutes)
	routes.GET("/rooms/:roomID/minutes.md", s.handleDownloadMinutes)
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

func (s *Server) handleListAgentKnowledge(c *gin.Context) {
	agentID := strings.TrimSpace(c.Param("agentID"))
	if agentID == "" {
		writeError(c, http.StatusBadRequest, "missing agent id")
		return
	}

	documents, err := s.rooms.ListAgentKnowledge(c.Request.Context(), agentID)
	if err != nil {
		if errors.Is(err, service.ErrAgentNotFound) {
			writeError(c, http.StatusNotFound, "agent not found")
			return
		}
		s.logger.Error("list agent knowledge", "agent_id", agentID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to list agent knowledge")
		return
	}

	c.JSON(http.StatusOK, model.KnowledgeDocumentsResponse{Documents: documents})
}

func (s *Server) handleUploadAgentKnowledge(c *gin.Context) {
	agentID := strings.TrimSpace(c.Param("agentID"))
	if agentID == "" {
		writeError(c, http.StatusBadRequest, "missing agent id")
		return
	}

	fileName, content, ok := readMarkdownUpload(c)
	if !ok {
		return
	}

	document, err := s.rooms.UploadAgentKnowledge(c.Request.Context(), agentID, fileName, content)
	if err != nil {
		s.writeKnowledgeError(c, err, "failed to upload agent knowledge")
		return
	}

	c.JSON(http.StatusCreated, model.UploadKnowledgeResponse{Document: document})
}

func (s *Server) handleCreateRoom(c *gin.Context) {
	var request model.CreateRoomRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&request); err != nil && !errors.Is(err, context.Canceled) {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	dialoguePolicy := request.DialoguePolicy.Resolve()

	currentRoom, err := s.rooms.CreateRoom(c.Request.Context(), request.Name, request.AgentIDs, request.Passcode, dialoguePolicy)
	if err != nil {
		s.logger.Error("create room", "room_name", request.Name, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to create room")
		return
	}

	c.JSON(http.StatusCreated, model.CreateRoomResponse{Room: currentRoom.Info()})
}

func (s *Server) handleGetRoom(c *gin.Context) {
	currentRoom, ok := s.getRoomForRead(c)
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
	currentRoom, ok := s.getRoomForRead(c)
	if !ok {
		return
	}

	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	before := strings.TrimSpace(c.Query("before"))

	page, err := s.rooms.ListMessagesPage(c.Request.Context(), currentRoom, limit, before)
	if err != nil {
		if errors.Is(err, store.ErrInvalidMessageCursor) {
			writeError(c, http.StatusBadRequest, "invalid message cursor")
			return
		}
		s.logger.Error("list room messages", "room_id", currentRoom.Info().ID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to load room messages")
		return
	}

	c.JSON(http.StatusOK, model.GetMessagesResponse{
		Messages:   page.Messages,
		HasMore:    page.HasMore,
		NextBefore: page.NextBefore,
	})
}

func (s *Server) handleGetRoomActivity(c *gin.Context) {
	currentRoom, ok := s.getRoomForRead(c)
	if !ok {
		return
	}

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100
	}

	activity, err := s.rooms.ListRoomActivity(c.Request.Context(), currentRoom, limit)
	if err != nil {
		s.logger.Error("list room activity", "room_id", currentRoom.Info().ID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to list room activity")
		return
	}

	c.JSON(http.StatusOK, activity)
}

func (s *Server) handleGenerateMinutes(c *gin.Context) {
	currentRoom, ok := s.getRoomForMinutesWrite(c)
	if !ok {
		return
	}

	markdown, minutes, err := s.rooms.GenerateMinutes(c.Request.Context(), currentRoom)
	if err != nil {
		if errors.Is(err, service.ErrRoomClosed) || errors.Is(err, service.ErrRoomArchived) {
			writeError(c, http.StatusForbidden, "meeting is read-only")
			return
		}
		s.logger.Error("generate minutes", "room_id", currentRoom.Info().ID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to generate meeting minutes")
		return
	}

	response := model.GenerateMinutesResponse{Markdown: markdown}
	if minutes.ID != "" {
		saved := minutes
		response.Minutes = &saved
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) handleDownloadMinutes(c *gin.Context) {
	currentRoom, ok := s.getRoomForRead(c)
	if !ok {
		return
	}

	markdown, ok, err := s.rooms.LatestPersistedMinutesMarkdown(c.Request.Context(), currentRoom)
	if err != nil {
		s.logger.Error("download minutes", "room_id", currentRoom.Info().ID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to export meeting minutes")
		return
	}
	if !ok {
		writeError(c, http.StatusNotFound, "meeting minutes not found")
		return
	}

	fileName := minutesFilename(currentRoom.Info())
	c.Header("Content-Type", "text/markdown; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	c.String(http.StatusOK, markdown)
}

func (s *Server) handleAdminVerify(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleListRooms(c *gin.Context) {
	query := store.ListRoomsQuery{
		Status: strings.TrimSpace(c.Query("status")),
		Limit:  50,
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			query.Limit = parsed
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			query.Offset = parsed
		}
	}

	rooms, err := s.rooms.ListRooms(c.Request.Context(), query)
	if err != nil {
		s.logger.Error("list rooms", "error", err)
		writeError(c, http.StatusInternalServerError, "failed to list rooms")
		return
	}
	c.JSON(http.StatusOK, model.ListRoomsResponse{Rooms: rooms})
}

func (s *Server) handleArchiveRoom(c *gin.Context) {
	s.changeRoomStatus(c, true)
}

func (s *Server) handleReopenRoom(c *gin.Context) {
	roomID := strings.TrimSpace(c.Param("roomID"))
	if roomID == "" {
		writeError(c, http.StatusBadRequest, "missing room id")
		return
	}

	if err := s.rooms.ReopenRoom(c.Request.Context(), roomID); err != nil {
		s.writeLifecycleError(c, err)
		return
	}

	currentRoom, ok := s.getRoomForAdmin(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, model.CreateRoomResponse{Room: currentRoom.Info()})
}

func (s *Server) handleRestoreRoom(c *gin.Context) {
	s.changeRoomStatus(c, false)
}

func (s *Server) changeRoomStatus(c *gin.Context, archive bool) {
	roomID := strings.TrimSpace(c.Param("roomID"))
	if roomID == "" {
		writeError(c, http.StatusBadRequest, "missing room id")
		return
	}

	var err error
	if archive {
		err = s.rooms.ArchiveRoom(c.Request.Context(), roomID)
	} else {
		err = s.rooms.RestoreRoom(c.Request.Context(), roomID)
	}
	if err != nil {
		s.writeLifecycleError(c, err)
		return
	}
	currentRoom, ok := s.getRoomForAdmin(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, model.CreateRoomResponse{Room: currentRoom.Info()})
}

func (s *Server) handleListMinutes(c *gin.Context) {
	currentRoom, ok := s.getRoomForAdmin(c)
	if !ok {
		return
	}

	minutes, err := s.rooms.ListMinutes(c.Request.Context(), currentRoom)
	if err != nil {
		s.logger.Error("list minutes", "room_id", currentRoom.Info().ID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to list meeting minutes")
		return
	}
	c.JSON(http.StatusOK, model.MinutesHistoryResponse{Minutes: minutes})
}

func (s *Server) handleSaveMinutes(c *gin.Context) {
	currentRoom, ok := s.getRoomForAdmin(c)
	if !ok {
		return
	}

	var request model.SaveMinutesRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&request); err != nil && !errors.Is(err, context.Canceled) {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	saved, err := s.rooms.SaveManualMinutes(c.Request.Context(), currentRoom, request.Content)
	if err != nil {
		if strings.Contains(err.Error(), "must not be empty") {
			writeError(c, http.StatusBadRequest, "minutes content must not be empty")
			return
		}
		s.logger.Error("save minutes", "room_id", currentRoom.Info().ID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to save meeting minutes")
		return
	}
	c.JSON(http.StatusOK, saved)
}

func (s *Server) handleListRoomKnowledge(c *gin.Context) {
	currentRoom, ok := s.getRoomForRead(c)
	if !ok {
		return
	}

	documents, err := s.rooms.ListRoomKnowledge(c.Request.Context(), currentRoom.Info().ID)
	if err != nil {
		if errors.Is(err, service.ErrRoomNotFound) || strings.Contains(err.Error(), "room not found") {
			writeError(c, http.StatusNotFound, "room not found")
			return
		}
		s.logger.Error("list room knowledge", "room_id", currentRoom.Info().ID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to list room knowledge")
		return
	}

	c.JSON(http.StatusOK, model.KnowledgeDocumentsResponse{Documents: documents})
}

func (s *Server) handleUploadRoomKnowledge(c *gin.Context) {
	roomID := strings.TrimSpace(c.Param("roomID"))
	if roomID == "" {
		writeError(c, http.StatusBadRequest, "missing room id")
		return
	}

	fileName, content, ok := readMarkdownUpload(c)
	if !ok {
		return
	}

	document, err := s.rooms.UploadRoomKnowledge(c.Request.Context(), roomID, fileName, content)
	if err != nil {
		s.writeKnowledgeError(c, err, "failed to upload room knowledge")
		return
	}

	c.JSON(http.StatusCreated, model.UploadKnowledgeResponse{Document: document})
}

func (s *Server) handleDeleteKnowledgeDocument(c *gin.Context) {
	documentID := strings.TrimSpace(c.Param("documentID"))
	if documentID == "" {
		writeError(c, http.StatusBadRequest, "missing document id")
		return
	}

	if err := s.rooms.DeleteKnowledgeDocument(c.Request.Context(), documentID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(c, http.StatusNotFound, "knowledge document not found")
			return
		}
		s.logger.Error("delete knowledge document", "document_id", documentID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to delete knowledge document")
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

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

	savedParticipant, err := s.rooms.JoinParticipant(c.Request.Context(), currentRoom, name)
	if err != nil {
		_ = connection.WriteJSON(model.ServerEvent{Type: model.EventTypeError, Error: lifecycleErrorMessage(err)})
		_ = connection.Close()
		return
	}

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
	initialSnapshot := snapshotEvent(currentRoom.Snapshot())
	initialSnapshot.ParticipantID = savedParticipant.ID
	client.Send <- initialSnapshot
	currentRoom.Hub().BroadcastExcept(snapshotEvent(currentRoom.Snapshot()), client)

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

		savedMessage, focusPoints, err := s.rooms.HandleHumanMessage(context.Background(), currentRoom, participant, content)
		if err != nil {
			errMessage := "failed to send message, please try again"
			if errors.Is(err, service.ErrRoomArchived) {
				errMessage = "this meeting has been archived and is read-only"
			} else if errors.Is(err, service.ErrRoomClosed) {
				errMessage = "this meeting is closed and read-only"
			}
			sendClientEvent(client, model.ServerEvent{
				Type:  model.EventTypeError,
				Error: errMessage,
			})
			return
		}

		currentRoom.Hub().Broadcast(model.ServerEvent{Type: model.EventTypeMessage, Message: &savedMessage})

		if len(focusPoints) > 0 {
			currentRoom.Hub().Broadcast(model.ServerEvent{
				Type:        model.EventTypeFocusUpdate,
				FocusPoints: focusPoints,
			})
		}

		s.rooms.TriggerAgentResponses(context.Background(), currentRoom, savedMessage)
	case "close_room":
		if err := s.rooms.CloseRoomByOwner(context.Background(), currentRoom, participant.ID); err != nil {
			sendClientEvent(client, model.ServerEvent{Type: model.EventTypeError, Error: lifecycleErrorMessage(err)})
		}
	case "transfer_owner":
		targetParticipantID := strings.TrimSpace(event.ParticipantID)
		if targetParticipantID == "" {
			sendClientEvent(client, model.ServerEvent{Type: model.EventTypeError, Error: "missing target participant"})
			return
		}
		if err := s.rooms.TransferRoomOwner(context.Background(), currentRoom, participant.ID, targetParticipantID); err != nil {
			sendClientEvent(client, model.ServerEvent{Type: model.EventTypeError, Error: lifecycleErrorMessage(err)})
		}
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

func (s *Server) getRoomForRead(c *gin.Context) (*room.Room, bool) {
	roomID := c.Param("roomID")
	currentRoom, ok := s.rooms.GetRoom(c.Request.Context(), roomID)
	if !ok {
		writeError(c, http.StatusNotFound, "room not found")
		return nil, false
	}
	if s.hasValidAdminKey(c) {
		return currentRoom, true
	}
	if !s.rooms.CanAccessRoom(currentRoom, roomPasscodeFromRequest(c.Request)) {
		writeError(c, http.StatusForbidden, "room passcode is required or invalid")
		return nil, false
	}
	if currentRoom.Info().IsArchived() {
		writeError(c, http.StatusForbidden, "meeting has been archived")
		return nil, false
	}
	return currentRoom, true
}

func (s *Server) getRoomForLive(c *gin.Context) (*room.Room, bool) {
	currentRoom, ok := s.getRoomForRead(c)
	if !ok {
		return nil, false
	}
	switch currentRoom.Info().Status {
	case model.RoomStatusActive, "":
		return currentRoom, true
	case model.RoomStatusClosed:
		writeError(c, http.StatusConflict, "meeting is closed; read-only only")
		return nil, false
	default:
		writeError(c, http.StatusForbidden, "meeting has been archived")
		return nil, false
	}
}

func (s *Server) getRoomForMinutesWrite(c *gin.Context) (*room.Room, bool) {
	roomID := c.Param("roomID")
	currentRoom, ok := s.rooms.GetRoom(c.Request.Context(), roomID)
	if !ok {
		writeError(c, http.StatusNotFound, "room not found")
		return nil, false
	}
	if s.hasValidAdminKey(c) {
		return currentRoom, true
	}
	if !s.rooms.CanAccessRoom(currentRoom, roomPasscodeFromRequest(c.Request)) {
		writeError(c, http.StatusForbidden, "room passcode is required or invalid")
		return nil, false
	}
	if !currentRoom.Info().IsActive() {
		writeError(c, http.StatusForbidden, "meeting is read-only")
		return nil, false
	}
	return currentRoom, true
}

// hasValidAdminKey reports whether the request carries the configured admin key.
// When no admin key is configured, it returns false so normal passcode rules apply.
func (s *Server) hasValidAdminKey(c *gin.Context) bool {
	configured := strings.TrimSpace(s.config.AdminAPIKey)
	return configured != "" && c.GetHeader("X-Admin-Key") == configured
}

// getRoomForAdmin resolves a room for admin-gated routes. Authorization is the
// admin key (requireAdmin), so the per-room passcode is not required here.
func (s *Server) getRoomForAdmin(c *gin.Context) (*room.Room, bool) {
	roomID := c.Param("roomID")
	currentRoom, ok := s.rooms.GetRoom(c.Request.Context(), roomID)
	if !ok {
		writeError(c, http.StatusNotFound, "room not found")
		return nil, false
	}
	return currentRoom, true
}

func (s *Server) writeLifecycleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrRoomNotFound) || strings.Contains(err.Error(), "not found"):
		writeError(c, http.StatusNotFound, "room not found")
	case errors.Is(err, service.ErrInvalidRoomTransition):
		writeError(c, http.StatusConflict, "invalid room transition")
	case errors.Is(err, service.ErrRoomArchived):
		writeError(c, http.StatusForbidden, "meeting has been archived")
	case errors.Is(err, service.ErrRoomClosed):
		writeError(c, http.StatusConflict, "meeting is closed; read-only only")
	case errors.Is(err, service.ErrNotRoomOwner):
		writeError(c, http.StatusForbidden, "only the current meeting owner can do that")
	case errors.Is(err, service.ErrOwnerTargetNotOnline):
		writeError(c, http.StatusConflict, "owner can only be transferred to an online participant")
	default:
		s.logger.Error("room lifecycle mutation failed", "error", err)
		writeError(c, http.StatusInternalServerError, "failed to update room status")
	}
}

func lifecycleErrorMessage(err error) string {
	switch {
	case errors.Is(err, service.ErrRoomArchived):
		return "this meeting has been archived"
	case errors.Is(err, service.ErrRoomClosed):
		return "this meeting is closed and read-only"
	case errors.Is(err, service.ErrNotRoomOwner):
		return "only the current meeting owner can do that"
	case errors.Is(err, service.ErrOwnerTargetNotOnline):
		return "owner can only be transferred to an online participant"
	case err != nil:
		return err.Error()
	default:
		return "meeting action failed"
	}
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

func readMarkdownUpload(c *gin.Context) (string, []byte, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		writeError(c, http.StatusBadRequest, "missing markdown file")
		return "", nil, false
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		writeError(c, http.StatusBadRequest, "failed to read markdown file")
		return "", nil, false
	}

	return header.Filename, content, true
}

func (s *Server) writeKnowledgeError(c *gin.Context, err error, fallback string) {
	if errors.Is(err, service.ErrAgentNotFound) {
		writeError(c, http.StatusNotFound, "agent not found")
		return
	}
	if errors.Is(err, service.ErrKnowledgeInvalidFile) {
		writeError(c, http.StatusBadRequest, "only non-empty .md files are supported")
		return
	}
	if errors.Is(err, service.ErrKnowledgeTooLarge) {
		writeError(c, http.StatusRequestEntityTooLarge, "markdown file must be 1MB or smaller")
		return
	}
	if errors.Is(err, service.ErrKnowledgeInvalidScope) {
		writeError(c, http.StatusBadRequest, "invalid knowledge scope")
		return
	}
	if strings.Contains(err.Error(), "room not found") {
		writeError(c, http.StatusNotFound, "room not found")
		return
	}
	s.logger.Error("knowledge request failed", "error", err)
	writeError(c, http.StatusInternalServerError, fallback)
}

func (s *Server) requireAdmin(c *gin.Context) {
	if strings.TrimSpace(s.config.AdminAPIKey) == "" {
		return
	}
	if c.GetHeader("X-Admin-Key") != s.config.AdminAPIKey {
		writeError(c, http.StatusUnauthorized, "admin api key is required")
		c.Abort()
		return
	}
}

func (s *Server) allowsOrigin(origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" || len(s.allowedOrigins) == 0 {
		return true
	}
	_, ok := s.allowedOrigins[origin]
	return ok
}

func originSet(origins []string) map[string]struct{} {
	if len(origins) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}
	return allowed
}

func roomPasscodeFromRequest(request *http.Request) string {
	if request == nil {
		return ""
	}
	if headerValue := strings.TrimSpace(request.Header.Get("X-Room-Passcode")); headerValue != "" {
		return headerValue
	}
	return strings.TrimSpace(request.URL.Query().Get("passcode"))
}

func minutesFilename(room model.RoomMeta) string {
	base := strings.TrimSpace(room.Name)
	if base == "" {
		base = room.ID
	}
	base = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		case r == ' ':
			return '-'
		default:
			return '-'
		}
	}, base)
	base = strings.Trim(base, "-")
	if base == "" {
		base = room.ID
	}
	return base + "-minutes.md"
}
