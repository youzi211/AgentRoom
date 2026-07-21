package store

import "errors"

var (
	ErrInvalidMessageCursor      = errors.New("invalid message cursor")
	ErrMessageNotFound           = errors.New("message not found")
	ErrAgentNotFound             = errors.New("agent not found")
	ErrRoomNotFound              = errors.New("room not found")
	ErrParticipantNotFound       = errors.New("participant not found")
	ErrKnowledgeDocumentNotFound = errors.New("knowledge document not found")
	ErrAgentRunNotFound          = errors.New("agent run not found")
	ErrAgentRunAlreadyFinished   = errors.New("agent run is already finished")
	ErrDialogueRunNotFound       = errors.New("dialogue run not found")
	ErrModelProfileNotFound      = errors.New("model profile not found")
	ErrModelProfileReferenced    = errors.New("model profile is referenced")
)
