package realtime

import (
	"time"

	"agentroom/backend/internal/model"
)

const (
	EventTypeMessage           = "message"
	EventTypeRoomSnapshot      = "room_snapshot"
	EventTypeRoomClosed        = "room_closed"
	EventTypeRoomArchived      = "room_archived"
	EventTypeParticipantJoined = "participant_joined"
	EventTypeParticipantLeft   = "participant_left"
	EventTypeError             = "error"
	EventTypeFocusUpdate       = "focus_update"
	EventTypeAgentActivity     = "agent_activity"
)

type Activity struct {
	Kind             string     `json:"kind"`
	Phase            string     `json:"phase"`
	ID               string     `json:"id"`
	RoomID           string     `json:"roomID"`
	AgentID          string     `json:"agentID,omitempty"`
	AgentName        string     `json:"agentName,omitempty"`
	TriggerMessageID string     `json:"triggerMessageID,omitempty"`
	Status           string     `json:"status,omitempty"`
	ErrorText        string     `json:"errorText,omitempty"`
	RuntimeEvent     string     `json:"runtimeEvent,omitempty"`
	ModelName        string     `json:"modelName,omitempty"`
	ToolName         string     `json:"toolName,omitempty"`
	TurnCount        int        `json:"turnCount,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
}

type Event struct {
	Type          string              `json:"type"`
	Room          *model.RoomMeta     `json:"room,omitempty"`
	Participants  []model.Participant `json:"participants,omitempty"`
	Agents        []model.Agent       `json:"agents,omitempty"`
	Messages      []model.Message     `json:"messages,omitempty"`
	Message       *model.Message      `json:"message,omitempty"`
	Participant   *model.Participant  `json:"participant,omitempty"`
	ParticipantID string              `json:"participantID,omitempty"`
	FocusPoints   []model.FocusPoint  `json:"focusPoints,omitempty"`
	Activity      *Activity           `json:"activity,omitempty"`
	Error         string              `json:"error,omitempty"`
}
