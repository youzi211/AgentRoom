package service

import "errors"

var (
	ErrAgentNotFound             = errors.New("agent not found")
	ErrAgentMentionExists        = errors.New("agent mention already exists")
	ErrInvalidAgentRuntime       = errors.New("invalid agent runtime")
	ErrRoomNotFound              = errors.New("room not found")
	ErrRoomClosed                = errors.New("room is closed")
	ErrRoomArchived              = errors.New("room is archived")
	ErrInvalidRoomTransition     = errors.New("invalid room transition")
	ErrNotRoomOwner              = errors.New("not room owner")
	ErrOwnerTargetNotOnline      = errors.New("owner transfer target is not an online participant")
	ErrKnowledgeDocumentNotFound = errors.New("knowledge document not found")
	ErrKnowledgeInvalidScope     = errors.New("invalid knowledge scope")
	ErrKnowledgeInvalidFile      = errors.New("invalid knowledge file")
	ErrKnowledgeTooLarge         = errors.New("knowledge file is too large")
	ErrMinutesContentEmpty       = errors.New("minutes content must not be empty")
	ErrMessageContentEmpty       = errors.New("message content must not be empty")
	ErrMessageNotFound           = errors.New("message not found")
	ErrMessageArtifactNotFound   = errors.New("message artifact not found")
)
