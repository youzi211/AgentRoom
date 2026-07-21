from __future__ import annotations

import asyncio
import os
from pathlib import Path
from typing import AsyncIterator, Callable

from openai import AuthenticationError, APITimeoutError, PermissionDeniedError, RateLimitError

from agentroom_deepagent.config import CustomEndpoint, MissingCredentials, PROJECT_DIR, Settings
from agentroom_deepagent.report import RunRecorder
from agentroom_deepagent.research import ResearchStreamEvent, stream_research
from agentroom_deepagent.tools import SearchToolError

from ..context import RunContext
from ..events import EventPayload, payload
from ..v1 import agent_runtime_pb2


ResearchRunner = Callable[[str, Settings, RunRecorder], AsyncIterator[ResearchStreamEvent]]


class DeepAgentExecutor:
    kind = agent_runtime_pb2.EXECUTOR_KIND_DEEPAGENT

    def __init__(self, runner: ResearchRunner = stream_research) -> None:
        self._runner = runner

    async def execute(self, run: RunContext) -> AsyncIterator[EventPayload]:
        question = _research_question(run)
        if not question:
            yield _failed(
                agent_runtime_pb2.RUN_ERROR_CODE_INVALID_REQUEST,
                "DeepAgent research question is empty",
            )
            return

        settings = _request_settings(run)
        recorder = RunRecorder(run.work_dir, "research")
        report_path: Path | None = None
        model_name = run.request.model.model_name
        try:
            async for event in self._runner(question, settings, recorder):
                if run.cancel_event.is_set():
                    raise asyncio.CancelledError
                if event.type == "model_started":
                    yield payload(
                        "model_started",
                        agent_runtime_pb2.ModelStartedEvent(model_name=model_name),
                    )
                elif event.type == "model_completed":
                    yield payload(
                        "model_completed",
                        agent_runtime_pb2.ModelCompletedEvent(model_name=model_name),
                    )
                elif event.type == "tool_started":
                    yield payload(
                        "tool_started",
                        agent_runtime_pb2.ToolStartedEvent(
                            tool_call_id=event.call_id,
                            tool_name=event.name,
                            input_summary="Research search requested",
                        ),
                    )
                elif event.type == "tool_completed":
                    yield payload(
                        "tool_completed",
                        agent_runtime_pb2.ToolCompletedEvent(
                            tool_call_id=event.call_id,
                            tool_name=event.name,
                            output_summary="Research search completed",
                        ),
                    )
                elif event.type == "stage" and event.message:
                    yield payload(
                        "output_delta",
                        agent_runtime_pb2.OutputDeltaEvent(text=f"[{event.message}]\n"),
                    )
                elif event.type == "report":
                    report_path = event.report_path
        except asyncio.CancelledError:
            raise
        except SearchToolError:
            yield payload(
                "tool_failed",
                agent_runtime_pb2.ToolFailedEvent(
                    tool_call_id="internet_search",
                    tool_name="internet_search",
                    failure=agent_runtime_pb2.RunFailure(
                        code=agent_runtime_pb2.RUN_ERROR_CODE_TOOL_FAILED,
                        message="Research search provider request failed",
                        retryable=True,
                    ),
                ),
            )
            yield _failed(
                agent_runtime_pb2.RUN_ERROR_CODE_TOOL_FAILED,
                "Research search provider request failed",
                retryable=True,
            )
            return
        except MissingCredentials:
            yield _failed(
                agent_runtime_pb2.RUN_ERROR_CODE_MODEL_NOT_CONFIGURED,
                "DeepAgent credentials are not configured",
            )
            return
        except (AuthenticationError, PermissionDeniedError):
            yield _failed(
                agent_runtime_pb2.RUN_ERROR_CODE_MODEL_AUTHENTICATION_FAILED,
                "DeepAgent model authentication failed",
            )
            return
        except RateLimitError:
            yield _failed(
                agent_runtime_pb2.RUN_ERROR_CODE_MODEL_RATE_LIMITED,
                "DeepAgent model rate limit exceeded",
                retryable=True,
            )
            return
        except (APITimeoutError, TimeoutError):
            yield _failed(
                agent_runtime_pb2.RUN_ERROR_CODE_MODEL_TIMEOUT,
                "DeepAgent model request timed out",
                retryable=True,
            )
            return
        except ValueError:
            yield _failed(
                agent_runtime_pb2.RUN_ERROR_CODE_OUTPUT_INVALID,
                "DeepAgent did not produce a valid report",
            )
            return
        except Exception:
            yield _failed(
                agent_runtime_pb2.RUN_ERROR_CODE_INTERNAL,
                "DeepAgent research failed",
            )
            return

        if report_path is None:
            yield _failed(
                agent_runtime_pb2.RUN_ERROR_CODE_OUTPUT_INVALID,
                "DeepAgent did not produce a report",
            )
            return
        report = report_path.read_bytes()
        artifact = agent_runtime_pb2.Artifact(
            id="report",
            type="markdown_report",
            title="DeepAgent Research Report",
            file_name=f"deepagent-research-{run.request.run_id}.md",
            mime_type="text/markdown",
            content=report,
        )
        yield payload(
            "artifact_ready",
            agent_runtime_pb2.ArtifactReadyEvent(artifact=artifact),
        )
        yield payload(
            "completed",
            agent_runtime_pb2.CompletedEvent(
                content="Research report is ready. You can download the Markdown report below.",
                artifacts=[artifact],
                model=agent_runtime_pb2.ModelAudit(
                    profile_id=run.request.model.profile_id,
                    source=run.request.model.source,
                    model_name=model_name,
                ),
            ),
        )


def _research_question(run: RunContext) -> str:
    question = run.request.trigger.content.strip()
    mention = run.request.agent.mention.strip()
    if mention and question.startswith(mention):
        question = question[len(mention) :].strip()
    return question


def _request_settings(run: RunContext) -> Settings:
    connection = run.request.model
    runtime_env = {"TAVILY_API_KEY": os.environ.get("TAVILY_API_KEY", "")}
    custom = CustomEndpoint(
        enabled=True,
        protocol="anthropic" if connection.protocol.lower().startswith("anthropic") else "openai",
        base_url=connection.base_url,
        api_key=connection.api_key,
        model_name=connection.model_name,
    )
    return Settings(
        config_path=PROJECT_DIR / "deepagent.toml",
        env_path=PROJECT_DIR / ".env",
        model_name=connection.model_name,
        search_max_results=5,
        search_topic="general",
        include_raw_content=False,
        output_dir=run.work_dir,
        stream_updates=True,
        custom=custom,
        env=runtime_env,
    )


def _failed(code: int, message: str, *, retryable: bool = False) -> EventPayload:
    return payload(
        "failed",
        agent_runtime_pb2.FailedEvent(
            failure=agent_runtime_pb2.RunFailure(
                code=code,
                message=message,
                retryable=retryable,
            )
        ),
    )
