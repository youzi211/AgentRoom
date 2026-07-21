from __future__ import annotations

import logging
import time
from collections import Counter
from dataclasses import dataclass

from .v1 import agent_runtime_pb2


@dataclass
class CallMetrics:
    started_at: float
    wait_started_at: float
    queue_ms: int = 0
    state: str = "waiting"


class RuntimeTelemetry:
    """Small in-process metric set with structured-log parity."""

    def __init__(self, logger: logging.Logger) -> None:
        self._logger = logger
        self.active = 0
        self.waiting = 0
        self.outcomes: Counter[str] = Counter()
        self.grpc_statuses: Counter[str] = Counter()

    def begin(self, request: agent_runtime_pb2.ExecuteAgentRequest) -> CallMetrics:
        now = time.monotonic()
        self.waiting += 1
        self._log("agent_run_waiting", request, active=self.active, waiting=self.waiting)
        return CallMetrics(started_at=now, wait_started_at=now)

    def activate(self, request: agent_runtime_pb2.ExecuteAgentRequest, call: CallMetrics) -> None:
        if call.state != "waiting":
            return
        call.queue_ms = int((time.monotonic() - call.wait_started_at) * 1000)
        call.state = "active"
        self.waiting -= 1
        self.active += 1
        self._log(
            "agent_run_started",
            request,
            active=self.active,
            waiting=self.waiting,
            queue_ms=call.queue_ms,
        )

    def finish(
        self,
        request: agent_runtime_pb2.ExecuteAgentRequest,
        call: CallMetrics,
        *,
        outcome: str,
        grpc_status: str = "OK",
    ) -> None:
        if call.state == "waiting":
            self.waiting -= 1
        elif call.state == "active":
            self.active -= 1
        call.state = "finished"
        self.outcomes[outcome] += 1
        self.grpc_statuses[grpc_status] += 1
        self._log(
            "agent_run_finished",
            request,
            active=self.active,
            waiting=self.waiting,
            outcome=outcome,
            grpc_status=grpc_status,
            queue_ms=call.queue_ms,
            duration_ms=int((time.monotonic() - call.started_at) * 1000),
        )

    def snapshot(self) -> dict[str, object]:
        return {
            "active": self.active,
            "waiting": self.waiting,
            "outcomes": dict(self.outcomes),
            "grpc_statuses": dict(self.grpc_statuses),
        }

    def _log(self, event: str, request: agent_runtime_pb2.ExecuteAgentRequest, **fields: object) -> None:
        self._logger.info(
            event,
            extra={
                "run_id": request.run_id,
                "room_id": request.room.id,
                "agent_id": request.agent.id,
                "dialogue_run_id": request.dialogue_run_id,
                "trace_id": request.trace_id,
                "executor_kind": agent_runtime_pb2.ExecutorKind.Name(request.executor_kind),
                **fields,
            },
        )
