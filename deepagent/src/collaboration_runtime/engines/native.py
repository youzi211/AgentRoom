from __future__ import annotations

import asyncio
from collections import defaultdict, deque
from collections.abc import Mapping

from ..executor import AgentExecutor, AgentTurnRequest, ExecutorEvent, ExecutorEventKind
from ..models import (
    AgentSnapshot,
    CollaborationRequest,
    EngineEvent,
    EventKind,
    MessageSnapshot,
    ModelReference,
)
from ._shared import (
    completed_data,
    cooldown_cancelled,
    executor_failure_code,
    initial_candidates,
    map_executor_event,
    mentioned_agents,
    message_snapshot_from_trigger,
    normalize_output,
    pending_deque,
    protocol_failure,
    text,
)


class NativeCollaborationEngine:
    """Framework-neutral baseline Engine built out incrementally."""

    name = "native"
    version = "native-v1"

    def __init__(self, executor: AgentExecutor | None = None) -> None:
        self._executor = executor

    async def execute(
        self,
        request: CollaborationRequest,
        cancel_event: asyncio.Event,
    ):
        yield EngineEvent(EventKind.COLLABORATION_STARTED)
        await asyncio.sleep(0)
        if cancel_event.is_set():
            yield EngineEvent(
                EventKind.CANCELLED,
                data={"reason": "cancelled", "turn_count": 0},
            )
            return

        candidates = initial_candidates(request)
        if self._executor is None or not candidates:
            yield EngineEvent(
                EventKind.STOPPED,
                data={"reason": "no_eligible_agent", "turn_count": 0},
            )
            return

        initial_reason = (
            "explicit_mention"
            if request.initial_candidate_agent_ids
            else "automatic_default"
        )
        pending = pending_deque(candidates, request.trigger, initial_reason)
        transcript = list(request.transcript)
        turns_by_agent: defaultdict[str, int] = defaultdict(int)
        accepted_outputs: set[str] = set()
        turn_count = 0
        blocked_by_agent_limit = False

        while pending:
            if cancel_event.is_set():
                yield EngineEvent(
                    EventKind.CANCELLED,
                    data={"reason": "cancelled", "turn_count": turn_count},
                )
                return
            if turn_count >= request.policy.max_turns:
                yield EngineEvent(
                    EventKind.STOPPED,
                    data={"reason": "max_turns", "turn_count": turn_count},
                )
                return

            agent, trigger, reason_category = pending.popleft()
            if turns_by_agent[agent.id] >= request.policy.max_turns_per_agent:
                blocked_by_agent_limit = True
                continue
            if (
                trigger.sender_type == "agent"
                and trigger.sender_id == agent.id
                and not request.policy.allow_self_followup
            ):
                continue

            if turn_count > 0 and await cooldown_cancelled(
                request.policy.cooldown_seconds,
                cancel_event,
            ):
                yield EngineEvent(
                    EventKind.CANCELLED,
                    data={"reason": "cancelled", "turn_count": turn_count},
                )
                return

            turn_index = turn_count + 1
            turn_id = f"{request.collaboration_run_id}:turn:{turn_index}"
            yield EngineEvent(
                EventKind.SPEAKER_SELECTED,
                turn_id=turn_id,
                agent_id=agent.id,
                data={"reason_category": reason_category},
            )
            yield EngineEvent(
                EventKind.AGENT_TURN_STARTED,
                turn_id=turn_id,
                agent_id=agent.id,
            )
            model_reference = next(
                model for model in request.model_references if model.id == agent.model_reference_id
            )
            completed_payload = None
            executor_terminal = False
            async for event in self._executor.execute(
                AgentTurnRequest(
                    collaboration_run_id=request.collaboration_run_id,
                    trace_id=request.trace_id,
                    turn_id=turn_id,
                    turn_index=turn_index,
                    room=request.room,
                    agent=agent,
                    trigger=trigger,
                    transcript=tuple(transcript),
                    knowledge_chunks=request.knowledge_chunks,
                    model_reference=model_reference,
                    limits=request.limits,
                ),
                cancel_event,
            ):
                if cancel_event.is_set():
                    yield EngineEvent(
                        EventKind.CANCELLED,
                        data={"reason": "cancelled", "turn_count": turn_count},
                    )
                    return
                if event.kind is ExecutorEventKind.COMPLETED:
                    if executor_terminal:
                        yield protocol_failure(turn_count)
                        return
                    try:
                        completed_payload = completed_data(event.data, model_reference)
                    except (AttributeError, TypeError, ValueError, OverflowError):
                        yield protocol_failure(turn_count)
                        return
                    executor_terminal = True
                elif event.kind is ExecutorEventKind.FAILED:
                    if executor_terminal:
                        yield protocol_failure(turn_count)
                        return
                    yield EngineEvent(
                        EventKind.FAILED,
                        data={
                            "reason": "engine_failure",
                            "code": executor_failure_code(event.data.get("code")),
                            "retryable": bool(event.data.get("retryable", False)),
                            "turn_count": turn_count,
                        },
                    )
                    return
                else:
                    if executor_terminal:
                        yield protocol_failure(turn_count)
                        return
                    try:
                        mapped = map_executor_event(
                            event,
                            turn_id,
                            agent.id,
                            model_reference,
                        )
                    except (AttributeError, TypeError, ValueError, OverflowError):
                        mapped = None
                    if mapped is None:
                        yield protocol_failure(turn_count)
                        return
                    yield mapped
            if completed_payload is None:
                yield protocol_failure(turn_count)
                return

            content = str(completed_payload.get("content", ""))
            normalized = normalize_output(content)
            if request.policy.stop_on_empty_output and not normalized:
                yield EngineEvent(
                    EventKind.STOPPED,
                    data={"reason": "empty_output", "turn_count": turn_count},
                )
                return
            if request.policy.stop_on_repeated_output and normalized in accepted_outputs:
                yield EngineEvent(
                    EventKind.STOPPED,
                    data={"reason": "duplicate_output", "turn_count": turn_count},
                )
                return
            yield EngineEvent(
                EventKind.AGENT_MESSAGE_COMPLETED,
                turn_id=turn_id,
                agent_id=agent.id,
                data=completed_payload,
            )
            turn_count += 1
            turns_by_agent[agent.id] += 1
            accepted_outputs.add(normalized)
            transcript.append(
                message_snapshot_from_trigger(
                    request, turn_id, agent, content, turn_index, trigger.id
                )
            )

            if request.policy.allow_agent_handoff:
                for target in mentioned_agents(content, request.agents):
                    if target.id == agent.id and not request.policy.allow_self_followup:
                        continue
                    if turns_by_agent[target.id] >= request.policy.max_turns_per_agent:
                        blocked_by_agent_limit = True
                        continue
                    yield EngineEvent(
                        EventKind.HANDOFF_REQUESTED,
                        turn_id=turn_id,
                        agent_id=agent.id,
                        data={
                            "target_agent_id": target.id,
                            "reason_category": "explicit_mention",
                        },
                    )
                    pending.append((target, transcript[-1], "handoff"))

        if blocked_by_agent_limit:
            yield EngineEvent(
                EventKind.STOPPED,
                data={"reason": "max_turns_per_agent", "turn_count": turn_count},
            )
            return
        yield EngineEvent(
            EventKind.COMPLETED,
            data={"reason": "completed", "turn_count": turn_count},
        )
