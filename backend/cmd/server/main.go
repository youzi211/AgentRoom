package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/api"
	"agentroom/backend/internal/config"
	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/room"
	"agentroom/backend/internal/store/mysql"
)

func main() {
	if err := config.LoadDotEnv(filepath.Join("..", ".env")); err != nil {
		log.Printf("load .env: %v", err)
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	// ── Database setup ───────────────────────────────────────────────
	dbConfig := config.LoadDBConfig()
	if dbConfig.DSN == "" {
		log.Fatalf("MYSQL_DSN is required. Set it in .env or environment variables.")
	}

	store, err := mysql.Open(dbConfig.DSN)
	if err != nil {
		log.Fatalf("connect to mysql: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Ping(ctx); err != nil {
		log.Fatalf("ping mysql: %v", err)
	}
	log.Println("connected to mysql")

	if dbConfig.AutoMigrate {
		if err := store.Migrate(ctx); err != nil {
			log.Fatalf("run migrations: %v", err)
		}
		log.Println("database migrations applied")
	}

	// Seed default agents if the table is empty
	if err := store.SeedAgents(ctx, agent.PredefinedAgents()); err != nil {
		log.Fatalf("seed agents: %v", err)
	}

	// Load current agents from store for Manager initialization
	agents, err := store.ListAgents(ctx)
	if err != nil {
		log.Fatalf("load agents: %v", err)
	}

	// ── Application setup ────────────────────────────────────────────
	manager := room.NewManager(store, agents)
	runner := agent.NewRunner(llm.NewClientFromEnv(), store)
	server := api.NewServer(manager, runner)

	// Mark any orphaned active participants as left (from a previous unclean shutdown)
	now := time.Now().UTC()
	if err := store.MarkAllActiveParticipantsLeft(ctx, now); err != nil {
		// Non-fatal: best-effort cleanup
		log.Printf("warn: could not mark orphaned participants as left: %v", err)
	}

	address := ":" + port
	log.Printf("AgentRoom backend listening on %s", address)
	if err := http.ListenAndServe(address, server.Routes()); err != nil {
		log.Fatalf("start server: %v", err)
	}
}
