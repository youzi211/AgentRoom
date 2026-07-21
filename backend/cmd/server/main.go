package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/api"
	"agentroom/backend/internal/config"
	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/logging"
	"agentroom/backend/internal/model"
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
	agentRuntimeConfig, err := config.LoadAgentRuntimeConfig()
	if err != nil {
		fatal(logger, "configure Agent Runtime transport", err)
	}
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
	interruptedRuns, err := store.ReconcileActiveAgentRuns(ctx, time.Now().UTC())
	if err != nil {
		fatal(logger, "reconcile interrupted agent runs", err)
	}
	if interruptedRuns > 0 {
		logger.Warn("reconciled interrupted agent runs", "count", interruptedRuns)
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

	agentService := service.NewAgentService(store, agents).WithModelProfiles(store)
	var secretCipher *service.SecretCipher
	if encryptionKey := strings.TrimSpace(os.Getenv("MODEL_CONFIG_ENCRYPTION_KEY")); encryptionKey != "" {
		secretCipher, err = service.NewSecretCipher(encryptionKey)
		if err != nil {
			fatal(logger, "configure model profile encryption", err)
		}
	}
	modelProfileService := service.NewModelProfileService(store, secretCipher, nil)
	modelResolver := service.NewModelResolver(store, secretCipher, map[string]service.EnvironmentModelConfig{
		model.ModelRuntimeGo:        {BaseURL: os.Getenv("LLM_BASE_URL"), ModelName: os.Getenv("LLM_MODEL"), APIKey: os.Getenv("LLM_API_KEY")},
		model.ModelRuntimeDeepAgent: {BaseURL: os.Getenv("MODEL_BASE_URL"), ModelName: os.Getenv("MODEL_NAME"), APIKey: os.Getenv("MODEL_API_KEY")},
	})
	knowledgeService := service.NewKnowledgeService(store)
	manager := room.NewManager(store, agentService.ResolveForRoom)
	llmClient := llm.NewResolvingClient(modelResolver, model.ModelRuntimeGo)
	runner := agent.NewRunner(llmClient, store).WithKnowledge(knowledgeService)
	var remoteClient *agent.RemoteRuntimeClient
	if agentRuntimeConfig.Transport == config.AgentRuntimeTransportGRPC {
		remoteClient, err = agent.NewRemoteRuntimeClient(agentRuntimeConfig)
		if err != nil {
			fatal(logger, "create remote Agent Runtime client", err)
		}
		defer remoteClient.Close()
		remoteLLM, err := agent.NewRemotePythonRuntime(model.AgentRuntimeLLM, remoteClient, agentRuntimeConfig.LLMTimeout, modelResolver)
		if err != nil {
			fatal(logger, "configure remote LLM runtime", err)
		}
		remoteDeepAgent, err := agent.NewRemotePythonRuntime(model.AgentRuntimeDeepAgent, remoteClient, agentRuntimeConfig.DeepTimeout, modelResolver)
		if err != nil {
			fatal(logger, "configure remote DeepAgent runtime", err)
		}
		runner.WithRuntimeRegistry(agent.NewRuntimeRegistry(remoteLLM, remoteDeepAgent))
	} else {
		runner.WithRuntimeRegistry(agent.NewRuntimeRegistry(
			agent.NewLLMAgentRuntime(llmClient, 45*time.Second).WithModelResolver(modelResolver),
			agent.NewDeepAgentRuntime(agent.DeepAgentRuntimeConfig{
				Command:     deepAgentConfig.Command,
				WorkDir:     deepAgentConfig.WorkDir,
				Config:      deepAgentConfig.Config,
				Timeout:     deepAgentConfig.Timeout,
				Concurrency: deepAgentConfig.Concurrency,
				Resolver:    modelResolver,
			}),
		))
	}
	focusService := service.NewFocusService(llmClient)
	minutesService := service.NewMinutesService(llmClient)
	roomService := service.NewRoomService(manager, agentService, knowledgeService, runner, focusService, store).WithMinutes(minutesService)
	server := api.NewServerWithConfig(api.Dependencies{
		Queries:       roomService.Queries(),
		Commands:      roomService.Commands(),
		Access:        roomService.Access(),
		ModelProfiles: modelProfileService,
		AgentRuntime:  remoteClient,
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
	listener, err := net.Listen("tcp", address)
	if err != nil {
		fatal(logger, "listen for backend requests", err)
	}
	httpServer := &http.Server{Handler: server.Routes(), ReadHeaderTimeout: 10 * time.Second}
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- httpServer.Serve(listener)
	}()

	shutdownSignal, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	select {
	case serveErr := <-serveErrors:
		if serveErr != nil && serveErr != http.ErrServerClosed {
			fatal(logger, "serve backend requests", serveErr)
		}
		return
	case <-shutdownSignal.Done():
		logger.Info("backend shutdown requested")
	}

	_ = listener.Close()
	if remoteClient != nil {
		remoteClient.CancelAll()
	}
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Warn("graceful backend shutdown", "error", err)
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
