from __future__ import annotations

import asyncio
import shutil
import tempfile
from dataclasses import dataclass, field
from pathlib import Path

from .v1 import agent_runtime_pb2


@dataclass
class RunContext:
    request: agent_runtime_pb2.ExecuteAgentRequest
    work_dir: Path
    cancel_event: asyncio.Event = field(default_factory=asyncio.Event)

    @classmethod
    def create(
        cls,
        request: agent_runtime_pb2.ExecuteAgentRequest,
        work_root: Path,
    ) -> "RunContext":
        work_root.mkdir(parents=True, exist_ok=True)
        work_dir = Path(tempfile.mkdtemp(prefix="run-", dir=work_root))
        return cls(request=request, work_dir=work_dir)

    def cleanup(self) -> None:
        self.cancel_event.set()
        if self.request.HasField("model"):
            self.request.model.api_key = ""
        shutil.rmtree(self.work_dir, ignore_errors=True)


class ActiveRunRegistry:
    def __init__(self) -> None:
        self._lock = asyncio.Lock()
        self._tasks: dict[str, asyncio.Task[object]] = {}
        self._empty = asyncio.Event()
        self._empty.set()

    async def register(self, run_id: str) -> bool:
        task = asyncio.current_task()
        if task is None:
            raise RuntimeError("ExecuteAgent must run inside an asyncio task")
        async with self._lock:
            if run_id in self._tasks:
                return False
            self._tasks[run_id] = task
            self._empty.clear()
            return True

    async def unregister(self, run_id: str) -> None:
        async with self._lock:
            self._tasks.pop(run_id, None)
            if not self._tasks:
                self._empty.set()

    async def cancel_all(self) -> None:
        async with self._lock:
            tasks = list(self._tasks.values())
        current = asyncio.current_task()
        for task in tasks:
            if task is not current and not task.done():
                task.cancel()

    async def wait_empty(self, timeout: float | None = None) -> None:
        if timeout is None:
            await self._empty.wait()
            return
        await asyncio.wait_for(self._empty.wait(), timeout=timeout)

    async def count(self) -> int:
        async with self._lock:
            return len(self._tasks)
