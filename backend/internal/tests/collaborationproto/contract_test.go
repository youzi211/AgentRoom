package collaborationproto_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	collaborationruntimev1 "agentroom/backend/internal/collaborationproto/v1"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type contractFixture struct {
	Request json.RawMessage   `json:"request"`
	Events  []json.RawMessage `json:"events"`
}

func loadContractFixture(t *testing.T) contractFixture {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "proto", "collaboration_runtime", "v1", "testdata", "contract_golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture contractFixture
	if err := json.Unmarshal(payload, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestCollaborationContractGoldenParsesAndValidates(t *testing.T) {
	fixture := loadContractFixture(t)
	var request collaborationruntimev1.ExecuteConversationRequest
	if err := protojson.Unmarshal(fixture.Request, &request); err != nil {
		t.Fatal(err)
	}
	if err := collaborationruntimev1.ValidateProtocolVersion(request.GetProtocolVersion()); err != nil {
		t.Fatal(err)
	}
	if request.GetCollaborationRunId() != "collaboration_contract" || request.GetEngine() != collaborationruntimev1.CollaborationEngine_COLLABORATION_ENGINE_NATIVE {
		t.Fatalf("unexpected request identity: %#v", &request)
	}

	validator := collaborationruntimev1.NewEventSequenceValidator(request.GetCollaborationRunId())
	for index, raw := range fixture.Events {
		var event collaborationruntimev1.CollaborationEvent
		if err := protojson.Unmarshal(raw, &event); err != nil {
			t.Fatalf("parse event %d: %v", index, err)
		}
		if err := validator.Validate(&event); err != nil {
			t.Fatalf("validate event %d: %v", index, err)
		}
	}
	if !validator.TerminalSeen() {
		t.Fatal("golden stream must end in a terminal event")
	}
}

func TestExecuteConversationRequestIgnoresUnknownBinaryField(t *testing.T) {
	request := &collaborationruntimev1.ExecuteConversationRequest{
		ProtocolVersion:    collaborationruntimev1.ProtocolVersion,
		CollaborationRunId: "collaboration_unknown",
	}
	payload, err := proto.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	// Future field 99, varint value 1.
	payload = append(payload, 0x98, 0x06, 0x01)

	var decoded collaborationruntimev1.ExecuteConversationRequest
	if err := proto.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.GetCollaborationRunId() != request.GetCollaborationRunId() {
		t.Fatalf("known fields changed after unknown-field decode: %#v", &decoded)
	}
}

func TestCollaborationProtocolRejectsUnsupportedVersion(t *testing.T) {
	if err := collaborationruntimev1.ValidateProtocolVersion("v2"); err == nil {
		t.Fatal("expected unsupported protocol version error")
	}
}

func TestCollaborationEventValidatorRejectsOutOfOrderAndDuplicateTerminal(t *testing.T) {
	validator := collaborationruntimev1.NewEventSequenceValidator("collaboration_sequence")
	if err := validator.Validate(acceptedEvent("collaboration_sequence", 1)); err != nil {
		t.Fatal(err)
	}
	if err := validator.Validate(startedEvent("collaboration_sequence", 3)); err != nil {
		t.Fatal(err)
	}
	if err := validator.Validate(startedEvent("collaboration_sequence", 2)); err == nil {
		t.Fatal("expected out-of-order sequence rejection")
	}

	terminalValidator := collaborationruntimev1.NewEventSequenceValidator("collaboration_terminal")
	if err := terminalValidator.Validate(acceptedEvent("collaboration_terminal", 1)); err != nil {
		t.Fatal(err)
	}
	completed := &collaborationruntimev1.CollaborationEvent{
		ProtocolVersion: collaborationruntimev1.ProtocolVersion, CollaborationRunId: "collaboration_terminal", Sequence: 2,
		Payload: &collaborationruntimev1.CollaborationEvent_Completed{Completed: &collaborationruntimev1.CompletedEvent{}},
	}
	if err := terminalValidator.Validate(completed); err != nil {
		t.Fatal(err)
	}
	failed := &collaborationruntimev1.CollaborationEvent{
		ProtocolVersion: collaborationruntimev1.ProtocolVersion, CollaborationRunId: "collaboration_terminal", Sequence: 3,
		Payload: &collaborationruntimev1.CollaborationEvent_Failed{Failed: &collaborationruntimev1.FailedEvent{}},
	}
	if err := terminalValidator.Validate(failed); err == nil {
		t.Fatal("expected duplicate terminal rejection")
	}
}

func TestCollaborationModelReferenceHasNoCredentialFields(t *testing.T) {
	fixture := loadContractFixture(t)
	lower := strings.ToLower(string(fixture.Request))
	for _, forbidden := range []string{"api_key", "apikey", "authorization", "provider_response", "providerresponse"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("contract fixture contains sensitive field %q", forbidden)
		}
	}
	fields := (&collaborationruntimev1.ModelSelection{}).ProtoReflect().Descriptor().Fields()
	for _, forbidden := range []protoreflect.Name{"api_key", "authorization", "provider_response"} {
		if fields.ByName(forbidden) != nil {
			t.Fatalf("model selection exposes sensitive field %q", forbidden)
		}
	}
}

func acceptedEvent(runID string, sequence uint64) *collaborationruntimev1.CollaborationEvent {
	return &collaborationruntimev1.CollaborationEvent{
		ProtocolVersion: collaborationruntimev1.ProtocolVersion, CollaborationRunId: runID, Sequence: sequence,
		Payload: &collaborationruntimev1.CollaborationEvent_Accepted{Accepted: &collaborationruntimev1.AcceptedEvent{}},
	}
}

func startedEvent(runID string, sequence uint64) *collaborationruntimev1.CollaborationEvent {
	return &collaborationruntimev1.CollaborationEvent{
		ProtocolVersion: collaborationruntimev1.ProtocolVersion, CollaborationRunId: runID, Sequence: sequence,
		Payload: &collaborationruntimev1.CollaborationEvent_CollaborationStarted{CollaborationStarted: &collaborationruntimev1.CollaborationStartedEvent{}},
	}
}
