package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

func (r *Runner) handleGuidedDialogue(ctx context.Context, currentRoom RuntimeRoom, trigger model.Message, policy model.DialoguePolicy) {
	mentioned := MentionedAgents(trigger, currentRoom.Agents())
	if len(mentioned) == 0 {
		return
	}

	roomInfo := currentRoom.Info()
	fullAgents := currentRoom.AgentsWithPrompts()
	fullAgentByID := make(map[string]model.Agent, len(fullAgents))
	for _, candidate := range fullAgents {
		fullAgentByID[candidate.ID] = candidate
	}

	pending := resolveGuidedCandidates(mentioned, fullAgentByID)
	if len(pending) == 0 {
		return
	}

	runID := model.NewID("dialogue")
	dialogueRun := store.DialogueRun{
		ID:               runID,
		RoomID:           roomInfo.ID,
		TriggerMessageID: trigger.ID,
		Mode:             policy.Mode,
		Status:           model.DialogueRunStatusRunning,
		StartedAt:        time.Now().UTC(),
	}
	persistedRun := true
	if err := r.store.CreateDialogueRun(ctx, dialogueRun); err != nil {
		persistedRun = false
		r.logger.Error("create dialogue run", "room_id", roomInfo.ID, "trigger_message_id", trigger.ID, "error", err)
	}
	r.broadcastDialogueRunActivity(currentRoom, dialogueRun, "started", model.DialogueRunStatusRunning, 0, nil)

	status := model.DialogueRunStatusSucceeded
	turnCount := 0
	lastSpeakerID := ""
	parentMessage := trigger
	turnsByAgent := make(map[string]int, len(fullAgents))
	recentNormalized := make([]string, 0, policy.MaxAutonomousTurns)

	for turnCount < policy.MaxAutonomousTurns {
		responder, remaining := selectNextDialogueSpeaker(pending, turnsByAgent, lastSpeakerID, policy)
		pending = remaining
		if responder.ID == "" {
			break
		}

		if turnCount > 0 {
			if err := waitForCooldown(ctx, time.Duration(policy.CooldownMS)*time.Millisecond); err != nil {
				status = dialogueFailureStatus(err)
				break
			}
		}

		eligiblePeers := eligibleDialoguePeers(fullAgents, turnsByAgent, responder.ID, lastSpeakerID, policy)
		response, knowledgeChunks, err := r.generateGuidedResponse(ctx, currentRoom, responder, parentMessage, trigger, eligiblePeers, policy, turnCount+1)
		if err != nil {
			status = dialogueFailureStatus(err)
			content := fmt.Sprintf("Agent %s failed to respond: %s", responder.Name, shortReason(err))
			r.persistAndBroadcast(ctx, currentRoom, currentRoom.NewSystemMessage(content))
			break
		}

		normalized := normalizeGeneratedContent(response.Content)
		if normalized == "" {
			status = model.DialogueRunStatusStoppedEmpty
			break
		}
		if isDuplicateDialogueTurn(normalized, recentNormalized) {
			status = model.DialogueRunStatusStoppedDuplicate
			break
		}

		agentMessage := currentRoom.NewAgentMessage(responder, response.Content)
		agentMessage.DialogueRunID = runID
		agentMessage.TurnIndex = turnCount + 1
		agentMessage.ParentMessageID = parentMessage.ID
		agentMessage.KnowledgeSources = knowledgeSourcesFromChunks(knowledgeChunks)
		agentMessage.Artifacts = messageArtifactsFromRuntime(response.Artifacts)
		r.persistAndBroadcast(ctx, currentRoom, agentMessage)

		turnCount++
		turnsByAgent[responder.ID]++
		lastSpeakerID = responder.ID
		parentMessage = agentMessage
		recentNormalized = append(recentNormalized, normalized)

		if !policy.AllowAgentToAgentMentions {
			continue
		}

		nextCandidates := resolveGuidedCandidates(DetectMentions(response.Content, currentRoom.Agents()), fullAgentByID)
		pending = appendUniqueDialogueCandidates(pending, nextCandidates)
	}

	if status == model.DialogueRunStatusSucceeded && turnCount == policy.MaxAutonomousTurns && len(pending) > 0 {
		status = model.DialogueRunStatusStoppedLimit
	}

	completedAt := time.Now().UTC()
	if persistedRun {
		if err := r.store.FinishDialogueRun(ctx, runID, status, turnCount, completedAt); err != nil {
			r.logger.Error("finish dialogue run", "run_id", runID, "status", status, "turn_count", turnCount, "error", err)
		}
	}
	r.broadcastDialogueRunActivity(currentRoom, dialogueRun, "finished", status, turnCount, &completedAt)
}

func (r *Runner) generateGuidedResponse(ctx context.Context, currentRoom RuntimeRoom, responder model.Agent, trigger model.Message, rootHumanTrigger model.Message, eligiblePeers []model.Agent, policy model.DialoguePolicy, turnIndex int) (AgentRuntimeResponse, []model.KnowledgeChunk, error) {
	runID := model.NewID("run")
	roomInfo := currentRoom.Info()
	agentRun := store.AgentRun{
		ID:               runID,
		RoomID:           roomInfo.ID,
		AgentID:          responder.ID,
		TriggerMessageID: trigger.ID,
		Status:           "running",
		StartedAt:        time.Now().UTC(),
	}
	if err := r.store.CreateAgentRun(ctx, agentRun); err != nil {
		r.logger.Error("create agent run", "room_id", roomInfo.ID, "agent_id", responder.ID, "error", err)
	}
	r.broadcastAgentRunActivity(currentRoom, agentRun, responder, "started", "", "", nil)

	knowledgeChunks := r.searchKnowledge(ctx, currentRoom, responder, trigger)
	recentMessages := currentRoom.RecentMessages(r.contextLimit)
	promptContext := NewGuidedPromptContext(currentRoom, recentMessages, responder, trigger, rootHumanTrigger, eligiblePeers, policy, turnIndex, knowledgeChunks)
	runtime, err := r.runtimes.Resolve(responder)
	if err != nil {
		now := time.Now().UTC()
		errText := shortReason(err)
		if finishErr := r.store.FinishAgentRun(ctx, runID, "failed", errText, now); finishErr != nil {
			r.logger.Error("finish agent run", "run_id", runID, "status", "failed", "error", finishErr)
		}
		r.broadcastAgentRunActivity(currentRoom, agentRun, responder, "finished", "failed", errText, &now)
		return AgentRuntimeResponse{}, knowledgeChunks, err
	}

	response, err := runtime.Respond(ctx, AgentRuntimeRequest{
		RunID:           runID,
		Room:            currentRoom,
		Agent:           responder,
		Trigger:         trigger,
		RecentMessages:  recentMessages,
		KnowledgeChunks: knowledgeChunks,
		PromptContext:   promptContext,
	})
	if err != nil {
		now := time.Now().UTC()
		status := "failed"
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			status = "timeout"
		}
		errText := shortReason(err)
		if finishErr := r.store.FinishAgentRun(ctx, runID, status, errText, now); finishErr != nil {
			r.logger.Error("finish agent run", "run_id", runID, "status", status, "error", finishErr)
		}
		r.broadcastAgentRunActivity(currentRoom, agentRun, responder, "finished", status, errText, &now)
		return AgentRuntimeResponse{}, knowledgeChunks, err
	}

	now := time.Now().UTC()
	if finishErr := r.store.FinishAgentRun(ctx, runID, "succeeded", "", now); finishErr != nil {
		r.logger.Error("finish agent run", "run_id", runID, "status", "succeeded", "error", finishErr)
	}
	r.broadcastAgentRunActivity(currentRoom, agentRun, responder, "finished", "succeeded", "", &now)
	return response, knowledgeChunks, nil
}

func resolveGuidedCandidates(candidates []model.Agent, fullAgentByID map[string]model.Agent) []model.Agent {
	result := make([]model.Agent, 0, len(candidates))
	for _, candidate := range candidates {
		full, ok := fullAgentByID[candidate.ID]
		if !ok {
			continue
		}
		result = append(result, full)
	}
	return result
}

func selectNextDialogueSpeaker(pending []model.Agent, turnsByAgent map[string]int, lastSpeakerID string, policy model.DialoguePolicy) (model.Agent, []model.Agent) {
	for index, candidate := range pending {
		if turnsByAgent[candidate.ID] >= policy.MaxTurnsPerAgent {
			continue
		}
		if !policy.AllowSelfFollowup && candidate.ID == lastSpeakerID {
			continue
		}

		next := make([]model.Agent, 0, len(pending)-1)
		next = append(next, pending[:index]...)
		next = append(next, pending[index+1:]...)
		return candidate, next
	}
	return model.Agent{}, pending
}

func appendUniqueDialogueCandidates(pending []model.Agent, additions []model.Agent) []model.Agent {
	if len(additions) == 0 {
		return pending
	}

	seen := make(map[string]struct{}, len(pending))
	for _, candidate := range pending {
		seen[candidate.ID] = struct{}{}
	}

	for _, candidate := range additions {
		if _, ok := seen[candidate.ID]; ok {
			continue
		}
		seen[candidate.ID] = struct{}{}
		pending = append(pending, candidate)
	}
	return pending
}

func eligibleDialoguePeers(agents []model.Agent, turnsByAgent map[string]int, currentSpeakerID string, lastSpeakerID string, policy model.DialoguePolicy) []model.Agent {
	result := make([]model.Agent, 0, len(agents))
	for _, candidate := range agents {
		if candidate.ID == currentSpeakerID {
			continue
		}
		if turnsByAgent[candidate.ID] >= policy.MaxTurnsPerAgent {
			continue
		}
		if !policy.AllowSelfFollowup && candidate.ID == lastSpeakerID {
			continue
		}
		result = append(result, candidate.Public())
	}
	return result
}

func waitForCooldown(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func dialogueFailureStatus(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return model.DialogueRunStatusTimeout
	}
	return model.DialogueRunStatusFailed
}

func normalizeGeneratedContent(text string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(text)))
	filtered := make([]string, 0, len(fields))
	for _, field := range fields {
		if strings.HasPrefix(field, "@") {
			continue
		}
		filtered = append(filtered, field)
	}
	return strings.Join(filtered, " ")
}

func isDuplicateDialogueTurn(candidate string, existing []string) bool {
	for _, item := range existing {
		if item == candidate {
			return true
		}
	}
	return false
}
