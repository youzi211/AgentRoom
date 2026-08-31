import asyncio
from dataclasses import replace
from pathlib import Path
from time import monotonic

import pytest

from collaboration_engine_contract import assert_engine_contract, contract_request
from collaboration_runtime.executor import ExecutorEvent, ExecutorEventKind
from collaboration_runtime.engines import NativeCollaborationEngine
from collaboration_runtime.models import AgentSnapshot, ModelReference


def test_native_engine_passes_shared_collaboration_contract():
    assert_engine_contract(NativeCollaborationEngine)


def test_native_engine_has_no_framework_transport_or_database_dependency():
    source = Path(
        __import__(
            "collaboration_runtime.engines.native",
            fromlist=["NativeCollaborationEngine"],
        ).__file__
    ).read_text(encoding="utf-8").lower()

    for forbidden in ("autogen", "grpc", "protobuf", "mysql", "openai", "anthropic"):
        assert forbidden not in source


def test_native_engine_preserves_deduplicated_eligible_initial_candidate_order():
    base = contract_request()
    second = replace(
        base.agents[0],
        id="agent_second",
        name="Second",
        mention="@Second",
        model_reference_id="model_second",
    )
    unsupported = AgentSnapshot(
        id="agent_unsupported",
        name="Unsupported",
        mention="@Unsupported",
        role="assistant",
        description="Unsupported runtime",
        system_prompt="Respond",
        runtime="unknown",
        model_reference_id="model_contract",
    )
    request = replace(
        base,
        agents=(base.agents[0], second, unsupported),
        model_references=(
            *base.model_references,
            ModelReference(
                id="model_second",
                profile_id="profile_second",
                source="test",
                protocol="fake",
                model_name="fake-second",
                runtime_scope="collaboration",
            ),
        ),
        initial_candidate_agent_ids=(
            "agent_second",
            "agent_contract",
            "agent_second",
            "missing",
            "agent_unsupported",
        ),
    )

    class Executor:
        def __init__(self):
            self.agent_ids = []

        async def execute(self, turn, _cancel_event):
            self.agent_ids.append(turn.agent.id)
            yield ExecutorEvent(
                ExecutorEventKind.COMPLETED,
                data={"content": f"reply from {turn.agent.id}"},
            )

    executor = Executor()

    async def scenario():
        return [
            event
            async for event in NativeCollaborationEngine(executor).execute(
                request,
                asyncio.Event(),
            )
        ]

    events = asyncio.run(scenario())

    selected = [event.agent_id for event in events if event.kind == "speaker_selected"]
    completed = [event.agent_id for event in events if event.kind == "agent_message_completed"]
    assert selected == ["agent_second", "agent_contract"]
    assert completed == selected
    assert executor.agent_ids == selected
    assert events[-1].kind == "completed"
    assert events[-1].data["turn_count"] == 2


def test_native_engine_follows_handoffs_with_self_and_turn_limits():
    base = contract_request()
    reviewer = replace(
        base.agents[0], id="reviewer", name="Reviewer", mention="@Reviewer"
    )
    author = replace(base.agents[0], id="author", name="Author", mention="@Author")
    request = replace(
        base,
        agents=(author, reviewer),
        initial_candidate_agent_ids=("author",),
        policy=replace(
            base.policy,
            max_turns=3,
            max_turns_per_agent=2,
            allow_agent_handoff=True,
            allow_self_followup=False,
        ),
    )

    class Executor:
        responses = {
            ("author", 1): "@Author @Reviewer please review.",
            ("reviewer", 1): "@Author revised concerns.",
            ("author", 2): "Done.",
        }

        async def execute(self, turn, _cancel_event):
            count = sum(
                message.sender_id == turn.agent.id
                for message in turn.transcript
                if message.sender_type == "agent"
            )
            yield ExecutorEvent(
                ExecutorEventKind.COMPLETED,
                data={"content": self.responses[(turn.agent.id, count + 1)]},
            )

    async def scenario():
        return [
            event
            async for event in NativeCollaborationEngine(Executor()).execute(
                request, asyncio.Event()
            )
        ]

    events = asyncio.run(scenario())
    selected = [event.agent_id for event in events if event.kind == "speaker_selected"]
    handoffs = [
        event.data["target_agent_id"]
        for event in events
        if event.kind == "handoff_requested"
    ]
    assert selected == ["author", "reviewer", "author"]
    assert handoffs == ["reviewer", "author"]
    assert events[-1].kind == "completed"
    assert events[-1].data["turn_count"] == 3


def test_native_engine_stops_at_per_agent_and_total_turn_limits():
    base = contract_request()
    reviewer = replace(
        base.agents[0], id="reviewer", name="Reviewer", mention="@Reviewer"
    )
    author = replace(base.agents[0], id="author", name="Author", mention="@Author")

    class AlternatingExecutor:
        async def execute(self, turn, _cancel_event):
            target = "@Reviewer" if turn.agent.id == "author" else "@Author"
            yield ExecutorEvent(
                ExecutorEventKind.COMPLETED,
                data={"content": f"{target} follow up from {turn.agent.id}"},
            )

    async def collect(request):
        return [
            event
            async for event in NativeCollaborationEngine(AlternatingExecutor()).execute(
                request, asyncio.Event()
            )
        ]

    per_agent = replace(
        base,
        agents=(author, reviewer),
        initial_candidate_agent_ids=("author",),
        policy=replace(base.policy, max_turns=4, max_turns_per_agent=1),
    )
    per_agent_events = asyncio.run(collect(per_agent))
    assert [
        event.agent_id for event in per_agent_events if event.kind == "speaker_selected"
    ] == ["author", "reviewer"]
    assert per_agent_events[-1].kind == "stopped"
    assert per_agent_events[-1].data["reason"] == "max_turns_per_agent"

    total = replace(
        per_agent,
        policy=replace(per_agent.policy, max_turns=2, max_turns_per_agent=2),
    )
    total_events = asyncio.run(collect(total))
    assert total_events[-1].kind == "stopped"
    assert total_events[-1].data == {"reason": "max_turns", "turn_count": 2}


def test_native_engine_allows_explicit_self_followup_when_enabled():
    base = contract_request()
    request = replace(
        base,
        policy=replace(
            base.policy,
            max_turns=2,
            max_turns_per_agent=2,
            allow_self_followup=True,
        ),
    )

    class Executor:
        async def execute(self, turn, _cancel_event):
            content = turn.agent.mention if turn.turn_index == 1 else "Done."
            yield ExecutorEvent(ExecutorEventKind.COMPLETED, data={"content": content})

    async def scenario():
        return [
            event
            async for event in NativeCollaborationEngine(Executor()).execute(
                request, asyncio.Event()
            )
        ]

    events = asyncio.run(scenario())
    assert [event.agent_id for event in events if event.kind == "speaker_selected"] == [
        "agent_contract",
        "agent_contract",
    ]
    assert events[-1].kind == "completed"


def test_native_engine_applies_cooldown_between_turns():
    base = contract_request()
    reviewer = replace(
        base.agents[0], id="reviewer", name="Reviewer", mention="@Reviewer"
    )
    request = replace(
        base,
        agents=(base.agents[0], reviewer),
        initial_candidate_agent_ids=(base.agents[0].id, reviewer.id),
        policy=replace(base.policy, max_turns=2, cooldown_seconds=0.02),
    )
    starts = []

    class Executor:
        async def execute(self, _turn, _cancel_event):
            starts.append(monotonic())
            yield ExecutorEvent(ExecutorEventKind.COMPLETED, data={"content": "ok"})

    async def scenario():
        return [
            event
            async for event in NativeCollaborationEngine(Executor()).execute(
                request, asyncio.Event()
            )
        ]

    events = asyncio.run(scenario())
    assert starts[1] - starts[0] >= 0.015
    assert events[-1].kind == "stopped"
    assert events[-1].data["reason"] == "duplicate_output"


def test_native_engine_stops_empty_output_without_completing_message():
    base = contract_request()

    class Executor:
        async def execute(self, _turn, _cancel_event):
            yield ExecutorEvent(ExecutorEventKind.COMPLETED, data={"content": " \n\t "})

    async def scenario():
        return [
            event
            async for event in NativeCollaborationEngine(Executor()).execute(
                base, asyncio.Event()
            )
        ]

    events = asyncio.run(scenario())
    assert all(event.kind != "agent_message_completed" for event in events)
    assert events[-1].kind == "stopped"
    assert events[-1].data == {"reason": "empty_output", "turn_count": 0}


def test_native_engine_stops_normalized_duplicate_output():
    base = contract_request()
    reviewer = replace(
        base.agents[0], id="reviewer", name="Reviewer", mention="@Reviewer"
    )
    request = replace(
        base,
        agents=(base.agents[0], reviewer),
        initial_candidate_agent_ids=(base.agents[0].id, reviewer.id),
        policy=replace(base.policy, max_turns=2),
    )
    responses = iter(("Same answer", "  SAME\nanswer "))

    class Executor:
        async def execute(self, _turn, _cancel_event):
            yield ExecutorEvent(
                ExecutorEventKind.COMPLETED,
                data={"content": next(responses)},
            )

    async def scenario():
        return [
            event
            async for event in NativeCollaborationEngine(Executor()).execute(
                request, asyncio.Event()
            )
        ]

    events = asyncio.run(scenario())
    assert sum(event.kind == "agent_message_completed" for event in events) == 1
    assert events[-1].kind == "stopped"
    assert events[-1].data == {"reason": "duplicate_output", "turn_count": 1}


def test_native_engine_automatic_mode_selects_only_first_eligible_default():
    base = contract_request()
    unsupported = replace(
        base.agents[0],
        id="unsupported",
        name="Unsupported",
        runtime="unknown",
    )
    first = replace(base.agents[0], id="first", name="First")
    second = replace(base.agents[0], id="second", name="Second")
    request = replace(
        base,
        agents=(unsupported, first, second),
        initial_candidate_agent_ids=(),
        policy=replace(base.policy, trigger_mode="automatic"),
    )
    executed = []

    class Executor:
        async def execute(self, turn, _cancel_event):
            executed.append(turn.agent.id)
            yield ExecutorEvent(ExecutorEventKind.COMPLETED, data={"content": "reply"})

    async def scenario():
        return [
            event
            async for event in NativeCollaborationEngine(Executor()).execute(
                request, asyncio.Event()
            )
        ]

    events = asyncio.run(scenario())
    selected = [event for event in events if event.kind == "speaker_selected"]
    assert executed == ["first"]
    assert [event.agent_id for event in selected] == ["first"]
    assert selected[0].data["reason_category"] == "automatic_default"
    assert events[-1].kind == "completed"


def test_native_engine_mention_only_without_candidates_does_not_execute():
    base = replace(contract_request(), initial_candidate_agent_ids=())

    class Executor:
        async def execute(self, _turn, _cancel_event):
            raise AssertionError("mention_only must not execute without a candidate")
            yield

    async def scenario():
        return [
            event
            async for event in NativeCollaborationEngine(Executor()).execute(
                base, asyncio.Event()
            )
        ]

    events = asyncio.run(scenario())
    assert events[-1].kind == "stopped"
    assert events[-1].data["reason"] == "no_eligible_agent"


def test_native_engine_maps_executor_activity_and_completed_message_data():
    request = contract_request()
    artifact = {
        "id": "artifact_1",
        "type": "document",
        "title": "Review",
        "file_name": "review.txt",
        "mime_type": "text/plain",
        "content": b"review content",
        "external_uri": "",
    }

    class Executor:
        async def execute(self, _turn, _cancel_event):
            yield ExecutorEvent(
                ExecutorEventKind.MODEL_STARTED,
                data={"model_reference_id": "spoofed_model"},
            )
            yield ExecutorEvent(
                ExecutorEventKind.TOOL_STARTED,
                data={
                    "tool_call_id": "tool_1",
                    "tool_name": "search",
                    "input_summary": "query",
                },
            )
            yield ExecutorEvent(
                ExecutorEventKind.TOOL_COMPLETED,
                data={
                    "tool_call_id": "tool_1",
                    "tool_name": "search",
                    "output_summary": "one result",
                },
            )
            yield ExecutorEvent(
                ExecutorEventKind.TOOL_FAILED,
                data={
                    "tool_call_id": "tool_2",
                    "tool_name": "fetch",
                    "code": "provider said secret-token",
                    "message": "secret-token",
                    "retryable": True,
                },
            )
            yield ExecutorEvent(
                ExecutorEventKind.OUTPUT_DELTA,
                data={"text": "final "},
            )
            yield ExecutorEvent(
                ExecutorEventKind.ARTIFACT_READY,
                data={"artifact": artifact},
            )
            yield ExecutorEvent(
                ExecutorEventKind.MODEL_COMPLETED,
                data={
                    "model_reference_id": "spoofed_model",
                    "usage": {
                        "input_tokens": 11,
                        "output_tokens": 7,
                        "total_tokens": 18,
                    },
                },
            )
            yield ExecutorEvent(
                ExecutorEventKind.COMPLETED,
                data={
                    "content": "final answer",
                    "artifacts": (artifact,),
                    "knowledge_sources": (
                        {
                            "document_id": "doc_1",
                            "document_name": "Plan.md",
                            "scope": "room",
                        },
                    ),
                    "model": {
                        "model_reference_id": "spoofed_model",
                        "profile_id": "spoofed_profile",
                        "source": "spoofed_source",
                        "model_name": "spoofed_name",
                    },
                    "usage": {
                        "input_tokens": 11,
                        "output_tokens": 7,
                        "total_tokens": 18,
                    },
                },
            )

    async def scenario():
        return [
            event
            async for event in NativeCollaborationEngine(Executor()).execute(
                request, asyncio.Event()
            )
        ]

    events = asyncio.run(scenario())
    assert [event.kind for event in events] == [
        "collaboration_started",
        "speaker_selected",
        "agent_turn_started",
        "model_started",
        "tool_started",
        "tool_completed",
        "tool_failed",
        "output_delta",
        "artifact_ready",
        "model_completed",
        "agent_message_completed",
        "completed",
    ]
    turn_events = events[1:-1]
    assert {event.turn_id for event in turn_events} == {
        "collaboration_contract:turn:1"
    }
    assert {event.agent_id for event in turn_events} == {"agent_contract"}
    assert events[3].data == {"model_reference_id": "model_contract"}
    assert events[6].data == {
        "tool_call_id": "tool_2",
        "tool_name": "fetch",
        "code": "tool_failed",
        "retryable": True,
    }
    assert events[9].data == {
        "model_reference_id": "model_contract",
        "usage": {"input_tokens": 11, "output_tokens": 7, "total_tokens": 18},
    }
    assert events[10].data == {
        "content": "final answer",
        "artifacts": (artifact,),
        "knowledge_sources": (
            {
                "document_id": "doc_1",
                "document_name": "Plan.md",
                "scope": "room",
            },
        ),
        "model": {
            "model_reference_id": "model_contract",
            "profile_id": "profile_contract",
            "source": "test",
            "model_name": "fake-model",
        },
        "usage": {"input_tokens": 11, "output_tokens": 7, "total_tokens": 18},
    }
    assert "secret-token" not in repr(events)


def test_native_engine_does_not_expose_executor_failure_text():
    class Executor:
        async def execute(self, _turn, _cancel_event):
            yield ExecutorEvent(
                ExecutorEventKind.FAILED,
                data={
                    "code": "Authorization: Bearer secret-token",
                    "message": "provider response secret-token",
                    "retryable": True,
                },
            )

    async def scenario():
        return [
            event
            async for event in NativeCollaborationEngine(Executor()).execute(
                contract_request(), asyncio.Event()
            )
        ]

    events = asyncio.run(scenario())
    assert events[-1].kind == "failed"
    assert events[-1].data == {
        "reason": "engine_failure",
        "code": "internal",
        "retryable": True,
        "turn_count": 0,
    }
    assert "secret-token" not in repr(events)


@pytest.mark.parametrize(
    "trailing_event",
    [
        ExecutorEvent(ExecutorEventKind.OUTPUT_DELTA, data={"text": "late"}),
        ExecutorEvent(ExecutorEventKind.COMPLETED, data={"content": "duplicate"}),
    ],
)
def test_native_engine_rejects_executor_events_after_terminal(trailing_event):
    class Executor:
        async def execute(self, _turn, _cancel_event):
            yield ExecutorEvent(ExecutorEventKind.COMPLETED, data={"content": "ok"})
            yield trailing_event

    async def scenario():
        return [
            event
            async for event in NativeCollaborationEngine(Executor()).execute(
                contract_request(), asyncio.Event()
            )
        ]

    events = asyncio.run(scenario())
    terminal = [
        event
        for event in events
        if event.kind in {"completed", "stopped", "cancelled", "failed"}
    ]
    assert terminal == [events[-1]]
    assert events[-1].kind == "failed"
    assert events[-1].data == {
        "reason": "protocol_error",
        "code": "protocol_error",
        "turn_count": 0,
    }
    assert all(event.kind != "agent_message_completed" for event in events)
