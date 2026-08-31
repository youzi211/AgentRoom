package service

import "time"

type RoomActivity struct {
	AgentRuns         []AgentRunActivity
	DialogueRuns      []DialogueRunActivity
	CollaborationRuns []CollaborationRunActivity
}

type AgentRunActivity struct {
	ID               string
	RoomID           string
	AgentID          string
	AgentName        string
	TriggerMessageID string
	Status           string
	ErrorText        string
	CreatedAt        time.Time
	CompletedAt      *time.Time
}

type DialogueRunActivity struct {
	ID               string
	RoomID           string
	TriggerMessageID string
	Mode             string
	TurnCount        int
	Status           string
	CreatedAt        time.Time
	CompletedAt      *time.Time
}

type CollaborationRunActivity struct {
	ID            string
	RoomID        string
	RootMessageID string
	Engine        string
	EngineVersion string
	PolicyVersion string
	Status        string
	StopReason    string
	TurnCount     int
	ErrorText     string
	CreatedAt     time.Time
	StartedAt     *time.Time
	CompletedAt   *time.Time
}
