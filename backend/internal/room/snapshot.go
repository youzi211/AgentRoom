package room

import "agentroom/backend/internal/model"

type Snapshot struct {
	Room         model.RoomMeta      `json:"room"`
	Participants []model.Participant `json:"participants"`
	Agents       []model.Agent       `json:"agents"`
	Messages     []model.Message     `json:"messages,omitempty"`
}
