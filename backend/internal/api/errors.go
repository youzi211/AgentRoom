package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"agentroom/backend/internal/api/contracts"
	"agentroom/backend/internal/service"
)

type errorResponseSpec struct {
	statusCode int
	message    string
}

func writeError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, contracts.ErrorResponse{Error: message})
}

func agentDeleteError(err error) (errorResponseSpec, bool) {
	if errors.Is(err, service.ErrAgentNotFound) {
		return errorResponseSpec{statusCode: http.StatusNotFound, message: "agent not found"}, true
	}
	return errorResponseSpec{}, false
}

func minutesSaveError(err error) (errorResponseSpec, bool) {
	if errors.Is(err, service.ErrMinutesContentEmpty) {
		return errorResponseSpec{statusCode: http.StatusBadRequest, message: "minutes content must not be empty"}, true
	}
	return errorResponseSpec{}, false
}

func roomKnowledgeError(err error) (errorResponseSpec, bool) {
	if errors.Is(err, service.ErrRoomNotFound) {
		return errorResponseSpec{statusCode: http.StatusNotFound, message: "room not found"}, true
	}
	return errorResponseSpec{}, false
}

func lifecycleHTTPError(err error) (errorResponseSpec, bool) {
	switch {
	case errors.Is(err, service.ErrRoomNotFound):
		return errorResponseSpec{statusCode: http.StatusNotFound, message: "room not found"}, true
	case errors.Is(err, service.ErrInvalidRoomTransition):
		return errorResponseSpec{statusCode: http.StatusConflict, message: "invalid room transition"}, true
	case errors.Is(err, service.ErrRoomArchived):
		return errorResponseSpec{statusCode: http.StatusForbidden, message: "meeting has been archived"}, true
	case errors.Is(err, service.ErrRoomClosed):
		return errorResponseSpec{statusCode: http.StatusConflict, message: "meeting is closed; read-only only"}, true
	case errors.Is(err, service.ErrNotRoomOwner):
		return errorResponseSpec{statusCode: http.StatusForbidden, message: "only the current meeting owner can do that"}, true
	case errors.Is(err, service.ErrOwnerTargetNotOnline):
		return errorResponseSpec{statusCode: http.StatusConflict, message: "owner can only be transferred to an online participant"}, true
	default:
		return errorResponseSpec{}, false
	}
}

func (s *Server) writeLifecycleError(c *gin.Context, err error) {
	if response, ok := lifecycleHTTPError(err); ok {
		writeError(c, response.statusCode, response.message)
		return
	}
	s.logger.Error("room lifecycle mutation failed", "error", err)
	writeError(c, http.StatusInternalServerError, "failed to update room status")
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

func knowledgeHTTPError(err error) (errorResponseSpec, bool) {
	switch {
	case errors.Is(err, service.ErrAgentNotFound):
		return errorResponseSpec{statusCode: http.StatusNotFound, message: "agent not found"}, true
	case errors.Is(err, service.ErrRoomNotFound):
		return errorResponseSpec{statusCode: http.StatusNotFound, message: "room not found"}, true
	case errors.Is(err, service.ErrKnowledgeDocumentNotFound):
		return errorResponseSpec{statusCode: http.StatusNotFound, message: "knowledge document not found"}, true
	case errors.Is(err, service.ErrKnowledgeInvalidFile):
		return errorResponseSpec{statusCode: http.StatusBadRequest, message: "only non-empty .md files are supported"}, true
	case errors.Is(err, service.ErrKnowledgeTooLarge):
		return errorResponseSpec{statusCode: http.StatusRequestEntityTooLarge, message: "markdown file must be 1MB or smaller"}, true
	case errors.Is(err, service.ErrKnowledgeInvalidScope):
		return errorResponseSpec{statusCode: http.StatusBadRequest, message: "invalid knowledge scope"}, true
	default:
		return errorResponseSpec{}, false
	}
}

func (s *Server) writeKnowledgeError(c *gin.Context, err error, fallback string) {
	if response, ok := knowledgeHTTPError(err); ok {
		writeError(c, response.statusCode, response.message)
		return
	}
	s.logger.Error("knowledge request failed", "error", err)
	writeError(c, http.StatusInternalServerError, fallback)
}
