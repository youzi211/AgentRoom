from __future__ import annotations

import asyncio
from dataclasses import dataclass
from typing import Protocol, runtime_checkable

from agent_runtime.model_config import ModelConfig, ModelConfigResolver


@dataclass(frozen=True)
class ModelResponse:
    content: str
    input_tokens: int = 0
    output_tokens: int = 0


@runtime_checkable
class ModelClient(Protocol):
    async def complete(self, config: ModelConfig, messages: tuple, *, cancel_event: asyncio.Event, timeout_seconds: float | None = None) -> ModelResponse: ...


@runtime_checkable
class ModelClientFactory(Protocol):
    def create(self, config: ModelConfig) -> ModelClient: ...


class ModelExecutionService:
    def __init__(self, resolver: ModelConfigResolver, factory: ModelClientFactory) -> None:
        self._resolver = resolver
        self._factory = factory

    @property
    def ready(self) -> bool:
        return self._resolver is not None and self._factory is not None

    async def complete(self, selection, messages: tuple, *, cancel_event: asyncio.Event, timeout_seconds: float | None = None):
        from .models import ModelSelection
        config = self._resolver.resolve(
            profile_id=selection.profile_id,
            source=selection.source,
            protocol=selection.protocol,
            model_name=selection.model_name,
            runtime_scope=selection.runtime_scope,
            credential_ref=selection.credential_ref,
        )
        client = self._factory.create(config)
        return await client.complete(
            config,
            messages,
            cancel_event=cancel_event,
            timeout_seconds=timeout_seconds,
        )
