package collaboration_test

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"agentroom/backend/internal/collaboration"
)

func TestFakeRuntimeRecordsRequestAndStreamsEventsInOrder(t *testing.T) {
	wantRequest := collaboration.Request{
		ProtocolVersion:    "v1",
		CollaborationRunID: "collab_1",
		TraceID:            "trace_1",
		Engine:             collaboration.EngineNative,
		Snapshot: collaboration.ConversationSnapshot{
			Room:   collaboration.RoomSnapshot{ID: "room_1", Name: "Planning", Status: "active"},
			Agents: []collaboration.AgentSnapshot{{ID: "agent_1", Name: "Architect"}},
			Policy: collaboration.PolicySnapshot{
				Version: "v1", Engine: collaboration.EngineNative,
				TriggerMode: collaboration.TriggerMentionOnly, MaxTurns: 3, MaxTurnsPerAgent: 1,
			},
			Limits: collaboration.ExecutionLimits{Timeout: time.Second},
		},
	}
	wantEvents := []collaboration.Event{
		{CollaborationRunID: "collab_1", Sequence: 1, Kind: collaboration.EventAccepted},
		{CollaborationRunID: "collab_1", Sequence: 2, Kind: collaboration.EventCompleted, Terminal: &collaboration.Terminal{
			TurnCount: 0, Reason: collaboration.StopReasonCompleted,
		}},
	}
	fake := &Runtime{Events: wantEvents}

	stream, err := fake.ExecuteConversation(context.Background(), wantRequest)
	if err != nil {
		t.Fatal(err)
	}
	var gotEvents []collaboration.Event
	for {
		event, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			t.Fatal(recvErr)
		}
		gotEvents = append(gotEvents, event)
	}

	if gotRequests := fake.Requests(); !reflect.DeepEqual(gotRequests, []collaboration.Request{wantRequest}) {
		t.Fatalf("unexpected recorded requests: %#v", gotRequests)
	}
	if !reflect.DeepEqual(gotEvents, wantEvents) {
		t.Fatalf("unexpected streamed events: %#v", gotEvents)
	}
}

func TestFakeRuntimeInjectsExecuteAndStreamErrors(t *testing.T) {
	executeErr := errors.New("execute failed")
	fake := &Runtime{ExecuteErr: executeErr}
	if _, err := fake.ExecuteConversation(context.Background(), collaboration.Request{}); !errors.Is(err, executeErr) {
		t.Fatalf("expected execute error, got %v", err)
	}

	streamErr := errors.New("stream failed")
	fake = &Runtime{StreamErr: streamErr}
	stream, err := fake.ExecuteConversation(context.Background(), collaboration.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); !errors.Is(err, streamErr) {
		t.Fatalf("expected stream error, got %v", err)
	}
}

func TestFakeRuntimeObservesContextCancellation(t *testing.T) {
	fake := &Runtime{
		WaitForCancellation: true,
		Started:             make(chan struct{}),
		Cancelled:           make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := fake.ExecuteConversation(ctx, collaboration.Request{})
	if err != nil {
		t.Fatal(err)
	}
	<-fake.Started
	cancel()
	if _, err := stream.Recv(); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	select {
	case <-fake.Cancelled:
	case <-time.After(time.Second):
		t.Fatal("fake runtime did not observe cancellation")
	}
}

func TestCollaborationPortHasNoTransportOrFrameworkDependencies(t *testing.T) {
	// After merging collaboration subpackages, only the neutral type files
	// (types.go, events.go, runtime.go) must stay free of transport/framework deps.
	neutralFiles := []string{
		filepath.Join("..", "..", "collaboration", "types.go"),
		filepath.Join("..", "..", "collaboration", "events.go"),
		filepath.Join("..", "..", "collaboration", "runtime.go"),
	}
	fset := token.NewFileSet()
	for _, path := range neutralFiles {
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			if imported.Path.Value != `"context"` && imported.Path.Value != `"time"` && imported.Path.Value != `"errors"` {
				t.Fatalf("neutral collaboration file imports %s in %s", imported.Path.Value, path)
			}
		}
	}
}
