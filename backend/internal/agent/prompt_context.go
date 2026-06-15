package agent

import "agentroom/backend/internal/model"

type PromptContext struct {
	RoomName                  string
	DialogueMode              string
	OnlineHumanParticipants   []model.Participant
	RoomAgents                []model.Agent
	TriggerSender             string
	TriggerSenderType         string
	TriggerContent            string
	LatestVisibleSpeaker      string
	LatestVisibleSpeakerType  string
	Transcript                []model.Message
	KnowledgeChunks           []model.KnowledgeChunk
	CurrentSpeaker            model.Agent
	RootHumanTriggerSender    string
	RootHumanTriggerType      string
	RootHumanTriggerContent   string
	AutonomousTurnIndex       int
	MaxAutonomousTurns        int
	AllowSelfFollowup         bool
	AllowAgentToAgentMentions bool
	MaxTurnsPerAgent          int
	ResponseStrategy          string
	EligiblePeers             []model.Agent
}

func NewMentionPromptContext(currentRoom RuntimeRoom, recentMessages []model.Message, trigger model.Message, knowledgeChunks []model.KnowledgeChunk) PromptContext {
	roomInfo := currentRoom.Info()
	transcript := visibleTranscript(recentMessages)
	latestSpeaker, latestSpeakerType := latestVisibleSpeaker(transcript, trigger)

	return PromptContext{
		RoomName:                 roomInfo.Name,
		DialogueMode:             roomInfo.DialoguePolicy.WithDefaults().Mode,
		OnlineHumanParticipants:  cloneParticipants(currentRoom.Participants()),
		RoomAgents:               clonePublicAgents(currentRoom.Agents()),
		TriggerSender:            trigger.SenderName,
		TriggerSenderType:        trigger.SenderType,
		TriggerContent:           trigger.Content,
		LatestVisibleSpeaker:     latestSpeaker,
		LatestVisibleSpeakerType: latestSpeakerType,
		Transcript:               transcript,
		KnowledgeChunks:          cloneKnowledgeChunks(knowledgeChunks),
	}
}

func NewGuidedPromptContext(currentRoom RuntimeRoom, recentMessages []model.Message, responder model.Agent, trigger model.Message, rootHumanTrigger model.Message, eligiblePeers []model.Agent, policy model.DialoguePolicy, turnIndex int, knowledgeChunks []model.KnowledgeChunk) PromptContext {
	policy = policy.WithDefaults()
	if rootHumanTrigger.ID == "" {
		rootHumanTrigger = trigger
	}

	base := NewMentionPromptContext(currentRoom, recentMessages, trigger, knowledgeChunks)
	base.DialogueMode = policy.Mode
	base.CurrentSpeaker = responder.Public()
	base.RootHumanTriggerSender = rootHumanTrigger.SenderName
	base.RootHumanTriggerType = rootHumanTrigger.SenderType
	base.RootHumanTriggerContent = rootHumanTrigger.Content
	base.AutonomousTurnIndex = turnIndex
	base.MaxAutonomousTurns = policy.MaxAutonomousTurns
	base.AllowSelfFollowup = policy.AllowSelfFollowup
	base.AllowAgentToAgentMentions = policy.AllowAgentToAgentMentions
	base.MaxTurnsPerAgent = policy.MaxTurnsPerAgent
	base.ResponseStrategy = policy.ResponseStrategy
	base.EligiblePeers = clonePublicAgents(eligiblePeers)
	return base
}

func visibleTranscript(messages []model.Message) []model.Message {
	transcript := make([]model.Message, 0, len(messages))
	for _, message := range messages {
		if message.SenderType == model.SenderTypeSystem {
			continue
		}
		transcript = append(transcript, message)
	}
	return transcript
}

func latestVisibleSpeaker(transcript []model.Message, trigger model.Message) (string, string) {
	if len(transcript) == 0 {
		return trigger.SenderName, trigger.SenderType
	}
	last := transcript[len(transcript)-1]
	return last.SenderName, last.SenderType
}

func cloneParticipants(participants []model.Participant) []model.Participant {
	result := make([]model.Participant, len(participants))
	copy(result, participants)
	return result
}

func clonePublicAgents(agents []model.Agent) []model.Agent {
	result := make([]model.Agent, 0, len(agents))
	for _, agent := range agents {
		result = append(result, agent.Public())
	}
	return result
}

func cloneKnowledgeChunks(chunks []model.KnowledgeChunk) []model.KnowledgeChunk {
	result := make([]model.KnowledgeChunk, len(chunks))
	copy(result, chunks)
	return result
}
