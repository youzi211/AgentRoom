from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator
from typing import Protocol

from .models import CollaborationRequest, EngineEvent


class CollaborationEngine(Protocol):
    name: str
    version: str

    def execute(
        self,
        request: CollaborationRequest,
        cancel_event: asyncio.Event,
    ) -> AsyncIterator[EngineEvent]: ...
