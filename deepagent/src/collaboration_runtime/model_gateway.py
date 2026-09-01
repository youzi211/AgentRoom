from __future__ import annotations

import asyncio
import time
from dataclasses import dataclass
from enum import StrEnum
from typing import Protocol, runtime_checkable

from .model_client import (
    CollaborationModelError,
    CollaborationModelErrorCode,
    CollaborationModelRequest,
    CollaborationModelResponse,
    CollaborationModelUsage,
)


@dataclass(frozen=True)
class ModelGatewayMessage:
    role: str
    content: str
    name: str = ""


@dataclass(frozen=True)
class ModelGatewayCapabilities:
    text_generation: bool = True


@dataclass(frozen=True)
class ModelGatewayRequest:
    request_id: str
    trace_id: str
    purpose: str
    profile_id: str
    source: str
    protocol: str
    model_name: str
    runtime_scope: str
    messages: tuple[ModelGatewayMessage, ...]
    agent_id: str = ""
    candidate_agent_ids: tuple[str, ...] = ()


@dataclass(frozen=True)
class ModelGatewayUsage:
    input_tokens: int = 0
    output_tokens: int = 0
    total_tokens: int = 0


@dataclass(frozen=True)
class ModelGatewayResponse:
    content: str
    usage: ModelGatewayUsage = ModelGatewayUsage()


class ModelGatewayErrorCode(StrEnum):
    NOT_CONFIGURED = "model_not_configured"
    AUTHENTICATION_FAILED = "model_authentication_failed"
    RATE_LIMITED = "model_rate_limited"
    TIMEOUT = "model_timeout"
    INTERNAL = "internal"


class ModelGatewayError(RuntimeError):
    def __init__(
        self,
        code: ModelGatewayErrorCode,
        *,
        retryable: bool = False,
    ) -> None:
        super().__init__(code.value)
        self.code = code
        self.retryable = retryable


@runtime_checkable
class ModelGatewayCore(Protocol):
    def ready(self) -> bool: ...

    def capabilities(self, request: ModelGatewayRequest) -> ModelGatewayCapabilities: ...

    async def generate(
        self,
        request: ModelGatewayRequest,
        cancel_event: asyncio.Event,
    ) -> ModelGatewayResponse: ...


class ModelGatewayCollaborationModelClient:
    def __init__(self, core: ModelGatewayCore) -> None:
        self._core = core

    async def complete(
        self,
        request: CollaborationModelRequest,
        cancel_event: asyncio.Event,
    ) -> CollaborationModelResponse:
        if cancel_event.is_set():
            raise asyncio.CancelledError

        gateway_request = _gateway_request(request)
        try:
            capabilities = self._core.capabilities(gateway_request)
            if not capabilities.text_generation:
                raise CollaborationModelError(
                    CollaborationModelErrorCode.NOT_CONFIGURED,
                    "Model Profile does not support text generation",
                )
            gateway_response = await _generate(
                self._core,
                gateway_request,
                cancel_event,
                request.deadline_monotonic,
            )
            usage = _usage(gateway_response.usage)
        except CollaborationModelError:
            raise
        except ModelGatewayError as exc:
            raise _gateway_error(exc) from exc
        except TimeoutError as exc:
            raise CollaborationModelError(
                CollaborationModelErrorCode.TIMEOUT,
                "Model request timed out",
                retryable=True,
            ) from exc
        except Exception as exc:
            raise CollaborationModelError(
                CollaborationModelErrorCode.INTERNAL,
                "Model request failed",
            ) from exc

        return CollaborationModelResponse(
            content=gateway_response.content,
            usage=usage,
        )


def _gateway_request(request: CollaborationModelRequest) -> ModelGatewayRequest:
    model = request.model_selection
    if not model.profile_id or not model.protocol or not model.model_name:
        raise CollaborationModelError(
            CollaborationModelErrorCode.NOT_CONFIGURED,
            "Model Profile is incomplete",
        )
    return ModelGatewayRequest(
        request_id=request.request_id,
        trace_id=request.trace_id,
        purpose=request.purpose.value,
        profile_id=model.profile_id,
        source=model.source,
        protocol=model.protocol,
        model_name=model.model_name,
        runtime_scope=model.runtime_scope,
        messages=tuple(
            ModelGatewayMessage(
                role=message.role,
                content=message.content,
                name=message.name,
            )
            for message in request.messages
        ),
        agent_id=request.agent_id,
        candidate_agent_ids=request.candidate_agent_ids,
    )


async def _generate(
    core: ModelGatewayCore,
    request: ModelGatewayRequest,
    cancel_event: asyncio.Event,
    deadline_monotonic: float | None,
) -> ModelGatewayResponse:
    timeout = None
    if deadline_monotonic is not None:
        timeout = deadline_monotonic - time.monotonic()
        if timeout <= 0:
            raise TimeoutError

    generation = asyncio.create_task(core.generate(request, cancel_event))
    cancellation = asyncio.create_task(cancel_event.wait())
    try:
        done, _ = await asyncio.wait(
            (generation, cancellation),
            timeout=timeout,
            return_when=asyncio.FIRST_COMPLETED,
        )
        if cancellation in done and cancel_event.is_set():
            raise asyncio.CancelledError
        if generation in done:
            return generation.result()
        raise TimeoutError
    finally:
        for task in (generation, cancellation):
            if not task.done():
                task.cancel()
        await asyncio.gather(generation, cancellation, return_exceptions=True)


def _usage(usage: ModelGatewayUsage) -> CollaborationModelUsage:
    values = (usage.input_tokens, usage.output_tokens, usage.total_tokens)
    if any(value < 0 for value in values):
        raise ValueError("Model Gateway returned invalid usage")
    total_tokens = max(
        usage.total_tokens,
        usage.input_tokens + usage.output_tokens,
    )
    return CollaborationModelUsage(
        input_tokens=usage.input_tokens,
        output_tokens=usage.output_tokens,
        total_tokens=total_tokens,
    )


def _gateway_error(error: ModelGatewayError) -> CollaborationModelError:
    code = CollaborationModelErrorCode(error.code.value)
    messages = {
        CollaborationModelErrorCode.NOT_CONFIGURED: "Model Profile is not configured",
        CollaborationModelErrorCode.AUTHENTICATION_FAILED: "Model authentication failed",
        CollaborationModelErrorCode.RATE_LIMITED: "Model request was rate limited",
        CollaborationModelErrorCode.TIMEOUT: "Model request timed out",
        CollaborationModelErrorCode.INTERNAL: "Model request failed",
    }
    return CollaborationModelError(code, messages[code], retryable=error.retryable)
