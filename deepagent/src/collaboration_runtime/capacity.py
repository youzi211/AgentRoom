from __future__ import annotations

import asyncio
from contextlib import asynccontextmanager
from typing import AsyncIterator


class CollaborationCapacityExceeded(RuntimeError):
    """Raised when the bounded collaboration waiting queue is full."""


class CollaborationRoomBusy(RuntimeError):
    """Raised when a room already has a pending or active run."""


class CollaborationCapacityLimiter:
    def __init__(self, max_concurrency: int, max_pending: int) -> None:
        if max_concurrency <= 0:
            raise ValueError("max_concurrency must be positive")
        if max_pending < 0:
            raise ValueError("max_pending must not be negative")
        self._max_concurrency = max_concurrency
        self._max_pending = max_pending
        self._active = 0
        self._pending = 0
        self._rooms: set[str] = set()
        self._condition = asyncio.Condition()

    @asynccontextmanager
    async def slot(self, room_id: str) -> AsyncIterator[None]:
        if not room_id:
            raise ValueError("room_id is required")

        reserved = False
        active = False
        try:
            async with self._condition:
                if room_id in self._rooms:
                    raise CollaborationRoomBusy("room already has an active collaboration run")
                if self._active >= self._max_concurrency and self._pending >= self._max_pending:
                    raise CollaborationCapacityExceeded(
                        "Collaboration Runtime waiting capacity is full"
                    )

                self._rooms.add(room_id)
                reserved = True
                if self._active >= self._max_concurrency:
                    self._pending += 1
                    try:
                        await self._condition.wait_for(
                            lambda: self._active < self._max_concurrency
                        )
                    finally:
                        self._pending -= 1
                self._active += 1
                active = True

            yield
        finally:
            if active or reserved:
                async with self._condition:
                    if active:
                        self._active -= 1
                    if reserved:
                        self._rooms.discard(room_id)
                    self._condition.notify_all()

    async def active(self) -> int:
        async with self._condition:
            return self._active

    async def pending(self) -> int:
        async with self._condition:
            return self._pending

    async def room_count(self) -> int:
        async with self._condition:
            return len(self._rooms)
