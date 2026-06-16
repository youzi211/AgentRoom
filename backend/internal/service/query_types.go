package service

import "agentroom/backend/internal/model"

type ListRoomsInput struct {
	Status string
	Limit  int
	Offset int
}

type MessagePage struct {
	Messages   []model.Message
	HasMore    bool
	NextBefore string
}
