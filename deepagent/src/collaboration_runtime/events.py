from __future__ import annotations

import hashlib
from collections.abc import Mapping

from google.protobuf.timestamp_pb2 import Timestamp

from .models import Checkpoint, EngineEvent, EventKind
from .protocol import (
    EventSequenceValidator,
    PROTOCOL_VERSION,
    validate_artifact_size,
    validate_checkpoint_size,
    validate_event_size,
)
from .v1 import collaboration_runtime_pb2


TERMINAL_KINDS = {EventKind.COMPLETED, EventKind.STOPPED, EventKind.CANCELLED, EventKind.FAILED}

STOP_REASONS = {
    "completed": collaboration_runtime_pb2.COLLABORATION_STOP_REASON_COMPLETED,
    "max_turns": collaboration_runtime_pb2.COLLABORATION_STOP_REASON_MAX_TURNS,
    "max_turns_per_agent": collaboration_runtime_pb2.COLLABORATION_STOP_REASON_MAX_TURNS_PER_AGENT,
    "empty_output": collaboration_runtime_pb2.COLLABORATION_STOP_REASON_EMPTY_OUTPUT,
    "duplicate_output": collaboration_runtime_pb2.COLLABORATION_STOP_REASON_DUPLICATE_OUTPUT,
    "no_eligible_agent": collaboration_runtime_pb2.COLLABORATION_STOP_REASON_NO_ELIGIBLE_AGENT,
    "cancelled": collaboration_runtime_pb2.COLLABORATION_STOP_REASON_CANCELLED,
    "deadline_exceeded": collaboration_runtime_pb2.COLLABORATION_STOP_REASON_DEADLINE_EXCEEDED,
    "interrupted": collaboration_runtime_pb2.COLLABORATION_STOP_REASON_INTERRUPTED,
    "engine_failure": collaboration_runtime_pb2.COLLABORATION_STOP_REASON_ENGINE_FAILURE,
    "protocol_error": collaboration_runtime_pb2.COLLABORATION_STOP_REASON_PROTOCOL_ERROR,
}

ERROR_CODES = {
    "invalid_request": collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_INVALID_REQUEST,
    "unsupported_version": collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_UNSUPPORTED_VERSION,
    "engine_unavailable": collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_ENGINE_UNAVAILABLE,
    "resource_exhausted": collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_RESOURCE_EXHAUSTED,
    "duplicate_run": collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_DUPLICATE_RUN,
    "room_busy": collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_ROOM_BUSY,
    "model_not_configured": collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_MODEL_NOT_CONFIGURED,
    "model_authentication_failed": collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_MODEL_AUTHENTICATION_FAILED,
    "model_rate_limited": collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_MODEL_RATE_LIMITED,
    "model_timeout": collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_MODEL_TIMEOUT,
    "tool_failed": collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_TOOL_FAILED,
    "output_invalid": collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_OUTPUT_INVALID,
    "checkpoint_invalid": collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_CHECKPOINT_INVALID,
    "protocol_error": collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_PROTOCOL_ERROR,
    "cancelled": collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_CANCELLED,
    "deadline_exceeded": collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_DEADLINE_EXCEEDED,
    "internal": collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_INTERNAL,
}


class EventSequenceError(RuntimeError):
    pass


class ResourceLimitError(RuntimeError):
    pass


class CollaborationEventWriter:
    def __init__(
        self,
        run_id: str,
        *,
        max_event_bytes: int,
        max_artifact_bytes: int,
        max_output_bytes: int,
        max_checkpoint_bytes: int,
        allowed_agent_ids: frozenset[str] = frozenset(),
    ) -> None:
        self._run_id = run_id
        self._sequence = 0
        self._accepted = False
        self._terminal = False
        self._validator = EventSequenceValidator(run_id)
        self._max_event_bytes = max_event_bytes
        self._max_artifact_bytes = max_artifact_bytes
        self._max_output_bytes = max_output_bytes
        self._max_checkpoint_bytes = max_checkpoint_bytes
        self._allowed_agent_ids = allowed_agent_ids

    @property
    def terminal(self) -> bool:
        return self._terminal

    def accepted(self):
        if self._accepted:
            raise EventSequenceError("accepted may only be emitted once")
        wrapped = self._wrap(
            EventKind.ACCEPTED,
            "accepted",
            collaboration_runtime_pb2.AcceptedEvent(),
        )
        self._accepted = True
        return wrapped

    def write(self, event: EngineEvent):
        if not self._accepted:
            raise EventSequenceError("accepted must be emitted before Engine events")
        if event.kind is EventKind.ACCEPTED:
            raise EventSequenceError("Engine cannot emit accepted")
        if self._terminal:
            raise EventSequenceError("events cannot be emitted after a terminal event")
        if event.agent_id and self._allowed_agent_ids and event.agent_id not in self._allowed_agent_ids:
            raise EventSequenceError("Engine event references an Agent outside the request snapshot")
        field, message = self._payload(event)
        wrapped = self._wrap(event.kind, field, message, turn_id=event.turn_id, agent_id=event.agent_id)
        if event.kind in TERMINAL_KINDS:
            self._terminal = True
        return wrapped

    def failed(self, reason: str, code: str):
        return self.write(
            EngineEvent(
                EventKind.FAILED,
                data={"reason": reason, "code": code, "turn_count": 0},
            )
        )

    def _wrap(self, kind: EventKind, field: str, message, *, turn_id: str = "", agent_id: str = ""):
        sequence = self._sequence + 1
        occurred_at = Timestamp()
        occurred_at.GetCurrentTime()
        event = collaboration_runtime_pb2.CollaborationEvent(
            protocol_version=PROTOCOL_VERSION,
            collaboration_run_id=self._run_id,
            sequence=sequence,
            occurred_at=occurred_at,
            turn_id=turn_id,
            agent_id=agent_id,
            **{field: message},
        )
        try:
            validate_event_size(event, self._max_event_bytes)
            self._validator.validate(event)
        except ValueError as exc:
            if "exceeds" in str(exc):
                raise ResourceLimitError("serialized CollaborationEvent exceeds the configured limit") from exc
            raise EventSequenceError(str(exc)) from exc
        self._sequence = sequence
        return event

    def _payload(self, event: EngineEvent):
        data = event.data
        if event.kind is EventKind.COLLABORATION_STARTED:
            return "collaboration_started", collaboration_runtime_pb2.CollaborationStartedEvent()
        if event.kind is EventKind.SPEAKER_SELECTED:
            return "speaker_selected", collaboration_runtime_pb2.SpeakerSelectedEvent(
                reason_category=_text(data, "reason_category")
            )
        if event.kind is EventKind.AGENT_TURN_STARTED:
            return "agent_turn_started", collaboration_runtime_pb2.AgentTurnStartedEvent()
        if event.kind is EventKind.MODEL_STARTED:
            return "model_started", collaboration_runtime_pb2.ModelStartedEvent(
                model_selection_id=_text(data, "model_selection_id")
            )
        if event.kind is EventKind.MODEL_COMPLETED:
            return "model_completed", collaboration_runtime_pb2.ModelCompletedEvent(
                model_selection_id=_text(data, "model_selection_id"), usage=_usage(data.get("usage", {}))
            )
        if event.kind is EventKind.TOOL_STARTED:
            return "tool_started", collaboration_runtime_pb2.ToolStartedEvent(
                tool_call_id=_text(data, "tool_call_id"),
                tool_name=_text(data, "tool_name"),
                input_summary=_text(data, "input_summary"),
            )
        if event.kind is EventKind.TOOL_COMPLETED:
            return "tool_completed", collaboration_runtime_pb2.ToolCompletedEvent(
                tool_call_id=_text(data, "tool_call_id"),
                tool_name=_text(data, "tool_name"),
                output_summary=_text(data, "output_summary"),
            )
        if event.kind is EventKind.TOOL_FAILED:
            return "tool_failed", collaboration_runtime_pb2.ToolFailedEvent(
                tool_call_id=_text(data, "tool_call_id"),
                tool_name=_text(data, "tool_name"),
                failure=_failure(data),
            )
        if event.kind is EventKind.OUTPUT_DELTA:
            text = _text(data, "text")
            if len(text.encode("utf-8")) > self._max_output_bytes:
                raise ResourceLimitError("output delta exceeds the configured limit")
            return "output_delta", collaboration_runtime_pb2.OutputDeltaEvent(text=text)
        if event.kind is EventKind.ARTIFACT_READY:
            artifact = _artifact(data.get("artifact", {}))
            _validate_artifact(artifact, self._max_artifact_bytes)
            return "artifact_ready", collaboration_runtime_pb2.ArtifactReadyEvent(artifact=artifact)
        if event.kind is EventKind.HANDOFF_REQUESTED:
            return "handoff_requested", collaboration_runtime_pb2.HandoffRequestedEvent(
                target_agent_id=_text(data, "target_agent_id"),
                reason_category=_text(data, "reason_category"),
            )
        if event.kind is EventKind.AGENT_MESSAGE_COMPLETED:
            content = _text(data, "content")
            if len(content.encode("utf-8")) > self._max_output_bytes:
                raise ResourceLimitError("Agent message exceeds the configured limit")
            artifacts = [_artifact(item) for item in data.get("artifacts", ())]
            for artifact in artifacts:
                _validate_artifact(artifact, self._max_artifact_bytes)
            return "agent_message_completed", collaboration_runtime_pb2.AgentMessageCompletedEvent(
                content=content,
                artifacts=artifacts,
                knowledge_sources=[_knowledge_source(item) for item in data.get("knowledge_sources", ())],
                model=_model_audit(data.get("model", {})),
                usage=_usage(data.get("usage", {})),
            )
        if event.kind is EventKind.CHECKPOINT:
            checkpoint = _checkpoint(data.get("checkpoint"))
            try:
                validate_checkpoint_size(checkpoint, self._max_checkpoint_bytes)
            except ValueError as exc:
                raise ResourceLimitError("checkpoint exceeds the configured limit") from exc
            return "checkpoint", collaboration_runtime_pb2.CheckpointEvent(checkpoint=checkpoint)
        if event.kind is EventKind.COMPLETED:
            return "completed", collaboration_runtime_pb2.CompletedEvent(
                turn_count=_integer(data, "turn_count"), reason=_stop_reason(data, "completed")
            )
        if event.kind is EventKind.STOPPED:
            return "stopped", collaboration_runtime_pb2.StoppedEvent(
                turn_count=_integer(data, "turn_count"), reason=_stop_reason(data, "no_eligible_agent")
            )
        if event.kind is EventKind.CANCELLED:
            return "cancelled", collaboration_runtime_pb2.CancelledEvent(
                turn_count=_integer(data, "turn_count"), reason=_stop_reason(data, "cancelled")
            )
        if event.kind is EventKind.FAILED:
            return "failed", collaboration_runtime_pb2.FailedEvent(
                turn_count=_integer(data, "turn_count"),
                reason=_stop_reason(data, "engine_failure"),
                failure=_failure(data),
            )
        raise EventSequenceError(f"unsupported Engine event kind {event.kind!r}")


def _text(data: Mapping[str, object], key: str) -> str:
    value = data.get(key, "")
    return str(value) if value is not None else ""


def _integer(data: Mapping[str, object], key: str) -> int:
    return int(data.get(key, 0))


def _usage(data) -> collaboration_runtime_pb2.Usage:
    return collaboration_runtime_pb2.Usage(
        input_tokens=int(data.get("input_tokens", 0)),
        output_tokens=int(data.get("output_tokens", 0)),
        total_tokens=int(data.get("total_tokens", 0)),
    )


def _artifact(data) -> collaboration_runtime_pb2.Artifact:
    return collaboration_runtime_pb2.Artifact(
        id=str(data.get("id", "")),
        type=str(data.get("type", "")),
        title=str(data.get("title", "")),
        file_name=str(data.get("file_name", "")),
        mime_type=str(data.get("mime_type", "")),
        content=bytes(data.get("content", b"")),
        external_uri=str(data.get("external_uri", "")),
    )


def _validate_artifact(artifact, max_bytes: int) -> None:
    try:
        validate_artifact_size(artifact, max_bytes)
    except ValueError as exc:
        raise ResourceLimitError("inline artifact exceeds the configured limit") from exc


def _knowledge_source(data) -> collaboration_runtime_pb2.KnowledgeSource:
    return collaboration_runtime_pb2.KnowledgeSource(
        document_id=str(data.get("document_id", "")),
        document_name=str(data.get("document_name", "")),
        scope=str(data.get("scope", "")),
    )


def _model_audit(data) -> collaboration_runtime_pb2.ModelAudit:
    return collaboration_runtime_pb2.ModelAudit(
        model_selection_id=str(data.get("model_selection_id", "")),
        profile_id=str(data.get("profile_id", "")),
        source=str(data.get("source", "")),
        model_name=str(data.get("model_name", "")),
    )


def _checkpoint(value) -> collaboration_runtime_pb2.OpaqueCheckpoint:
    if not isinstance(value, Checkpoint):
        raise EventSequenceError("checkpoint event requires a neutral Checkpoint")
    engine = {
        "native": collaboration_runtime_pb2.COLLABORATION_ENGINE_NATIVE,
        "autogen": collaboration_runtime_pb2.COLLABORATION_ENGINE_AUTOGEN,
    }.get(value.engine)
    if engine is None:
        raise EventSequenceError("checkpoint Engine is unsupported")
    if not value.engine_version or not value.format_version or not value.sha256:
        raise EventSequenceError("checkpoint metadata is incomplete")
    if hashlib.sha256(value.payload).hexdigest() != value.sha256.lower():
        raise EventSequenceError("checkpoint SHA-256 does not match payload")
    return collaboration_runtime_pb2.OpaqueCheckpoint(
        engine=engine,
        engine_version=value.engine_version,
        format_version=value.format_version,
        sha256=value.sha256,
        size_bytes=value.size_bytes,
        payload=value.payload,
    )


def _stop_reason(data: Mapping[str, object], fallback: str) -> int:
    return STOP_REASONS.get(_text(data, "reason") or fallback, collaboration_runtime_pb2.COLLABORATION_STOP_REASON_PROTOCOL_ERROR)


def _failure(data: Mapping[str, object]) -> collaboration_runtime_pb2.CollaborationFailure:
    code_name = _text(data, "code") or "internal"
    code = ERROR_CODES.get(code_name, collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_INTERNAL)
    return collaboration_runtime_pb2.CollaborationFailure(
        code=code,
        message=_safe_failure_message(code_name),
        retryable=bool(data.get("retryable", False)),
    )


def _safe_failure_message(code: str) -> str:
    return {
        "invalid_request": "Collaboration request is invalid",
        "unsupported_version": "Collaboration protocol version is unsupported",
        "engine_unavailable": "Collaboration Engine is unavailable",
        "resource_exhausted": "Collaboration resource limit exceeded",
        "duplicate_run": "Collaboration run is already active",
        "room_busy": "Collaboration room already has an active run",
        "model_not_configured": "Collaboration model is not configured",
        "model_authentication_failed": "Collaboration model authentication failed",
        "model_rate_limited": "Collaboration model rate limit exceeded",
        "model_timeout": "Collaboration model timed out",
        "tool_failed": "Collaboration tool failed",
        "output_invalid": "Collaboration output is invalid",
        "checkpoint_invalid": "Collaboration checkpoint is invalid",
        "protocol_error": "Collaboration Engine violated the event protocol",
        "cancelled": "Collaboration was cancelled",
        "deadline_exceeded": "Collaboration deadline exceeded",
    }.get(code, "Collaboration Engine failed")
