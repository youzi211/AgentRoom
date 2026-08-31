from __future__ import annotations

import logging
import time
from collections import Counter
from dataclasses import dataclass

from .models import CollaborationRequest


@dataclass
class CollaborationCallMetrics:
    started_at: float
    wait_started_at: float
    queue_ms: int = 0
    state: str = "waiting"
    outcome: str = "failed"
    grpc_status: str = "OK"
    event_count: int = 0
    turn_count: int = 0


class CollaborationRuntimeTelemetry:
    def __init__(self, logger: logging.Logger) -> None:
        self._logger = logger
        self.active = 0
        self.waiting = 0
        self.outcomes: Counter[str] = Counter()
        self.grpc_statuses: Counter[str] = Counter()
        self.engines: Counter[str] = Counter()
        self.events = 0
        self.turns = 0

    def begin(self, request: CollaborationRequest) -> CollaborationCallMetrics:
        now = time.monotonic()
        self.waiting += 1
        self._log(
            "collaboration_run_waiting",
            request,
            active=self.active,
            waiting=self.waiting,
        )
        return CollaborationCallMetrics(started_at=now, wait_started_at=now)

    def activate(
        self,
        request: CollaborationRequest,
        call: CollaborationCallMetrics,
    ) -> None:
        if call.state != "waiting":
            return
        call.queue_ms = int((time.monotonic() - call.wait_started_at) * 1000)
        call.state = "active"
        self.waiting -= 1
        self.active += 1
        self.engines[request.engine] += 1
        self._log(
            "collaboration_run_started",
            request,
            active=self.active,
            waiting=self.waiting,
            queue_ms=call.queue_ms,
        )

    def observe(self, call: CollaborationCallMetrics, payload: str) -> None:
        call.event_count += 1
        self.events += 1
        if payload == "agent_message_completed":
            call.turn_count += 1
            self.turns += 1
        call.outcome = {
            "completed": "succeeded",
            "stopped": "stopped",
            "cancelled": "cancelled",
            "failed": "failed",
        }.get(payload, call.outcome)

    def finish(
        self,
        request: CollaborationRequest,
        call: CollaborationCallMetrics,
    ) -> None:
        if call.state == "waiting":
            self.waiting -= 1
        elif call.state == "active":
            self.active -= 1
        call.state = "finished"
        self.outcomes[call.outcome] += 1
        self.grpc_statuses[call.grpc_status] += 1
        self._log(
            "collaboration_run_finished",
            request,
            active=self.active,
            waiting=self.waiting,
            outcome=call.outcome,
            grpc_status=call.grpc_status,
            queue_ms=call.queue_ms,
            duration_ms=int((time.monotonic() - call.started_at) * 1000),
            event_count=call.event_count,
            turn_count=call.turn_count,
        )

    def snapshot(self) -> dict[str, object]:
        return {
            "active": self.active,
            "waiting": self.waiting,
            "outcomes": dict(self.outcomes),
            "grpc_statuses": dict(self.grpc_statuses),
            "engines": dict(self.engines),
            "events": self.events,
            "turns": self.turns,
        }

    def _log(
        self,
        event: str,
        request: CollaborationRequest,
        **fields: object,
    ) -> None:
        self._logger.info(
            event,
            extra={
                "collaboration_run_id": request.collaboration_run_id,
                "room_id": request.room.id,
                "trace_id": request.trace_id,
                "engine": request.engine,
                **fields,
            },
        )
