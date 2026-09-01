from __future__ import annotations

import asyncio
from collections import deque
from collections.abc import Iterable
from dataclasses import dataclass
from enum import StrEnum
from typing import Protocol, runtime_checkable

from .models import ModelSelection


class CollaborationModelPurpose(StrEnum):
    SELECTOR = "selector"
    PARTICIPANT = "participant"


class CollaborationModelErrorCode(StrEnum):
    NOT_CONFIGURED = "model_not_configured"
    AUTHENTICATION_FAILED = "model_authentication_failed"
    RATE_LIMITED = "model_rate_limited"
    TIMEOUT = "model_timeout"
    INTERNAL = "internal"


class CollaborationModelError(RuntimeError):
    def __init__(
        self,
        code: CollaborationModelErrorCode,
        message: str,
        *,
        retryable: bool = False,
    ) -> None:
        super().__init__(message)
        self.code = code
        self.retryable = retryable


@dataclass(frozen=True)
class CollaborationModelMessage:
    role: str
    content: str
    name: str = ""


@dataclass(frozen=True)
class CollaborationModelRequest:
    request_id: str
    collaboration_run_id: str
    trace_id: str
    purpose: CollaborationModelPurpose
    model_selection: ModelSelection
    messages: tuple[CollaborationModelMessage, ...]
    agent_id: str = ""
    candidate_agent_ids: tuple[str, ...] = ()
    deadline_monotonic: float | None = None


@dataclass(frozen=True)
class CollaborationModelUsage:
    input_tokens: int = 0
    output_tokens: int = 0
    total_tokens: int = 0


@dataclass(frozen=True)
class CollaborationModelResponse:
    content: str
    usage: CollaborationModelUsage = CollaborationModelUsage()


@runtime_checkable
class CollaborationModelClient(Protocol):
    async def complete(
        self,
        request: CollaborationModelRequest,
        cancel_event: asyncio.Event,
    ) -> CollaborationModelResponse: ...


class FakeCollaborationModelClient:
    def __init__(
        self,
        responses: Iterable[CollaborationModelResponse | BaseException] = (),
    ) -> None:
        self._responses = deque(responses)
        self._requests: list[CollaborationModelRequest] = []

    @property
    def requests(self) -> tuple[CollaborationModelRequest, ...]:
        return tuple(self._requests)

    async def complete(
        self,
        request: CollaborationModelRequest,
        cancel_event: asyncio.Event,
    ) -> CollaborationModelResponse:
        if cancel_event.is_set():
            raise asyncio.CancelledError

        self._requests.append(request)
        await asyncio.sleep(0)
        if cancel_event.is_set():
            raise asyncio.CancelledError
        if not self._responses:
            raise AssertionError("no fake collaboration model response configured")

        response = self._responses.popleft()
        if isinstance(response, BaseException):
            raise response
        return response


from agent_runtime.model_config import ModelConfig


@runtime_checkable
class ModelClientFactory(Protocol):
    def create(self, config: ModelConfig) -> CollaborationModelClient: ...
