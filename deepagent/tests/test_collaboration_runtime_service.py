import asyncio
import logging

import grpc
import pytest
from google.protobuf import duration_pb2

from collaboration_runtime.events import CollaborationEventWriter, EventSequenceError
from collaboration_runtime.models import EngineEvent, EventKind
from collaboration_runtime.registry import CollaborationEngineRegistry
from collaboration_runtime.service import CollaborationRuntimeServicer
from collaboration_runtime.v1 import collaboration_runtime_pb2


class AbortedCall(RuntimeError):
    def __init__(self, code, details):
        super().__init__(details)
        self.code = code
        self.details = details


class FakeContext:
    def __init__(self):
        self._cancelled = False

    def cancelled(self):
        return self._cancelled

    async def abort(self, code, details):
        raise AbortedCall(code, details)


class FakeEngine:
    name = "native"
    version = "fake-v1"
    events = ()
    error = None

    async def execute(self, request, cancel_event):
        for event in self.events:
            yield event
        if self.error is not None:
            raise self.error


def request(run_id="collaboration_test"):
    return collaboration_runtime_pb2.ExecuteConversationRequest(
        protocol_version="v1",
        collaboration_run_id=run_id,
        trace_id="trace_test",
        engine=collaboration_runtime_pb2.COLLABORATION_ENGINE_NATIVE,
        snapshot=collaboration_runtime_pb2.ConversationSnapshot(
            room=collaboration_runtime_pb2.RoomSnapshot(
                id="room_test",
                name="Test room",
                status="active",
            ),
            agents=[
                collaboration_runtime_pb2.AgentSnapshot(
                    id="agent_test",
                    name="Test agent",
                    runtime="llm",
                    model_selection_id="model_test",
                )
            ],
            trigger=collaboration_runtime_pb2.MessageSnapshot(
                id="message_test",
                sender_id="human_test",
                sender_name="Human",
                sender_type=collaboration_runtime_pb2.SENDER_TYPE_HUMAN,
                content="Test",
            ),
            model_selections=[
                collaboration_runtime_pb2.ModelSelection(
                    id="model_test",
                    profile_id="profile_test",
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
                max_turns=3,
                max_turns_per_agent=1,
                allow_agent_handoff=True,
                stop_on_empty_output=True,
                stop_on_repeated_output=True,
            ),
            limits=collaboration_runtime_pb2.ExecutionLimits(
                timeout=duration_pb2.Duration(seconds=30),
                max_output_bytes=1024,
                max_artifact_bytes=1024,
                max_tool_steps=8,
                max_request_bytes=8192,
                max_event_bytes=4096,
                max_checkpoint_bytes=1024,
            ),
            initial_candidate_agent_ids=["agent_test"],
        ),
    )


def servicer(events=(), *, error=None):
    class Engine(FakeEngine):
        pass

    Engine.events = events
    Engine.error = error
    registry = CollaborationEngineRegistry()
    registry.register("native", Engine)
    return CollaborationRuntimeServicer(registry)


async def consume(runtime, outgoing):
    return [event async for event in runtime.ExecuteConversation(outgoing, FakeContext())]


def run(coro):
    return asyncio.run(coro)


def test_service_maps_request_and_streams_service_owned_ordered_terminal():
    events = (
        EngineEvent(EventKind.COLLABORATION_STARTED),
        EngineEvent(
            EventKind.SPEAKER_SELECTED,
            turn_id="turn_1",
            agent_id="agent_test",
            data={"reason_category": "mention"},
        ),
        EngineEvent(
            EventKind.AGENT_TURN_STARTED,
            turn_id="turn_1",
            agent_id="agent_test",
        ),
        EngineEvent(
            EventKind.AGENT_MESSAGE_COMPLETED,
            turn_id="turn_1",
            agent_id="agent_test",
            data={"content": "response"},
        ),
        EngineEvent(EventKind.COMPLETED, data={"turn_count": 1, "reason": "completed"}),
    )

    streamed = run(consume(servicer(events), request()))

    assert [event.sequence for event in streamed] == list(range(1, len(streamed) + 1))
    assert [event.WhichOneof("payload") for event in streamed] == [
        "accepted",
        "collaboration_started",
        "speaker_selected",
        "agent_turn_started",
        "agent_message_completed",
        "completed",
    ]
    assert streamed[4].agent_message_completed.content == "response"


@pytest.mark.parametrize(
    ("mutate", "status"),
    [
        (lambda value: setattr(value, "protocol_version", "v2"), grpc.StatusCode.UNIMPLEMENTED),
        (
            lambda value: value.snapshot.initial_candidate_agent_ids.append("agent_unknown"),
            grpc.StatusCode.INVALID_ARGUMENT,
        ),
        (
            lambda value: setattr(value.snapshot.agents[0], "model_selection_id", "model_unknown"),
            grpc.StatusCode.INVALID_ARGUMENT,
        ),
    ],
)
def test_service_rejects_invalid_requests_before_acceptance(mutate, status):
    outgoing = request()
    mutate(outgoing)

    with pytest.raises(AbortedCall) as error:
        run(consume(servicer(), outgoing))

    assert error.value.code is status


def test_service_rejects_unknown_engine_before_acceptance():
    outgoing = request()
    outgoing.engine = collaboration_runtime_pb2.COLLABORATION_ENGINE_AUTOGEN
    outgoing.snapshot.policy.engine = collaboration_runtime_pb2.COLLABORATION_ENGINE_AUTOGEN

    with pytest.raises(AbortedCall) as error:
        run(consume(servicer(), outgoing))

    assert error.value.code is grpc.StatusCode.UNAVAILABLE
    assert error.value.details == "Collaboration Engine is unavailable"


def test_service_sanitizes_engine_initialization_failure():
    registry = CollaborationEngineRegistry()

    def broken_factory():
        raise RuntimeError("Authorization: Bearer factory-secret")

    registry.register("native", broken_factory)
    runtime = CollaborationRuntimeServicer(registry)

    with pytest.raises(AbortedCall) as error:
        run(consume(runtime, request("collaboration_init_error")))

    assert error.value.code is grpc.StatusCode.UNAVAILABLE
    assert error.value.details == "Collaboration Engine is unavailable"
    assert run(runtime.active.count()) == 0


def test_service_synthesizes_one_safe_terminal_for_protocol_violations():
    no_terminal = run(
        consume(
            servicer((EngineEvent(EventKind.COLLABORATION_STARTED),)),
            request("collaboration_no_terminal"),
        )
    )
    outside_agent = run(
        consume(
            servicer(
                (
                    EngineEvent(
                        EventKind.SPEAKER_SELECTED,
                        turn_id="turn_unknown",
                        agent_id="agent_unknown",
                    ),
                )
            ),
            request("collaboration_outside_agent"),
        )
    )

    for streamed in (no_terminal, outside_agent):
        assert streamed[-1].WhichOneof("payload") == "failed"
        assert streamed[-1].failed.reason == collaboration_runtime_pb2.COLLABORATION_STOP_REASON_PROTOCOL_ERROR
        assert streamed[-1].failed.failure.code == collaboration_runtime_pb2.COLLABORATION_ERROR_CODE_PROTOCOL_ERROR
        assert sum(event.WhichOneof("payload") in {"completed", "stopped", "cancelled", "failed"} for event in streamed) == 1


def test_service_does_not_expose_engine_exception_text(caplog):
    secret = "Authorization: Bearer provider-secret"
    runtime = servicer(error=RuntimeError(secret))

    with caplog.at_level(logging.ERROR, logger="collaboration_runtime.service"):
        streamed = run(consume(runtime, request("collaboration_error")))

    assert streamed[-1].WhichOneof("payload") == "failed"
    assert streamed[-1].failed.failure.message == "Collaboration Engine failed"
    assert secret not in caplog.text
    assert secret not in str(streamed[-1])
    assert run(runtime.active.count()) == 0


def test_service_does_not_retry_started_engine_after_execution_failure():
    async def scenario():
        creations = 0
        executions = 0

        class StartedThenFailsEngine:
            name = "native"
            version = "fake-v1"

            async def execute(self, _request, _cancel_event):
                nonlocal executions
                executions += 1
                yield EngineEvent(EventKind.COLLABORATION_STARTED)
                raise RuntimeError("provider failed after model start")

        def factory():
            nonlocal creations
            creations += 1
            return StartedThenFailsEngine()

        registry = CollaborationEngineRegistry()
        registry.register("native", factory)
        runtime = CollaborationRuntimeServicer(registry)

        streamed = await consume(runtime, request("collaboration_started_failure"))

        assert [event.WhichOneof("payload") for event in streamed] == [
            "accepted",
            "collaboration_started",
            "failed",
        ]
        assert creations == 1
        assert executions == 1
        assert await runtime.active.count() == 0

    run(scenario())


def test_writer_requires_exactly_one_service_owned_accepted_event():
    writer = CollaborationEventWriter(
        "collaboration_writer",
        max_event_bytes=1024,
        max_artifact_bytes=1024,
        max_output_bytes=1024,
        max_checkpoint_bytes=1024,
    )

    with pytest.raises(EventSequenceError, match="accepted"):
        writer.write(EngineEvent(EventKind.COLLABORATION_STARTED))
    writer.accepted()
    with pytest.raises(EventSequenceError, match="only be emitted once"):
        writer.accepted()
    with pytest.raises(EventSequenceError, match="Engine cannot emit accepted"):
        writer.write(EngineEvent(EventKind.ACCEPTED))


def test_service_rejects_event_limit_that_cannot_hold_accepted():
    outgoing = request("collaboration_small_event")
    outgoing.snapshot.limits.max_event_bytes = 1

    with pytest.raises(AbortedCall) as error:
        run(consume(servicer(), outgoing))

    assert error.value.code is grpc.StatusCode.RESOURCE_EXHAUSTED
    assert error.value.details == "Collaboration event exceeds resource limits"


def test_capacity_wait_cancellation_does_not_create_engine():
    async def scenario():
        started = asyncio.Event()
        release = asyncio.Event()
        creations = 0

        class BlockingEngine:
            name = "native"
            version = "fake-v1"

            async def execute(self, request, cancel_event):
                started.set()
                await release.wait()
                yield EngineEvent(
                    EventKind.COMPLETED,
                    data={"turn_count": 0, "reason": "completed"},
                )

        def factory():
            nonlocal creations
            creations += 1
            return BlockingEngine()

        registry = CollaborationEngineRegistry()
        registry.register("native", factory)
        runtime = CollaborationRuntimeServicer(registry, max_concurrency=1, max_pending=1)

        first = asyncio.create_task(consume(runtime, request("collaboration_active")))
        await started.wait()
        queued_request = request("collaboration_queued")
        queued_request.snapshot.room.id = "room_queued"
        queued = asyncio.create_task(consume(runtime, queued_request))
        while await runtime.capacity.pending() != 1:
            await asyncio.sleep(0)

        assert creations == 1
        queued.cancel()
        with pytest.raises(asyncio.CancelledError):
            await queued
        assert creations == 1
        assert await runtime.capacity.pending() == 0

        release.set()
        streamed = await first
        assert streamed[-1].WhichOneof("payload") == "completed"
        assert await runtime.capacity.active() == 0
        assert await runtime.capacity.room_count() == 0
        assert await runtime.active.count() == 0

    run(scenario())


def test_collaboration_telemetry_is_structured_and_excludes_content(caplog):
    prompt = "private collaboration prompt"
    secret = "Bearer telemetry-secret"
    outgoing = request("collaboration_logging")
    outgoing.trace_id = "trace_logging"
    outgoing.snapshot.trigger.content = prompt
    runtime = servicer(
        (
            EngineEvent(EventKind.COLLABORATION_STARTED),
            EngineEvent(
                EventKind.AGENT_MESSAGE_COMPLETED,
                turn_id="turn_logging",
                agent_id="agent_test",
                data={"content": "safe response"},
            ),
            EngineEvent(
                EventKind.COMPLETED,
                data={"turn_count": 1, "reason": "completed", "ignored": secret},
            ),
        )
    )

    with caplog.at_level(logging.INFO, logger="collaboration_runtime.service"):
        run(consume(runtime, outgoing))

    assert prompt not in caplog.text
    assert secret not in caplog.text
    finished = next(
        record for record in caplog.records if record.msg == "collaboration_run_finished"
    )
    assert finished.collaboration_run_id == "collaboration_logging"
    assert finished.room_id == "room_test"
    assert finished.trace_id == "trace_logging"
    assert finished.engine == "native"
    assert finished.outcome == "succeeded"
    assert finished.turn_count == 1
    assert runtime.telemetry.snapshot() == {
        "active": 0,
        "waiting": 0,
        "outcomes": {"succeeded": 1},
        "grpc_statuses": {"OK": 1},
        "engines": {"native": 1},
        "events": 4,
        "turns": 1,
    }


def test_service_rejects_duplicate_run_room_and_exhausted_capacity_before_engine_creation():
    async def scenario():
        started = asyncio.Event()
        release = asyncio.Event()
        creations = 0

        class BlockingEngine:
            name = "native"
            version = "fake-v1"

            async def execute(self, request, cancel_event):
                started.set()
                await release.wait()
                yield EngineEvent(
                    EventKind.COMPLETED,
                    data={"turn_count": 0, "reason": "completed"},
                )

        def factory():
            nonlocal creations
            creations += 1
            return BlockingEngine()

        registry = CollaborationEngineRegistry()
        registry.register("native", factory)
        runtime = CollaborationRuntimeServicer(
            registry,
            max_concurrency=1,
            max_pending=0,
        )
        first_request = request("collaboration_guarded")
        first = asyncio.create_task(consume(runtime, first_request))
        await started.wait()

        with pytest.raises(AbortedCall) as duplicate_run:
            await consume(runtime, request("collaboration_guarded"))
        assert duplicate_run.value.code is grpc.StatusCode.ALREADY_EXISTS

        same_room = request("collaboration_same_room")
        with pytest.raises(AbortedCall) as room_busy:
            await consume(runtime, same_room)
        assert room_busy.value.code is grpc.StatusCode.FAILED_PRECONDITION

        another_room = request("collaboration_no_capacity")
        another_room.snapshot.room.id = "room_other"
        with pytest.raises(AbortedCall) as exhausted:
            await consume(runtime, another_room)
        assert exhausted.value.code is grpc.StatusCode.RESOURCE_EXHAUSTED
        assert creations == 1

        release.set()
        await first
        assert await runtime.active.count() == 0
        assert await runtime.capacity.active() == 0

    run(scenario())
