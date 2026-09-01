import asyncio
import os
from dataclasses import replace
from pathlib import Path

import pytest

from collaboration_engine_contract import contract_request
from agent_runtime.executors import FakeExecutor
from agent_runtime.registry import ExecutorRegistry
from agent_runtime.v1 import agent_runtime_pb2
from collaboration_runtime.agent_executor import RuntimeRegistryAgentExecutor
from collaboration_runtime.executor import (
    AgentExecutor,
    AgentTurnRequest,
    ExecutorEvent,
    ExecutorEventKind,
)


class RecordingExecutor:
    def __init__(self):
        self.requests = []

    async def execute(self, request, cancel_event):
        self.requests.append((request, cancel_event))
        yield ExecutorEvent(
            ExecutorEventKind.COMPLETED,
            data={"content": f"reply from {request.agent.runtime}"},
        )


def test_runtime_registry_agent_executor_delegates_llm_turn_to_agent_runtime_executor(tmp_path):
    # Set environment credentials for the resolver to find
    os.environ["MODEL_API_KEY"] = "fake-test-key"
    os.environ["MODEL_BASE_URL"] = "https://test.example.com/v1"

    collaboration = contract_request()
    request = AgentTurnRequest(
        collaboration_run_id=collaboration.collaboration_run_id,
        trace_id=collaboration.trace_id,
        turn_id="turn_1",
        turn_index=1,
        room=collaboration.room,
        agent=collaboration.agents[0],
        trigger=collaboration.trigger,
        transcript=collaboration.transcript,
        knowledge_chunks=collaboration.knowledge_chunks,
        model_selection=collaboration.model_selections[0],
        limits=collaboration.limits,
    )
    executor = RuntimeRegistryAgentExecutor(
        ExecutorRegistry([FakeExecutor(agent_runtime_pb2.EXECUTOR_KIND_LLM)]),
        work_dir=tmp_path,
    )

    async def scenario():
        return [event async for event in executor.execute(request, asyncio.Event())]

    events = asyncio.run(scenario())

    assert [event.kind for event in events] == [
        ExecutorEventKind.MODEL_STARTED,
        ExecutorEventKind.MODEL_COMPLETED,
        ExecutorEventKind.COMPLETED,
    ]
    assert events[-1].data["content"] == "fake response"
    assert events[-1].data["model"]["profile_id"] == "profile_contract"


@pytest.mark.parametrize("runtime", ["llm", "deepagent"])
def test_agent_executor_is_shared_by_llm_and_deepagent(runtime):
    collaboration = contract_request()
    agent = replace(collaboration.agents[0], runtime=runtime)
    request = AgentTurnRequest(
        collaboration_run_id=collaboration.collaboration_run_id,
        trace_id=collaboration.trace_id,
        turn_id="turn_1",
        turn_index=1,
        room=collaboration.room,
        agent=agent,
        trigger=collaboration.trigger,
        transcript=collaboration.transcript,
        knowledge_chunks=collaboration.knowledge_chunks,
        model_selection=collaboration.model_selections[0],
        limits=collaboration.limits,
    )
    executor = RecordingExecutor()

    async def scenario():
        cancel_event = asyncio.Event()
        events = [event async for event in executor.execute(request, cancel_event)]
        return cancel_event, events

    cancel_event, events = asyncio.run(scenario())

    assert isinstance(executor, AgentExecutor)
    assert executor.requests == [(request, cancel_event)]
    assert events == [
        ExecutorEvent(
            ExecutorEventKind.COMPLETED,
            data={"content": f"reply from {runtime}"},
        )
    ]


def test_agent_executor_boundary_has_no_transport_or_framework_dependency():
    source = Path(
        __import__(
            "collaboration_runtime.executor",
            fromlist=["AgentExecutor"],
        ).__file__
    ).read_text(encoding="utf-8").lower()

    for forbidden in ("localhost", "grpc", "protobuf", "autogen", "mysql"):
        assert forbidden not in source



# ---------------------------------------------------------------------------
# Task 7: Event semantics, failure mapping, and no-fallback
# ---------------------------------------------------------------------------


def test_preparation_failure_yields_failed_without_model_events(tmp_path):
    """Preparation failure must produce only a FAILED event — no model_started."""
    collaboration = contract_request(credential_ref="profile:nonexistent")
    request = AgentTurnRequest(
        collaboration_run_id=collaboration.collaboration_run_id,
        trace_id=collaboration.trace_id,
        turn_id="turn_prep_fail",
        turn_index=1,
        room=collaboration.room,
        agent=collaboration.agents[0],
        trigger=collaboration.trigger,
        transcript=collaboration.transcript,
        knowledge_chunks=collaboration.knowledge_chunks,
        model_selection=collaboration.model_selections[0],
        limits=collaboration.limits,
    )
    executor = RuntimeRegistryAgentExecutor(
        ExecutorRegistry([FakeExecutor(agent_runtime_pb2.EXECUTOR_KIND_LLM)]),
        work_dir=tmp_path,
    )

    async def scenario():
        return [event async for event in executor.execute(request, asyncio.Event())]

    events = asyncio.run(scenario())
    assert len(events) == 1
    assert events[0].kind == ExecutorEventKind.FAILED
    assert events[0].data["code"] == "model_not_configured"
    assert events[0].data["retryable"] is False


def test_preparation_failure_does_not_leak_credential_ref(tmp_path):
    """FAILED event must not contain credential_ref or provider text."""
    collaboration = contract_request(credential_ref="profile:secret-profile-id")
    request = AgentTurnRequest(
        collaboration_run_id=collaboration.collaboration_run_id,
        trace_id=collaboration.trace_id,
        turn_id="turn_leak_check",
        turn_index=1,
        room=collaboration.room,
        agent=collaboration.agents[0],
        trigger=collaboration.trigger,
        transcript=collaboration.transcript,
        knowledge_chunks=collaboration.knowledge_chunks,
        model_selection=collaboration.model_selections[0],
        limits=collaboration.limits,
    )
    executor = RuntimeRegistryAgentExecutor(
        ExecutorRegistry([FakeExecutor(agent_runtime_pb2.EXECUTOR_KIND_LLM)]),
        work_dir=tmp_path,
    )

    async def scenario():
        return [event async for event in executor.execute(request, asyncio.Event())]

    events = asyncio.run(scenario())
    assert events[0].kind == ExecutorEventKind.FAILED
    serialized = repr(events)
    assert "secret-profile-id" not in serialized
    assert "credential_ref" not in serialized


def test_preparation_failure_maps_credential_not_found_to_model_not_configured(tmp_path):
    """CredentialNotFoundError must map to model_not_configured, retryable=false."""
    from agent_runtime.model_config import ModelConfigPreparationError

    class FailingResolver:
        def resolve(self, **kwargs):
            # ModelConfigResolver wraps CredentialNotFoundError in ModelConfigPreparationError
            raise ModelConfigPreparationError("credential not found")

    collaboration = contract_request()
    request = AgentTurnRequest(
        collaboration_run_id=collaboration.collaboration_run_id,
        trace_id=collaboration.trace_id,
        turn_id="turn_cred_not_found",
        turn_index=1,
        room=collaboration.room,
        agent=collaboration.agents[0],
        trigger=collaboration.trigger,
        transcript=collaboration.transcript,
        knowledge_chunks=collaboration.knowledge_chunks,
        model_selection=collaboration.model_selections[0],
        limits=collaboration.limits,
    )
    executor = RuntimeRegistryAgentExecutor(
        ExecutorRegistry([FakeExecutor(agent_runtime_pb2.EXECUTOR_KIND_LLM)]),
        work_dir=tmp_path,
        config_resolver=FailingResolver(),
    )

    async def scenario():
        return [event async for event in executor.execute(request, asyncio.Event())]

    events = asyncio.run(scenario())
    assert events[0].kind == ExecutorEventKind.FAILED
    assert events[0].data["code"] == "model_not_configured"
    assert events[0].data["retryable"] is False


def test_preparation_failure_maps_access_denied_to_authentication_failed(tmp_path):
    """CredentialAccessDeniedError must map to model_authentication_failed."""
    from agent_runtime.model_config import CredentialAccessDeniedError

    class FailingResolver:
        def resolve(self, **kwargs):
            raise CredentialAccessDeniedError("access denied")

    collaboration = contract_request()
    request = AgentTurnRequest(
        collaboration_run_id=collaboration.collaboration_run_id,
        trace_id=collaboration.trace_id,
        turn_id="turn_access_denied",
        turn_index=1,
        room=collaboration.room,
        agent=collaboration.agents[0],
        trigger=collaboration.trigger,
        transcript=collaboration.transcript,
        knowledge_chunks=collaboration.knowledge_chunks,
        model_selection=collaboration.model_selections[0],
        limits=collaboration.limits,
    )
    executor = RuntimeRegistryAgentExecutor(
        ExecutorRegistry([FakeExecutor(agent_runtime_pb2.EXECUTOR_KIND_LLM)]),
        work_dir=tmp_path,
        config_resolver=FailingResolver(),
    )

    async def scenario():
        return [event async for event in executor.execute(request, asyncio.Event())]

    events = asyncio.run(scenario())
    assert events[0].kind == ExecutorEventKind.FAILED
    assert events[0].data["code"] == "model_authentication_failed"
    assert events[0].data["retryable"] is False


def test_preparation_failure_maps_provider_unavailable_to_engine_unavailable(tmp_path):
    """CredentialProviderUnavailableError must map to engine_unavailable, retryable=true."""
    from agent_runtime.model_config import CredentialProviderUnavailableError

    class FailingResolver:
        def resolve(self, **kwargs):
            raise CredentialProviderUnavailableError("provider down")

    collaboration = contract_request()
    request = AgentTurnRequest(
        collaboration_run_id=collaboration.collaboration_run_id,
        trace_id=collaboration.trace_id,
        turn_id="turn_provider_unavail",
        turn_index=1,
        room=collaboration.room,
        agent=collaboration.agents[0],
        trigger=collaboration.trigger,
        transcript=collaboration.transcript,
        knowledge_chunks=collaboration.knowledge_chunks,
        model_selection=collaboration.model_selections[0],
        limits=collaboration.limits,
    )
    executor = RuntimeRegistryAgentExecutor(
        ExecutorRegistry([FakeExecutor(agent_runtime_pb2.EXECUTOR_KIND_LLM)]),
        work_dir=tmp_path,
        config_resolver=FailingResolver(),
    )

    async def scenario():
        return [event async for event in executor.execute(request, asyncio.Event())]

    events = asyncio.run(scenario())
    assert events[0].kind == ExecutorEventKind.FAILED
    assert events[0].data["code"] == "engine_unavailable"
    assert events[0].data["retryable"] is True
