from __future__ import annotations

import hashlib

from .models import (
    AgentSnapshot,
    Checkpoint,
    CollaborationPolicy,
    CollaborationRequest,
    ExecutionLimits,
    KnowledgeChunk,
    MessageSnapshot,
    ModelReference,
    RoomSnapshot,
)
from .protocol import validate_checkpoint_size, validate_deadline
from .v1 import collaboration_runtime_pb2


ENGINE_NAMES = {
    collaboration_runtime_pb2.COLLABORATION_ENGINE_NATIVE: "native",
    collaboration_runtime_pb2.COLLABORATION_ENGINE_AUTOGEN: "autogen",
}
TRIGGER_MODE_NAMES = {
    collaboration_runtime_pb2.TRIGGER_MODE_MENTION_ONLY: "mention_only",
    collaboration_runtime_pb2.TRIGGER_MODE_AUTOMATIC: "automatic",
}
SENDER_TYPE_NAMES = {
    collaboration_runtime_pb2.SENDER_TYPE_HUMAN: "human",
    collaboration_runtime_pb2.SENDER_TYPE_AGENT: "agent",
    collaboration_runtime_pb2.SENDER_TYPE_SYSTEM: "system",
}


def map_request(request) -> CollaborationRequest:
    snapshot = request.snapshot
    limits = snapshot.limits
    validate_deadline(limits.timeout)
    checkpoint = _checkpoint(request.checkpoint) if request.HasField("checkpoint") else None
    if checkpoint is not None:
        validate_checkpoint_size(request.checkpoint, limits.max_checkpoint_bytes)
        digest = hashlib.sha256(checkpoint.payload).hexdigest()
        if checkpoint.sha256.lower() != digest:
            raise ValueError("checkpoint SHA-256 does not match payload")

    return CollaborationRequest(
        protocol_version=request.protocol_version,
        collaboration_run_id=request.collaboration_run_id,
        trace_id=request.trace_id,
        engine=_required_name(ENGINE_NAMES, request.engine, "engine"),
        room=RoomSnapshot(id=snapshot.room.id, name=snapshot.room.name, status=snapshot.room.status),
        agents=tuple(_agent(item) for item in snapshot.agents),
        trigger=_message(snapshot.trigger),
        transcript=tuple(_message(item) for item in snapshot.transcript),
        knowledge_chunks=tuple(_knowledge(item) for item in snapshot.knowledge_chunks),
        model_references=tuple(_model(item) for item in snapshot.model_references),
        policy=_policy(snapshot.policy),
        limits=ExecutionLimits(
            timeout_seconds=limits.timeout.ToTimedelta().total_seconds(),
            max_output_bytes=limits.max_output_bytes,
            max_artifact_bytes=limits.max_artifact_bytes,
            max_tool_steps=limits.max_tool_steps,
            max_request_bytes=limits.max_request_bytes,
            max_event_bytes=limits.max_event_bytes,
            max_checkpoint_bytes=limits.max_checkpoint_bytes,
        ),
        initial_candidate_agent_ids=tuple(snapshot.initial_candidate_agent_ids),
        checkpoint=checkpoint,
    )


def _agent(item) -> AgentSnapshot:
    return AgentSnapshot(
        id=item.id,
        name=item.name,
        mention=item.mention,
        role=item.role,
        description=item.description,
        system_prompt=item.system_prompt,
        runtime=item.runtime,
        model_reference_id=item.model_reference_id,
        tool_names=tuple(item.tool_names),
    )


def _message(item) -> MessageSnapshot:
    created_at = item.created_at.ToDatetime() if item.HasField("created_at") else None
    return MessageSnapshot(
        id=item.id,
        sender_id=item.sender_id,
        sender_name=item.sender_name,
        sender_type=_required_name(SENDER_TYPE_NAMES, item.sender_type, "sender type"),
        content=item.content,
        created_at=created_at,
        collaboration_run_id=item.collaboration_run_id,
        turn_index=item.turn_index,
        parent_message_id=item.parent_message_id,
    )


def _knowledge(item) -> KnowledgeChunk:
    return KnowledgeChunk(
        id=item.id,
        document_id=item.document_id,
        document_name=item.document_name,
        scope=item.scope,
        scope_id=item.scope_id,
        chunk_index=item.chunk_index,
        content=item.content,
    )


def _model(item) -> ModelReference:
    return ModelReference(
        id=item.id,
        profile_id=item.profile_id,
        source=item.source,
        protocol=item.protocol,
        model_name=item.model_name,
        runtime_scope=item.runtime_scope,
    )


def _policy(item) -> CollaborationPolicy:
    return CollaborationPolicy(
        version=item.version,
        engine=_required_name(ENGINE_NAMES, item.engine, "policy engine"),
        trigger_mode=_required_name(TRIGGER_MODE_NAMES, item.trigger_mode, "trigger mode"),
        max_turns=item.max_turns,
        max_turns_per_agent=item.max_turns_per_agent,
        allow_agent_handoff=item.allow_agent_handoff,
        allow_self_followup=item.allow_self_followup,
        cooldown_seconds=item.cooldown.ToTimedelta().total_seconds() if item.HasField("cooldown") else 0,
        stop_on_empty_output=item.stop_on_empty_output,
        stop_on_repeated_output=item.stop_on_repeated_output,
    )


def _checkpoint(item) -> Checkpoint:
    return Checkpoint(
        engine=_required_name(ENGINE_NAMES, item.engine, "checkpoint engine"),
        engine_version=item.engine_version,
        format_version=item.format_version,
        sha256=item.sha256,
        size_bytes=item.size_bytes,
        payload=bytes(item.payload),
    )


def _required_name(mapping: dict[int, str], value: int, label: str) -> str:
    try:
        return mapping[value]
    except KeyError as exc:
        raise ValueError(f"{label} is required") from exc
