package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"agentroom/backend/internal/api/contracts"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/store"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleListModelProfiles(c *gin.Context) {
	profiles, err := s.modelProfiles.List(c)
	if err != nil {
		writeError(c, 500, "failed to list model profiles")
		return
	}
	c.JSON(200, contracts.ModelProfilesResponse{Profiles: profiles})
}
func (s *Server) handleCreateModelProfile(c *gin.Context) {
	var r contracts.CreateModelProfileRequest
	if json.NewDecoder(c.Request.Body).Decode(&r) != nil {
		writeError(c, 400, "invalid request body")
		return
	}
	if r.Protocol == "" {
		r.Protocol = model.ModelProtocolOpenAIChatCompletions
	}
	p, err := s.modelProfiles.Create(c, service.CreateModelProfileInput{Name: r.Name, RuntimeScope: r.RuntimeScope, Protocol: r.Protocol, BaseURL: r.BaseURL, ModelName: r.ModelName, APIKey: r.APIKey, Enabled: r.Enabled, IsDefault: r.IsDefault})
	if err != nil {
		s.writeModelProfileError(c, err)
		return
	}
	c.JSON(http.StatusCreated, p)
}
func (s *Server) handleUpdateModelProfile(c *gin.Context) {
	var r contracts.UpdateModelProfileRequest
	if json.NewDecoder(c.Request.Body).Decode(&r) != nil {
		writeError(c, 400, "invalid request body")
		return
	}
	p, err := s.modelProfiles.Update(c, strings.TrimSpace(c.Param("profileID")), service.UpdateModelProfileInput{Name: r.Name, BaseURL: r.BaseURL, ModelName: r.ModelName, APIKey: r.APIKey, ClearAPIKey: r.ClearAPIKey, Enabled: r.Enabled})
	if err != nil {
		s.writeModelProfileError(c, err)
		return
	}
	c.JSON(200, p)
}
func (s *Server) handleSetDefaultModelProfile(c *gin.Context) {
	if err := s.modelProfiles.SetDefault(c, strings.TrimSpace(c.Param("profileID"))); err != nil {
		s.writeModelProfileError(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (s *Server) handleDeleteModelProfile(c *gin.Context) {
	if err := s.modelProfiles.Delete(c, strings.TrimSpace(c.Param("profileID"))); err != nil {
		s.writeModelProfileError(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (s *Server) handleTestDraftModelProfile(c *gin.Context) {
	var r contracts.TestModelProfileRequest
	if json.NewDecoder(c.Request.Body).Decode(&r) != nil {
		writeError(c, 400, "invalid request body")
		return
	}
	s.testModelProfile(c, service.TestModelProfileInput{BaseURL: r.BaseURL, ModelName: r.ModelName, APIKey: r.APIKey})
}
func (s *Server) handleTestSavedModelProfile(c *gin.Context) {
	s.testModelProfile(c, service.TestModelProfileInput{ProfileID: strings.TrimSpace(c.Param("profileID"))})
}
func (s *Server) testModelProfile(c *gin.Context, in service.TestModelProfileInput) {
	result, err := s.modelProfiles.TestConnection(c, in)
	if err != nil {
		s.writeModelProfileError(c, err)
		return
	}
	c.JSON(200, result)
}
func (s *Server) writeModelProfileError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrModelProfileNotFound):
		writeError(c, 404, "model profile not found")
	case errors.Is(err, store.ErrModelProfileReferenced):
		writeError(c, 409, "model profile is still referenced")
	case errors.Is(err, service.ErrDefaultModelProfile):
		writeError(c, 409, "default model profile must be replaced before it can be disabled")
	case errors.Is(err, service.ErrModelEncryptionNotConfigured):
		writeError(c, http.StatusServiceUnavailable, "model profile encryption is not configured; set MODEL_CONFIG_ENCRYPTION_KEY and restart the backend")
	case errors.Is(err, service.ErrInvalidModelProfile), errors.Is(err, service.ErrModelProfileDisabled):
		writeError(c, 400, err.Error())
	default:
		writeError(c, 500, "model profile operation failed")
	}
}
