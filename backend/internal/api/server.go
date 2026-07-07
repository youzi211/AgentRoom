package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"agentroom/backend/internal/logging"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/service"
)

type Server struct {
	roomQueries    RoomQueryService
	roomCommands   RoomCommandService
	roomAccess     RoomAccessService
	logger         *slog.Logger
	config         Config
	allowedOrigins map[string]struct{}
	upgrader       websocket.Upgrader
}

type Config struct {
	AdminAPIKey    string
	AllowedOrigins []string
}

type RoomQueryService interface {
	Ping(ctx context.Context) error
	Agents() []model.AgentConfig
	GetRoom(ctx context.Context, roomID string) (*room.Room, bool)
	ListRooms(ctx context.Context, query service.ListRoomsInput) ([]model.RoomSummary, error)
	ListAgentKnowledge(ctx context.Context, agentID string) ([]model.KnowledgeDocument, error)
	ListMessagesPage(ctx context.Context, currentRoom *room.Room, limit int, before string) (service.MessagePage, error)
	GetMessageArtifact(ctx context.Context, currentRoom *room.Room, messageID string, artifactID string) (model.MessageArtifact, error)
	ListRoomActivity(ctx context.Context, currentRoom *room.Room, limit int) (service.RoomActivity, error)
	LatestPersistedMinutesMarkdown(ctx context.Context, currentRoom *room.Room) (string, bool, error)
	ListMinutes(ctx context.Context, currentRoom *room.Room) ([]model.MeetingMinutes, error)
	ListRoomKnowledge(ctx context.Context, roomID string) ([]model.KnowledgeDocument, error)
}

type RoomCommandService interface {
	UpdateAgent(ctx context.Context, agentID string, input service.UpdateAgentInput) (model.Agent, error)
	CreateAgent(ctx context.Context, name, role, description, systemPrompt string, enabled bool, runtime string) (model.Agent, error)
	DeleteAgent(ctx context.Context, agentID string) error
	UploadAgentKnowledge(ctx context.Context, agentID string, fileName string, content []byte) (model.KnowledgeDocument, error)
	CreateRoom(ctx context.Context, name string, agentIDs []string, passcode string, dialoguePolicy model.DialoguePolicy) (*room.Room, error)
	GenerateMinutes(ctx context.Context, currentRoom *room.Room) (string, model.MeetingMinutes, error)
	ReopenRoom(ctx context.Context, roomID string) error
	ArchiveRoom(ctx context.Context, roomID string) error
	RestoreRoom(ctx context.Context, roomID string) error
	SaveManualMinutes(ctx context.Context, currentRoom *room.Room, content string) (model.MeetingMinutes, error)
	UploadRoomKnowledge(ctx context.Context, roomID string, fileName string, content []byte) (model.KnowledgeDocument, error)
	DeleteKnowledgeDocument(ctx context.Context, documentID string) error
	OpenRealtimeSession(ctx context.Context, currentRoom *room.Room, name string) (*service.RealtimeSession, error)
	CloseRealtimeSession(ctx context.Context, session *service.RealtimeSession)
	PostRealtimeMessage(ctx context.Context, session *service.RealtimeSession, content string) error
	CloseRealtimeRoom(ctx context.Context, session *service.RealtimeSession) error
	TransferRealtimeOwner(ctx context.Context, session *service.RealtimeSession, targetParticipantID string) error
}

type RoomAccessService interface {
	CanAccessRoom(currentRoom *room.Room, passcode string) bool
}

type Dependencies struct {
	Queries  RoomQueryService
	Commands RoomCommandService
	Access   RoomAccessService
}

func NewServer(deps Dependencies) *Server {
	return NewServerWithConfig(deps, Config{})
}

func NewServerWithConfig(deps Dependencies, config Config) *Server {
	server := &Server{
		roomQueries:    deps.Queries,
		roomCommands:   deps.Commands,
		roomAccess:     deps.Access,
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
	routes.GET("/recent-rooms", s.handleListRecentRooms)
	routes.POST("/rooms/:roomID/archive", s.requireAdmin, s.handleArchiveRoom)
	routes.POST("/rooms/:roomID/reopen", s.requireAdmin, s.handleReopenRoom)
	routes.POST("/rooms/:roomID/restore", s.requireAdmin, s.handleRestoreRoom)
	routes.GET("/rooms/:roomID/minutes/history", s.requireAdmin, s.handleListMinutes)
	routes.PUT("/rooms/:roomID/minutes", s.requireAdmin, s.handleSaveMinutes)
	routes.GET("/agents", s.handleAgents)
	routes.GET("/agent-templates", s.handleAgentTemplates)
	routes.GET("/agent-role-sets", s.handleAgentRoleSets)
	routes.POST("/agents", s.requireAdmin, s.handleCreateAgent)
	routes.PUT("/agents/:agentID", s.requireAdmin, s.handleUpdateAgent)
	routes.DELETE("/agents/:agentID", s.requireAdmin, s.handleDeleteAgent)
	routes.GET("/agents/:agentID/knowledge", s.handleListAgentKnowledge)
	routes.POST("/agents/:agentID/knowledge", s.requireAdmin, s.handleUploadAgentKnowledge)
	routes.POST("/rooms", s.handleCreateRoom)
	routes.GET("/rooms/:roomID", s.handleGetRoom)
	routes.GET("/rooms/:roomID/messages", s.handleGetMessages)
	routes.GET("/rooms/:roomID/messages/:messageID/artifacts/:artifactID", s.handleDownloadMessageArtifact)
	routes.GET("/rooms/:roomID/activity", s.handleGetRoomActivity)
	routes.GET("/rooms/:roomID/knowledge", s.handleListRoomKnowledge)
	routes.POST("/rooms/:roomID/knowledge", s.requireAdmin, s.handleUploadRoomKnowledge)
	routes.DELETE("/knowledge/:documentID", s.requireAdmin, s.handleDeleteKnowledgeDocument)
	routes.POST("/rooms/:roomID/minutes", s.handleGenerateMinutes)
	routes.GET("/rooms/:roomID/minutes.md", s.handleDownloadMinutes)
	routes.GET("/rooms/:roomID/ws", s.handleRoomWebSocket)
}
