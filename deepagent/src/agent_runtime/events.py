from __future__ import annotations

from dataclasses import dataclass

from google.protobuf.message import Message
from google.protobuf.timestamp_pb2 import Timestamp

from .protocol import PROTOCOL_VERSION
from .v1 import agent_runtime_pb2


TERMINAL_PAYLOADS = {"completed", "failed"}


class EventSequenceError(RuntimeError):
    """Raised when an Executor violates the event stream contract."""


class ResourceLimitError(RuntimeError):
    """Raised rather than silently truncating output or artifacts."""


@dataclass(frozen=True)
class EventPayload:
    field: str
    message: Message


def payload(field: str, message: Message) -> EventPayload:
    return EventPayload(field=field, message=message)


class EventWriter:
    def __init__(
        self,
        run_id: str,
        *,
        max_event_bytes: int,
        max_artifact_bytes: int,
        max_output_bytes: int,
    ) -> None:
        self._run_id = run_id
        self._sequence = 0
        self._started = False
        self._terminal = False
        self._max_event_bytes = max_event_bytes
        self._max_artifact_bytes = max_artifact_bytes
        self._max_output_bytes = max_output_bytes

    @property
    def terminal(self) -> bool:
        return self._terminal

    def accepted(self) -> agent_runtime_pb2.AgentEvent:
        if self._started:
            raise EventSequenceError("accepted may only be emitted once")
        self._started = True
        return self._wrap("accepted", agent_runtime_pb2.AcceptedEvent())

    def write(self, item: EventPayload) -> agent_runtime_pb2.AgentEvent:
        if not self._started:
            raise EventSequenceError("accepted must be emitted before Executor events")
        if self._terminal:
            raise EventSequenceError("events cannot be emitted after a terminal event")
        if item.field == "accepted":
            raise EventSequenceError("Executor cannot emit accepted")
        self._validate_payload(item)
        event = self._wrap(item.field, item.message)
        if item.field in TERMINAL_PAYLOADS:
            self._terminal = True
        return event

    def failed(self, code: int, message: str, *, retryable: bool = False) -> agent_runtime_pb2.AgentEvent:
        return self.write(
            payload(
                "failed",
                agent_runtime_pb2.FailedEvent(
                    failure=agent_runtime_pb2.RunFailure(
                        code=code,
                        message=message,
                        retryable=retryable,
                    )
                ),
            )
        )

    def _wrap(self, field: str, message: Message) -> agent_runtime_pb2.AgentEvent:
        self._sequence += 1
        occurred_at = Timestamp()
        occurred_at.GetCurrentTime()
        event = agent_runtime_pb2.AgentEvent(
            protocol_version=PROTOCOL_VERSION,
            run_id=self._run_id,
            sequence=self._sequence,
            occurred_at=occurred_at,
            **{field: message},
        )
        if event.ByteSize() > self._max_event_bytes:
            raise ResourceLimitError("serialized AgentEvent exceeds the configured limit")
        return event

    def _validate_payload(self, item: EventPayload) -> None:
        if item.field == "output_delta":
            size = len(getattr(item.message, "text", "").encode("utf-8"))
            if size > self._max_output_bytes:
                raise ResourceLimitError("output delta exceeds the configured limit")
        artifacts = []
        if item.field == "artifact_ready" and getattr(item.message, "artifact", None):
            artifacts = [item.message.artifact]
        elif item.field == "completed":
            content = getattr(item.message, "content", "")
            if len(content.encode("utf-8")) > self._max_output_bytes:
                raise ResourceLimitError("completed output exceeds the configured limit")
            artifacts = list(getattr(item.message, "artifacts", []))
        for artifact in artifacts:
            if len(artifact.content) > self._max_artifact_bytes:
                raise ResourceLimitError("inline artifact exceeds the configured limit")
