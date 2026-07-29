import grpc
import pytest
from google.protobuf import duration_pb2

from collaboration_runtime.protocol import (
    grpc_status_for_error_code,
    validate_artifact_size,
    validate_checkpoint_size,
    validate_deadline,
    validate_event_size,
    validate_request_size,
)
from collaboration_runtime.v1 import collaboration_runtime_pb2


def test_collaboration_deadline_must_be_positive():
    validate_deadline(duration_pb2.Duration(seconds=30))
    for timeout in (None, duration_pb2.Duration(), duration_pb2.Duration(seconds=-1)):
        with pytest.raises(ValueError, match="deadline"):
            validate_deadline(timeout)


def test_collaboration_resource_limits():
    request = collaboration_runtime_pb2.ExecuteConversationRequest(trace_id="r" * 64)
    with pytest.raises(ValueError, match="request"):
        validate_request_size(request, 8)

    event = collaboration_runtime_pb2.CollaborationEvent(
        output_delta=collaboration_runtime_pb2.OutputDeltaEvent(text="e" * 64)
    )
    with pytest.raises(ValueError, match="event"):
        validate_event_size(event, 8)

    artifact = collaboration_runtime_pb2.Artifact(content=b"a" * 9)
    with pytest.raises(ValueError, match="artifact"):
        validate_artifact_size(artifact, 8)

    checkpoint = collaboration_runtime_pb2.OpaqueCheckpoint(size_bytes=9, payload=b"c" * 9)
    with pytest.raises(ValueError, match="checkpoint"):
        validate_checkpoint_size(checkpoint, 8)

    mismatch = collaboration_runtime_pb2.OpaqueCheckpoint(size_bytes=8, payload=b"c" * 7)
    with pytest.raises(ValueError, match="declared size"):
        validate_checkpoint_size(mismatch, 8)


def test_collaboration_grpc_status_mapping_includes_cancellation_and_deadline():
    expected = {
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_INVALID_REQUEST: grpc.StatusCode.INVALID_ARGUMENT,
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_UNSUPPORTED_VERSION: grpc.StatusCode.UNIMPLEMENTED,
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_ENGINE_UNAVAILABLE: grpc.StatusCode.UNAVAILABLE,
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_RESOURCE_EXHAUSTED: grpc.StatusCode.RESOURCE_EXHAUSTED,
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_DUPLICATE_RUN: grpc.StatusCode.ALREADY_EXISTS,
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_ROOM_BUSY: grpc.StatusCode.FAILED_PRECONDITION,
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_CANCELLED: grpc.StatusCode.CANCELLED,
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_DEADLINE_EXCEEDED: grpc.StatusCode.DEADLINE_EXCEEDED,
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_INTERNAL: grpc.StatusCode.INTERNAL,
    }
    for error_code, status in expected.items():
        assert grpc_status_for_error_code(error_code) is status
