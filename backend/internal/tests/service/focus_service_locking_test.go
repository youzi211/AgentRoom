package service_test

import (
	"context"
	"testing"
	"time"

	"agentroom/backend/internal/llm"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/service"
)

type blockingFocusLLM struct {
	started chan struct{}
	release chan struct{}
}

func (c *blockingFocusLLM) Complete(context.Context, []llm.ChatMessage) (string, error) {
	select {
	case c.started <- struct{}{}:
	default:
	}
	<-c.release
	return `[{"content":"排期风险","category":"风险"}]`, nil
}

func TestFocusServiceAllowsConcurrentReadsDuringAnalysis(t *testing.T) {
	client := &blockingFocusLLM{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	focusService := service.NewFocusService(client)
	base := time.Now().UTC()

	for i := 0; i < 2; i++ {
		focusService.AddMessage(context.Background(), "room-1", model.Message{
			ID:         "msg_warmup",
			RoomID:     "room-1",
			SenderID:   "human-1",
			SenderName: "Alice",
			SenderType: model.SenderTypeHuman,
			Content:    "讨论发布风险",
			CreatedAt:  base.Add(time.Duration(i) * time.Minute),
		})
	}

	addDone := make(chan []model.FocusPoint, 1)
	go func() {
		addDone <- focusService.AddMessage(context.Background(), "room-1", model.Message{
			ID:         "msg_trigger",
			RoomID:     "room-1",
			SenderID:   "human-1",
			SenderName: "Alice",
			SenderType: model.SenderTypeHuman,
			Content:    "需要分析新的会议焦点",
			CreatedAt:  base.Add(2 * time.Minute),
		})
	}()

	select {
	case <-client.started:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected third message to start focus analysis")
	}

	readDone := make(chan []model.FocusPoint, 1)
	go func() {
		readDone <- focusService.GetFocusPoints("room-1")
	}()

	select {
	case <-readDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected GetFocusPoints not to block while focus analysis is in flight")
	}

	close(client.release)

	select {
	case points := <-addDone:
		if len(points) != 1 || points[0].Content != "排期风险" {
			t.Fatalf("expected analysis result after release, got %#v", points)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected AddMessage to finish after releasing analysis")
	}
}
