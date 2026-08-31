from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator
from dataclasses import dataclass, field
from enum import StrEnum
from typing import Any, Mapping, Protocol, runtime_checkable

from .models import (
    AgentSnapshot,
    ExecutionLimits,
    KnowledgeChunk,
    MessageSnapshot,
    ModelReference,
    RoomSnapshot,
)


class ExecutorEventKind(StrEnum):
    MODEL_STARTED = "model_started"
    MODEL_COMPLETED = "model_completed"
    TOOL_STARTED = "tool_started"
    TOOL_COMPLETED = "tool_completed"
    TOOL_FAILED = "tool_failed"
    OUTPUT_DELTA = "output_delta"
    ARTIFACT_READY = "artifact_ready"
    COMPLETED = "completed"
    FAILED = "failed"


@dataclass(frozen=True)
class AgentTurnRequest:
    collaboration_run_id: str
    trace_id: str
    turn_id: str
    turn_index: int
    room: RoomSnapshot
    agent: AgentSnapshot
    trigger: MessageSnapshot
    transcript: tuple[MessageSnapshot, ...]
    knowledge_chunks: tuple[KnowledgeChunk, ...]
    model_reference: ModelReference
    limits: ExecutionLimits


@dataclass(frozen=True)
class ExecutorEvent:
    kind: ExecutorEventKind
    data: Mapping[str, Any] = field(default_factory=dict)


@runtime_checkable
class AgentExecutor(Protocol):
    def execute(
        self,
        request: AgentTurnRequest,
        cancel_event: asyncio.Event,
    ) -> AsyncIterator[ExecutorEvent]: ...
