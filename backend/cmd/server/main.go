package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"agentroom/backend/internal/agent"
	"agentroom/backend/internal/api"
	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/room"
)

func main() {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	manager := room.NewManager(agent.PredefinedAgents())
	runner := agent.NewRunner(llm.NewClientFromEnv())
	server := api.NewServer(manager, runner)

	address := ":" + port
	log.Printf("AgentRoom backend listening on %s", address)
	if err := http.ListenAndServe(address, server.Routes()); err != nil {
		log.Fatalf("start server: %v", err)
	}
}
