from __future__ import annotations

import asyncio
from typing import AsyncIterator

from ..context import RunContext
from ..events import EventPayload, payload
from ..model_adapter import ModelAdapterError, ModelCompletion, ModelDelta, OpenAIModelAdapter
from ..prompt import compose_prompt
from ..v1 import agent_runtime_pb2


class LLMExecutor:
    kind = agent_runtime_pb2.EXECUTOR_KIND_LLM

    def __init__(self, adapter: OpenAIModelAdapter | None = None) -> None:
        self.adapter = adapter or OpenAIModelAdapter()

    async def execute(self, run: RunContext) -> AsyncIterator[EventPayload]:
        request = run.request
        model_name = request.model.model_name
        yield payload("model_started", agent_runtime_pb2.ModelStartedEvent(model_name=model_name))

        completion: ModelCompletion | None = None
        try:
            messages = compose_prompt(request)
            async for item in self.adapter.stream(request, messages):
                if isinstance(item, ModelDelta):
                    yield payload("output_delta", agent_runtime_pb2.OutputDeltaEvent(text=item.text))
                else:
                    completion = item
        except asyncio.CancelledError:
            raise
        except ModelAdapterError as exc:
            yield payload(
                "failed",
                agent_runtime_pb2.FailedEvent(
                    failure=agent_runtime_pb2.RunFailure(
                        code=exc.code,
                        message=exc.safe_message,
                        retryable=exc.retryable,
                    )
                ),
            )
            return

        if completion is None:
            yield payload(
                "failed",
                agent_runtime_pb2.FailedEvent(
                    failure=agent_runtime_pb2.RunFailure(
                        code=agent_runtime_pb2.RUN_ERROR_CODE_OUTPUT_INVALID,
                        message="Model stream ended without a completion",
                    )
                ),
            )
            return

        usage = agent_runtime_pb2.Usage(
            input_tokens=completion.input_tokens,
            output_tokens=completion.output_tokens,
            total_tokens=completion.total_tokens,
        )
        yield payload(
            "model_completed",
            agent_runtime_pb2.ModelCompletedEvent(model_name=model_name, usage=usage),
        )
        completed = agent_runtime_pb2.CompletedEvent(
            content=completion.content,
            model=agent_runtime_pb2.ModelAudit(
                profile_id=request.model.profile_id,
                source=request.model.source,
                model_name=model_name,
            ),
            usage=usage,
        )
        seen_sources: set[tuple[str, str, str]] = set()
        for chunk in request.knowledge_chunks:
            key = (chunk.document_id, chunk.document_name, chunk.scope)
            if key in seen_sources or not (chunk.document_id or chunk.document_name):
                continue
            seen_sources.add(key)
            completed.knowledge_sources.append(
                agent_runtime_pb2.KnowledgeSource(
                    document_id=chunk.document_id,
                    document_name=chunk.document_name,
                    scope=chunk.scope,
                )
            )
        yield payload("completed", completed)
