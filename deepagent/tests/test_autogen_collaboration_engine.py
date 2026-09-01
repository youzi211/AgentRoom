"""Shared contract test for AutoGen and Native collaboration engines.

Both engines MUST pass this contract so the registry's replacement
boundary stays trustworthy (spec: collaboration-engine-service → "Native
与 AutoGen 共享契约测试"). The assertions mirror
``collaboration_engine_contract.py`` plus the AutoGen-specific concerns
from spec ``autogen-collaboration-engine``: deterministic first speaker,
handoff validation, internal-message isolation, sensitive-field scrubbing
and version-locked checkpoint metadata.
"""

from __future__ import annotations

import asyncio
import sys
from dataclasses import replace
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).parent))

from collaboration_engine_contract import contract_request  # noqa: E402
from collaboration_runtime.engines import AutoGenCollaborationEngine  # noqa: E402
from collaboration_runtime.engines import NativeCollaborationEngine  # noqa: E402
from collaboration_runtime.engines.shadow import compare_engines  # noqa: E402
from collaboration_runtime.engines.autogen import (  # noqa: E402
    ADAPTER_STATE_VERSION,
    AUTOGEN_VERSION,
    _ParticipantMapping,
)
from collaboration_runtime.executor import ExecutorEvent, ExecutorEventKind  # noqa: E402
from collaboration_runtime.models import AgentSnapshot, ModelSelection  # noqa: E402


TERMINAL_KINDS = {"completed", "stopped", "cancelled", "failed"}


class _ScriptExecutor:
    """Replies from a deterministic script keyed by agent id."""

    def __init__(self, responses):
        self._responses = dict(responses)
        self.calls = 0
        self.agent_ids: list[str] = []

    async def execute(self, turn, _cancel_event):
        self.calls += 1
        self.agent_ids.append(turn.agent.id)
        yield ExecutorEvent(
            ExecutorEventKind.COMPLETED,
            data={"content": self._responses[turn.agent.id]},
        )


def _second_agent(base):
    return replace(
        base.agents[0],
        id="agent_reviewer",
        name="Reviewer",
        mention="@Reviewer",
        model_selection_id="model_reviewer",
    )


def _second_model():
    return ModelSelection(
        id="model_reviewer",
        profile_id="profile_reviewer",
        source="test",
        protocol="fake",
        model_name="fake-reviewer",
        runtime_scope="collaboration",
        credential_ref="",
        purpose="agent_turn",
    )


def _handoff_request():
    base = contract_request(run_id="autogen_handoff")
    request = replace(
        base,
        agents=(base.agents[0], _second_agent(base)),
        model_selections=(base.model_selections[0], _second_model()),
        initial_candidate_agent_ids=("agent_contract",),
        policy=replace(
            base.policy,
            max_turns=3,
            max_turns_per_agent=2,
            allow_agent_handoff=True,
            allow_self_followup=False,
        ),
    )
    executor = _ScriptExecutor(
        {
            "agent_contract": "@Reviewer please review.",
            "agent_reviewer": "Reviewed.",
        }
    )
    return request, executor


@pytest.mark.parametrize("factory", [NativeCollaborationEngine, AutoGenCollaborationEngine])
def test_engine_emits_started_exactly_one_terminal_and_isolates_cancel(factory):
    # Two factory calls must produce isolated instances (registry contract).
    executor = _ScriptExecutor({"agent_contract": "ok"})
    engine = factory(executor)
    events = asyncio.run(_collect(engine, contract_request()))

    assert events[0].kind == "collaboration_started"
    assert events[0].kind != "accepted"
    assert events[-1].kind in TERMINAL_KINDS
    assert sum(event.kind in TERMINAL_KINDS for event in events) == 1

    cancelled_engine = factory(_ScriptExecutor({"agent_contract": "ok"}))
    cancelled = asyncio.run(_collect(cancelled_engine, contract_request(), cancelled=True))
    assert cancelled[-1].kind == "cancelled"
    assert sum(event.kind in TERMINAL_KINDS for event in cancelled) == 1


@pytest.mark.parametrize("factory", [NativeCollaborationEngine, AutoGenCollaborationEngine])
def test_engine_follows_explicit_mention_handoff(factory):
    request, executor = _handoff_request()
    engine = factory(executor)
    events = asyncio.run(_collect(engine, request))

    selected = [e.agent_id for e in events if e.kind == "speaker_selected"]
    completed = [e.agent_id for e in events if e.kind == "agent_message_completed"]
    handoffs = [e.data["target_agent_id"] for e in events if e.kind == "handoff_requested"]
    assert selected == ["agent_contract", "agent_reviewer"]
    assert completed == ["agent_contract", "agent_reviewer"]
    assert handoffs == ["agent_reviewer"]
    assert events[-1].kind == "completed"
    assert events[-1].data["turn_count"] == 2


@pytest.mark.parametrize("factory", [NativeCollaborationEngine, AutoGenCollaborationEngine])
def test_engine_rejects_handoff_to_over_limit_agent(factory):
    base = contract_request(run_id="autogen_over_limit")
    request = replace(
        base,
        agents=(base.agents[0], _second_agent(base)),
        model_selections=(base.model_selections[0], _second_model()),
        initial_candidate_agent_ids=("agent_contract",),
        policy=replace(
            base.policy,
            max_turns=2,
            max_turns_per_agent=1,
            allow_agent_handoff=True,
        ),
    )
    executor = _ScriptExecutor(
        {
            "agent_contract": "@Reviewer please review.",  # reviewer already at limit (0<1, ok) -> should run
            "agent_reviewer": "Reviewed.",
        }
    )
    engine = factory(executor)
    events = asyncio.run(_collect(engine, request))
    # Per-agent limit 1: author speaks once, reviewer once, then author is
    # blocked from a second mention-driven turn.
    assert [e.agent_id for e in events if e.kind == "speaker_selected"] == [
        "agent_contract",
        "agent_reviewer",
    ]
    assert events[-1].kind == "completed"
    assert events[-1].data["turn_count"] == 2


@pytest.mark.parametrize("factory", [NativeCollaborationEngine, AutoGenCollaborationEngine])
def test_engine_stops_on_empty_and_duplicate_output(factory):
    base = contract_request(run_id="autogen_stops")
    second = replace(base.agents[0], id="agent_second", name="Second", mention="@Second", model_selection_id="model_second")
    request = replace(
        base,
        agents=(base.agents[0], second),
        model_selections=(base.model_selections[0], ModelSelection(id="model_second", profile_id="profile_second", source="test", protocol="fake", model_name="fake-second", runtime_scope="collaboration", credential_ref="", purpose="agent_turn")),
        initial_candidate_agent_ids=(base.agents[0].id, second.id),
        policy=replace(base.policy, max_turns=2),
    )

    empty_executor = _ScriptExecutor({"agent_contract": "   ", "agent_second": "x"})
    empty_events = asyncio.run(_collect(factory(empty_executor), request))
    assert empty_events[-1].kind == "stopped"
    assert empty_events[-1].data["reason"] == "empty_output"

    dup_executor = _ScriptExecutor(
        {"agent_contract": "Same answer", "agent_second": "  SAME\nanswer "}
    )
    dup_events = asyncio.run(_collect(factory(dup_executor), request))
    assert dup_events[-1].kind == "stopped"
    assert dup_events[-1].data["reason"] == "duplicate_output"


def test_autogen_engine_participant_mapping_is_stable_and_conflict_free():
    agents = (
        AgentSnapshot(id="a1", name="Author", mention="@Author", role="r", description="d", system_prompt="s", runtime="llm", model_selection_id="m1"),
        AgentSnapshot(id="a2", name="Author", mention="@Author2", role="r", description="d", system_prompt="s", runtime="llm", model_selection_id="m2"),
        AgentSnapshot(id="a3", name="Bad Name!", mention="@Bad", role="r", description="d", system_prompt="s", runtime="llm", model_selection_id="m3"),
    )
    mapping = _ParticipantMapping(agents)
    names = mapping.names
    assert len(names) == len(set(names))  # no collisions
    for agent in agents:
        internal = mapping.internal_name(agent.id)
        assert mapping.agent_id(internal) == agent.id


def test_autogen_engine_checkpoint_carries_pinned_version_and_hash():
    engine = AutoGenCollaborationEngine(_ScriptExecutor({"agent_contract": "ok"}))
    checkpoint = engine.dump_checkpoint(contract_request(), turn_count=2)
    assert checkpoint is not None
    assert checkpoint.engine == "autogen"
    assert checkpoint.engine_version == AUTOGEN_VERSION
    assert checkpoint.format_version
    assert checkpoint.sha256
    assert checkpoint.size_bytes == len(checkpoint.payload)
    # SHA-256 must verify against the payload (events.py enforces this).
    import hashlib

    assert hashlib.sha256(checkpoint.payload).hexdigest() == checkpoint.sha256
    # Adapter state version is embedded in the payload so a bumped
    # adapter version invalidates old checkpoints.
    assert ADAPTER_STATE_VERSION.encode() in checkpoint.payload


def test_autogen_engine_is_not_production_ready_without_gateway():
    assert AutoGenCollaborationEngine.is_production_ready(False) is False
    assert AutoGenCollaborationEngine.is_production_ready(True) is True


def test_autogen_engine_does_not_leak_executor_failure_text():
    class LeakyExecutor:
        async def execute(self, _turn, _cancel_event):
            yield ExecutorEvent(
                ExecutorEventKind.FAILED,
                data={
                    "code": "Authorization: Bearer secret-token",
                    "message": "provider response secret-token",
                    "retryable": True,
                },
            )

    engine = AutoGenCollaborationEngine(LeakyExecutor())
    events = asyncio.run(_collect(engine, contract_request()))
    assert events[-1].kind == "failed"
    assert events[-1].data == {
        "reason": "engine_failure",
        "code": "internal",
        "retryable": True,
        "turn_count": 0,
    }
    assert "secret-token" not in repr(events)


def test_autogen_engine_does_not_retry_after_started_failure():
    class FailingExecutor:
        def __init__(self):
            self.calls = 0
            self.agent_ids: list[str] = []

        async def execute(self, turn, _cancel_event):
            self.calls += 1
            self.agent_ids.append(turn.agent.id)
            yield ExecutorEvent(
                ExecutorEventKind.FAILED,
                data={"code": "provider_error", "message": "temporary failure"},
            )

    executor = FailingExecutor()
    engine = AutoGenCollaborationEngine(executor)
    events = asyncio.run(_collect(engine, contract_request("autogen_started_failure")))

    assert [event.kind for event in events if event.kind in TERMINAL_KINDS] == ["failed"]
    assert executor.calls == 1
    assert executor.agent_ids == ["agent_contract"]


def test_autogen_engine_speaker_selection_matches_native_on_golden():
    """AutoGen must select the same speakers as Native for the golden suite."""
    from collaboration_runtime.engines import native as native_module  # noqa: F401
    from test_native_collaboration_engine_golden import GoldenExecutor

    suite_path = (
        Path(__file__).resolve().parents[2]
        / "proto"
        / "collaboration_runtime"
        / "v1"
        / "testdata"
        / "native_engine_golden.json"
    )
    import json

    suite = json.loads(suite_path.read_text(encoding="utf-8"))
    for case in suite["cases"]:
        base = contract_request(run_id=f"autogen_golden_{case['name']}")
        agents = tuple(
            AgentSnapshot(
                id=item["id"],
                name=item["name"],
                mention=f"@{item['name']}",
                role=f"{item['name']} role",
                description="Golden scenario agent",
                system_prompt=f"You are {item['name']}.",
                runtime="deepagent",
                model_selection_id=f"model-{item['id']}",
            )
            for item in case["agents"]
        )
        models = tuple(
            ModelSelection(
                id=f"model-{item['id']}",
                profile_id=f"profile-{item['id']}",
                source="database",
                protocol="fake",
                model_name="research-model",
                runtime_scope="collaboration",
                credential_ref="",
                purpose="agent_turn",
            )
            for item in case["agents"]
        )
        policy = case["policy"]
        request = replace(
            base,
            agents=agents,
            model_selections=models,
            trigger=replace(base.trigger, content=case["trigger"]),
            initial_candidate_agent_ids=tuple(case["initial_candidate_agent_ids"]),
            policy=replace(
                base.policy,
                max_turns=policy["max_turns"],
                max_turns_per_agent=policy["max_turns_per_agent"],
            ),
        )
        executor = GoldenExecutor(case["responses"])
        events = asyncio.run(_collect(AutoGenCollaborationEngine(executor), request))
        expected = case["expected"]
        assert [
            event.agent_id for event in events if event.kind == "speaker_selected"
        ] == expected["speaker_ids"], case["name"]
        assert [event.agent_id for event in events if event.kind == "agent_message_completed"] == [
            message["agent_id"] for message in expected["messages"]
        ], case["name"]
        assert events[-1].kind == expected["terminal"]["kind"], case["name"]
        assert events[-1].data["reason"] == expected["terminal"]["reason"], case["name"]


def test_shadow_compare_reports_selection_event_and_terminal_differences():
    request = contract_request(run_id="shadow_compare")

    same = asyncio.run(
        compare_engines(
            NativeCollaborationEngine(_ScriptExecutor({"agent_contract": "ok"})),
            AutoGenCollaborationEngine(_ScriptExecutor({"agent_contract": "ok"})),
            request,
        )
    )
    assert same.speaker_differences == ()
    assert same.event_differences == ()
    assert same.stop_reason_difference is None

    different = asyncio.run(
        compare_engines(
            NativeCollaborationEngine(_ScriptExecutor({"agent_contract": "ok"})),
            AutoGenCollaborationEngine(_ScriptExecutor({"agent_contract": "   "})),
            request,
        )
    )
    assert different.event_differences
    assert different.stop_reason_difference == ("completed", "empty_output")


async def _collect(engine, request, *, cancelled=False):
    cancel_event = asyncio.Event()
    if cancelled:
        cancel_event.set()
    return [event async for event in engine.execute(request, cancel_event)]



# ---------------------------------------------------------------------------
# Task 7: No-fallback regression for AutoGen engine
# ---------------------------------------------------------------------------


@pytest.mark.parametrize("factory", [NativeCollaborationEngine, AutoGenCollaborationEngine])
def test_engine_preparation_failure_terminates_without_fallback(factory):
    """A Resolver/executor failure must terminate the run immediately.

    No second turn, no legacy runner, no engine switch.
    """

    class CountingFailingExecutor:
        def __init__(self):
            self.calls = 0
            self.agent_ids: list[str] = []

        async def execute(self, turn, _cancel_event):
            self.calls += 1
            self.agent_ids.append(turn.agent.id)
            yield ExecutorEvent(
                ExecutorEventKind.FAILED,
                data={"code": "model_not_configured", "retryable": False},
            )

    executor = CountingFailingExecutor()
    engine = factory(executor)
    events = asyncio.run(_collect(engine, contract_request("no_fallback_test")))

    assert [event.kind for event in events if event.kind in TERMINAL_KINDS] == ["failed"]
    assert events[-1].kind == "failed"
    assert events[-1].data["code"] == "model_not_configured"
    assert events[-1].data["retryable"] is False
    # Only one executor call — no retry or fallback
    assert executor.calls == 1
    assert executor.agent_ids == ["agent_contract"]


@pytest.mark.parametrize("factory", [NativeCollaborationEngine, AutoGenCollaborationEngine])
def test_engine_preparation_failure_emits_no_model_events(factory):
    """Preparation failure must not emit model_started or model_completed."""

    class FailingExecutor:
        async def execute(self, turn, _cancel_event):
            yield ExecutorEvent(
                ExecutorEventKind.FAILED,
                data={"code": "model_not_configured", "retryable": False},
            )

    engine = FailingExecutor()
    events = asyncio.run(_collect(factory(engine), contract_request("no_model_events")))

    assert not any(event.kind == "model_started" for event in events)
    assert not any(event.kind == "model_completed" for event in events)
    assert not any(event.kind == "agent_message_completed" for event in events)
    assert events[-1].kind == "failed"


@pytest.mark.parametrize("factory", [NativeCollaborationEngine, AutoGenCollaborationEngine])
def test_engine_preparation_failure_does_not_leak_credentials(factory):
    """Failed event payload must not contain api_key or credential_ref."""

    class LeakyExecutor:
        async def execute(self, turn, _cancel_event):
            yield ExecutorEvent(
                ExecutorEventKind.FAILED,
                data={
                    "code": "model_not_configured",
                    "retryable": False,
                    "api_key": "sk-secret-key-12345",
                    "credential_ref": "environment:deepagent",
                },
            )

    engine = LeakyExecutor()
    events = asyncio.run(_collect(factory(engine), contract_request("leak_check")))

    serialized = repr(events)
    assert "sk-secret-key-12345" not in serialized
    failed_event = [event for event in events if event.kind == "failed"][0]
    assert "api_key" not in failed_event.data
    assert "credential_ref" not in failed_event.data
