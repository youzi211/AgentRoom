package store

import "errors"

var (
	ErrInvalidMessageCursor       = errors.New("invalid message cursor")
	ErrAgentNotFound              = errors.New("agent not found")
	ErrRoomNotFound               = errors.New("room not found")
	ErrParticipantNotFound        = errors.New("participant not found")
	ErrKnowledgeDocumentNotFound  = errors.New("knowledge document not found")
	ErrAgentRunNotFound           = errors.New("agent run not found")
	ErrDialogueRunNotFound        = errors.New("dialogue run not found")
)
