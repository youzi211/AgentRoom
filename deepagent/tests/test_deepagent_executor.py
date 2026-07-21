from __future__ import annotations

import asyncio
from pathlib import Path

import grpc
import pytest

from agent_runtime.config import RuntimeSettings
from agent_runtime.context import RunContext
from agent_runtime.executors import DeepAgentExecutor
from agent_runtime.registry import ExecutorRegistry
from agent_runtime.server import RuntimeServer
from agent_runtime.v1 import agent_runtime_pb2, agent_runtime_pb2_grpc
from agentroom_deepagent.research import ResearchStreamEvent
from agentroom_deepagent.config import MissingCredentials
from agentroom_deepagent.tools import SearchToolError


def request(run_id: str, *, content: str = "@Research investigate", api_key: str = "model-secret"):
    return agent_runtime_pb2.ExecuteAgentRequest(
        protocol_version="v1",
        run_id=run_id,
        executor_kind=agent_runtime_pb2.EXECUTOR_KIND_DEEPAGENT,
        room=agent_runtime_pb2.RoomSnapshot(id="room", name="Room"),
        agent=agent_runtime_pb2.AgentSnapshot(id="research", name="Research", mention="@Research"),
        trigger=agent_runtime_pb2.MessageSnapshot(id="message", content=content),
        model=agent_runtime_pb2.ModelConnection(
            protocol="openai_chat_completions",
            base_url="https://model.example/v1",
            model_name="research-model",
            api_key=api_key,
            profile_id="profile",
            source="room",
        ),
        limits=agent_runtime_pb2.ExecutionLimits(
            max_output_bytes=4096,
            max_artifact_bytes=4096,
        ),
    )


def collect(executor: DeepAgentExecutor, run: RunContext):
    async def scenario():
        return [item async for item in executor.execute(run)]

    return asyncio.run(scenario())


def test_deepagent_uses_request_content_and_returns_markdown_artifact(tmp_path):
    captured = {}

    async def runner(question, settings, recorder):
        captured["question"] = question
        captured["work_dir"] = settings.output_dir
        captured["api_key"] = settings.custom.api_key
        yield ResearchStreamEvent(type="model_started", name="main")
        yield ResearchStreamEvent(type="tool_started", name="internet_search", call_id="tool-1")
        yield ResearchStreamEvent(type="tool_completed", name="internet_search", call_id="tool-1")
        yield ResearchStreamEvent(type="model_completed", name="main")
        report_path = recorder.write_report("# Result\n\nEvidence.")
        yield ResearchStreamEvent(type="report", report_path=report_path)

    run = RunContext.create(request("run-success", content="@Research --config stolen.toml"), tmp_path)
    try:
        events = collect(DeepAgentExecutor(runner), run)
        assert captured["question"] == "--config stolen.toml"
        assert captured["work_dir"] == run.work_dir
        assert captured["api_key"] == "model-secret"
        assert [event.field for event in events] == [
            "model_started",
            "tool_started",
            "tool_completed",
            "model_completed",
            "artifact_ready",
            "completed",
        ]
        artifact = events[-1].message.artifacts[0]
        assert artifact.mime_type == "text/markdown"
        assert artifact.content.replace(b"\r\n", b"\n") == b"# Result\n\nEvidence.\n"
        assert events[-1].message.model.profile_id == "profile"
    finally:
        run.cleanup()

    assert not any(tmp_path.iterdir())


def test_deepagent_redacts_search_failure(tmp_path):
    secret = "tavily-secret-that-must-not-escape"

    async def failing_runner(_question, _settings, _recorder):
        if False:
            yield ResearchStreamEvent(type="stage")
        raise SearchToolError(f"provider rejected {secret}")

    run = RunContext.create(request("run-search-error"), tmp_path)
    try:
        events = collect(DeepAgentExecutor(failing_runner), run)
        assert [event.field for event in events] == ["tool_failed", "failed"]
        serialized = b"".join(event.message.SerializeToString() for event in events)
        assert secret.encode() not in serialized
        assert events[-1].message.failure.code == agent_runtime_pb2.RUN_ERROR_CODE_TOOL_FAILED
    finally:
        run.cleanup()


@pytest.mark.parametrize(
    ("raised", "expected_code", "expected_message"),
    [
        (
            MissingCredentials("MODEL_API_KEY=secret"),
            agent_runtime_pb2.RUN_ERROR_CODE_MODEL_NOT_CONFIGURED,
            "DeepAgent credentials are not configured",
        ),
        (
            TimeoutError("provider timeout included a secret"),
            agent_runtime_pb2.RUN_ERROR_CODE_MODEL_TIMEOUT,
            "DeepAgent model request timed out",
        ),
    ],
)
def test_deepagent_model_errors_are_classified_and_redacted(
    tmp_path,
    raised,
    expected_code,
    expected_message,
):
    async def failing_runner(_question, _settings, _recorder):
        if False:
            yield ResearchStreamEvent(type="stage")
        raise raised

    run = RunContext.create(request(f"run-model-error-{expected_code}"), tmp_path)
    try:
        events = collect(DeepAgentExecutor(failing_runner), run)
        assert len(events) == 1
        assert events[0].field == "failed"
        assert events[0].message.failure.code == expected_code
        assert events[0].message.failure.message == expected_message
        assert b"secret" not in events[0].message.SerializeToString()
    finally:
        run.cleanup()


def test_deepagent_concurrent_runs_isolate_workdirs_and_model_configuration(tmp_path):
    observed = []
    both_started = asyncio.Event()

    async def runner(question, settings, recorder):
        observed.append((question, settings.output_dir, settings.custom.api_key))
        if len(observed) == 2:
            both_started.set()
        await both_started.wait()
        report_path = recorder.write_report(f"# {question}")
        yield ResearchStreamEvent(type="report", report_path=report_path)

    async def scenario():
        first = RunContext.create(request("run-one", content="@Research first", api_key="key-one"), tmp_path)
        second = RunContext.create(request("run-two", content="@Research second", api_key="key-two"), tmp_path)
        executor = DeepAgentExecutor(runner)
        try:
            await asyncio.gather(
                _consume(executor.execute(first)),
                _consume(executor.execute(second)),
            )
            assert first.work_dir != second.work_dir
        finally:
            first.cleanup()
            second.cleanup()

    asyncio.run(scenario())
    assert {(item[0], item[2]) for item in observed} == {("first", "key-one"), ("second", "key-two")}
    assert len({item[1] for item in observed}) == 2
    assert not any(tmp_path.iterdir())


def test_deepagent_report_limit_cancellation_and_graceful_shutdown(tmp_path):
    started = asyncio.Event()
    cancelled = asyncio.Event()

    async def runner(_question, _settings, recorder):
        started.set()
        try:
            await asyncio.sleep(10)
        finally:
            cancelled.set()
        report_path = recorder.write_report("x" * 128)
        yield ResearchStreamEvent(type="report", report_path=report_path)

    async def scenario():
        settings = RuntimeSettings(
            host="127.0.0.1",
            port=0,
            insecure=True,
            work_dir=tmp_path,
            max_artifact_bytes=32,
            shutdown_grace_seconds=0.01,
        )
        runtime = RuntimeServer(settings, ExecutorRegistry([DeepAgentExecutor(runner)]))
        port = await runtime.start()
        channel = grpc.aio.insecure_channel(f"127.0.0.1:{port}")
        stub = agent_runtime_pb2_grpc.AgentRuntimeServiceStub(channel)
        call = stub.ExecuteAgent(request("run-cancel"), timeout=2)
        consumer = asyncio.create_task(_consume(call))
        await started.wait()
        await runtime.stop()
        result = await asyncio.gather(consumer, return_exceptions=True)
        assert isinstance(result[0], grpc.aio.AioRpcError)
        assert result[0].code() in {grpc.StatusCode.CANCELLED, grpc.StatusCode.UNAVAILABLE}
        assert cancelled.is_set()
        assert await runtime.servicer.active.count() == 0
        await channel.close()

    asyncio.run(scenario())
    assert not any(tmp_path.iterdir())


def test_deepagent_oversized_report_is_rejected_without_truncation(tmp_path):
    async def runner(_question, _settings, recorder):
        report_path = recorder.write_report("x" * 128)
        yield ResearchStreamEvent(type="report", report_path=report_path)

    async def scenario():
        settings = RuntimeSettings(
            host="127.0.0.1",
            port=0,
            insecure=True,
            work_dir=tmp_path,
            max_artifact_bytes=32,
        )
        runtime = RuntimeServer(settings, ExecutorRegistry([DeepAgentExecutor(runner)]))
        port = await runtime.start()
        channel = grpc.aio.insecure_channel(f"127.0.0.1:{port}")
        try:
            stub = agent_runtime_pb2_grpc.AgentRuntimeServiceStub(channel)
            outgoing = request("run-limit")
            outgoing.limits.max_artifact_bytes = 32
            with pytest.raises(grpc.aio.AioRpcError) as error:
                await _consume(stub.ExecuteAgent(outgoing))
            assert error.value.code() == grpc.StatusCode.RESOURCE_EXHAUSTED
        finally:
            await channel.close()
            await runtime.stop()

    asyncio.run(scenario())
    assert not any(tmp_path.iterdir())


def test_deepagent_dedicated_capacity_does_not_start_excess_work(tmp_path):
    started = asyncio.Event()
    release = asyncio.Event()
    starts = 0

    async def runner(_question, _settings, recorder):
        nonlocal starts
        starts += 1
        started.set()
        await release.wait()
        report_path = recorder.write_report("# Done")
        yield ResearchStreamEvent(type="report", report_path=report_path)

    async def scenario():
        settings = RuntimeSettings(
            host="127.0.0.1",
            port=0,
            insecure=True,
            work_dir=tmp_path,
            max_concurrency=2,
            deepagent_concurrency=1,
            max_pending=0,
        )
        runtime = RuntimeServer(settings, ExecutorRegistry([DeepAgentExecutor(runner)]))
        port = await runtime.start()
        channel = grpc.aio.insecure_channel(f"127.0.0.1:{port}")
        stub = agent_runtime_pb2_grpc.AgentRuntimeServiceStub(channel)
        first = asyncio.create_task(_consume(stub.ExecuteAgent(request("run-capacity-one"))))
        await started.wait()
        try:
            with pytest.raises(grpc.aio.AioRpcError) as error:
                await _consume(stub.ExecuteAgent(request("run-capacity-two")))
            assert error.value.code() == grpc.StatusCode.RESOURCE_EXHAUSTED
            assert starts == 1
        finally:
            release.set()
            await first
            await channel.close()
            await runtime.stop()

    asyncio.run(scenario())
    assert not any(tmp_path.iterdir())


async def _consume(stream):
    return [item async for item in stream]
