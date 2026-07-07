package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
)

func (s *Server) getRoomForRead(c *gin.Context) (*room.Room, bool) {
	roomID := c.Param("roomID")
	currentRoom, ok := s.roomQueries.GetRoom(c.Request.Context(), roomID)
	if !ok {
		writeError(c, http.StatusNotFound, "room not found")
		return nil, false
	}
	if s.hasValidAdminKey(c) {
		return currentRoom, true
	}
	if !s.roomAccess.CanAccessRoom(currentRoom, roomPasscodeFromRequest(c.Request)) {
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
	currentRoom, ok := s.roomQueries.GetRoom(c.Request.Context(), roomID)
	if !ok {
		writeError(c, http.StatusNotFound, "room not found")
		return nil, false
	}
	if s.hasValidAdminKey(c) {
		return currentRoom, true
	}
	if !s.roomAccess.CanAccessRoom(currentRoom, roomPasscodeFromRequest(c.Request)) {
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
	currentRoom, ok := s.roomQueries.GetRoom(c.Request.Context(), roomID)
	if !ok {
		writeError(c, http.StatusNotFound, "room not found")
		return nil, false
	}
	return currentRoom, true
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

func minutesFilename(roomInfo model.RoomMeta) string {
	base := strings.TrimSpace(roomInfo.Name)
	if base == "" {
		base = roomInfo.ID
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
		base = roomInfo.ID
	}
	return base + "-minutes.md"
}

func artifactDownloadFilename(artifact model.MessageArtifact) string {
	base := strings.TrimSpace(artifact.FileName)
	if base == "" {
		base = strings.TrimSpace(artifact.Title)
	}
	if base == "" {
		base = strings.TrimSpace(artifact.ID)
	}
	base = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_' || r == '.':
			return r
		case r == ' ':
			return '-'
		default:
			return '-'
		}
	}, base)
	base = strings.Trim(base, "-")
	if base == "" {
		base = "artifact"
	}
	return base
}
