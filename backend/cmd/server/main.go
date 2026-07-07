package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/api"
	"agentroom/backend/internal/config"
	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/logging"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/store/mysql"
)

func main() {
	envErr := config.LoadDotEnv(filepath.Join("..", ".env"))
	logging.Init()
	logger := logging.Component("server")

	if envErr != nil {
		logger.Warn("load .env", "error", envErr)
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	dbConfig := config.LoadDBConfig()
	securityConfig := config.LoadSecurityConfig()
	deepAgentConfig := config.LoadDeepAgentConfig()
	if dbConfig.DSN == "" {
		fatal(logger, "MYSQL_DSN is required. Set it in .env or environment variables.", nil)
	}

	store, err := mysql.Open(dbConfig.DSN)
	if err != nil {
		fatal(logger, "connect to mysql", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Ping(ctx); err != nil {
		fatal(logger, "ping mysql", err)
	}
	logger.Info("connected to mysql")

	if dbConfig.AutoMigrate {
		if err := store.Migrate(ctx); err != nil {
			fatal(logger, "run migrations", err)
		}
		logger.Info("database migrations applied")
	}

	deepAgentRegistryPath := filepath.Join(deepAgentConfig.WorkDir, deepAgentConfig.Registry)
	deepAgentAgents, err := agent.LoadDeepAgentRegistryAgents(deepAgentRegistryPath)
	if err != nil {
		fatal(logger, "load deepagent registry", err)
	}
	agentDefinitions := agent.MergeAgentDefinitions(agent.PredefinedAgents(), deepAgentAgents)
	if err := store.SeedAgents(ctx, agentDefinitions); err != nil {
		fatal(logger, "seed agents", err)
	}
	if len(deepAgentAgents) > 0 {
		logger.Info("loaded deepagent registry agents", "count", len(deepAgentAgents), "path", deepAgentRegistryPath)
	}

	agents, err := store.ListAgents(ctx)
	if err != nil {
		fatal(logger, "load agents", err)
	}

	agentService := service.NewAgentService(store, agents)
	knowledgeService := service.NewKnowledgeService(store)
	manager := room.NewManager(store, agentService.ResolveForRoom)
	llmClient := llm.NewClientFromEnv()
	runner := agent.NewRunner(llmClient, store).WithKnowledge(knowledgeService)
	runner.WithRuntimeRegistry(agent.NewRuntimeRegistry(
		agent.NewLLMAgentRuntime(llmClient, 45*time.Second),
		agent.NewDeepAgentRuntime(agent.DeepAgentRuntimeConfig{
			Command:     deepAgentConfig.Command,
			WorkDir:     deepAgentConfig.WorkDir,
			Config:      deepAgentConfig.Config,
			Timeout:     deepAgentConfig.Timeout,
			Concurrency: deepAgentConfig.Concurrency,
		}),
	))
	focusService := service.NewFocusService(llmClient)
	minutesService := service.NewMinutesService(llmClient)
	roomService := service.NewRoomService(manager, agentService, knowledgeService, runner, focusService, store).WithMinutes(minutesService)
	server := api.NewServerWithConfig(api.Dependencies{
		Queries:  roomService.Queries(),
		Commands: roomService.Commands(),
		Access:   roomService.Access(),
	}, api.Config{
		AdminAPIKey:    securityConfig.AdminAPIKey,
		AllowedOrigins: securityConfig.AllowedOrigins,
	})

	now := time.Now().UTC()
	if err := store.MarkAllActiveParticipantsLeft(ctx, now); err != nil {
		logger.Warn("could not mark orphaned participants as left", "error", err)
	}

	address := ":" + port
	logger.Info("AgentRoom backend listening", "address", address)
	if err := http.ListenAndServe(address, server.Routes()); err != nil {
		fatal(logger, "start server", err)
	}
}

func fatal(logger *slog.Logger, message string, err error) {
	if err != nil {
		logger.Error(message, "error", err)
	} else {
		logger.Error(message)
	}
	os.Exit(1)
}
