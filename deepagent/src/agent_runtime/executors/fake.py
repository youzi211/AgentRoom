from __future__ import annotations

import asyncio
from typing import AsyncIterator

from ..context import RunContext
from ..events import EventPayload, payload
from ..v1 import agent_runtime_pb2


class FakeExecutor:
    """Deterministic Executor used only by contract and transport tests."""

    def __init__(self, kind: int = agent_runtime_pb2.EXECUTOR_KIND_LLM) -> None:
        self.kind = kind

    async def execute(self, run: RunContext) -> AsyncIterator[EventPayload]:
        mode = run.request.metadata.get("fake_mode", "success")
        delay_ms = int(run.request.metadata.get("fake_delay_ms", "0"))
        if delay_ms > 0:
            await asyncio.sleep(delay_ms / 1000)

        yield payload(
            "model_started",
            agent_runtime_pb2.ModelStartedEvent(model_name=run.request.model.model_name or "fake-model"),
        )

        if mode in {"tool", "artifact"}:
            yield payload(
                "tool_started",
                agent_runtime_pb2.ToolStartedEvent(
                    tool_call_id="tool_fake",
                    tool_name="fake_search",
                    input_summary="safe fake input",
                ),
            )
            yield payload(
                "tool_completed",
                agent_runtime_pb2.ToolCompletedEvent(
                    tool_call_id="tool_fake",
                    tool_name="fake_search",
                    output_summary="safe fake output",
                ),
            )

        if mode == "failed":
            yield payload(
                "failed",
                agent_runtime_pb2.FailedEvent(
                    failure=agent_runtime_pb2.RunFailure(
                        code=agent_runtime_pb2.RUN_ERROR_CODE_TOOL_FAILED,
                        message="fake failure",
                    )
                ),
            )
            return

        if mode == "out_of_order":
            yield payload("completed", agent_runtime_pb2.CompletedEvent(content="premature"))
            yield payload(
                "output_delta",
                agent_runtime_pb2.OutputDeltaEvent(text="event after terminal"),
            )
            return

        if mode == "stream":
            count = int(run.request.metadata.get("fake_delta_count", "32"))
            for index in range(count):
                yield payload(
                    "output_delta",
                    agent_runtime_pb2.OutputDeltaEvent(text=f"{index},"),
                )

        artifact = None
        if mode == "artifact":
            size = int(run.request.metadata.get("fake_artifact_bytes", "16"))
            artifact = agent_runtime_pb2.Artifact(
                id="artifact_fake",
                type="report",
                title="Fake report",
                file_name="report.md",
                mime_type="text/markdown",
                content=b"x" * size,
            )
            yield payload("artifact_ready", agent_runtime_pb2.ArtifactReadyEvent(artifact=artifact))

        yield payload(
            "model_completed",
            agent_runtime_pb2.ModelCompletedEvent(
                model_name=run.request.model.model_name or "fake-model",
                usage=agent_runtime_pb2.Usage(input_tokens=10, output_tokens=5, total_tokens=15),
            ),
        )
        content = run.request.metadata.get("fake_content", "fake response")
        if mode == "stream":
            count = int(run.request.metadata.get("fake_delta_count", "32"))
            content = "".join(f"{index}," for index in range(count))
        completed = agent_runtime_pb2.CompletedEvent(
            content=content,
            model=agent_runtime_pb2.ModelAudit(
                profile_id=run.request.model.profile_id,
                source=run.request.model.source,
                model_name=run.request.model.model_name or "fake-model",
            ),
        )
        if artifact is not None:
            completed.artifacts.append(artifact)
        yield payload("completed", completed)
