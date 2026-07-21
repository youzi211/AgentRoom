import asyncio

import grpc
import pytest
from grpc_health.v1 import health_pb2, health_pb2_grpc

from agent_runtime.config import RuntimeSettings
from agent_runtime.executors import FakeExecutor
from agent_runtime.registry import ExecutorRegistry
from agent_runtime.server import SERVICE_NAME, RuntimeServer
from agent_runtime.v1 import agent_runtime_pb2, agent_runtime_pb2_grpc


def request(run_id="run_test", *, kind=agent_runtime_pb2.EXECUTOR_KIND_LLM, **metadata):
    return agent_runtime_pb2.ExecuteAgentRequest(
        protocol_version="v1",
        run_id=run_id,
        executor_kind=kind,
        room=agent_runtime_pb2.RoomSnapshot(id="room_test", name="Test room"),
        agent=agent_runtime_pb2.AgentSnapshot(id="agent_test", name="Test agent"),
        trigger=agent_runtime_pb2.MessageSnapshot(id="message_test", content="Test"),
        model=agent_runtime_pb2.ModelConnection(model_name="fake-model", api_key="secret"),
        limits=agent_runtime_pb2.ExecutionLimits(
            max_output_bytes=1024,
            max_artifact_bytes=1024,
        ),
        metadata=metadata,
    )


def settings(tmp_path, **overrides):
    values = {
        "host": "127.0.0.1",
        "port": 0,
        "insecure": True,
        "work_dir": tmp_path,
        "shutdown_grace_seconds": 0.1,
    }
    values.update(overrides)
    return RuntimeSettings(**values)


async def start_runtime(tmp_path, *, runtime_settings=None):
    registry = ExecutorRegistry(
        [
            FakeExecutor(agent_runtime_pb2.EXECUTOR_KIND_LLM),
            FakeExecutor(agent_runtime_pb2.EXECUTOR_KIND_DEEPAGENT),
        ]
    )
    runtime = RuntimeServer(runtime_settings or settings(tmp_path), registry)
    port = await runtime.start()
    channel = grpc.aio.insecure_channel(f"127.0.0.1:{port}")
    return runtime, channel, agent_runtime_pb2_grpc.AgentRuntimeServiceStub(channel)


def run(coro):
    return asyncio.run(coro)


async def consume(call):
    return [event async for event in call]


def test_registry_rejects_duplicate_registration_and_unknown_kind():
    registry = ExecutorRegistry([FakeExecutor(agent_runtime_pb2.EXECUTOR_KIND_LLM)])

    with pytest.raises(ValueError, match="already registered"):
        registry.register(FakeExecutor(agent_runtime_pb2.EXECUTOR_KIND_LLM))
    with pytest.raises(LookupError, match="not configured"):
        registry.resolve(agent_runtime_pb2.EXECUTOR_KIND_DEEPAGENT)


def test_server_reports_serving_and_streams_ordered_success(tmp_path):
    async def scenario():
        runtime, channel, stub = await start_runtime(tmp_path)
        try:
            health_stub = health_pb2_grpc.HealthStub(channel)
            health = await health_stub.Check(health_pb2.HealthCheckRequest(service=SERVICE_NAME))
            assert health.status == health_pb2.HealthCheckResponse.SERVING

            events = [event async for event in stub.ExecuteAgent(request())]
            assert [event.sequence for event in events] == list(range(1, len(events) + 1))
            assert events[0].WhichOneof("payload") == "accepted"
            assert events[-1].WhichOneof("payload") == "completed"
            assert events[-1].completed.content == "fake response"
            metrics = runtime.servicer.telemetry.snapshot()
            assert metrics["active"] == 0
            assert metrics["waiting"] == 0
            assert metrics["outcomes"] == {"succeeded": 1}
            assert metrics["grpc_statuses"] == {"OK": 1}
        finally:
            await channel.close()
            await runtime.stop()

    run(scenario())


def test_structured_runtime_logs_correlate_without_prompt_or_credentials(tmp_path, caplog):
    secret = "request-api-key-never-log"
    prompt = "private prompt text never log"

    async def scenario():
        runtime, channel, stub = await start_runtime(tmp_path)
        outgoing = request("run_logging")
        outgoing.trace_id = "trace_logging"
        outgoing.dialogue_run_id = "dialogue_logging"
        outgoing.trigger.content = prompt
        outgoing.model.api_key = secret
        try:
            await consume(stub.ExecuteAgent(outgoing))
        finally:
            await channel.close()
            await runtime.stop()

    with caplog.at_level("INFO", logger="agent_runtime.service"):
        run(scenario())

    assert secret not in caplog.text
    assert prompt not in caplog.text
    finished = next(record for record in caplog.records if record.msg == "agent_run_finished")
    assert finished.run_id == "run_logging"
    assert finished.room_id == "room_test"
    assert finished.agent_id == "agent_test"
    assert finished.dialogue_run_id == "dialogue_logging"
    assert finished.trace_id == "trace_logging"
    assert finished.outcome == "succeeded"


def test_fake_executor_streams_tools_artifact_and_failure(tmp_path):
    async def scenario():
        runtime, channel, stub = await start_runtime(tmp_path)
        try:
            artifact_events = [
                event async for event in stub.ExecuteAgent(request("run_artifact", fake_mode="artifact"))
            ]
            payloads = [event.WhichOneof("payload") for event in artifact_events]
            assert "tool_started" in payloads
            assert "tool_completed" in payloads
            assert "artifact_ready" in payloads
            assert artifact_events[-1].completed.artifacts[0].file_name == "report.md"

            failed_events = [
                event async for event in stub.ExecuteAgent(request("run_failed", fake_mode="failed"))
            ]
            assert failed_events[-1].WhichOneof("payload") == "failed"
        finally:
            await channel.close()
            await runtime.stop()

    run(scenario())


def test_service_rejects_unknown_version_and_executor_before_acceptance(tmp_path):
    async def scenario():
        runtime, channel, stub = await start_runtime(tmp_path)
        try:
            invalid_version = request("run_version")
            invalid_version.protocol_version = "v2"
            with pytest.raises(grpc.aio.AioRpcError) as version_error:
                await stub.ExecuteAgent(invalid_version).read()
            assert version_error.value.code() == grpc.StatusCode.UNIMPLEMENTED

            with pytest.raises(grpc.aio.AioRpcError) as executor_error:
                await stub.ExecuteAgent(request("run_executor", kind=99)).read()
            assert executor_error.value.code() == grpc.StatusCode.UNIMPLEMENTED
        finally:
            await channel.close()
            await runtime.stop()

    run(scenario())


def test_duplicate_run_is_rejected_while_first_is_active(tmp_path):
    async def scenario():
        runtime, channel, stub = await start_runtime(tmp_path)
        try:
            first = stub.ExecuteAgent(request("run_duplicate", fake_delay_ms="200"))
            first_task = asyncio.create_task(first.read())
            await asyncio.sleep(0.03)
            with pytest.raises(grpc.aio.AioRpcError) as duplicate_error:
                await stub.ExecuteAgent(request("run_duplicate")).read()
            assert duplicate_error.value.code() == grpc.StatusCode.ALREADY_EXISTS
            first.cancel()
            await asyncio.gather(first_task, return_exceptions=True)
        finally:
            await channel.close()
            await runtime.stop()

    run(scenario())


def test_deadline_and_capacity_wait_observe_cancellation(tmp_path):
    async def scenario():
        limited = settings(tmp_path, max_concurrency=1, deepagent_concurrency=1, max_pending=1)
        runtime, channel, stub = await start_runtime(tmp_path, runtime_settings=limited)
        try:
            with pytest.raises(grpc.aio.AioRpcError) as deadline_error:
                await consume(
                    stub.ExecuteAgent(
                        request("run_deadline", fake_delay_ms="500"),
                        timeout=0.03,
                    )
                )
            assert deadline_error.value.code() == grpc.StatusCode.DEADLINE_EXCEEDED

            first = stub.ExecuteAgent(request("run_capacity_1", fake_delay_ms="300"))
            second = stub.ExecuteAgent(request("run_capacity_2", fake_delay_ms="300"))
            first_task = asyncio.create_task(consume(first))
            await asyncio.sleep(0.02)
            second_task = asyncio.create_task(consume(second))
            await asyncio.sleep(0.02)
            with pytest.raises(grpc.aio.AioRpcError) as capacity_error:
                await stub.ExecuteAgent(request("run_capacity_3")).read()
            assert capacity_error.value.code() == grpc.StatusCode.RESOURCE_EXHAUSTED
            second.cancel()
            first.cancel()
            await asyncio.gather(first_task, second_task, return_exceptions=True)
        finally:
            await channel.close()
            await runtime.stop()

    run(scenario())


def test_slow_consumer_preserves_critical_events_in_order(tmp_path):
    async def scenario():
        runtime, channel, stub = await start_runtime(
            tmp_path,
            runtime_settings=settings(
                tmp_path,
                event_buffer_size=1,
                max_output_bytes=4096,
            ),
        )
        try:
            payloads = []
            sequences = []
            deltas = []
            async for event in stub.ExecuteAgent(
                request("run_slow", fake_mode="stream", fake_delta_count="64")
            ):
                payloads.append(event.WhichOneof("payload"))
                sequences.append(event.sequence)
                if event.WhichOneof("payload") == "output_delta":
                    deltas.append(event.output_delta.text)
                await asyncio.sleep(0.01)

            assert sequences == list(range(1, len(sequences) + 1))
            assert payloads[0] == "accepted"
            assert "model_started" in payloads
            assert "model_completed" in payloads
            assert payloads[-1] == "completed"
            assert "".join(deltas) == "".join(f"{index}," for index in range(64))
        finally:
            await channel.close()
            await runtime.stop()

    run(scenario())


def test_request_and_event_limits_abort_without_truncation(tmp_path):
    async def scenario():
        limited = settings(tmp_path, max_request_bytes=256, max_output_bytes=4)
        runtime, channel, stub = await start_runtime(tmp_path, runtime_settings=limited)
        try:
            oversized_request = request("run_request_limit")
            oversized_request.trigger.content = "x" * 512
            with pytest.raises(grpc.aio.AioRpcError) as request_error:
                await stub.ExecuteAgent(oversized_request).read()
            assert request_error.value.code() == grpc.StatusCode.RESOURCE_EXHAUSTED

            with pytest.raises(grpc.aio.AioRpcError) as output_error:
                await consume(
                    stub.ExecuteAgent(request("run_output_limit", fake_content="not truncated"))
                )
            assert output_error.value.code() == grpc.StatusCode.RESOURCE_EXHAUSTED
        finally:
            await channel.close()
            await runtime.stop()

    run(scenario())


def test_out_of_order_executor_input_is_detected(tmp_path):
    async def scenario():
        runtime, channel, stub = await start_runtime(tmp_path)
        try:
            events = []
            with pytest.raises(grpc.aio.AioRpcError) as sequence_error:
                async for event in stub.ExecuteAgent(
                    request("run_out_of_order", fake_mode="out_of_order")
                ):
                    events.append(event)
            assert events[-1].WhichOneof("payload") == "completed"
            assert sequence_error.value.code() == grpc.StatusCode.INTERNAL
        finally:
            await channel.close()
            await runtime.stop()

    run(scenario())


def test_graceful_stop_rejects_new_calls_and_cleans_active_runs(tmp_path):
    async def scenario():
        runtime, channel, stub = await start_runtime(
            tmp_path,
            runtime_settings=settings(tmp_path, shutdown_grace_seconds=0.02),
        )
        active_call = stub.ExecuteAgent(request("run_shutdown", fake_delay_ms="500"))
        active_task = asyncio.create_task(consume(active_call))
        await asyncio.sleep(0.03)

        await runtime.stop()
        result = await asyncio.gather(active_task, return_exceptions=True)
        assert isinstance(result[0], grpc.aio.AioRpcError)
        assert result[0].code() in {grpc.StatusCode.CANCELLED, grpc.StatusCode.UNAVAILABLE}
        assert await runtime.servicer.active.count() == 0
        assert not any(tmp_path.iterdir())

        with pytest.raises(grpc.aio.AioRpcError) as stopped_error:
            await stub.ExecuteAgent(request("run_after_shutdown")).read()
        assert stopped_error.value.code() == grpc.StatusCode.UNAVAILABLE
        await channel.close()

    run(scenario())


def test_oversized_artifact_aborts_without_truncation_and_cleans_secret(tmp_path):
    async def scenario():
        limited = settings(tmp_path, max_artifact_bytes=4)
        runtime, channel, stub = await start_runtime(tmp_path, runtime_settings=limited)
        outgoing = request("run_oversized", fake_mode="artifact", fake_artifact_bytes="16")
        try:
            call = stub.ExecuteAgent(outgoing)
            with pytest.raises(grpc.aio.AioRpcError) as size_error:
                async for _event in call:
                    pass
            assert size_error.value.code() == grpc.StatusCode.RESOURCE_EXHAUSTED
            assert not any(tmp_path.iterdir())
        finally:
            await channel.close()
            await runtime.stop()

    run(scenario())
