package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"agentroom/backend/internal/llm"
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
		response, err := r.generateGuidedResponse(ctx, currentRoom, responder, parentMessage, eligiblePeers, policy, turnCount+1)
		if err != nil {
			status = dialogueFailureStatus(err)
			content := fmt.Sprintf("Agent %s failed to respond: %s", responder.Name, shortReason(err))
			r.persistAndBroadcast(ctx, currentRoom, currentRoom.NewSystemMessage(content))
			break
		}

		normalized := normalizeGeneratedContent(response)
		if normalized == "" {
			status = model.DialogueRunStatusStoppedEmpty
			break
		}
		if isDuplicateDialogueTurn(normalized, recentNormalized) {
			status = model.DialogueRunStatusStoppedDuplicate
			break
		}

		agentMessage := currentRoom.NewAgentMessage(responder, response)
		agentMessage.DialogueRunID = runID
		agentMessage.TurnIndex = turnCount + 1
		agentMessage.ParentMessageID = parentMessage.ID
		r.persistAndBroadcast(ctx, currentRoom, agentMessage)

		turnCount++
		turnsByAgent[responder.ID]++
		lastSpeakerID = responder.ID
		parentMessage = agentMessage
		recentNormalized = append(recentNormalized, normalized)

		if !policy.AllowAgentToAgentMentions {
			continue
		}

		nextCandidates := resolveGuidedCandidates(DetectMentions(response, currentRoom.Agents()), fullAgentByID)
		pending = appendUniqueDialogueCandidates(pending, nextCandidates)
	}

	if status == model.DialogueRunStatusSucceeded && turnCount == policy.MaxAutonomousTurns && len(pending) > 0 {
		status = model.DialogueRunStatusStoppedLimit
	}

	if persistedRun {
		if err := r.store.FinishDialogueRun(ctx, runID, status, turnCount, time.Now().UTC()); err != nil {
			r.logger.Error("finish dialogue run", "run_id", runID, "status", status, "turn_count", turnCount, "error", err)
		}
	}
}

func (r *Runner) generateGuidedResponse(ctx context.Context, currentRoom RuntimeRoom, responder model.Agent, trigger model.Message, eligiblePeers []model.Agent, policy model.DialoguePolicy, turnIndex int) (string, error) {
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

	knowledgeChunks := r.searchKnowledge(ctx, currentRoom, responder, trigger)
	prompt := buildGuidedPrompt(currentRoom.RecentMessages(r.contextLimit), responder, trigger, eligiblePeers, policy, turnIndex, knowledgeChunks)
	requestCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	response, err := r.client.Complete(requestCtx, []llm.ChatMessage{
		{Role: llm.RoleSystem, Content: responder.SystemPrompt},
		{Role: llm.RoleUser, Content: prompt},
	})
	now := time.Now().UTC()
	if err != nil {
		status := "failed"
		if errors.Is(requestCtx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			status = "timeout"
		}
		if finishErr := r.store.FinishAgentRun(ctx, runID, status, shortReason(err), now); finishErr != nil {
			r.logger.Error("finish agent run", "run_id", runID, "status", status, "error", finishErr)
		}
		return "", err
	}

	cleaned, err := StripThinkBlocks(response)
	if err != nil {
		if finishErr := r.store.FinishAgentRun(ctx, runID, "failed", shortReason(err), now); finishErr != nil {
			r.logger.Error("finish agent run", "run_id", runID, "status", "failed", "error", finishErr)
		}
		return "", err
	}
	if finishErr := r.store.FinishAgentRun(ctx, runID, "succeeded", "", now); finishErr != nil {
		r.logger.Error("finish agent run", "run_id", runID, "status", "succeeded", "error", finishErr)
	}
	return cleaned, nil
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

func buildGuidedPrompt(messages []model.Message, responder model.Agent, trigger model.Message, eligiblePeers []model.Agent, policy model.DialoguePolicy, turnIndex int, knowledgeChunks []model.KnowledgeChunk) string {
	var builder strings.Builder
	builder.WriteString("You are participating in a bounded multi-agent room dialogue.\n")
	builder.WriteString("Current speaker: ")
	builder.WriteString(responder.Name)
	builder.WriteString("\n")
	builder.WriteString("Autonomous turn: ")
	builder.WriteString(fmt.Sprintf("%d/%d", turnIndex, policy.MaxAutonomousTurns))
	builder.WriteString("\n")
	builder.WriteString("Response strategy: ")
	builder.WriteString(policy.ResponseStrategy)
	builder.WriteString("\n")
	builder.WriteString("Allow self follow-up: ")
	builder.WriteString(fmt.Sprintf("%t", policy.AllowSelfFollowup))
	builder.WriteString("\n")
	builder.WriteString("Allow agent-to-agent mentions: ")
	builder.WriteString(fmt.Sprintf("%t", policy.AllowAgentToAgentMentions))
	builder.WriteString("\n")
	builder.WriteString("Max turns per agent: ")
	builder.WriteString(fmt.Sprintf("%d", policy.MaxTurnsPerAgent))
	builder.WriteString("\n")
	if len(eligiblePeers) > 0 {
		builder.WriteString("Eligible peers for follow-up: ")
		for index, peer := range eligiblePeers {
			if index > 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(peer.Mention)
		}
		builder.WriteString("\n")
	}
	builder.WriteString("Trigger message sender: ")
	builder.WriteString(trigger.SenderName)
	builder.WriteString(" (")
	builder.WriteString(trigger.SenderType)
	builder.WriteString(")\n")
	builder.WriteString("Trigger message content:\n")
	builder.WriteString(trigger.Content)
	builder.WriteString("\n\nRecent room messages:\n")
	for _, message := range messages {
		builder.WriteString("- ")
		builder.WriteString(message.SenderName)
		builder.WriteString(" (")
		builder.WriteString(message.SenderType)
		builder.WriteString("): ")
		builder.WriteString(message.Content)
		builder.WriteString("\n")
	}
	if len(knowledgeChunks) > 0 {
		builder.WriteString("\nKnowledge snippets:\n")
		for _, chunk := range knowledgeChunks {
			builder.WriteString("- [")
			builder.WriteString(chunk.Scope)
			builder.WriteString("] ")
			builder.WriteString(chunk.Content)
			builder.WriteString("\n")
		}
	}
	builder.WriteString("\nReply with exactly one visible room message. Keep it concise, stay in character, and do not reveal hidden reasoning.")
	return builder.String()
}
