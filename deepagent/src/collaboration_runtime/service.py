from __future__ import annotations

import asyncio
import logging
from dataclasses import replace

import grpc

from .capacity import (
    CollaborationCapacityExceeded,
    CollaborationCapacityLimiter,
    CollaborationRoomBusy,
)
from .context import ActiveCollaborationRegistry, CollaborationRunContext
from .events import CollaborationEventWriter, EventSequenceError, ResourceLimitError
from .mapping import map_request
from .protocol import (
    DEFAULT_MAX_REQUEST_BYTES,
    DEFAULT_MAX_ARTIFACT_BYTES,
    DEFAULT_MAX_CHECKPOINT_BYTES,
    DEFAULT_MAX_EVENT_BYTES,
    DEFAULT_MAX_OUTPUT_BYTES,
    ProtocolVersionError,
    validate_protocol_version,
    validate_request_size,
)
from .registry import CollaborationEngineNotFound, CollaborationEngineRegistry
from .telemetry import CollaborationCallMetrics, CollaborationRuntimeTelemetry
from .v1 import collaboration_runtime_pb2, collaboration_runtime_pb2_grpc


LOGGER = logging.getLogger(__name__)


class CollaborationRuntimeServicer(
    collaboration_runtime_pb2_grpc.CollaborationRuntimeServiceServicer
):
    def __init__(
        self,
        registry: CollaborationEngineRegistry,
        *,
        active: ActiveCollaborationRegistry | None = None,
        capacity: CollaborationCapacityLimiter | None = None,
        max_concurrency: int = 4,
        max_pending: int = 16,
        max_request_bytes: int = DEFAULT_MAX_REQUEST_BYTES,
        max_event_bytes: int = DEFAULT_MAX_EVENT_BYTES,
        max_artifact_bytes: int = DEFAULT_MAX_ARTIFACT_BYTES,
        max_output_bytes: int = DEFAULT_MAX_OUTPUT_BYTES,
        max_checkpoint_bytes: int = DEFAULT_MAX_CHECKPOINT_BYTES,
    ) -> None:
        if any(
            value <= 0
            for value in (
                max_request_bytes,
                max_event_bytes,
                max_artifact_bytes,
                max_output_bytes,
                max_checkpoint_bytes,
            )
        ):
            raise ValueError("Collaboration Runtime resource limits must be positive")
        self.registry = registry
        self.active = active or ActiveCollaborationRegistry()
        self.capacity = capacity or CollaborationCapacityLimiter(max_concurrency, max_pending)
        self.max_request_bytes = max_request_bytes
        self.max_event_bytes = max_event_bytes
        self.max_artifact_bytes = max_artifact_bytes
        self.max_output_bytes = max_output_bytes
        self.max_checkpoint_bytes = max_checkpoint_bytes
        self.telemetry = CollaborationRuntimeTelemetry(LOGGER)

    async def ExecuteConversation(self, request, context):  # noqa: N802
        try:
            self._validate_request(request)
            neutral_request = map_request(request)
            neutral_request = self._apply_service_limits(neutral_request)
        except ProtocolVersionError:
            await context.abort(
                grpc.StatusCode.UNIMPLEMENTED,
                "Collaboration protocol version is unsupported",
            )
            return
        except ResourceLimitError:
            await context.abort(
                grpc.StatusCode.RESOURCE_EXHAUSTED,
                "Collaboration request exceeds resource limits",
            )
            return
        except ValueError:
            await context.abort(
                grpc.StatusCode.INVALID_ARGUMENT,
                "Collaboration request is invalid",
            )
            return

        run = CollaborationRunContext(neutral_request)
        if await self.active.register(run) is None:
            await context.abort(
                grpc.StatusCode.ALREADY_EXISTS,
                "Collaboration run is already active",
            )
            return

        call_metrics = self.telemetry.begin(neutral_request)
        try:
            async with self.capacity.slot(neutral_request.room.id):
                self.telemetry.activate(neutral_request, call_metrics)
                async for event in self._execute_admitted(
                    neutral_request,
                    run,
                    context,
                    call_metrics,
                ):
                    self.telemetry.observe(call_metrics, event.WhichOneof("payload"))
                    yield event
        except CollaborationCapacityExceeded:
            call_metrics.grpc_status = "RESOURCE_EXHAUSTED"
            await context.abort(
                grpc.StatusCode.RESOURCE_EXHAUSTED,
                "Collaboration Runtime waiting capacity is full",
            )
            return
        except CollaborationRoomBusy:
            call_metrics.grpc_status = "FAILED_PRECONDITION"
            await context.abort(
                grpc.StatusCode.FAILED_PRECONDITION,
                "Collaboration room already has an active run",
            )
            return
        except asyncio.CancelledError:
            run.cancel()
            remaining = getattr(context, "time_remaining", lambda: None)()
            if remaining == 0:
                call_metrics.outcome = "timeout"
                call_metrics.grpc_status = "DEADLINE_EXCEEDED"
            else:
                call_metrics.outcome = "cancelled"
                call_metrics.grpc_status = "CANCELLED"
            raise
        finally:
            self.telemetry.finish(neutral_request, call_metrics)
            run.cancel()
            await self.active.unregister(neutral_request.collaboration_run_id)

    async def cancel_active(self) -> None:
        await self.active.cancel_all()

    async def _execute_admitted(
        self,
        request,
        run,
        context,
        call_metrics: CollaborationCallMetrics,
    ):
        try:
            engine = self.registry.resolve(request.engine)
        except CollaborationEngineNotFound:
            call_metrics.grpc_status = "UNAVAILABLE"
            await context.abort(
                grpc.StatusCode.UNAVAILABLE,
                "Collaboration Engine is unavailable",
            )
            return
        except Exception:
            call_metrics.grpc_status = "UNAVAILABLE"
            LOGGER.error(
                "Collaboration Engine initialization failed",
                extra={
                    "collaboration_run_id": request.collaboration_run_id,
                    "room_id": request.room.id,
                    "trace_id": request.trace_id,
                    "engine": request.engine,
                },
            )
            await context.abort(
                grpc.StatusCode.UNAVAILABLE,
                "Collaboration Engine is unavailable",
            )
            return

        writer = CollaborationEventWriter(
            request.collaboration_run_id,
            max_event_bytes=request.limits.max_event_bytes,
            max_artifact_bytes=request.limits.max_artifact_bytes,
            max_output_bytes=request.limits.max_output_bytes,
            max_checkpoint_bytes=request.limits.max_checkpoint_bytes,
            allowed_agent_ids=frozenset(agent.id for agent in request.agents),
        )
        try:
            yield writer.accepted()
            try:
                stream = engine.execute(request, run.cancel_event)
                try:
                    async for item in stream:
                        if context.cancelled():
                            raise asyncio.CancelledError
                        event = writer.write(item)
                        yield event
                        if writer.terminal:
                            break
                finally:
                    close = getattr(stream, "aclose", None)
                    if close is not None:
                        await close()
                if not writer.terminal:
                    yield writer.failed("protocol_error", "protocol_error")
            except ResourceLimitError:
                raise
            except EventSequenceError:
                if not writer.terminal:
                    yield writer.failed("protocol_error", "protocol_error")
            except asyncio.CancelledError:
                run.cancel()
                raise
            except Exception:
                LOGGER.error(
                    "Collaboration Engine failed",
                    extra={
                        "collaboration_run_id": request.collaboration_run_id,
                        "room_id": request.room.id,
                        "trace_id": request.trace_id,
                        "engine": request.engine,
                    },
                )
                if not writer.terminal:
                    yield writer.failed("engine_failure", "internal")
        except ResourceLimitError:
            call_metrics.grpc_status = "RESOURCE_EXHAUSTED"
            await context.abort(
                grpc.StatusCode.RESOURCE_EXHAUSTED,
                "Collaboration event exceeds resource limits",
            )
            return

    def _apply_service_limits(self, request):
        limits = replace(
            request.limits,
            max_event_bytes=min(request.limits.max_event_bytes, self.max_event_bytes),
            max_artifact_bytes=min(request.limits.max_artifact_bytes, self.max_artifact_bytes),
            max_output_bytes=min(request.limits.max_output_bytes, self.max_output_bytes),
            max_checkpoint_bytes=min(
                request.limits.max_checkpoint_bytes,
                self.max_checkpoint_bytes,
            ),
        )
        if request.checkpoint is not None and len(request.checkpoint.payload) > limits.max_checkpoint_bytes:
            raise ResourceLimitError("checkpoint exceeds the configured limit")
        return replace(request, limits=limits)

    def _validate_request(
        self,
        request: collaboration_runtime_pb2.ExecuteConversationRequest,
    ) -> None:
        try:
            validate_request_size(request, self.max_request_bytes)
        except ValueError as exc:
            raise ResourceLimitError("ExecuteConversationRequest exceeds the configured limit") from exc

        validate_protocol_version(request.protocol_version)
        if not request.collaboration_run_id.strip():
            raise ValueError("collaboration_run_id is required")
        if not request.trace_id.strip():
            raise ValueError("trace_id is required")
        if request.engine == collaboration_runtime_pb2.COLLABORATION_ENGINE_UNSPECIFIED:
            raise ValueError("engine is required")
        if not request.HasField("snapshot"):
            raise ValueError("conversation snapshot is required")

        snapshot = request.snapshot
        if not snapshot.HasField("room") or not snapshot.room.id.strip():
            raise ValueError("room snapshot is required")
        if not snapshot.HasField("trigger") or not snapshot.trigger.id.strip():
            raise ValueError("trigger message is required")
        if snapshot.trigger.sender_type != collaboration_runtime_pb2.SENDER_TYPE_HUMAN:
            raise ValueError("trigger message must be human")
        if not snapshot.HasField("policy") or not snapshot.policy.version.strip():
            raise ValueError("collaboration policy is required")
        if snapshot.policy.engine != request.engine:
            raise ValueError("request and policy Engines must match")
        if snapshot.policy.trigger_mode == collaboration_runtime_pb2.TRIGGER_MODE_UNSPECIFIED:
            raise ValueError("trigger mode is required")
        if snapshot.policy.max_turns == 0 or snapshot.policy.max_turns_per_agent == 0:
            raise ValueError("turn limits must be positive")
        if snapshot.policy.max_turns_per_agent > snapshot.policy.max_turns:
            raise ValueError("per-Agent turn limit exceeds total turn limit")
        if (
            snapshot.policy.HasField("cooldown")
            and snapshot.policy.cooldown.ToTimedelta().total_seconds() < 0
        ):
            raise ValueError("collaboration cooldown cannot be negative")

        if not snapshot.HasField("limits"):
            raise ValueError("execution limits are required")
        limits = snapshot.limits
        positive_limits = (
            limits.max_output_bytes,
            limits.max_artifact_bytes,
            limits.max_tool_steps,
            limits.max_request_bytes,
            limits.max_event_bytes,
            limits.max_checkpoint_bytes,
        )
        if any(value == 0 for value in positive_limits):
            raise ValueError("execution limits must be positive")
        try:
            validate_request_size(
                request,
                min(self.max_request_bytes, limits.max_request_bytes),
            )
        except ValueError as exc:
            raise ResourceLimitError("ExecuteConversationRequest exceeds the configured limit") from exc

        agent_ids: set[str] = set()
        referenced_models: set[str] = set()
        if not snapshot.agents:
            raise ValueError("at least one Agent snapshot is required")
        for agent in snapshot.agents:
            agent_id = agent.id.strip()
            if not agent_id or agent_id in agent_ids:
                raise ValueError("Agent IDs must be non-empty and unique")
            if not agent.name.strip() or not agent.runtime.strip() or not agent.model_reference_id.strip():
                raise ValueError("Agent identity, runtime, and model reference are required")
            agent_ids.add(agent_id)
            referenced_models.add(agent.model_reference_id)

        model_ids: set[str] = set()
        for model in snapshot.model_references:
            model_id = model.id.strip()
            if not model_id or model_id in model_ids:
                raise ValueError("model reference IDs must be non-empty and unique")
            if not all(
                value.strip()
                for value in (
                    model.profile_id,
                    model.source,
                    model.protocol,
                    model.model_name,
                    model.runtime_scope,
                )
            ):
                raise ValueError("model reference metadata is required")
            model_ids.add(model_id)
        if not referenced_models.issubset(model_ids):
            raise ValueError("Agent references an unknown model")

        candidate_ids = list(snapshot.initial_candidate_agent_ids)
        if len(set(candidate_ids)) != len(candidate_ids) or not set(candidate_ids).issubset(agent_ids):
            raise ValueError("initial candidates must be unique Agents from the snapshot")

        for message in snapshot.transcript:
            if not message.id.strip():
                raise ValueError("transcript message ID is required")
            if message.sender_type == collaboration_runtime_pb2.SENDER_TYPE_UNSPECIFIED:
                raise ValueError("transcript sender type is required")

        if request.HasField("checkpoint"):
            checkpoint = request.checkpoint
            if checkpoint.engine != request.engine:
                raise ValueError("checkpoint Engine does not match request Engine")
            if not checkpoint.engine_version or not checkpoint.format_version or not checkpoint.sha256:
                raise ValueError("checkpoint metadata is required")
