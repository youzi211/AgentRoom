package model

import "time"

const (
	SenderTypeHuman  = "human"
	SenderTypeAgent  = "agent"
	SenderTypeSystem = "system"
)

const (
	EventTypeMessage           = "message"
	EventTypeRoomSnapshot      = "room_snapshot"
	EventTypeParticipantJoined = "participant_joined"
	EventTypeParticipantLeft   = "participant_left"
	EventTypeError             = "error"
)

type Room struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Participants map[string]*Participant `json:"-"`
	Agents       map[string]*Agent       `json:"-"`
	Messages     []Message               `json:"-"`
	CreatedAt    time.Time               `json:"createdAt"`
}

type RoomMeta struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type RoomState struct {
	Room         RoomMeta      `json:"room"`
	Participants []Participant `json:"participants"`
	Agents       []Agent       `json:"agents"`
	Messages     []Message     `json:"messages,omitempty"`
}

type Participant struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	JoinedAt time.Time `json:"joinedAt"`
}

type Agent struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Mention      string `json:"mention"`
	Role         string `json:"role"`
	Description  string `json:"description"`
	Enabled      bool   `json:"enabled"`
	SystemPrompt string `json:"-"`
}

type AgentConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Mention      string `json:"mention"`
	Role         string `json:"role"`
	Description  string `json:"description"`
	Enabled      bool   `json:"enabled"`
	SystemPrompt string `json:"systemPrompt"`
}

type UpdateAgentRequest struct {
	Name         string `json:"name"`
	Role         string `json:"role"`
	Description  string `json:"description"`
	SystemPrompt string `json:"systemPrompt"`
	Enabled      *bool  `json:"enabled"`
}

type Message struct {
	ID         string    `json:"id"`
	RoomID     string    `json:"roomID"`
	SenderID   string    `json:"senderID"`
	SenderName string    `json:"senderName"`
	SenderType string    `json:"senderType"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"createdAt"`
}

type HealthResponse struct {
	OK bool `json:"ok"`
}

type AgentsResponse struct {
	Agents []AgentConfig `json:"agents"`
}

type CreateRoomRequest struct {
	Name string `json:"name"`
}

type CreateRoomResponse struct {
	Room RoomMeta `json:"room"`
}

type GetRoomResponse struct {
	Room         RoomMeta      `json:"room"`
	Participants []Participant `json:"participants"`
	Agents       []Agent       `json:"agents"`
}

type GetMessagesResponse struct {
	Messages []Message `json:"messages"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type ClientEvent struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
}

type ServerEvent struct {
	Type          string        `json:"type"`
	Room          *RoomMeta     `json:"room,omitempty"`
	Participants  []Participant `json:"participants,omitempty"`
	Agents        []Agent       `json:"agents,omitempty"`
	Messages      []Message     `json:"messages,omitempty"`
	Message       *Message      `json:"message,omitempty"`
	Participant   *Participant  `json:"participant,omitempty"`
	ParticipantID string        `json:"participantID,omitempty"`
	Error         string        `json:"error,omitempty"`
}

func (a Agent) Public() Agent {
	a.SystemPrompt = ""
	return a
}

func (a Agent) Config() AgentConfig {
	return AgentConfig{
		ID:           a.ID,
		Name:         a.Name,
		Mention:      a.Mention,
		Role:         a.Role,
		Description:  a.Description,
		Enabled:      a.Enabled,
		SystemPrompt: a.SystemPrompt,
	}
}
