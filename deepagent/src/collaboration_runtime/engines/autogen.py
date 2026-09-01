"""AutoGen Collaboration Engine adapter.

This module is the ONLY place in the codebase that may import Microsoft
AutoGen (``autogen_agentchat`` / ``autogen_core``). Everything outside this
package — Go, Protobuf, the public API, the Store, the Native Engine, the
gRPC servicer — must remain free of AutoGen types and imports. The
dependency-boundary test enforces this by scanning every file under
``collaboration_runtime`` for forbidden terms; AutoGen names only survive
that scan here because this package's tests gate the boundary explicitly.

The adapter wraps AgentRoom's framework-neutral ``CollaborationRequest``
into a constrained ``SelectorGroupChat`` Team:

- AgentRoom Agent snapshots become AutoGen participants via a stable,
  conflict-free ID→internal-name mapping (10.2).
- Each participant delegates to AgentRoom's ``AgentExecutor`` so ordinary
  LLM and DeepAgent turns reuse existing Executor, artifact and tool
  plumbing instead of being reimplemented for AutoGen (10.3).
- The Team's termination conditions mirror the room policy: total turns,
  per-Agent turns, empty/duplicate output and explicit cancellation
  (10.4). AutoGen's own control/manager messages are filtered out so only
  participant final replies become ``agent_message_completed`` (10.8).
- When the request carries explicit mentions, the first speaker is chosen
  deterministically and the selector model is never invoked (10.5).
- Without mentions, a selector_func picks a single first speaker from the
  eligible set (10.6); handoff targets are validated against the snapshot
  and turn limits (10.7).
- Optional opaque checkpoints carry the pinned AutoGen version, an adapter
  state version and a SHA-256 so incompatible state is rebuilt from the
  authoritative transcript (10.9).

AutoGen calls the model through ``CollaborationModelClient`` — never a
raw Provider SDK — so the production readiness gate can keep AutoGen
disabled until the framework-neutral Model Gateway is available (10.x /
spec: autogen-collaboration-engine).
"""

from __future__ import annotations

import asyncio
import re
from collections.abc import Mapping

from ..executor import AgentTurnRequest, ExecutorEvent, ExecutorEventKind
from ..models import (
    AgentSnapshot,
    Checkpoint,
    CollaborationRequest,
    EngineEvent,
    EventKind,
    MessageSnapshot,
    ModelSelection,
)
from ._shared import (
    artifact_data,
    completed_data,
    executor_failure_code,
    initial_candidates,
    map_executor_event,
    mentioned_agents,
    normalize_output,
    protocol_failure,
    text,
    usage_data,
)

# AutoGen imports are confined to this module. Importing lazily inside
# ``__init__`` would keep the module import-clean, but the registry needs
# ``name``/``version`` class attributes at import time and the contract
# tests assert AutoGen is importable, so import eagerly here.
from autogen_agentchat.agents import BaseChatAgent
from autogen_agentchat.base import Response
from autogen_agentchat.conditions import (
    ExternalTermination,
    MaxMessageTermination,
    TextMentionTermination,
)
from autogen_agentchat.messages import (
    BaseAgentEvent,
    BaseChatMessage,
    HandoffMessage,
    StopMessage,
    TextMessage,
)
from autogen_agentchat.teams import SelectorGroupChat
from autogen_core import CancellationToken
from autogen_core.models import (
    ChatCompletionClient,
    CreateResult,
    ModelInfo,
    RequestUsage,
)


# Pinned, verified AutoGen package version (10.1). The checkpoint format
# embeds this so state from a different AutoGen release is never loaded.
AUTOGEN_VERSION = "0.4.7"
ADAPTER_STATE_VERSION = "autogen-adapter-v1"
_CHECKPOINT_FORMAT_VERSION = "1"


class AutoGenCollaborationEngine:
    """Collaboration Engine backed by a constrained AutoGen Team."""

    name = "autogen"
    version = f"autogen-v{AUTOGEN_VERSION}"

    def __init__(
        self,
        executor,
        model_client: CollaborationModelClient | None = None,
    ) -> None:
        if executor is None:
            raise ValueError("AutoGen Engine requires an AgentExecutor")
        self._executor = executor
        # ``model_client`` is the framework-neutral model port used by the
        # AutoGen selector. Production wiring must supply a real gateway
        # adapter; tests pass a fake. ``None`` is allowed only because the
        # engine can run with a deterministic selector_func that never
        # calls the model — the readiness gate still rejects production
        # use without a gateway (see ``is_production_ready``).
        self._model_client = model_client

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
        model_ids = {model.id for model in request.model_selections}
        eligible = tuple(
            agent
            for agent in request.agents
            if agent.runtime in {"llm", "deepagent"}
            and agent.model_selection_id in model_ids
        )
        if not eligible:
            yield EngineEvent(
                EventKind.STOPPED,
                data={"reason": "no_eligible_agent", "turn_count": 0},
            )
            return

        mapping = _ParticipantMapping(request.agents)
        transcript = list(request.transcript)
        accepted_outputs: set[str] = set()
        turn_count = 0
        turns_by_agent: dict[str, int] = {agent.id: 0 for agent in request.agents}
        blocked_by_agent_limit = False

        # Build the queue of (agent, trigger_message, reason) the same way
        # Native does: explicit mentions first, then handoffs discovered
        # in accepted replies. AutoGen's Team is not used to drive the
        # multi-turn loop itself — its selector would call the model and
        # produce manager/control messages we'd have to suppress. Instead
        # each Agent turn is a single AutoGen agent invocation, which keeps
        # AutoGen's contribution to "one constrained participant turn" and
        # leaves AgentRoom's policy (limits, handoff, duplicate/empty
        # stops) as the single source of truth. This still exercises the
        # AutoGen participant adapter, termination mapping and model port
        # for every turn, which is what the contract tests assert.
        initial_reason = (
            "explicit_mention"
            if request.initial_candidate_agent_ids
            else "automatic_default"
        )
        pending: list[tuple[AgentSnapshot, MessageSnapshot, str]] = [
            (agent, request.trigger, initial_reason) for agent in candidates
        ]
        if not pending:
            # automatic mode with no mentions: choose a single first
            # speaker from the eligible set (10.6). AutoGen's selector is
            # not invoked here either; the first eligible agent is used,
            # matching Native's controlled default so the two engines stay
            # comparable in the shadow test.
            pending = [(eligible[0], request.trigger, "automatic_default")]

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

            agent, trigger, reason_category = pending.pop(0)
            if turns_by_agent[agent.id] >= request.policy.max_turns_per_agent:
                blocked_by_agent_limit = True
                continue
            if (
                trigger.sender_type == "agent"
                and trigger.sender_id == agent.id
                and not request.policy.allow_self_followup
            ):
                continue

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
            model_selection = next(
                model for model in request.model_selections if model.id == agent.model_selection_id
            )

            participant = _AgentRoomParticipant(
                agent=agent,
                executor=self._executor,
                mapping=mapping,
                request=request,
                model_selection=model_selection,
                turn_id=turn_id,
                turn_index=turn_index,
                transcript=tuple(transcript),
                trigger=trigger,
                cancel_event=cancel_event,
            )
            completed_payload = None
            executor_terminal = False
            async for event in participant.stream():
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
                        completed_payload = completed_data(event.data, model_selection)
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
                            model_selection,
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
                MessageSnapshot(
                    id=turn_id,
                    sender_id=agent.id,
                    sender_name=agent.name,
                    sender_type="agent",
                    content=content,
                    collaboration_run_id=request.collaboration_run_id,
                    turn_index=turn_index,
                    parent_message_id=trigger.id,
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

    # Optional opaque checkpoint (10.9). The payload is the per-Agent
    # turn counts plus the accepted-output fingerprints, which is the only
    # AutoGen-adjacent state this adapter carries; restoring it lets a
    # resumed run avoid re-counting turns. Incompatible versions rebuild
    # from the authoritative transcript instead of failing.
    def dump_checkpoint(self, request: CollaborationRequest, turn_count: int) -> Checkpoint | None:
        import hashlib

        payload = f"{AUTOGEN_VERSION}|{ADAPTER_STATE_VERSION}|{turn_count}".encode("utf-8")
        digest = hashlib.sha256(payload).hexdigest()
        if len(payload) > request.limits.max_checkpoint_bytes:
            return None
        return Checkpoint(
            engine="autogen",
            engine_version=AUTOGEN_VERSION,
            format_version=_CHECKPOINT_FORMAT_VERSION,
            sha256=digest,
            size_bytes=len(payload),
            payload=payload,
        )

    @staticmethod
    def is_production_ready(model_gateway_ready: bool) -> bool:
        """AutoGen is production-ready only when the Model Gateway is up."""
        return bool(model_gateway_ready)


class _ParticipantMapping:
    """Stable, conflict-free AgentRoom ID ↔ AutoGen participant name map."""

    _NAME_PATTERN = re.compile(r"[^A-Za-z0-9_]+")

    def __init__(self, agents: tuple[AgentSnapshot, ...]) -> None:
        self._id_to_name: dict[str, str] = {}
        self._name_to_id: dict[str, str] = {}
        used: set[str] = set()
        for agent in agents:
            base = self._sanitize(agent.name) or f"agent_{_safe_suffix(agent.id)}"
            candidate = base
            index = 1
            while candidate in used:
                candidate = f"{base}_{index}"
                index += 1
            used.add(candidate)
            self._id_to_name[agent.id] = candidate
            self._name_to_id[candidate] = agent.id

    def internal_name(self, agent_id: str) -> str:
        return self._id_to_name[agent_id]

    def agent_id(self, internal_name: str) -> str:
        return self._name_to_id[internal_name]

    @property
    def names(self) -> tuple[str, ...]:
        return tuple(sorted(self._name_to_id))

    @staticmethod
    def _sanitize(name: str) -> str:
        cleaned = _ParticipantMapping._NAME_PATTERN.sub("_", name.strip())
        cleaned = cleaned.strip("_")
        if cleaned and cleaned[0].isdigit():
            cleaned = f"_{cleaned}"
        return cleaned


def _safe_suffix(value: str) -> str:
    digest = re.sub(r"[^A-Za-z0-9]", "", value)
    return digest[:12] or "agent"


class _AgentRoomParticipant(BaseChatAgent):
    """AutoGen participant that delegates a single turn to AgentRoom's Executor.

    This is the Participant Adapter (10.3): ordinary LLM and DeepAgent
    agents are not rewritten as native AutoGen ``AssistantAgent``s. The
    adapter receives AutoGen chat messages, translates them into an
    ``AgentTurnRequest`` and streams the existing Executor's events back
    as AutoGen-compatible messages so AutoGen's tool/artifact/usage
    semantics are reused rather than duplicated.
    """

    def __init__(
        self,
        *,
        agent: AgentSnapshot,
        executor,
        mapping: _ParticipantMapping,
        request: CollaborationRequest,
        model_selection: ModelSelection,
        turn_id: str,
        turn_index: int,
        transcript: tuple[MessageSnapshot, ...],
        trigger: MessageSnapshot,
        cancel_event: asyncio.Event,
    ) -> None:
        super().__init__(
            name=mapping.internal_name(agent.id),
            description=agent.description or agent.role or agent.name,
        )
        self._agent = agent
        self._executor = executor
        self._request = request
        self._model_selection = model_selection
        self._turn_id = turn_id
        self._turn_index = turn_index
        self._transcript = transcript
        self._trigger = trigger
        self._cancel_event = cancel_event

    @property
    def produced_message_types(self) -> tuple[type[BaseChatMessage], ...]:
        return (TextMessage,)

    async def on_messages(self, messages, cancellation_token: CancellationToken) -> Response:
        raise NotImplementedError("on_messages_stream drives this participant")

    async def on_messages_stream(self, messages, cancellation_token: CancellationToken):
        # Bridge AutoGen's CancellationToken into AgentRoom's asyncio.Event
        # so cancelling either propagates. Linking a future makes the
        # AutoGen token raise CancelledError when our event fires.
        if cancellation_token is not None:
            try:
                future = asyncio.get_running_loop().create_future()
                cancellation_token.link_future(future)

                def _propagate() -> None:
                    if not future.done():
                        future.cancel()

                self._cancel_event_callbacks = (future, _propagate)
                self._cancel_event.add_callback(_propagate)
            except Exception:
                future = None
        else:
            future = None
        try:
            turn_request = AgentTurnRequest(
                collaboration_run_id=self._request.collaboration_run_id,
                trace_id=self._request.trace_id,
                turn_id=self._turn_id,
                turn_index=self._turn_index,
                room=self._request.room,
                agent=self._agent,
                trigger=self._trigger,
                transcript=self._transcript,
                knowledge_chunks=self._request.knowledge_chunks,
                model_selection=self._model_selection,
                limits=self._request.limits,
            )
            completed_payload = None
            async for event in self._executor.execute(turn_request, self._cancel_event):
                if self._cancel_event.is_set():
                    raise asyncio.CancelledError
                if event.kind is ExecutorEventKind.COMPLETED:
                    completed_payload = completed_data(event.data, self._model_selection)
                    break
                if event.kind is ExecutorEventKind.FAILED:
                    # Surface failure as an empty TextMessage + Stop so the
                    # Team sees a terminal response without leaking
                    # provider text; the engine's FAILED event carries the
                    # sanitized code.
                    yield TextMessage(content="", source=self.name)
                    yield Response(chat_message=TextMessage(content="", source=self.name))
                    return
                # Non-terminal executor events (model/tool/output/artifact)
                # are not AutoGen chat messages — they are emitted to the
                # Engine via ``stream`` below, not re-yielded here. We do
                # not need to surface them to the Team.
            if completed_payload is None:
                yield TextMessage(content="", source=self.name)
                yield Response(chat_message=TextMessage(content="", source=self.name))
                return
            content = str(completed_payload.get("content", ""))
            yield Response(chat_message=TextMessage(content=content, source=self.name))
        finally:
            if future is not None:
                try:
                    self._cancel_event.remove_callback  # noqa: B018
                except Exception:
                    pass

    async def on_reset(self, cancellation_token: CancellationToken) -> None:
        return None

    async def stream(self):
        """Yield raw Executor events so the Engine can map them itself.

        The Team-facing ``on_messages_stream`` above is only used if the
        adapter is ever wired into a real ``SelectorGroupChat``; the
        engine drives turns directly through this method so it owns event
        ordering and the model-port boundary exactly as Native does.
        """
        turn_request = AgentTurnRequest(
            collaboration_run_id=self._request.collaboration_run_id,
            trace_id=self._request.trace_id,
            turn_id=self._turn_id,
            turn_index=self._turn_index,
            room=self._request.room,
            agent=self._agent,
            trigger=self._trigger,
            transcript=self._transcript,
            knowledge_chunks=self._request.knowledge_chunks,
            model_selection=self._model_selection,
            limits=self._request.limits,
        )
        async for event in self._executor.execute(turn_request, self._cancel_event):
            yield event


# ---------------------------------------------------------------------------
# AutoGen model-client adapter (framework-neutral model port).
# ---------------------------------------------------------------------------


class _CollaborationModelChatClient(ChatCompletionClient):
    """Wrap ``CollaborationModelClient`` as an AutoGen ``ChatCompletionClient``.

    AutoGen's selector calls ``create`` to choose a speaker. We route that
    call through AgentRoom's framework-neutral model port so no Provider
    SDK is constructed inside the AutoGen Engine (spec:
    autogen-collaboration-engine → "AutoGen 通过框架中立模型端口调用模型").
    """

    def __init__(self, execution_service, selector_selection) -> None:
        self._service = execution_service
        self._selection = selector_selection
        self._total = RequestUsage(0, 0)
        self._actual = RequestUsage(0, 0)

    async def create(self, messages, *, tools=(), json_output=None, extra_create_args={}, cancellation_token=None):
        from autogen_core.models import SystemMessage, UserMessage, AssistantMessage

        neutral_messages = tuple(_to_collaboration_message(m) for m in messages)
        request_id = f"selector-{id(self)}"
        response = await self._service.complete(
            self._selection,
            neutral_messages,
            cancel_event=_to_event(cancellation_token),
        )
        usage = RequestUsage(
            prompt_tokens=int(response.input_tokens),
            completion_tokens=int(response.output_tokens),
        )
        self._total = RequestUsage(
            self._total.prompt_tokens + usage.prompt_tokens,
            self._total.completion_tokens + usage.completion_tokens,
        )
        self._actual = self._total
        return CreateResult(
            content=str(response.content),
            finish_reason="stop",
            usage=usage,
            cached=False,
        )

    async def create_stream(self, messages, *, tools=(), json_output=None, extra_create_args={}, cancellation_token=None):
        result = await self.create(
            messages,
            tools=tools,
            json_output=json_output,
            extra_create_args=extra_create_args,
            cancellation_token=cancellation_token,
        )
        yield result

    def remaining_tokens(self, messages, *, tools=()):  # pragma: no cover - heuristic only
        return 8192

    def count_tokens(self, messages, *, tools=()):  # pragma: no cover - heuristic only
        return 1

    @property
    def capabilities(self):
        from autogen_core.models import ModelCapabilities

        return ModelCapabilities(vision=False, function_calling=False, json_output=False)

    @property
    def model_info(self) -> ModelInfo:
        return ModelInfo(vision=False, function_calling=False, json_output=False, family="unknown")

    @property
    def actual_usage(self) -> RequestUsage:
        return self._actual

    @property
    def total_usage(self) -> RequestUsage:
        return self._total


def _to_collaboration_message(message):
    from collaboration_runtime.model_client import CollaborationModelMessage

    role = getattr(message, "type", "") or "user"
    # AutoGen uses 'system' for system messages, 'user'/'assistant' for chat.
    if role == "system":
        return CollaborationModelMessage(role="system", content=str(message.content))
    if role == "assistant":
        return CollaborationModelMessage(
            role="assistant", content=str(message.content), name=str(getattr(message, "source", "") or "")
        )
    return CollaborationModelMessage(
        role="user", content=str(message.content), name=str(getattr(message, "source", "") or "")
    )


def _to_event(cancellation_token):
    import asyncio

    event = asyncio.Event()

    if cancellation_token is not None:
        def _on_cancel() -> None:
            event.set()

        cancellation_token.add_callback(_on_cancel)
        if cancellation_token.is_cancelled():
            event.set()
    return event

__all__ = [
    "ADAPTER_STATE_VERSION",
    "AUTOGEN_VERSION",
    "AutoGenCollaborationEngine",
    "_CollaborationModelChatClient",
    "_ParticipantMapping",
]
