package agent_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agentroom/backend/internal/agent"
	agentruntimev1 "agentroom/backend/internal/agentproto/v1"
	"agentroom/backend/internal/model"
	"google.golang.org/protobuf/encoding/protojson"
)

type promptGoldenCase struct {
	Name     string          `json:"name"`
	Request  json.RawMessage `json:"request"`
	Expected []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"expected"`
}

func TestGoPromptComposerMatchesCrossLanguageGoldenSamples(t *testing.T) {
	payload, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "proto", "agent_runtime", "v1", "testdata", "prompt_golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []promptGoldenCase
	if err := json.Unmarshal(payload, &cases); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			var request agentruntimev1.ExecuteAgentRequest
			if err := protojson.Unmarshal(testCase.Request, &request); err != nil {
				t.Fatal(err)
			}
			messages, err := agent.ComposePromptForRuntime(
				model.Agent{SystemPrompt: request.GetAgent().GetSystemPrompt()},
				promptContextFromProto(request.GetPromptContext()),
			)
			if err != nil {
				t.Fatal(err)
			}
			if len(messages) != len(testCase.Expected) {
				t.Fatalf("expected %d messages, got %d", len(testCase.Expected), len(messages))
			}
			for index, expected := range testCase.Expected {
				if string(messages[index].Role) != expected.Role || messages[index].Content != expected.Content {
					t.Fatalf("message %d did not match golden sample\nrole: %s\ncontent:\n%s", index, messages[index].Role, messages[index].Content)
				}
			}
		})
	}
}

func promptContextFromProto(input *agentruntimev1.PromptContextSnapshot) agent.PromptContext {
	result := agent.PromptContext{
		RoomName: input.GetRoomName(), DialogueMode: input.GetDialogueMode(),
		TriggerSender: input.GetTriggerSender(), TriggerSenderType: senderTypeFromProto(input.GetTriggerSenderType()),
		TriggerContent: input.GetTriggerContent(), LatestVisibleSpeaker: input.GetLatestVisibleSpeaker(),
		LatestVisibleSpeakerType: senderTypeFromProto(input.GetLatestVisibleSpeakerType()),
		CurrentSpeaker:           agentFromProto(input.GetCurrentSpeaker()),
		RootHumanTriggerSender:   input.GetRootHumanTriggerSender(),
		RootHumanTriggerType:     senderTypeFromProto(input.GetRootHumanTriggerType()),
		RootHumanTriggerContent:  input.GetRootHumanTriggerContent(),
		AutonomousTurnIndex:      int(input.GetAutonomousTurnIndex()), MaxAutonomousTurns: int(input.GetMaxAutonomousTurns()),
		AllowSelfFollowup: input.GetAllowSelfFollowup(), AllowAgentToAgentMentions: input.GetAllowAgentToAgentMentions(),
		MaxTurnsPerAgent: int(input.GetMaxTurnsPerAgent()), ResponseStrategy: input.GetResponseStrategy(),
	}
	for _, participant := range input.GetOnlineHumanParticipants() {
		result.OnlineHumanParticipants = append(result.OnlineHumanParticipants, model.Participant{ID: participant.GetId(), Name: participant.GetName()})
	}
	for _, candidate := range input.GetRoomAgents() {
		result.RoomAgents = append(result.RoomAgents, agentFromProto(candidate))
	}
	for _, message := range input.GetTranscript() {
		result.Transcript = append(result.Transcript, model.Message{
			ID: message.GetId(), SenderID: message.GetSenderId(), SenderName: message.GetSenderName(),
			SenderType: senderTypeFromProto(message.GetSenderType()), Content: message.GetContent(),
		})
	}
	for _, chunk := range input.GetKnowledgeChunks() {
		result.KnowledgeChunks = append(result.KnowledgeChunks, model.KnowledgeChunk{
			ID: chunk.GetId(), DocumentID: chunk.GetDocumentId(), DocumentName: chunk.GetDocumentName(),
			Scope: chunk.GetScope(), ScopeID: chunk.GetScopeId(), ChunkIndex: int(chunk.GetChunkIndex()), Content: chunk.GetContent(),
		})
	}
	for _, peer := range input.GetEligiblePeers() {
		result.EligiblePeers = append(result.EligiblePeers, agentFromProto(peer))
	}
	return result
}

func agentFromProto(input *agentruntimev1.AgentSnapshot) model.Agent {
	if input == nil {
		return model.Agent{}
	}
	return model.Agent{
		ID: input.GetId(), Name: input.GetName(), Mention: input.GetMention(), Role: input.GetRole(),
		Description: input.GetDescription(), SystemPrompt: input.GetSystemPrompt(), Runtime: input.GetRuntime(),
		ModelProfileID: input.GetModelProfileId(),
	}
}

func senderTypeFromProto(input agentruntimev1.SenderType) string {
	switch input {
	case agentruntimev1.SenderType_SENDER_TYPE_HUMAN:
		return model.SenderTypeHuman
	case agentruntimev1.SenderType_SENDER_TYPE_AGENT:
		return model.SenderTypeAgent
	case agentruntimev1.SenderType_SENDER_TYPE_SYSTEM:
		return model.SenderTypeSystem
	default:
		return ""
	}
}
