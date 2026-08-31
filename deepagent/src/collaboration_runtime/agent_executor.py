from __future__ import annotations

import asyncio
from pathlib import Path

from agent_runtime.context import RunContext
from agent_runtime.events import EventPayload
from agent_runtime.registry import ExecutorRegistry
from agent_runtime.v1 import agent_runtime_pb2
from google.protobuf.duration_pb2 import Duration

from .executor import AgentTurnRequest, ExecutorEvent, ExecutorEventKind


class RuntimeRegistryAgentExecutor:
    """Adapter from the single-Agent ExecutorRegistry to collaboration turns."""

    def __init__(self, registry: ExecutorRegistry, *, work_dir: Path) -> None:
        self._registry = registry
        self._work_dir = Path(work_dir)

    async def execute(self, request: AgentTurnRequest, cancel_event: asyncio.Event):
        runtime_request = _runtime_request(request)
        executor = self._registry.resolve(runtime_request.executor_kind)
        run = RunContext.create(runtime_request, self._work_dir)
        try:
            async for event in executor.execute(run):
                if cancel_event.is_set():
                    run.cancel_event.set()
                    raise asyncio.CancelledError
                mapped = _map_event(event)
                if mapped is not None:
                    yield mapped
        finally:
            run.cleanup()


def _runtime_request(request: AgentTurnRequest) -> agent_runtime_pb2.ExecuteAgentRequest:
    timeout = Duration(seconds=int(request.limits.timeout_seconds))
    runtime = request.agent.runtime.strip().lower()
    executor_kind = {
        "llm": agent_runtime_pb2.EXECUTOR_KIND_LLM,
        "deepagent": agent_runtime_pb2.EXECUTOR_KIND_DEEPAGENT,
    }.get(runtime, agent_runtime_pb2.EXECUTOR_KIND_UNSPECIFIED)
    return agent_runtime_pb2.ExecuteAgentRequest(
        protocol_version="v1",
        run_id=request.turn_id,
        trace_id=request.trace_id,
        executor_kind=executor_kind,
        room=agent_runtime_pb2.RoomSnapshot(
            id=request.room.id,
            name=request.room.name,
        ),
        agent=agent_runtime_pb2.AgentSnapshot(
            id=request.agent.id,
            name=request.agent.name,
            mention=request.agent.mention,
            role=request.agent.role,
            description=request.agent.description,
            system_prompt=request.agent.system_prompt,
            runtime=request.agent.runtime,
            model_profile_id=request.model_reference.profile_id,
        ),
        trigger=_message_snapshot(request.trigger),
        recent_messages=tuple(_message_snapshot(message) for message in request.transcript),
        knowledge_chunks=tuple(_knowledge_chunk(chunk) for chunk in request.knowledge_chunks),
        model=agent_runtime_pb2.ModelConnection(
            protocol=request.model_reference.protocol,
            model_name=request.model_reference.model_name,
            profile_id=request.model_reference.profile_id,
            source=request.model_reference.source,
        ),
        limits=agent_runtime_pb2.ExecutionLimits(
            timeout=timeout,
            max_output_bytes=request.limits.max_output_bytes,
            max_artifact_bytes=request.limits.max_artifact_bytes,
            max_tool_steps=request.limits.max_tool_steps,
        ),
    )


def _message_snapshot(message) -> agent_runtime_pb2.MessageSnapshot:
    sender_type = {
        "human": agent_runtime_pb2.SENDER_TYPE_HUMAN,
        "agent": agent_runtime_pb2.SENDER_TYPE_AGENT,
        "system": agent_runtime_pb2.SENDER_TYPE_SYSTEM,
    }.get(message.sender_type, agent_runtime_pb2.SENDER_TYPE_UNSPECIFIED)
    return agent_runtime_pb2.MessageSnapshot(
        id=message.id,
        sender_id=message.sender_id,
        sender_name=message.sender_name,
        sender_type=sender_type,
        content=message.content,
        dialogue_run_id=message.collaboration_run_id,
        turn_index=message.turn_index,
        parent_message_id=message.parent_message_id,
    )


def _knowledge_chunk(chunk) -> agent_runtime_pb2.KnowledgeChunk:
    return agent_runtime_pb2.KnowledgeChunk(
        id=chunk.id,
        document_id=chunk.document_id,
        document_name=chunk.document_name,
        scope=chunk.scope,
        scope_id=chunk.scope_id,
        chunk_index=chunk.chunk_index,
        content=chunk.content,
    )


def _map_event(event: EventPayload) -> ExecutorEvent | None:
    if event.field == "model_started":
        return ExecutorEvent(
            ExecutorEventKind.MODEL_STARTED,
            {"model_name": event.message.model_name},
        )
    if event.field == "model_completed":
        return ExecutorEvent(
            ExecutorEventKind.MODEL_COMPLETED,
            {"model_name": event.message.model_name, "usage": _usage(event.message.usage)},
        )
    if event.field == "tool_started":
        return ExecutorEvent(
            ExecutorEventKind.TOOL_STARTED,
            {
                "tool_call_id": event.message.tool_call_id,
                "tool_name": event.message.tool_name,
                "input_summary": event.message.input_summary,
            },
        )
    if event.field == "tool_completed":
        return ExecutorEvent(
            ExecutorEventKind.TOOL_COMPLETED,
            {
                "tool_call_id": event.message.tool_call_id,
                "tool_name": event.message.tool_name,
                "output_summary": event.message.output_summary,
            },
        )
    if event.field == "tool_failed":
        return ExecutorEvent(
            ExecutorEventKind.TOOL_FAILED,
            {
                "tool_call_id": event.message.tool_call_id,
                "tool_name": event.message.tool_name,
                "retryable": event.message.failure.retryable,
            },
        )
    if event.field == "output_delta":
        return ExecutorEvent(ExecutorEventKind.OUTPUT_DELTA, {"text": event.message.text})
    if event.field == "artifact_ready":
        return ExecutorEvent(
            ExecutorEventKind.ARTIFACT_READY,
            {"artifact": _artifact(event.message.artifact)},
        )
    if event.field == "completed":
        return ExecutorEvent(
            ExecutorEventKind.COMPLETED,
            {
                "content": event.message.content,
                "artifacts": tuple(_artifact(artifact) for artifact in event.message.artifacts),
                "knowledge_sources": tuple(
                    {
                        "document_id": source.document_id,
                        "document_name": source.document_name,
                        "scope": source.scope,
                    }
                    for source in event.message.knowledge_sources
                ),
                "model": {
                    "profile_id": event.message.model.profile_id,
                    "source": event.message.model.source,
                    "model_name": event.message.model.model_name,
                },
                "usage": _usage(event.message.usage),
            },
        )
    if event.field == "failed":
        return ExecutorEvent(
            ExecutorEventKind.FAILED,
            {"code": _failure_code(event.message.failure.code), "retryable": event.message.failure.retryable},
        )
    return None


def _usage(usage) -> dict[str, int]:
    return {
        "input_tokens": int(usage.input_tokens),
        "output_tokens": int(usage.output_tokens),
        "total_tokens": int(usage.total_tokens),
    }


def _artifact(artifact) -> dict[str, object]:
    return {
        "id": artifact.id,
        "type": artifact.type,
        "title": artifact.title,
        "file_name": artifact.file_name,
        "mime_type": artifact.mime_type,
        "content": bytes(artifact.content),
        "external_uri": artifact.external_uri,
    }


def _failure_code(code: int) -> str:
    mapping = {
        agent_runtime_pb2.RUN_ERROR_CODE_INVALID_REQUEST: "invalid_request",
        agent_runtime_pb2.RUN_ERROR_CODE_MODEL_NOT_CONFIGURED: "model_not_configured",
        agent_runtime_pb2.RUN_ERROR_CODE_MODEL_AUTHENTICATION_FAILED: "model_authentication_failed",
        agent_runtime_pb2.RUN_ERROR_CODE_MODEL_RATE_LIMITED: "model_rate_limited",
        agent_runtime_pb2.RUN_ERROR_CODE_MODEL_TIMEOUT: "model_timeout",
        agent_runtime_pb2.RUN_ERROR_CODE_CANCELLED: "cancelled",
        agent_runtime_pb2.RUN_ERROR_CODE_TOOL_FAILED: "tool_failed",
        agent_runtime_pb2.RUN_ERROR_CODE_OUTPUT_INVALID: "output_invalid",
        agent_runtime_pb2.RUN_ERROR_CODE_RESOURCE_EXHAUSTED: "resource_exhausted",
        agent_runtime_pb2.RUN_ERROR_CODE_PROTOCOL_ERROR: "protocol_error",
    }
    return mapping.get(code, "internal")
