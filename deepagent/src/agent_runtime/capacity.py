from __future__ import annotations

import asyncio
from contextlib import asynccontextmanager
from typing import AsyncIterator

from .v1 import agent_runtime_pb2


class CapacityExceeded(RuntimeError):
    """Raised when the bounded waiting capacity is already full."""


class CapacityLimiter:
    def __init__(self, total: int, deepagent: int, max_pending: int) -> None:
        self._total_limit = total
        self._deepagent_limit = deepagent
        self._max_pending = max_pending
        self._active = 0
        self._deepagent_active = 0
        self._pending = 0
        self._condition = asyncio.Condition()

    @asynccontextmanager
    async def slot(self, executor_kind: int) -> AsyncIterator[None]:
        is_deepagent = executor_kind == agent_runtime_pb2.EXECUTOR_KIND_DEEPAGENT

        async with self._condition:
            waiting = not self._has_capacity(is_deepagent)
            if waiting:
                if self._pending >= self._max_pending:
                    raise CapacityExceeded("Agent Runtime waiting capacity is full")
                self._pending += 1
                try:
                    await self._condition.wait_for(lambda: self._has_capacity(is_deepagent))
                finally:
                    self._pending -= 1
            self._active += 1
            if is_deepagent:
                self._deepagent_active += 1

        try:
            yield
        finally:
            async with self._condition:
                self._active -= 1
                if is_deepagent:
                    self._deepagent_active -= 1
                self._condition.notify_all()

    async def pending(self) -> int:
        async with self._condition:
            return self._pending

    def _has_capacity(self, is_deepagent: bool) -> bool:
        return self._active < self._total_limit and (
            not is_deepagent or self._deepagent_active < self._deepagent_limit
        )
