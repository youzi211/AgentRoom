"""Stable Collaboration Runtime v1 protocol validation."""

from datetime import timedelta

import grpc

from collaboration_runtime.v1 import collaboration_runtime_pb2


PROTOCOL_VERSION = "v1"
DEFAULT_MAX_REQUEST_BYTES = 8 * 1024 * 1024
DEFAULT_MAX_EVENT_BYTES = 4 * 1024 * 1024
DEFAULT_MAX_ARTIFACT_BYTES = 2 * 1024 * 1024
DEFAULT_MAX_OUTPUT_BYTES = 1 * 1024 * 1024
DEFAULT_MAX_CHECKPOINT_BYTES = 1 * 1024 * 1024


class ProtocolVersionError(ValueError):
    pass


def validate_protocol_version(version: str) -> None:
    if version != PROTOCOL_VERSION:
        raise ProtocolVersionError(f"unsupported Collaboration Runtime protocol version {version!r}")


def validate_deadline(timeout) -> None:
    if timeout is None or timeout.ToTimedelta() <= timedelta(0):
        raise ValueError("collaboration deadline must be positive")


def validate_request_size(request, max_bytes: int = 0) -> None:
    _validate_size("request", request.ByteSize(), max_bytes or DEFAULT_MAX_REQUEST_BYTES)


def validate_event_size(event, max_bytes: int = 0) -> None:
    _validate_size("event", event.ByteSize(), max_bytes or DEFAULT_MAX_EVENT_BYTES)


def validate_artifact_size(artifact, max_bytes: int = 0) -> None:
    _validate_size("artifact", len(artifact.content), max_bytes or DEFAULT_MAX_ARTIFACT_BYTES)


def validate_checkpoint_size(checkpoint, max_bytes: int = 0) -> None:
    if checkpoint.size_bytes != len(checkpoint.payload):
        raise ValueError("checkpoint declared size does not match payload")
    _validate_size("checkpoint", len(checkpoint.payload), max_bytes or DEFAULT_MAX_CHECKPOINT_BYTES)


def grpc_status_for_error_code(code):
    mapping = {
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_INVALID_REQUEST: grpc.StatusCode.INVALID_ARGUMENT,
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_UNSUPPORTED_VERSION: grpc.StatusCode.UNIMPLEMENTED,
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_ENGINE_UNAVAILABLE: grpc.StatusCode.UNAVAILABLE,
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_RESOURCE_EXHAUSTED: grpc.StatusCode.RESOURCE_EXHAUSTED,
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_DUPLICATE_RUN: grpc.StatusCode.ALREADY_EXISTS,
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_ROOM_BUSY: grpc.StatusCode.FAILED_PRECONDITION,
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_CANCELLED: grpc.StatusCode.CANCELLED,
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_DEADLINE_EXCEEDED: grpc.StatusCode.DEADLINE_EXCEEDED,
        collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_MODEL_TIMEOUT: grpc.StatusCode.DEADLINE_EXCEEDED,
    }
    return mapping.get(code, grpc.StatusCode.INTERNAL)


def _validate_size(resource: str, size: int, max_bytes: int) -> None:
    if size > max_bytes:
        raise ValueError(f"collaboration {resource} exceeds {max_bytes} bytes")


class EventSequenceValidator:
    """Validate stream envelopes without applying AgentRoom business policy."""

    _TURN_PAYLOADS = {
        "speaker_selected",
        "agent_turn_started",
        "model_started",
        "model_completed",
        "tool_started",
        "tool_completed",
        "tool_failed",
        "output_delta",
        "artifact_ready",
        "handoff_requested",
        "agent_message_completed",
    }
    _TERMINAL_PAYLOADS = {"completed", "stopped", "cancelled", "failed"}

    def __init__(self, run_id: str):
        self._run_id = run_id
        self._last_sequence = 0
        self._terminal_seen = False

    @property
    def terminal_seen(self) -> bool:
        return self._terminal_seen

    def validate(self, event: collaboration_runtime_pb2.CollaborationEvent) -> None:
        validate_protocol_version(event.protocol_version)
        if not event.collaboration_run_id or event.collaboration_run_id != self._run_id:
            raise ValueError(f"unexpected collaboration run ID {event.collaboration_run_id!r}")
        payload = event.WhichOneof("payload")
        if payload is None:
            raise ValueError("collaboration event payload is required")
        if self._terminal_seen:
            raise ValueError("collaboration event received after terminal event")
        if event.sequence == 0 or event.sequence <= self._last_sequence:
            raise ValueError("collaboration event sequence must increase")
        if self._last_sequence == 0 and payload != "accepted":
            raise ValueError("first collaboration event must be accepted")
        if payload in self._TURN_PAYLOADS:
            if not event.turn_id or not event.agent_id:
                raise ValueError("turn-scoped collaboration event requires turn and Agent IDs")
        elif event.turn_id or event.agent_id:
            raise ValueError("run-scoped collaboration event must not include turn identity")

        self._last_sequence = event.sequence
        self._terminal_seen = payload in self._TERMINAL_PAYLOADS
