from __future__ import annotations

from typing import AsyncIterator, Protocol

from ..context import RunContext
from ..events import EventPayload


class Executor(Protocol):
    kind: int

    def execute(self, run: RunContext) -> AsyncIterator[EventPayload]:
        """Yield ordered payloads, including exactly one completed or failed."""
        ...
