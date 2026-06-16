package service

import (
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/room"
)

func (s *RoomService) CanAccessRoom(currentRoom *room.Room, passcode string) bool {
	if currentRoom == nil {
		return false
	}
	return RoomPasscodeMatches(currentRoom.PasscodeHash(), passcode)
}

func (s *RoomService) agentByID(agentID string) (model.AgentConfig, bool) {
	for _, configuredAgent := range s.agents.Agents() {
		if configuredAgent.ID == agentID {
			return configuredAgent, true
		}
	}
	return model.AgentConfig{}, false
}
