package collaboration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

var ErrInvalidEvent = errors.New("invalid collaboration turn event")

const terminalWriteTimeout = 5 * time.Second

type turnState struct {
	runID     string
	turnID    string
	agentID   string
	committed bool
	run       store.AgentRun
}


type turnStore interface {
	CreateAgentRun(context.Context, store.AgentRun) error
	CommitAgentRunSuccess(context.Context, store.CommitAgentRunSuccessInput) (model.Message, error)
	FinishAgentRun(context.Context, string, string, string, time.Time) error
}


// Handler owns Agent run persistence for one validated collaboration stream.
type Handler struct {
	store               turnStore
	request             Request
	agentNames          map[string]string
	turns               map[string]turnState
	nextTurnIndex       int
	lastParentMessageID string
}

func NewTurnHandler(store turnStore, request Request) (*Handler, error) {
	if store == nil {
		return nil, fmt.Errorf("%w: store is required", ErrInvalidEvent)
	}
	if request.CollaborationRunID == "" || request.Snapshot.Room.ID == "" || request.Snapshot.Trigger.ID == "" {
		return nil, fmt.Errorf("%w: run, room, and trigger message IDs are required", ErrInvalidEvent)
	}
	agentNames := make(map[string]string, len(request.Snapshot.Agents))
	for _, agent := range request.Snapshot.Agents {
		agentNames[agent.ID] = agent.Name
	}
	return &Handler{
		store:               store,
		request:             request,
		agentNames:          agentNames,
		turns:               make(map[string]turnState),
		lastParentMessageID: request.Snapshot.Trigger.ID,
	}, nil
}

// Handle persists turn events and returns a message only after its transaction commits.
func (h *Handler) Handle(ctx context.Context, event Event) (model.Message, bool, error) {
	switch event.Kind {
	case EventSpeakerSelected, EventAgentTurnStarted:
		if err := h.ensureTurn(ctx, event); err != nil {
			return model.Message{}, false, err
		}
		return model.Message{}, false, nil
	case EventAgentMessageCompleted:
		message, err := h.commitMessage(ctx, event)
		if err != nil {
			return model.Message{}, false, err
		}
		return message, true, nil
	default:
		return model.Message{}, false, nil
	}
}

func (h *Handler) ensureTurn(ctx context.Context, event Event) error {
	if err := h.validateTurnIdentity(event); err != nil {
		return err
	}
	if existing, ok := h.turns[event.TurnID]; ok {
		if existing.run.AgentID != event.AgentID {
			return fmt.Errorf("%w: turn Agent identity changed", ErrInvalidEvent)
		}
		return nil
	}

	startedAt := eventTime(event.OccurredAt)
	h.nextTurnIndex++
	run := store.AgentRun{
		ID:                 model.NewID("run"),
		RoomID:             h.request.Snapshot.Room.ID,
		AgentID:            event.AgentID,
		TriggerMessageID:   h.request.Snapshot.Trigger.ID,
		CollaborationRunID: h.request.CollaborationRunID,
		TurnIndex:          h.nextTurnIndex,
		ParentMessageID:    h.lastParentMessageID,
		Status:             "running",
		StartedAt:          startedAt,
	}
	if err := h.store.CreateAgentRun(ctx, run); err != nil {
		h.nextTurnIndex--
		return err
	}
	h.turns[event.TurnID] = turnState{run: run}
	return nil
}

func (h *Handler) commitMessage(ctx context.Context, event Event) (model.Message, error) {
	if err := h.validateTurnIdentity(event); err != nil {
		return model.Message{}, err
	}
	turn, ok := h.turns[event.TurnID]
	if !ok {
		return model.Message{}, fmt.Errorf("%w: turn has no Agent run", ErrInvalidEvent)
	}
	if turn.run.AgentID != event.AgentID {
		return model.Message{}, fmt.Errorf("%w: turn Agent identity changed", ErrInvalidEvent)
	}
	if event.Message == nil {
		return model.Message{}, fmt.Errorf("%w: completed message payload is required", ErrInvalidEvent)
	}

	completedAt := eventTime(event.OccurredAt)
	message := model.Message{
		ID:               model.NewID("msg"),
		RoomID:           turn.run.RoomID,
		SenderID:         turn.run.AgentID,
		SenderName:       h.agentNames[turn.run.AgentID],
		SenderType:       model.SenderTypeAgent,
		Content:          event.Message.Content,
		CreatedAt:        completedAt,
		TurnIndex:        turn.run.TurnIndex,
		ParentMessageID:  turn.run.ParentMessageID,
		KnowledgeSources: mapKnowledgeSources(event.Message.KnowledgeSources),
		Artifacts:        mapArtifacts(event.Message.Artifacts),
	}
	saved, err := h.store.CommitAgentRunSuccess(ctx, store.CommitAgentRunSuccessInput{
		RunID:          turn.run.ID,
		Message:        message,
		CompletedAt:    completedAt,
		ModelProfileID: event.Message.Model.ProfileID,
		ModelSource:    event.Message.Model.Source,
		ModelName:      event.Message.Model.ModelName,
	})
	if err != nil {
		terminalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalWriteTimeout)
		defer cancel()
		finishErr := h.store.FinishAgentRun(terminalCtx, turn.run.ID, "failed", "Agent message commit failed", completedAt)
		if finishErr != nil && !errors.Is(finishErr, store.ErrAgentRunAlreadyFinished) {
			return model.Message{}, errors.Join(err, finishErr)
		}
		return model.Message{}, err
	}
	h.lastParentMessageID = saved.ID
	return saved, nil
}

func (h *Handler) validateTurnIdentity(event Event) error {
	if event.CollaborationRunID != h.request.CollaborationRunID {
		return fmt.Errorf("%w: unexpected collaboration run ID", ErrInvalidEvent)
	}
	if event.TurnID == "" || event.AgentID == "" {
		return fmt.Errorf("%w: turn and Agent IDs are required", ErrInvalidEvent)
	}
	if _, ok := h.agentNames[event.AgentID]; !ok {
		return fmt.Errorf("%w: Agent is not in the request snapshot", ErrInvalidEvent)
	}
	return nil
}

func mapKnowledgeSources(sources []KnowledgeSource) []model.MessageKnowledgeSource {
	if len(sources) == 0 {
		return nil
	}
	result := make([]model.MessageKnowledgeSource, 0, len(sources))
	for _, source := range sources {
		result = append(result, model.MessageKnowledgeSource{
			DocumentID: source.DocumentID, DocumentName: source.DocumentName, Scope: source.Scope,
		})
	}
	return result
}

func mapArtifacts(artifacts []Artifact) []model.MessageArtifact {
	if len(artifacts) == 0 {
		return nil
	}
	result := make([]model.MessageArtifact, 0, len(artifacts))
	for index, artifact := range artifacts {
		id := strings.TrimSpace(artifact.ID)
		if id == "" {
			id = fmt.Sprintf("artifact_%d", index+1)
		}
		fileName := strings.TrimSpace(artifact.FileName)
		if fileName == "" {
			fileName = id
		}
		result = append(result, model.MessageArtifact{
			ID: id, Type: strings.TrimSpace(artifact.Type), Title: strings.TrimSpace(artifact.Title),
			FileName: fileName, MIMEType: strings.TrimSpace(artifact.MIMEType), Content: string(artifact.Content),
		})
	}
	return result
}

