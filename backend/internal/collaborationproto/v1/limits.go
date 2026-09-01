package collaborationruntimev1

import (
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	DefaultMaxRequestBytes    uint32 = 8 * 1024 * 1024
	DefaultMaxEventBytes      uint32 = 4 * 1024 * 1024
	DefaultMaxArtifactBytes   uint32 = 2 * 1024 * 1024
	DefaultMaxCheckpointBytes uint32 = 1 * 1024 * 1024
)

func ValidateDeadline(timeout *durationpb.Duration) error {
	if timeout == nil {
		return fmt.Errorf("collaboration deadline is required")
	}
	if err := timeout.CheckValid(); err != nil {
		return fmt.Errorf("invalid collaboration deadline: %w", err)
	}
	if timeout.AsDuration() <= 0*time.Second {
		return fmt.Errorf("collaboration deadline must be positive")
	}
	return nil
}

func ValidateRequestSize(request *ExecuteConversationRequest, maxBytes uint32) error {
	return validateSerializedSize("request", proto.Size(request), limitOrDefault(maxBytes, DefaultMaxRequestBytes))
}

func ValidateEventSize(event *CollaborationEvent, maxBytes uint32) error {
	return validateSerializedSize("event", proto.Size(event), limitOrDefault(maxBytes, DefaultMaxEventBytes))
}

func ValidateArtifactSize(artifact *Artifact, maxBytes uint32) error {
	if artifact == nil {
		return fmt.Errorf("artifact is required")
	}
	return validateSerializedSize("artifact", len(artifact.GetContent()), limitOrDefault(maxBytes, DefaultMaxArtifactBytes))
}

func ValidateCheckpointSize(checkpoint *OpaqueCheckpoint, maxBytes uint32) error {
	if checkpoint == nil {
		return fmt.Errorf("checkpoint is required")
	}
	if checkpoint.GetSizeBytes() != uint64(len(checkpoint.GetPayload())) {
		return fmt.Errorf("checkpoint declared size does not match payload")
	}
	return validateSerializedSize("checkpoint", len(checkpoint.GetPayload()), limitOrDefault(maxBytes, DefaultMaxCheckpointBytes))
}

func GRPCCodeForErrorCode(code CollaborationErrorCode) codes.Code {
	switch code {
	case CollaborationErrorCode_COLLABORATION_ERROR_CODE_INVALID_REQUEST:
		return codes.InvalidArgument
	case CollaborationErrorCode_COLLABORATION_ERROR_CODE_UNSUPPORTED_VERSION:
		return codes.Unimplemented
	case CollaborationErrorCode_COLLABORATION_ERROR_CODE_ENGINE_UNAVAILABLE:
		return codes.Unavailable
	case CollaborationErrorCode_COLLABORATION_ERROR_CODE_RESOURCE_EXHAUSTED:
		return codes.ResourceExhausted
	case CollaborationErrorCode_COLLABORATION_ERROR_CODE_DUPLICATE_RUN:
		return codes.AlreadyExists
	case CollaborationErrorCode_COLLABORATION_ERROR_CODE_ROOM_BUSY:
		return codes.FailedPrecondition
	case CollaborationErrorCode_COLLABORATION_ERROR_CODE_CANCELLED:
		return codes.Canceled
	case CollaborationErrorCode_COLLABORATION_ERROR_CODE_DEADLINE_EXCEEDED,
		CollaborationErrorCode_COLLABORATION_ERROR_CODE_MODEL_TIMEOUT:
		return codes.DeadlineExceeded
	default:
		return codes.Internal
	}
}

func validateSerializedSize(resource string, size int, maxBytes uint32) error {
	if uint64(size) > uint64(maxBytes) {
		return fmt.Errorf("collaboration %s exceeds %d bytes", resource, maxBytes)
	}
	return nil
}

func limitOrDefault(value uint32, fallback uint32) uint32 {
	if value == 0 {
		return fallback
	}
	return value
}
