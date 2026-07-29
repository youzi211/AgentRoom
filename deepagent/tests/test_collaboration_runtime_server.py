import asyncio

import grpc
from google.protobuf import duration_pb2
from grpc_health.v1 import health_pb2, health_pb2_grpc

from agent_runtime.config import RuntimeSettings
from agent_runtime.executors import FakeExecutor
from agent_runtime.registry import ExecutorRegistry
from agent_runtime.server import (
    COLLABORATION_SERVICE_NAME,
    SERVICE_NAME,
    RuntimeServer,
)
from agent_runtime.v1 import agent_runtime_pb2
from collaboration_runtime.models import EngineEvent, EventKind
from collaboration_runtime.registry import CollaborationEngineRegistry
from collaboration_runtime.v1 import collaboration_runtime_pb2, collaboration_runtime_pb2_grpc


class FakeCollaborationEngine:
    name = "native"
    version = "fake-v1"

    async def execute(self, request, cancel_event):
        yield EngineEvent(
            EventKind.COMPLETED,
            data={"turn_count": 0, "reason": "completed"},
        )


def collaboration_request():
    return collaboration_runtime_pb2.ExecuteConversationRequest(
        protocol_version="v1",
        collaboration_run_id="collaboration_server",
        trace_id="trace_server",
        engine=collaboration_runtime_pb2.COLLABORATION_ENGINE_NATIVE,
        snapshot=collaboration_runtime_pb2.ConversationSnapshot(
            room=collaboration_runtime_pb2.RoomSnapshot(id="room_server", status="active"),
            agents=[
                collaboration_runtime_pb2.AgentSnapshot(
                    id="agent_server",
                    name="Server agent",
                    runtime="llm",
                    model_reference_id="model_server",
                )
            ],
            trigger=collaboration_runtime_pb2.MessageSnapshot(
                id="message_server",
                sender_type=collaboration_runtime_pb2.SENDER_TYPE_HUMAN,
            ),
            model_references=[
                collaboration_runtime_pb2.ModelReference(
                    id="model_server",
                    profile_id="profile_server",
                    source="test",
                    protocol="fake",
                    model_name="fake-model",
                    runtime_scope="collaboration",
                )
            ],
            policy=collaboration_runtime_pb2.CollaborationPolicySnapshot(
                version="v1",
                engine=collaboration_runtime_pb2.COLLABORATION_ENGINE_NATIVE,
                trigger_mode=collaboration_runtime_pb2.TRIGGER_MODE_MENTION_ONLY,
                max_turns=1,
                max_turns_per_agent=1,
            ),
            limits=collaboration_runtime_pb2.ExecutionLimits(
                timeout=duration_pb2.Duration(seconds=30),
                max_output_bytes=1024,
                max_artifact_bytes=1024,
                max_tool_steps=1,
                max_request_bytes=8192,
                max_event_bytes=4096,
                max_checkpoint_bytes=1024,
            ),
        ),
    )


def test_server_registers_collaboration_runtime_with_independent_health(tmp_path):
    async def scenario():
        settings = RuntimeSettings(
            host="127.0.0.1",
            port=0,
            insecure=True,
            work_dir=tmp_path,
            shutdown_grace_seconds=0.1,
        )
        agent_registry = ExecutorRegistry(
            [FakeExecutor(agent_runtime_pb2.EXECUTOR_KIND_LLM)]
        )
        collaboration_registry = CollaborationEngineRegistry()
        collaboration_registry.register("native", FakeCollaborationEngine)
        runtime = RuntimeServer(settings, agent_registry, collaboration_registry)
        port = await runtime.start()
        channel = grpc.aio.insecure_channel(f"127.0.0.1:{port}")
        try:
            health_stub = health_pb2_grpc.HealthStub(channel)
            agent_health = await health_stub.Check(
                health_pb2.HealthCheckRequest(service=SERVICE_NAME)
            )
            collaboration_health = await health_stub.Check(
                health_pb2.HealthCheckRequest(service=COLLABORATION_SERVICE_NAME)
            )
            assert agent_health.status == health_pb2.HealthCheckResponse.SERVING
            assert collaboration_health.status == health_pb2.HealthCheckResponse.SERVING

            stub = collaboration_runtime_pb2_grpc.CollaborationRuntimeServiceStub(channel)
            events = [event async for event in stub.ExecuteConversation(collaboration_request())]
            assert [event.WhichOneof("payload") for event in events] == [
                "accepted",
                "completed",
            ]
        finally:
            await channel.close()
            await runtime.stop()

    asyncio.run(scenario())


def test_graceful_stop_cancels_and_cleans_collaboration_calls(tmp_path):
    async def scenario():
        started = asyncio.Event()
        cancelled = asyncio.Event()

        class CancellableEngine:
            name = "native"
            version = "fake-v1"

            async def execute(self, request, cancel_event):
                started.set()
                await cancel_event.wait()
                cancelled.set()
                yield EngineEvent(
                    EventKind.CANCELLED,
                    data={"turn_count": 0, "reason": "cancelled"},
                )

        settings = RuntimeSettings(
            host="127.0.0.1",
            port=0,
            insecure=True,
            work_dir=tmp_path,
            shutdown_grace_seconds=0.1,
        )
        agent_registry = ExecutorRegistry(
            [FakeExecutor(agent_runtime_pb2.EXECUTOR_KIND_LLM)]
        )
        collaboration_registry = CollaborationEngineRegistry()
        collaboration_registry.register("native", CancellableEngine)
        runtime = RuntimeServer(settings, agent_registry, collaboration_registry)
        port = await runtime.start()
        channel = grpc.aio.insecure_channel(f"127.0.0.1:{port}")
        stub = collaboration_runtime_pb2_grpc.CollaborationRuntimeServiceStub(channel)
        call = stub.ExecuteConversation(collaboration_request())
        consume_task = asyncio.create_task(consume(call))
        await started.wait()

        await runtime.stop()
        result = await asyncio.gather(consume_task, return_exceptions=True)

        assert cancelled.is_set()
        if not isinstance(result[0], Exception):
            assert result[0][-1].WhichOneof("payload") == "cancelled"
        assert await runtime.collaboration_servicer.active.count() == 0
        assert await runtime.collaboration_servicer.capacity.active() == 0
        assert await runtime.collaboration_servicer.capacity.pending() == 0
        assert await runtime.collaboration_servicer.capacity.room_count() == 0
        await channel.close()

    async def consume(call):
        return [event async for event in call]

    asyncio.run(scenario())
