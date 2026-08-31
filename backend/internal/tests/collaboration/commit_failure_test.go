package collaboration_test

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"agentroom/backend/internal/tests/teststore"
	"agentroom/backend/internal/collaboration"
)

type candidateRuntime struct {
	events    []collaboration.Event
	cancelled chan struct{}
	once      sync.Once
	stream    *candidateStream
}

func (r *candidateRuntime) ExecuteConversation(ctx context.Context, _ collaboration.Request) (collaboration.EventStream, error) {
	r.stream = &candidateStream{events: r.events}
	go func() {
		<-ctx.Done()
		r.once.Do(func() { close(r.cancelled) })
	}()
	return r.stream, nil
}

type candidateStream struct {
	events []collaboration.Event
	next   int
}

func (s *candidateStream) Recv() (collaboration.Event, error) {
	if s.next >= len(s.events) {
		return collaboration.Event{}, io.EOF
	}
	event := s.events[s.next]
	s.next++
	return event, nil
}

func TestCommitFailureCancelsStreamAndDiscardsLaterCandidateEvents(t *testing.T) {
	commitErr := errors.New("commit unavailable")
	memory := &teststore.Store{CommitAgentRunErr: commitErr}
	request := testRequest()
	request.Snapshot.Policy = collaboration.PolicySnapshot{MaxTurns: 2, MaxTurnsPerAgent: 1}
	runtime := &candidateRuntime{
		cancelled: make(chan struct{}),
		events: []collaboration.Event{
			streamEvent(1, collaboration.EventAccepted, "", ""),
			streamEvent(2, collaboration.EventSpeakerSelected, "turn_1", "agent_1"),
			streamEvent(3, collaboration.EventAgentTurnStarted, "turn_1", "agent_1"),
			completedStreamEvent(4, "turn_1", "agent_1", "not persisted"),
			streamEvent(5, collaboration.EventSpeakerSelected, "turn_2", "agent_2"),
			completedStreamEvent(6, "turn_2", "agent_2", "must be discarded"),
		},
	}
	coordinator, err := collaboration.NewCoordinator(runtime, collaboration.Config{MaxConcurrent: 1})
	if err != nil {
		t.Fatal(err)
	}
	turns, err := collaboration.NewTurnHandler(memory, request)
	if err != nil {
		t.Fatal(err)
	}
	err = coordinator.Execute(context.Background(), request, func(ctx context.Context, event collaboration.Event) error {
		_, _, err := turns.Handle(ctx, event)
		return err
	})
	if !errors.Is(err, commitErr) {
		t.Fatalf("expected commit failure, got %v", err)
	}
	select {
	case <-runtime.cancelled:
	case <-time.After(time.Second):
		t.Fatal("Runtime did not observe cancellation")
	}
	if runtime.stream.next != 4 {
		t.Fatalf("expected stream consumption to stop at failed commit, received %d events", runtime.stream.next)
	}
	if len(memory.AgentRuns) != 1 || memory.AgentRuns[0].Status != "failed" {
		t.Fatalf("later candidate created a run or failed turn did not converge: %#v", memory.AgentRuns)
	}
	if len(memory.RoomMessages["room_1"]) != 0 {
		t.Fatalf("uncommitted candidate entered chat history: %#v", memory.RoomMessages["room_1"])
	}
}

func streamEvent(sequence uint64, kind collaboration.EventKind, turnID string, agentID string) collaboration.Event {
	return collaboration.Event{
		ProtocolVersion: collaboration.ProtocolVersion, CollaborationRunID: "collab_1", Sequence: sequence,
		Kind: kind, TurnID: turnID, AgentID: agentID, OccurredAt: time.Now().UTC(),
	}
}

func completedStreamEvent(sequence uint64, turnID string, agentID string, content string) collaboration.Event {
	event := streamEvent(sequence, collaboration.EventAgentMessageCompleted, turnID, agentID)
	event.Message = &collaboration.AgentMessage{Content: content}
	return event
}
