package collaborationproto_test

import (
	"strings"
	"testing"
	"time"

	collaborationruntimev1 "agentroom/backend/internal/collaboration/proto/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestCollaborationDeadlineMustBePositive(t *testing.T) {
	if err := collaborationruntimev1.ValidateDeadline(durationpb.New(30 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for _, timeout := range []*durationpb.Duration{nil, durationpb.New(0), durationpb.New(-time.Second)} {
		if err := collaborationruntimev1.ValidateDeadline(timeout); err == nil {
			t.Fatalf("expected invalid deadline for %#v", timeout)
		}
	}
}

func TestCollaborationResourceLimits(t *testing.T) {
	request := &collaborationruntimev1.ExecuteConversationRequest{TraceId: strings.Repeat("r", 64)}
	if err := collaborationruntimev1.ValidateRequestSize(request, 8); err == nil {
		t.Fatal("expected request size rejection")
	}
	event := &collaborationruntimev1.CollaborationEvent{
		Payload: &collaborationruntimev1.CollaborationEvent_OutputDelta{
			OutputDelta: &collaborationruntimev1.OutputDeltaEvent{Text: strings.Repeat("e", 64)},
		},
	}
	if err := collaborationruntimev1.ValidateEventSize(event, 8); err == nil {
		t.Fatal("expected event size rejection")
	}
	if err := collaborationruntimev1.ValidateArtifactSize(&collaborationruntimev1.Artifact{Content: make([]byte, 9)}, 8); err == nil {
		t.Fatal("expected artifact size rejection")
	}
	if err := collaborationruntimev1.ValidateCheckpointSize(&collaborationruntimev1.OpaqueCheckpoint{SizeBytes: 9, Payload: make([]byte, 9)}, 8); err == nil {
		t.Fatal("expected checkpoint size rejection")
	}
	if err := collaborationruntimev1.ValidateCheckpointSize(&collaborationruntimev1.OpaqueCheckpoint{SizeBytes: 8, Payload: make([]byte, 7)}, 8); err == nil {
		t.Fatal("expected checkpoint declared-size rejection")
	}
}

func TestCollaborationGRPCStatusMappingIncludesCancellationAndDeadline(t *testing.T) {
	tests := map[collaborationruntimev1.CollaborationErrorCode]codes.Code{
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_INVALID_REQUEST:     codes.InvalidArgument,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_UNSUPPORTED_VERSION: codes.Unimplemented,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_ENGINE_UNAVAILABLE:  codes.Unavailable,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_RESOURCE_EXHAUSTED:  codes.ResourceExhausted,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_DUPLICATE_RUN:       codes.AlreadyExists,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_ROOM_BUSY:           codes.FailedPrecondition,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_CANCELLED:           codes.Canceled,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_DEADLINE_EXCEEDED:   codes.DeadlineExceeded,
		collaborationruntimev1.CollaborationErrorCode_COLLABORATION_ERROR_CODE_INTERNAL:            codes.Internal,
	}
	for errorCode, want := range tests {
		if got := collaborationruntimev1.GRPCCodeForErrorCode(errorCode); got != want {
			t.Fatalf("status for %s: got %s want %s", errorCode, got, want)
		}
	}
}
