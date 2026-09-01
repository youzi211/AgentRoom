"""Pure helpers shared by Native and AutoGen collaboration engines.

These functions encode framework-neutral selection, normalization and
executor-event mapping rules. They MUST NOT import any collaboration
framework, transport, database or Provider SDK: both engines depend on
them, so a forbidden import here would contaminate every engine module
that the dependency-boundary test scans.
"""

from __future__ import annotations

import asyncio
from collections import defaultdict, deque
from collections.abc import Mapping

from ..executor import ExecutorEvent, ExecutorEventKind
from ..models import (
    AgentSnapshot,
    CollaborationRequest,
    EngineEvent,
    EventKind,
    MessageSnapshot,
    ModelSelection,
)


EXECUTOR_FAILURE_CODES = frozenset(
    {
        "invalid_request",
        "model_not_configured",
        "model_authentication_failed",
        "model_rate_limited",
        "model_timeout",
        "tool_failed",
        "output_invalid",
        "resource_exhausted",
        "protocol_error",
        "cancelled",
        "deadline_exceeded",
        "internal",
    }
)


def initial_candidates(request: CollaborationRequest) -> tuple[AgentSnapshot, ...]:
    """Return the ordered, de-duplicated, eligible first speakers.

    Explicit mention candidates are preserved in request order after
    eligibility filtering. When no candidates were supplied and the policy
    is ``automatic``, the first eligible agent is used as the default
    speaker so the engine never fires every agent at once.
    """
    agents_by_id = {agent.id: agent for agent in request.agents}
    model_ids = {model.id for model in request.model_selections}
    seen: set[str] = set()
    candidates: list[AgentSnapshot] = []
    for agent_id in request.initial_candidate_agent_ids:
        if agent_id in seen:
            continue
        seen.add(agent_id)
        agent = agents_by_id.get(agent_id)
        if agent is None:
            continue
        if agent.runtime not in {"llm", "deepagent"}:
            continue
        if agent.model_selection_id not in model_ids:
            continue
        candidates.append(agent)
    if not request.initial_candidate_agent_ids and request.policy.trigger_mode == "automatic":
        for agent in request.agents:
            if agent.runtime not in {"llm", "deepagent"}:
                continue
            if agent.model_selection_id not in model_ids:
                continue
            return (agent,)
    return tuple(candidates)


def eligible_agents(
    agents: tuple[AgentSnapshot, ...],
    model_ids: frozenset[str],
) -> tuple[AgentSnapshot, ...]:
    """Return agents that may respond, in snapshot order."""
    return tuple(
        agent
        for agent in agents
        if agent.runtime in {"llm", "deepagent"}
        and agent.model_selection_id in model_ids
    )


def mentioned_agents(content: str, agents: tuple[AgentSnapshot, ...]) -> tuple[AgentSnapshot, ...]:
    """Return agents whose mention appears in ``content``, in mention order."""
    normalized = normalize_mention_text(content)
    matches = []
    for order, agent in enumerate(agents):
        mention = normalize_mention_text(agent.mention)
        if not mention:
            continue
        index = normalized.find(mention)
        if index >= 0:
            matches.append((index, order, agent))
    matches.sort(key=lambda item: (item[0], item[1]))
    return tuple(item[2] for item in matches)


def normalize_mention_text(value: str) -> str:
    """Normalize text for mention matching (case-insensitive, whitespace-folded)."""
    result = []
    previous_was_space = False
    just_saw_at = False
    for character in value.strip():
        if character in {"@", "＠"}:
            result.append("@")
            previous_was_space = False
            just_saw_at = True
        elif character.isspace():
            if just_saw_at or previous_was_space:
                continue
            result.append(" ")
            previous_was_space = True
        else:
            result.append(character.lower())
            previous_was_space = False
            just_saw_at = False
    return "".join(result).strip()


def normalize_output(value: str) -> str:
    """Normalize an Agent reply for duplicate detection (drop @mentions)."""
    return " ".join(
        field for field in value.lower().split() if not field.startswith("@")
    )


async def cooldown_cancelled(delay: float, cancel_event: asyncio.Event) -> bool:
    """Return True if ``cancel_event`` was set during the cooldown wait."""
    if delay <= 0:
        return cancel_event.is_set()
    try:
        await asyncio.wait_for(cancel_event.wait(), timeout=delay)
    except TimeoutError:
        return False
    return True


def map_executor_event(
    event: ExecutorEvent,
    turn_id: str,
    agent_id: str,
    model_selection: ModelSelection,
) -> EngineEvent | None:
    """Map an Executor event to a neutral Engine event, or None to skip."""
    data = event.data
    if event.kind is ExecutorEventKind.MODEL_STARTED:
        return EngineEvent(
            EventKind.MODEL_STARTED,
            turn_id=turn_id,
            agent_id=agent_id,
            data={"model_selection_id": model_selection.id},
        )
    if event.kind is ExecutorEventKind.MODEL_COMPLETED:
        return EngineEvent(
            EventKind.MODEL_COMPLETED,
            turn_id=turn_id,
            agent_id=agent_id,
            data={
                "model_selection_id": model_selection.id,
                "usage": usage_data(data.get("usage")),
            },
        )
    if event.kind is ExecutorEventKind.TOOL_STARTED:
        return EngineEvent(
            EventKind.TOOL_STARTED,
            turn_id=turn_id,
            agent_id=agent_id,
            data={
                "tool_call_id": text(data, "tool_call_id"),
                "tool_name": text(data, "tool_name"),
                "input_summary": text(data, "input_summary"),
            },
        )
    if event.kind is ExecutorEventKind.TOOL_COMPLETED:
        return EngineEvent(
            EventKind.TOOL_COMPLETED,
            turn_id=turn_id,
            agent_id=agent_id,
            data={
                "tool_call_id": text(data, "tool_call_id"),
                "tool_name": text(data, "tool_name"),
                "output_summary": text(data, "output_summary"),
            },
        )
    if event.kind is ExecutorEventKind.TOOL_FAILED:
        return EngineEvent(
            EventKind.TOOL_FAILED,
            turn_id=turn_id,
            agent_id=agent_id,
            data={
                "tool_call_id": text(data, "tool_call_id"),
                "tool_name": text(data, "tool_name"),
                "code": "tool_failed",
                "retryable": bool(data.get("retryable", False)),
            },
        )
    if event.kind is ExecutorEventKind.OUTPUT_DELTA:
        return EngineEvent(
            EventKind.OUTPUT_DELTA,
            turn_id=turn_id,
            agent_id=agent_id,
            data={"text": text(data, "text")},
        )
    if event.kind is ExecutorEventKind.ARTIFACT_READY:
        return EngineEvent(
            EventKind.ARTIFACT_READY,
            turn_id=turn_id,
            agent_id=agent_id,
            data={"artifact": artifact_data(data.get("artifact"))},
        )
    return None


def completed_data(
    data: Mapping[str, object],
    model_selection: ModelSelection,
) -> dict[str, object]:
    """Build the ``agent_message_completed`` payload from executor output."""
    return {
        "content": text(data, "content"),
        "artifacts": tuple(artifact_data(item) for item in data.get("artifacts", ())),
        "knowledge_sources": tuple(
            knowledge_source_data(item) for item in data.get("knowledge_sources", ())
        ),
        "model": {
            "model_selection_id": model_selection.id,
            "profile_id": model_selection.profile_id,
            "source": model_selection.source,
            "model_name": model_selection.model_name,
        },
        "usage": usage_data(data.get("usage")),
    }


def artifact_data(value) -> dict[str, object]:
    data = value if isinstance(value, Mapping) else {}
    return {
        "id": text(data, "id"),
        "type": text(data, "type"),
        "title": text(data, "title"),
        "file_name": text(data, "file_name"),
        "mime_type": text(data, "mime_type"),
        "content": bytes(data.get("content", b"")),
        "external_uri": text(data, "external_uri"),
    }


def knowledge_source_data(value) -> dict[str, object]:
    data = value if isinstance(value, Mapping) else {}
    return {
        "document_id": text(data, "document_id"),
        "document_name": text(data, "document_name"),
        "scope": text(data, "scope"),
    }


def usage_data(value) -> dict[str, int]:
    data = value if isinstance(value, Mapping) else {}
    return {
        "input_tokens": int(data.get("input_tokens", 0)),
        "output_tokens": int(data.get("output_tokens", 0)),
        "total_tokens": int(data.get("total_tokens", 0)),
    }


def text(data: Mapping[str, object], key: str) -> str:
    value = data.get(key, "")
    return str(value) if value is not None else ""


def executor_failure_code(value: object) -> str:
    code = str(value) if value is not None else "internal"
    return code if code in EXECUTOR_FAILURE_CODES else "internal"


def protocol_failure(turn_count: int) -> EngineEvent:
    return EngineEvent(
        EventKind.FAILED,
        data={
            "reason": "protocol_error",
            "code": "protocol_error",
            "turn_count": turn_count,
        },
    )


def message_snapshot_from_trigger(
    request: CollaborationRequest,
    turn_id: str,
    agent: AgentSnapshot,
    content: str,
    turn_index: int,
    parent_message_id: str,
) -> MessageSnapshot:
    """Build a transcript message for an accepted Agent reply."""
    return MessageSnapshot(
        id=turn_id,
        sender_id=agent.id,
        sender_name=agent.name,
        sender_type="agent",
        content=content,
        collaboration_run_id=request.collaboration_run_id,
        turn_index=turn_index,
        parent_message_id=parent_message_id,
    )


def new_turn_counts() -> defaultdict[str, int]:
    return defaultdict(int)


def pending_deque(
    candidates: tuple[AgentSnapshot, ...],
    trigger,
    reason: str,
) -> deque:
    return deque((agent, trigger, reason) for agent in candidates)
