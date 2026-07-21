from __future__ import annotations

import asyncio
import logging

import grpc

from .capacity import CapacityExceeded, CapacityLimiter
from .config import RuntimeSettings
from .context import ActiveRunRegistry, RunContext
from .events import EventSequenceError, EventWriter, ResourceLimitError
from .protocol import ProtocolVersionError, validate_protocol_version
from .registry import ExecutorNotFound, ExecutorRegistry
from .stream import backpressured_events
from .telemetry import RuntimeTelemetry
from .v1 import agent_runtime_pb2, agent_runtime_pb2_grpc


LOGGER = logging.getLogger(__name__)


class AgentRuntimeServicer(agent_runtime_pb2_grpc.AgentRuntimeServiceServicer):
    def __init__(self, settings: RuntimeSettings, registry: ExecutorRegistry) -> None:
        self.settings = settings
        self.registry = registry
        self.capacity = CapacityLimiter(
            settings.max_concurrency,
            settings.deepagent_concurrency,
            settings.max_pending,
        )
        self.active = ActiveRunRegistry()
        self.telemetry = RuntimeTelemetry(LOGGER)

    async def ExecuteAgent(self, request, context):  # noqa: N802 - generated gRPC method name
        try:
            self._validate_request(request)
            executor = self.registry.resolve(request.executor_kind)
        except ProtocolVersionError as exc:
            await context.abort(grpc.StatusCode.UNIMPLEMENTED, str(exc))
            return
        except ExecutorNotFound as exc:
            await context.abort(grpc.StatusCode.UNIMPLEMENTED, str(exc))
            return
        except ResourceLimitError as exc:
            await context.abort(grpc.StatusCode.RESOURCE_EXHAUSTED, str(exc))
            return
        except ValueError as exc:
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(exc))
            return

        if not await self.active.register(request.run_id):
            await context.abort(grpc.StatusCode.ALREADY_EXISTS, "run_id is already active")
            return

        run = RunContext.create(request, self.settings.work_dir)
        call_metrics = self.telemetry.begin(request)
        outcome = "failed"
        grpc_status = "OK"
        writer = EventWriter(
            request.run_id,
            max_event_bytes=self.settings.max_event_bytes,
            max_artifact_bytes=min(
                self.settings.max_artifact_bytes,
                request.limits.max_artifact_bytes or self.settings.max_artifact_bytes,
            ),
            max_output_bytes=min(
                self.settings.max_output_bytes,
                request.limits.max_output_bytes or self.settings.max_output_bytes,
            ),
        )

        try:
            async with self.capacity.slot(request.executor_kind):
                self.telemetry.activate(request, call_metrics)
                yield writer.accepted()
                try:
                    async for item in backpressured_events(
                        executor.execute(run),
                        buffer_size=self.settings.event_buffer_size,
                        max_coalesced_bytes=min(
                            self.settings.max_event_bytes,
                            self.settings.max_output_bytes,
                        ),
                    ):
                        if context.cancelled():
                            raise asyncio.CancelledError
                        yield writer.write(item)
                        if item.field == "completed":
                            outcome = "succeeded"
                        elif item.field == "failed":
                            outcome = "failed"
                    if not writer.terminal:
                        yield writer.failed(
                            agent_runtime_pb2.RUN_ERROR_CODE_OUTPUT_INVALID,
                            "Executor ended without a terminal event",
                        )
                except ResourceLimitError as exc:
                    outcome = "failed"
                    grpc_status = "RESOURCE_EXHAUSTED"
                    await context.abort(grpc.StatusCode.RESOURCE_EXHAUSTED, str(exc))
                    return
                except EventSequenceError as exc:
                    outcome = "failed"
                    grpc_status = "INTERNAL"
                    await context.abort(grpc.StatusCode.INTERNAL, str(exc))
                    return
                except asyncio.CancelledError:
                    run.cancel_event.set()
                    outcome = "timeout" if context.time_remaining() == 0 else "cancelled"
                    grpc_status = "DEADLINE_EXCEEDED" if outcome == "timeout" else "CANCELLED"
                    raise
                except Exception:
                    LOGGER.error(
                        "Agent Executor failed",
                        extra={
                            "run_id": request.run_id,
                            "room_id": request.room.id,
                            "agent_id": request.agent.id,
                            "trace_id": request.trace_id,
                        },
                    )
                    if not writer.terminal:
                        yield writer.failed(
                            agent_runtime_pb2.RUN_ERROR_CODE_INTERNAL,
                            "Agent Executor failed",
                        )
        except CapacityExceeded as exc:
            outcome = "failed"
            grpc_status = "RESOURCE_EXHAUSTED"
            await context.abort(grpc.StatusCode.RESOURCE_EXHAUSTED, str(exc))
            return
        finally:
            self.telemetry.finish(
                request,
                call_metrics,
                outcome=outcome,
                grpc_status=grpc_status,
            )
            run.cleanup()
            await self.active.unregister(request.run_id)

    async def cancel_active(self) -> None:
        await self.active.cancel_all()

    def _validate_request(self, request: agent_runtime_pb2.ExecuteAgentRequest) -> None:
        if request.ByteSize() > self.settings.max_request_bytes:
            raise ResourceLimitError("ExecuteAgentRequest exceeds the configured limit")
        validate_protocol_version(request.protocol_version)
        if not request.run_id.strip():
            raise ValueError("run_id is required")
        if request.executor_kind == agent_runtime_pb2.EXECUTOR_KIND_UNSPECIFIED:
            raise ValueError("executor_kind is required")
        if not request.HasField("room") or not request.room.id.strip():
            raise ValueError("room snapshot is required")
        if not request.HasField("agent") or not request.agent.id.strip():
            raise ValueError("agent snapshot is required")
        if not request.HasField("trigger") or not request.trigger.id.strip():
            raise ValueError("trigger message is required")
