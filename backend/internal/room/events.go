package room

import (
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/realtime"
)

type MessageBroadcaster interface {
	BroadcastMessage(message model.Message)
	BroadcastEvent(event realtime.Event)
}

type RealtimeEvents interface {
	MessageBroadcaster
	Register(client *Client)
	Unregister(client *Client)
	BroadcastExcept(event realtime.Event, excluded *Client)
	BroadcastAndClose(event realtime.Event)
}
