from __future__ import annotations

import asyncio
from dataclasses import dataclass, field
from enum import StrEnum

from .models import CollaborationRequest


class RunNamespace(StrEnum):
    AGENT = "agent"
    COLLABORATION = "collaboration"


@dataclass(frozen=True)
class RunIdentity:
    namespace: RunNamespace
    value: str


@dataclass
class CollaborationRunContext:
    request: CollaborationRequest
    cancel_event: asyncio.Event = field(default_factory=asyncio.Event)

    @property
    def identity(self) -> RunIdentity:
        return RunIdentity(RunNamespace.COLLABORATION, self.request.collaboration_run_id)

    def cancel(self) -> None:
        self.cancel_event.set()


@dataclass(frozen=True)
class CollaborationCancellationHandle:
    identity: RunIdentity
    cancel_event: asyncio.Event

    def cancel(self) -> None:
        self.cancel_event.set()


class ActiveCollaborationRegistry:
    def __init__(self) -> None:
        self._lock = asyncio.Lock()
        self._handles: dict[RunIdentity, CollaborationCancellationHandle] = {}
        self._empty = asyncio.Event()
        self._empty.set()

    async def register(
        self,
        context: CollaborationRunContext,
    ) -> CollaborationCancellationHandle | None:
        identity = context.identity
        handle = CollaborationCancellationHandle(identity, context.cancel_event)
        async with self._lock:
            if identity in self._handles:
                return None
            self._handles[identity] = handle
            self._empty.clear()
            return handle

    async def unregister(self, run_id: str) -> None:
        identity = RunIdentity(RunNamespace.COLLABORATION, run_id)
        async with self._lock:
            self._handles.pop(identity, None)
            if not self._handles:
                self._empty.set()

    async def cancel(self, run_id: str) -> bool:
        identity = RunIdentity(RunNamespace.COLLABORATION, run_id)
        async with self._lock:
            handle = self._handles.get(identity)
        if handle is None:
            return False
        handle.cancel()
        return True

    async def cancel_all(self) -> None:
        async with self._lock:
            handles = list(self._handles.values())
        for handle in handles:
            handle.cancel()

    async def wait_empty(self, timeout: float | None = None) -> None:
        if timeout is None:
            await self._empty.wait()
            return
        await asyncio.wait_for(self._empty.wait(), timeout=timeout)

    async def count(self) -> int:
        async with self._lock:
            return len(self._handles)
