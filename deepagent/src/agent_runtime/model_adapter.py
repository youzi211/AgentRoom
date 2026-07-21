from __future__ import annotations

import asyncio
from dataclasses import dataclass
from typing import AsyncIterator, Callable

from openai import (
    APIConnectionError,
    APIStatusError,
    APITimeoutError,
    AsyncOpenAI,
    AuthenticationError,
    BadRequestError,
    PermissionDeniedError,
    RateLimitError,
)

from .prompt import ChatMessage
from .sanitize import ThinkBlockFilter
from .v1 import agent_runtime_pb2


@dataclass(frozen=True)
class ModelDelta:
    text: str


@dataclass(frozen=True)
class ModelCompletion:
    content: str
    input_tokens: int = 0
    output_tokens: int = 0
    total_tokens: int = 0


class ModelAdapterError(RuntimeError):
    def __init__(self, code: int, safe_message: str, *, retryable: bool = False) -> None:
        super().__init__(safe_message)
        self.code = code
        self.safe_message = safe_message
        self.retryable = retryable


class OpenAIModelAdapter:
    def __init__(self, client_factory: Callable[..., object] = AsyncOpenAI) -> None:
        self._client_factory = client_factory

    async def stream(
        self,
        request: agent_runtime_pb2.ExecuteAgentRequest,
        messages: list[ChatMessage],
    ) -> AsyncIterator[ModelDelta | ModelCompletion]:
        connection = request.model
        if not connection.api_key:
            raise ModelAdapterError(
                agent_runtime_pb2.RUN_ERROR_CODE_MODEL_NOT_CONFIGURED,
                "Model API key is not configured",
            )
        if not connection.model_name:
            raise ModelAdapterError(
                agent_runtime_pb2.RUN_ERROR_CODE_MODEL_NOT_CONFIGURED,
                "Model name is not configured",
            )

        timeout = request.limits.timeout.ToSeconds() if request.limits.HasField("timeout") else 60.0
        content_parts: list[str] = []
        output_bytes = 0
        output_filter = ThinkBlockFilter()
        input_tokens = output_tokens = total_tokens = 0
        try:
            async with self._client_factory(
                api_key=connection.api_key,
                base_url=connection.base_url or None,
                timeout=timeout,
                max_retries=0,
            ) as client:
                response = await client.chat.completions.create(
                    model=connection.model_name,
                    messages=[{"role": message.role, "content": message.content} for message in messages],
                    stream=True,
                    stream_options={"include_usage": True},
                )
                async for chunk in response:
                    usage = getattr(chunk, "usage", None)
                    if usage is not None:
                        input_tokens = int(getattr(usage, "prompt_tokens", 0) or 0)
                        output_tokens = int(getattr(usage, "completion_tokens", 0) or 0)
                        total_tokens = int(getattr(usage, "total_tokens", 0) or 0)
                    choices = getattr(chunk, "choices", None) or []
                    if not choices:
                        continue
                    text = getattr(choices[0].delta, "content", None)
                    if not text:
                        continue
                    safe_text = output_filter.feed(text)
                    if not safe_text:
                        continue
                    output_bytes += len(safe_text.encode("utf-8"))
                    if request.limits.max_output_bytes and output_bytes > request.limits.max_output_bytes:
                        raise ModelAdapterError(
                            agent_runtime_pb2.RUN_ERROR_CODE_RESOURCE_EXHAUSTED,
                            "Model output exceeds the configured limit",
                        )
                    content_parts.append(safe_text)
                    yield ModelDelta(text=safe_text)
        except asyncio.CancelledError:
            raise
        except ModelAdapterError:
            raise
        except (AuthenticationError, PermissionDeniedError) as exc:
            raise ModelAdapterError(
                agent_runtime_pb2.RUN_ERROR_CODE_MODEL_AUTHENTICATION_FAILED,
                "Model authentication failed",
            ) from exc
        except RateLimitError as exc:
            raise ModelAdapterError(
                agent_runtime_pb2.RUN_ERROR_CODE_MODEL_RATE_LIMITED,
                "Model rate limit exceeded",
                retryable=True,
            ) from exc
        except APITimeoutError as exc:
            raise ModelAdapterError(
                agent_runtime_pb2.RUN_ERROR_CODE_MODEL_TIMEOUT,
                "Model request timed out",
                retryable=True,
            ) from exc
        except BadRequestError as exc:
            raise ModelAdapterError(
                agent_runtime_pb2.RUN_ERROR_CODE_INVALID_REQUEST,
                "Model rejected the request",
            ) from exc
        except APIConnectionError as exc:
            raise ModelAdapterError(
                agent_runtime_pb2.RUN_ERROR_CODE_EXECUTOR_UNAVAILABLE,
                "Model provider is unavailable",
                retryable=True,
            ) from exc
        except APIStatusError as exc:
            raise ModelAdapterError(
                agent_runtime_pb2.RUN_ERROR_CODE_INTERNAL,
                "Model provider request failed",
                retryable=exc.status_code >= 500,
            ) from exc

        tail = output_filter.finish()
        if tail:
            output_bytes += len(tail.encode("utf-8"))
            if request.limits.max_output_bytes and output_bytes > request.limits.max_output_bytes:
                raise ModelAdapterError(
                    agent_runtime_pb2.RUN_ERROR_CODE_RESOURCE_EXHAUSTED,
                    "Model output exceeds the configured limit",
                )
            content_parts.append(tail)
            yield ModelDelta(text=tail)
        content = "".join(content_parts).strip()
        if not content:
            raise ModelAdapterError(
                agent_runtime_pb2.RUN_ERROR_CODE_OUTPUT_INVALID,
                "Model returned empty output",
            )
        yield ModelCompletion(
            content=content,
            input_tokens=input_tokens,
            output_tokens=output_tokens,
            total_tokens=total_tokens,
        )
