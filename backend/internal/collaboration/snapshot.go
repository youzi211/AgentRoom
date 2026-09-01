package collaboration

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"
	"agentroom/backend/internal/model"
)

type AgentBinding struct {
	Agent          model.Agent
	ToolNames      []string
	ModelSelection ModelSelection
}

type Input struct {
	CollaborationRunID       string
	TraceID                  string
	Room                     model.RoomMeta
	Agents                   []AgentBinding
	Trigger                  model.Message
	Transcript               []model.Message
	KnowledgeChunks          []model.KnowledgeChunk
	Policy                   model.CollaborationPolicy
	Limits                   ExecutionLimits
	InitialCandidateAgentIDs []string
	Checkpoint               *Checkpoint
}

func Build(input Input) (Request, error) {
	if strings.TrimSpace(input.CollaborationRunID) == "" {
		return Request{}, errors.New("collaboration run ID is required")
	}
	if strings.TrimSpace(input.Room.ID) == "" {
		return Request{}, errors.New("collaboration room ID is required")
	}
	if strings.TrimSpace(input.Trigger.ID) == "" {
		return Request{}, errors.New("collaboration trigger message ID is required")
	}
	policy := input.Policy.WithDefaults()
	if err := policy.Validate(); err != nil {
		return Request{}, err
	}
	limits, err := buildLimits(input.Limits)
	if err != nil {
		return Request{}, err
	}
	engine, err := mapEngine(policy.Engine)
	if err != nil {
		return Request{}, err
	}
	triggerMode, err := mapTriggerMode(policy.TriggerMode)
	if err != nil {
		return Request{}, err
	}

	agents, selections, agentIDs, err := buildAgents(input.Agents)
	if err != nil {
		return Request{}, err
	}
	candidates, err := buildCandidates(input.InitialCandidateAgentIDs, agentIDs)
	if err != nil {
		return Request{}, err
	}
	trigger, err := buildMessage(input.Trigger)
	if err != nil {
		return Request{}, err
	}
	if trigger.SenderType != SenderHuman {
		return Request{}, errors.New("collaboration trigger message must be human")
	}
	transcript, err := buildMessages(input.Transcript)
	if err != nil {
		return Request{}, err
	}
	knowledge, err := buildKnowledge(input.KnowledgeChunks)
	if err != nil {
		return Request{}, err
	}

	status := strings.TrimSpace(input.Room.Status)
	if status == "" {
		status = model.RoomStatusActive
	}
	request := Request{
		ProtocolVersion:    ProtocolVersion,
		CollaborationRunID: input.CollaborationRunID,
		TraceID:            input.TraceID,
		Engine:             engine,
		Snapshot: ConversationSnapshot{
			Room:   RoomSnapshot{ID: input.Room.ID, Name: input.Room.Name, Status: status},
			Agents: agents, Trigger: trigger, Transcript: transcript, KnowledgeChunks: knowledge,
			ModelSelections: selections,
			Policy: PolicySnapshot{
				Version: model.CollaborationPolicyVersion, Engine: engine, TriggerMode: triggerMode,
				MaxTurns: uint32(policy.MaxTurns), MaxTurnsPerAgent: uint32(policy.MaxTurnsPerAgent),
				AllowAgentHandoff: policy.AllowAgentHandoff, AllowSelfFollowup: policy.AllowSelfFollowup,
				Cooldown:          time.Duration(policy.CooldownMS) * time.Millisecond,
				StopOnEmptyOutput: true, StopOnRepeatedOutput: true,
			},
			Limits: limits, InitialCandidateAgentIDs: candidates,
		},
	}
	if input.Checkpoint != nil {
		if input.Checkpoint.Engine != engine {
			return Request{}, errors.New("collaboration checkpoint Engine does not match policy")
		}
		if strings.TrimSpace(input.Checkpoint.EngineVersion) == "" || strings.TrimSpace(input.Checkpoint.FormatVersion) == "" || strings.TrimSpace(input.Checkpoint.SHA256) == "" {
			return Request{}, errors.New("collaboration checkpoint metadata is required")
		}
		if input.Checkpoint.SizeBytes != uint64(len(input.Checkpoint.Payload)) {
			return Request{}, errors.New("collaboration checkpoint size does not match payload")
		}
		digest := fmt.Sprintf("%x", sha256.Sum256(input.Checkpoint.Payload))
		if !strings.EqualFold(input.Checkpoint.SHA256, digest) {
			return Request{}, errors.New("collaboration checkpoint SHA-256 does not match payload")
		}
		checkpoint := *input.Checkpoint
		checkpoint.Payload = append([]byte(nil), input.Checkpoint.Payload...)
		request.Checkpoint = &checkpoint
	}
	return request, nil
}

func buildAgents(bindings []AgentBinding) ([]AgentSnapshot, []ModelSelection, map[string]struct{}, error) {
	agents := make([]AgentSnapshot, 0, len(bindings))
	selections := make([]ModelSelection, 0, len(bindings))
	agentIDs := make(map[string]struct{}, len(bindings))
	selectionByID := make(map[string]ModelSelection, len(bindings))
	for _, binding := range bindings {
		agent := binding.Agent
		if strings.TrimSpace(agent.ID) == "" {
			return nil, nil, nil, errors.New("collaboration Agent ID is required")
		}
		if strings.TrimSpace(agent.Name) == "" {
			return nil, nil, nil, fmt.Errorf("collaboration Agent %q requires a name", agent.ID)
		}
		if !agent.Enabled {
			return nil, nil, nil, fmt.Errorf("collaboration Agent %q is disabled", agent.ID)
		}
		if _, duplicate := agentIDs[agent.ID]; duplicate {
			return nil, nil, nil, fmt.Errorf("duplicate collaboration Agent %q", agent.ID)
		}
		if !model.IsValidAgentRuntime(agent.Runtime) {
			return nil, nil, nil, fmt.Errorf("unsupported runtime for collaboration Agent %q", agent.ID)
		}
		selection := binding.ModelSelection
		if strings.TrimSpace(selection.ID) == "" || strings.TrimSpace(selection.ProfileID) == "" ||
			strings.TrimSpace(selection.Source) == "" || strings.TrimSpace(selection.Protocol) == "" ||
			strings.TrimSpace(selection.ModelName) == "" || strings.TrimSpace(selection.RuntimeScope) == "" {
			return nil, nil, nil, fmt.Errorf("collaboration Agent %q requires an approved model selection", agent.ID)
		}
		if existing, ok := selectionByID[selection.ID]; ok {
			if existing != selection {
				return nil, nil, nil, fmt.Errorf("conflicting collaboration model selection %q", selection.ID)
			}
		} else {
			selectionByID[selection.ID] = selection
			selections = append(selections, selection)
		}

		agentIDs[agent.ID] = struct{}{}
		agents = append(agents, AgentSnapshot{
			ID: agent.ID, Name: agent.Name, Mention: agent.Mention, Role: agent.Role,
			Description: agent.Description, SystemPrompt: agent.SystemPrompt,
			Runtime: model.NormalizeAgentRuntime(agent.Runtime), ModelSelectionID: selection.ID,
			ToolNames: append([]string(nil), binding.ToolNames...),
		})
	}
	if len(agents) == 0 {
		return nil, nil, nil, errors.New("collaboration requires at least one eligible Agent")
	}
	return agents, selections, agentIDs, nil
}

func buildCandidates(candidateIDs []string, agentIDs map[string]struct{}) ([]string, error) {
	candidates := make([]string, 0, len(candidateIDs))
	seen := make(map[string]struct{}, len(candidateIDs))
	for _, agentID := range candidateIDs {
		if _, eligible := agentIDs[agentID]; !eligible {
			return nil, fmt.Errorf("initial candidate %q is outside the Agent snapshot", agentID)
		}
		if _, duplicate := seen[agentID]; duplicate {
			continue
		}
		seen[agentID] = struct{}{}
		candidates = append(candidates, agentID)
	}
	return candidates, nil
}

func buildMessages(messages []model.Message) ([]MessageSnapshot, error) {
	mapped := make([]MessageSnapshot, 0, len(messages))
	for _, message := range messages {
		value, err := buildMessage(message)
		if err != nil {
			return nil, err
		}
		mapped = append(mapped, value)
	}
	return mapped, nil
}

func buildMessage(message model.Message) (MessageSnapshot, error) {
	if strings.TrimSpace(message.ID) == "" {
		return MessageSnapshot{}, errors.New("collaboration message ID is required")
	}
	if message.TurnIndex < 0 {
		return MessageSnapshot{}, fmt.Errorf("message %q has a negative turn index", message.ID)
	}
	senderType, err := mapSenderType(message.SenderType)
	if err != nil {
		return MessageSnapshot{}, err
	}
	return MessageSnapshot{
		ID: message.ID, SenderID: message.SenderID, SenderName: message.SenderName,
		SenderType: senderType, Content: message.Content, CreatedAt: message.CreatedAt,
		TurnIndex: uint32(message.TurnIndex), ParentMessageID: message.ParentMessageID,
	}, nil
}

func buildKnowledge(chunks []model.KnowledgeChunk) ([]KnowledgeChunk, error) {
	mapped := make([]KnowledgeChunk, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk.ChunkIndex < 0 {
			return nil, fmt.Errorf("knowledge chunk %q has a negative index", chunk.ID)
		}
		mapped = append(mapped, KnowledgeChunk{
			ID: chunk.ID, DocumentID: chunk.DocumentID, DocumentName: chunk.DocumentName,
			Scope: chunk.Scope, ScopeID: chunk.ScopeID, ChunkIndex: uint32(chunk.ChunkIndex), Content: chunk.Content,
		})
	}
	return mapped, nil
}

func buildLimits(limits ExecutionLimits) (ExecutionLimits, error) {
	if limits.Timeout <= 0 {
		return ExecutionLimits{}, errors.New("collaboration timeout must be positive")
	}
	if limits.MaxOutputBytes == 0 || limits.MaxArtifactBytes == 0 || limits.MaxToolSteps == 0 || limits.MaxRequestBytes == 0 || limits.MaxEventBytes == 0 || limits.MaxCheckpointBytes == 0 {
		return ExecutionLimits{}, errors.New("collaboration byte limits must be positive")
	}
	return limits, nil
}

func mapEngine(engine string) (Engine, error) {
	switch engine {
	case model.CollaborationEngineNative:
		return EngineNative, nil
	case model.CollaborationEngineAutoGen:
		return EngineAutoGen, nil
	default:
		return "", fmt.Errorf("unsupported collaboration Engine %q", engine)
	}
}

func mapTriggerMode(mode string) (TriggerMode, error) {
	switch mode {
	case model.CollaborationTriggerMentionOnly:
		return TriggerMentionOnly, nil
	case model.CollaborationTriggerAutomatic:
		return TriggerAutomatic, nil
	default:
		return "", fmt.Errorf("unsupported collaboration trigger mode %q", mode)
	}
}

func mapSenderType(senderType string) (SenderType, error) {
	switch senderType {
	case model.SenderTypeHuman:
		return SenderHuman, nil
	case model.SenderTypeAgent:
		return SenderAgent, nil
	case model.SenderTypeSystem:
		return SenderSystem, nil
	default:
		return "", fmt.Errorf("unsupported collaboration sender type %q", senderType)
	}
}
