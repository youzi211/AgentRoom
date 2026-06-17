package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/api/contracts"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/store"
)

func (s *Server) handleHealth(c *gin.Context) {
	dbOK := true
	if err := s.roomQueries.Ping(c.Request.Context()); err != nil {
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
	c.JSON(http.StatusOK, contracts.AgentsResponse{Agents: s.roomQueries.Agents()})
}

func (s *Server) handleAgentTemplates(c *gin.Context) {
	templates := agent.RoleTemplates()
	response := contracts.AgentTemplatesResponse{
		Templates: make([]contracts.RoleTemplate, 0, len(templates)),
	}
	for _, template := range templates {
		response.Templates = append(response.Templates, contracts.RoleTemplate{
			ID:           template.ID,
			Name:         template.Name,
			Role:         template.Role,
			Description:  template.Description,
			SystemPrompt: template.SystemPrompt,
		})
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) handleAgentRoleSets(c *gin.Context) {
	roleSets := agent.RoleSets()
	response := contracts.AgentRoleSetsResponse{
		RoleSets: make([]contracts.RoleSet, 0, len(roleSets)),
	}
	for _, roleSet := range roleSets {
		response.RoleSets = append(response.RoleSets, contracts.RoleSet{
			ID:          roleSet.ID,
			Name:        roleSet.Name,
			Description: roleSet.Description,
			TemplateIDs: append([]string(nil), roleSet.TemplateIDs...),
		})
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) handleUpdateAgent(c *gin.Context) {
	agentID := strings.TrimSpace(c.Param("agentID"))
	if agentID == "" {
		writeError(c, http.StatusBadRequest, "missing agent id")
		return
	}

	var request contracts.UpdateAgentRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&request); err != nil && !errors.Is(err, context.Canceled) {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	updated, err := s.roomCommands.UpdateAgent(c.Request.Context(), agentID, service.UpdateAgentInput{
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
	var request contracts.CreateAgentRequest
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

	created, err := s.roomCommands.CreateAgent(c.Request.Context(), name, request.Role, request.Description, request.SystemPrompt, enabled)
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

	if err := s.roomCommands.DeleteAgent(c.Request.Context(), agentID); err != nil {
		if response, ok := agentDeleteError(err); ok {
			writeError(c, response.statusCode, response.message)
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

	documents, err := s.roomQueries.ListAgentKnowledge(c.Request.Context(), agentID)
	if err != nil {
		if errors.Is(err, service.ErrAgentNotFound) {
			writeError(c, http.StatusNotFound, "agent not found")
			return
		}
		s.logger.Error("list agent knowledge", "agent_id", agentID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to list agent knowledge")
		return
	}

	c.JSON(http.StatusOK, contracts.KnowledgeDocumentsResponse{Documents: documents})
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

	document, err := s.roomCommands.UploadAgentKnowledge(c.Request.Context(), agentID, fileName, content)
	if err != nil {
		s.writeKnowledgeError(c, err, "failed to upload agent knowledge")
		return
	}

	c.JSON(http.StatusCreated, contracts.UploadKnowledgeResponse{Document: document})
}

func (s *Server) handleCreateRoom(c *gin.Context) {
	var request contracts.CreateRoomRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&request); err != nil && !errors.Is(err, context.Canceled) {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	dialoguePolicy := request.DialoguePolicy.Resolve()

	currentRoom, err := s.roomCommands.CreateRoom(c.Request.Context(), request.Name, request.AgentIDs, request.Passcode, dialoguePolicy)
	if err != nil {
		s.logger.Error("create room", "room_name", request.Name, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to create room")
		return
	}

	c.JSON(http.StatusCreated, contracts.CreateRoomResponse{Room: currentRoom.Info()})
}

func (s *Server) handleGetRoom(c *gin.Context) {
	currentRoom, ok := s.getRoomForRead(c)
	if !ok {
		return
	}

	c.JSON(http.StatusOK, contracts.GetRoomResponse{
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

	page, err := s.roomQueries.ListMessagesPage(c.Request.Context(), currentRoom, limit, before)
	if err != nil {
		if errors.Is(err, store.ErrInvalidMessageCursor) {
			writeError(c, http.StatusBadRequest, "invalid message cursor")
			return
		}
		s.logger.Error("list room messages", "room_id", currentRoom.Info().ID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to load room messages")
		return
	}

	c.JSON(http.StatusOK, contracts.GetMessagesResponse{
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

	activity, err := s.roomQueries.ListRoomActivity(c.Request.Context(), currentRoom, limit)
	if err != nil {
		s.logger.Error("list room activity", "room_id", currentRoom.Info().ID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to list room activity")
		return
	}

	c.JSON(http.StatusOK, roomActivityResponse(activity))
}

func (s *Server) handleGenerateMinutes(c *gin.Context) {
	currentRoom, ok := s.getRoomForMinutesWrite(c)
	if !ok {
		return
	}

	markdown, minutes, err := s.roomCommands.GenerateMinutes(c.Request.Context(), currentRoom)
	if err != nil {
		if errors.Is(err, service.ErrRoomClosed) || errors.Is(err, service.ErrRoomArchived) {
			writeError(c, http.StatusForbidden, "meeting is read-only")
			return
		}
		s.logger.Error("generate minutes", "room_id", currentRoom.Info().ID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to generate meeting minutes")
		return
	}

	response := contracts.GenerateMinutesResponse{Markdown: markdown}
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

	markdown, ok, err := s.roomQueries.LatestPersistedMinutesMarkdown(c.Request.Context(), currentRoom)
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
	query := service.ListRoomsInput{
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

	rooms, err := s.roomQueries.ListRooms(c.Request.Context(), query)
	if err != nil {
		s.logger.Error("list rooms", "error", err)
		writeError(c, http.StatusInternalServerError, "failed to list rooms")
		return
	}
	c.JSON(http.StatusOK, contracts.ListRoomsResponse{Rooms: rooms})
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

	if err := s.roomCommands.ReopenRoom(c.Request.Context(), roomID); err != nil {
		s.writeLifecycleError(c, err)
		return
	}

	currentRoom, ok := s.getRoomForAdmin(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, contracts.CreateRoomResponse{Room: currentRoom.Info()})
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
		err = s.roomCommands.ArchiveRoom(c.Request.Context(), roomID)
	} else {
		err = s.roomCommands.RestoreRoom(c.Request.Context(), roomID)
	}
	if err != nil {
		s.writeLifecycleError(c, err)
		return
	}
	currentRoom, ok := s.getRoomForAdmin(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, contracts.CreateRoomResponse{Room: currentRoom.Info()})
}

func (s *Server) handleListMinutes(c *gin.Context) {
	currentRoom, ok := s.getRoomForAdmin(c)
	if !ok {
		return
	}

	minutes, err := s.roomQueries.ListMinutes(c.Request.Context(), currentRoom)
	if err != nil {
		s.logger.Error("list minutes", "room_id", currentRoom.Info().ID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to list meeting minutes")
		return
	}
	c.JSON(http.StatusOK, contracts.MinutesHistoryResponse{Minutes: minutes})
}

func (s *Server) handleSaveMinutes(c *gin.Context) {
	currentRoom, ok := s.getRoomForAdmin(c)
	if !ok {
		return
	}

	var request contracts.SaveMinutesRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&request); err != nil && !errors.Is(err, context.Canceled) {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	saved, err := s.roomCommands.SaveManualMinutes(c.Request.Context(), currentRoom, request.Content)
	if err != nil {
		if response, ok := minutesSaveError(err); ok {
			writeError(c, response.statusCode, response.message)
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

	documents, err := s.roomQueries.ListRoomKnowledge(c.Request.Context(), currentRoom.Info().ID)
	if err != nil {
		if response, ok := roomKnowledgeError(err); ok {
			writeError(c, response.statusCode, response.message)
			return
		}
		s.logger.Error("list room knowledge", "room_id", currentRoom.Info().ID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to list room knowledge")
		return
	}

	c.JSON(http.StatusOK, contracts.KnowledgeDocumentsResponse{Documents: documents})
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

	document, err := s.roomCommands.UploadRoomKnowledge(c.Request.Context(), roomID, fileName, content)
	if err != nil {
		s.writeKnowledgeError(c, err, "failed to upload room knowledge")
		return
	}

	c.JSON(http.StatusCreated, contracts.UploadKnowledgeResponse{Document: document})
}

func (s *Server) handleDeleteKnowledgeDocument(c *gin.Context) {
	documentID := strings.TrimSpace(c.Param("documentID"))
	if documentID == "" {
		writeError(c, http.StatusBadRequest, "missing document id")
		return
	}

	if err := s.roomCommands.DeleteKnowledgeDocument(c.Request.Context(), documentID); err != nil {
		if response, ok := knowledgeHTTPError(err); ok {
			writeError(c, response.statusCode, response.message)
			return
		}
		s.logger.Error("delete knowledge document", "document_id", documentID, "error", err)
		writeError(c, http.StatusInternalServerError, "failed to delete knowledge document")
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
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

func roomActivityResponse(activity service.RoomActivity) contracts.RoomActivityResponse {
	response := contracts.RoomActivityResponse{
		AgentRuns:    make([]contracts.AgentRunActivity, 0, len(activity.AgentRuns)),
		DialogueRuns: make([]contracts.DialogueRunActivity, 0, len(activity.DialogueRuns)),
	}
	for _, run := range activity.AgentRuns {
		response.AgentRuns = append(response.AgentRuns, contracts.AgentRunActivity{
			ID:               run.ID,
			RoomID:           run.RoomID,
			AgentID:          run.AgentID,
			AgentName:        run.AgentName,
			TriggerMessageID: run.TriggerMessageID,
			Status:           run.Status,
			ErrorText:        run.ErrorText,
			CreatedAt:        run.CreatedAt,
			CompletedAt:      run.CompletedAt,
		})
	}
	for _, run := range activity.DialogueRuns {
		response.DialogueRuns = append(response.DialogueRuns, contracts.DialogueRunActivity{
			ID:               run.ID,
			RoomID:           run.RoomID,
			TriggerMessageID: run.TriggerMessageID,
			Mode:             run.Mode,
			TurnCount:        run.TurnCount,
			Status:           run.Status,
			CreatedAt:        run.CreatedAt,
			CompletedAt:      run.CompletedAt,
		})
	}
	return response
}
