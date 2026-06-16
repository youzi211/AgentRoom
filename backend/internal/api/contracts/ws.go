package contracts

import "agentroom/backend/internal/realtime"

const (
	EventTypeMessage           = realtime.EventTypeMessage
	EventTypeRoomSnapshot      = realtime.EventTypeRoomSnapshot
	EventTypeRoomClosed        = realtime.EventTypeRoomClosed
	EventTypeRoomArchived      = realtime.EventTypeRoomArchived
	EventTypeParticipantJoined = realtime.EventTypeParticipantJoined
	EventTypeParticipantLeft   = realtime.EventTypeParticipantLeft
	EventTypeError             = realtime.EventTypeError
	EventTypeFocusUpdate       = realtime.EventTypeFocusUpdate
	EventTypeAgentActivity     = realtime.EventTypeAgentActivity
)

type ClientEvent struct {
	Type          string `json:"type"`
	Content       string `json:"content,omitempty"`
	ParticipantID string `json:"participantID,omitempty"`
}

type ServerEvent = realtime.Event
type AgentActivityEvent = realtime.Activity
