from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from enum import StrEnum
from typing import Any, Mapping


class EventKind(StrEnum):
    ACCEPTED = "accepted"
    COLLABORATION_STARTED = "collaboration_started"
    SPEAKER_SELECTED = "speaker_selected"
    AGENT_TURN_STARTED = "agent_turn_started"
    MODEL_STARTED = "model_started"
    MODEL_COMPLETED = "model_completed"
    TOOL_STARTED = "tool_started"
    TOOL_COMPLETED = "tool_completed"
    TOOL_FAILED = "tool_failed"
    OUTPUT_DELTA = "output_delta"
    ARTIFACT_READY = "artifact_ready"
    HANDOFF_REQUESTED = "handoff_requested"
    AGENT_MESSAGE_COMPLETED = "agent_message_completed"
    CHECKPOINT = "checkpoint"
    COMPLETED = "completed"
    STOPPED = "stopped"
    CANCELLED = "cancelled"
    FAILED = "failed"


@dataclass(frozen=True)
class RoomSnapshot:
    id: str
    name: str
    status: str


@dataclass(frozen=True)
class AgentSnapshot:
    id: str
    name: str
    mention: str
    role: str
    description: str
    system_prompt: str
    runtime: str
    model_reference_id: str
    tool_names: tuple[str, ...] = ()


@dataclass(frozen=True)
class MessageSnapshot:
    id: str
    sender_id: str
    sender_name: str
    sender_type: str
    content: str
    created_at: datetime | None = None
    collaboration_run_id: str = ""
    turn_index: int = 0
    parent_message_id: str = ""


@dataclass(frozen=True)
class KnowledgeChunk:
    id: str
    document_id: str
    document_name: str
    scope: str
    scope_id: str
    chunk_index: int
    content: str


@dataclass(frozen=True)
class ModelReference:
    id: str
    profile_id: str
    source: str
    protocol: str
    model_name: str
    runtime_scope: str


@dataclass(frozen=True)
class CollaborationPolicy:
    version: str
    engine: str
    trigger_mode: str
    max_turns: int
    max_turns_per_agent: int
    allow_agent_handoff: bool
    allow_self_followup: bool
    cooldown_seconds: float
    stop_on_empty_output: bool
    stop_on_repeated_output: bool


@dataclass(frozen=True)
class ExecutionLimits:
    timeout_seconds: float
    max_output_bytes: int
    max_artifact_bytes: int
    max_tool_steps: int
    max_request_bytes: int
    max_event_bytes: int
    max_checkpoint_bytes: int


@dataclass(frozen=True)
class Checkpoint:
    engine: str
    engine_version: str
    format_version: str
    sha256: str
    size_bytes: int
    payload: bytes


@dataclass(frozen=True)
class CollaborationRequest:
    protocol_version: str
    collaboration_run_id: str
    trace_id: str
    engine: str
    room: RoomSnapshot
    agents: tuple[AgentSnapshot, ...]
    trigger: MessageSnapshot
    transcript: tuple[MessageSnapshot, ...]
    knowledge_chunks: tuple[KnowledgeChunk, ...]
    model_references: tuple[ModelReference, ...]
    policy: CollaborationPolicy
    limits: ExecutionLimits
    initial_candidate_agent_ids: tuple[str, ...] = ()
    checkpoint: Checkpoint | None = None


@dataclass(frozen=True)
class EngineEvent:
    kind: EventKind
    turn_id: str = ""
    agent_id: str = ""
    data: Mapping[str, Any] = field(default_factory=dict)
