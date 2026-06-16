package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"agentroom/backend/internal/room"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/store"
	"agentroom/backend/internal/tests/teststore"
)

func TestAgentServiceDeleteAgentMapsStoreNotFound(t *testing.T) {
	backingStore := &teststore.Store{
		DeleteAgentErr: fmt.Errorf("delete failed: %w", store.ErrAgentNotFound),
	}
	svc := service.NewAgentService(backingStore, nil)

	err := svc.DeleteAgent(context.Background(), "agent_missing")
	if !errors.Is(err, service.ErrAgentNotFound) {
		t.Fatalf("expected ErrAgentNotFound, got %v", err)
	}
}

func TestKnowledgeServiceDeleteDocumentMapsStoreNotFound(t *testing.T) {
	backingStore := &teststore.Store{
		DeleteDocumentErr: fmt.Errorf("delete failed: %w", store.ErrKnowledgeDocumentNotFound),
	}
	svc := service.NewKnowledgeService(backingStore)

	err := svc.DeleteDocument(context.Background(), "doc_missing")
	if !errors.Is(err, service.ErrKnowledgeDocumentNotFound) {
		t.Fatalf("expected ErrKnowledgeDocumentNotFound, got %v", err)
	}
}

func TestRoomServiceListRoomKnowledgeMissingRoomReturnsSentinel(t *testing.T) {
	backingStore := &teststore.Store{}
	agentService := service.NewAgentService(backingStore, nil)
	knowledgeService := service.NewKnowledgeService(backingStore)
	manager := room.NewManager(backingStore, agentService.ResolveForRoom)
	roomService := service.NewRoomService(manager, agentService, knowledgeService, nil, nil, backingStore)

	_, err := roomService.ListRoomKnowledge(context.Background(), "room_missing")
	if !errors.Is(err, service.ErrRoomNotFound) {
		t.Fatalf("expected ErrRoomNotFound, got %v", err)
	}
}

func TestRoomServiceSaveManualMinutesBlankContentReturnsSentinel(t *testing.T) {
	backingStore := &teststore.Store{}
	roomService := service.NewRoomService(nil, nil, nil, nil, nil, backingStore)
	currentRoom := room.New("room_1", "Planning", nil)

	_, err := roomService.SaveManualMinutes(context.Background(), currentRoom, " \n\t ")
	if !errors.Is(err, service.ErrMinutesContentEmpty) {
		t.Fatalf("expected ErrMinutesContentEmpty, got %v", err)
	}
}
