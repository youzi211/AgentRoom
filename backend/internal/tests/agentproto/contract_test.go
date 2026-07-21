package agentproto_test

import (
	"os"
	"path/filepath"
	"testing"

	agentruntimev1 "agentroom/backend/internal/agentproto/v1"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestExecuteAgentRequestContractFixture(t *testing.T) {
	payload, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "proto", "agent_runtime", "v1", "testdata", "execute_agent_request.json"))
	if err != nil {
		t.Fatal(err)
	}

	var request agentruntimev1.ExecuteAgentRequest
	if err := protojson.Unmarshal(payload, &request); err != nil {
		t.Fatal(err)
	}
	if err := agentruntimev1.ValidateProtocolVersion(request.GetProtocolVersion()); err != nil {
		t.Fatal(err)
	}
	if request.GetRunId() != "run_contract" || request.GetExecutorKind() != agentruntimev1.ExecutorKind_EXECUTOR_KIND_LLM {
		t.Fatalf("unexpected request identity: %#v", &request)
	}
	if request.GetModel().GetApiKey() != "contract-secret-not-for-logs" {
		t.Fatalf("model connection did not survive protobuf JSON parsing")
	}
}

func TestExecuteAgentRequestIgnoresUnknownBinaryField(t *testing.T) {
	request := &agentruntimev1.ExecuteAgentRequest{ProtocolVersion: agentruntimev1.ProtocolVersion, RunId: "run_unknown"}
	payload, err := proto.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	// Future field 99, varint value 1.
	payload = append(payload, 0x98, 0x06, 0x01)

	var decoded agentruntimev1.ExecuteAgentRequest
	if err := proto.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.GetRunId() != request.GetRunId() {
		t.Fatalf("known fields changed after unknown-field decode: %#v", &decoded)
	}
}

func TestAgentEventContractCoversEveryPayload(t *testing.T) {
	payloads := []*agentruntimev1.AgentEvent{
		{Payload: &agentruntimev1.AgentEvent_Accepted{Accepted: &agentruntimev1.AcceptedEvent{}}},
		{Payload: &agentruntimev1.AgentEvent_ModelStarted{ModelStarted: &agentruntimev1.ModelStartedEvent{}}},
		{Payload: &agentruntimev1.AgentEvent_ModelCompleted{ModelCompleted: &agentruntimev1.ModelCompletedEvent{}}},
		{Payload: &agentruntimev1.AgentEvent_ToolStarted{ToolStarted: &agentruntimev1.ToolStartedEvent{}}},
		{Payload: &agentruntimev1.AgentEvent_ToolCompleted{ToolCompleted: &agentruntimev1.ToolCompletedEvent{}}},
		{Payload: &agentruntimev1.AgentEvent_ToolFailed{ToolFailed: &agentruntimev1.ToolFailedEvent{}}},
		{Payload: &agentruntimev1.AgentEvent_OutputDelta{OutputDelta: &agentruntimev1.OutputDeltaEvent{}}},
		{Payload: &agentruntimev1.AgentEvent_ArtifactReady{ArtifactReady: &agentruntimev1.ArtifactReadyEvent{}}},
		{Payload: &agentruntimev1.AgentEvent_Completed{Completed: &agentruntimev1.CompletedEvent{}}},
		{Payload: &agentruntimev1.AgentEvent_Failed{Failed: &agentruntimev1.FailedEvent{}}},
	}

	for index, event := range payloads {
		event.ProtocolVersion = agentruntimev1.ProtocolVersion
		event.RunId = "run_events"
		event.Sequence = uint64(index + 1)
		event.OccurredAt = timestamppb.Now()
		payload, err := proto.Marshal(event)
		if err != nil {
			t.Fatalf("marshal payload %d: %v", index, err)
		}
		var decoded agentruntimev1.AgentEvent
		if err := proto.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("unmarshal payload %d: %v", index, err)
		}
		if decoded.GetPayload() == nil || decoded.GetSequence() != uint64(index+1) {
			t.Fatalf("payload %d lost its oneof or sequence: %#v", index, &decoded)
		}
	}
}

func TestUnsupportedProtocolVersionIsRejected(t *testing.T) {
	if err := agentruntimev1.ValidateProtocolVersion("v2"); err == nil {
		t.Fatal("expected unsupported protocol version error")
	}
}
