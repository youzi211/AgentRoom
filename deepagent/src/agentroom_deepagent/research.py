"""Research orchestration.

Coordinates the research use case: validate credentials, assemble the model +
tool + agent, invoke the agent, and persist the report and event log via the
`RunRecorder`. The agent itself is built by `agent.create_research_agent`; the
model and tool by `models.build_model` and `tools.build_search_tool`.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, AsyncIterator

from deepagents.backends import FilesystemBackend

from agentroom_deepagent.agent import create_research_agent
from agentroom_deepagent.config import MissingCredentials, Settings
from agentroom_deepagent.models import build_model
from agentroom_deepagent.report import ResearchEvent, RunRecorder
from agentroom_deepagent.tools import build_search_tool


@dataclass(frozen=True)
class ResearchStreamEvent:
    """Provider-neutral progress emitted by the callable research library."""

    type: str
    name: str = ""
    call_id: str = ""
    message: str = ""
    report_path: Path | None = None
    payload: dict[str, Any] = field(default_factory=dict)


def describe_model(settings: Settings) -> str:
    """Human-readable model descriptor for event logs and smoke reports."""
    if settings.custom.enabled:
        effective_model = settings.custom.model_name or settings.model_name
        return f"{settings.custom.protocol}:{effective_model} @ {settings.custom.base_url}"
    return settings.model_name


def run_research(question: str, settings: Settings, recorder: RunRecorder) -> Path:
    cleaned_question = question.strip()
    if not cleaned_question:
        raise ValueError("research question cannot be empty")

    settings.validate_credentials()
    recorder.record_event(ResearchEvent(type="start", message="Starting research run"))

    model = build_model(settings)
    search_tool = build_search_tool(settings)
    backend = FilesystemBackend(root_dir=str(recorder.run_dir.resolve()), virtual_mode=True)
    agent = create_research_agent(model, search_tool, backend=backend)

    recorder.record_event(
        ResearchEvent(
            type="agent",
            message="Created DeepAgent research runtime",
            payload={"model": describe_model(settings)},
        )
    )

    agent.invoke({"messages": [{"role": "user", "content": cleaned_question}]})
    report_path = _require_filesystem_report(recorder)
    recorder.record_event(
        ResearchEvent(
            type="final",
            message="Wrote research report",
            payload={"report_path": str(report_path)},
        )
    )
    return report_path


async def stream_research(
    question: str,
    settings: Settings,
    recorder: RunRecorder,
) -> AsyncIterator[ResearchStreamEvent]:
    """Run DeepAgents in-process and expose a stable async event stream.

    This is the service-facing API. CLI parsing deliberately stays in
    ``cli.py`` so request content such as ``--config`` remains research text.
    """
    cleaned_question = question.strip()
    if not cleaned_question:
        raise ValueError("research question cannot be empty")

    settings.validate_credentials()
    recorder.record_event(ResearchEvent(type="start", message="Starting research run"))
    yield ResearchStreamEvent(type="stage", message="Starting research")

    model = build_model(settings)
    search_tool = build_search_tool(settings)
    backend = FilesystemBackend(root_dir=str(recorder.run_dir.resolve()), virtual_mode=True)
    agent = create_research_agent(model, search_tool, backend=backend)
    recorder.record_event(
        ResearchEvent(
            type="agent",
            message="Created DeepAgent research runtime",
            payload={"model": describe_model(settings)},
        )
    )

    active_models: set[str] = set()
    active_tools: set[str] = set()
    async for chunk in agent.astream(
        {"messages": [{"role": "user", "content": cleaned_question}]},
        stream_mode=["updates", "messages", "custom"],
        subgraphs=True,
        version="v2",
    ):
        chunk_type = chunk.get("type", "")
        source = _stream_source(chunk.get("ns", ()))
        data = chunk.get("data")
        if chunk_type == "messages" and isinstance(data, tuple) and data:
            token = data[0]
            tool_chunks = getattr(token, "tool_call_chunks", None) or []
            for tool_call in tool_chunks:
                name = str(tool_call.get("name") or "")
                call_id = str(tool_call.get("id") or f"{source}:{name or 'tool'}")
                if name and call_id not in active_tools:
                    active_tools.add(call_id)
                    yield ResearchStreamEvent(type="tool_started", name=name, call_id=call_id)

            token_type = str(getattr(token, "type", ""))
            if token_type == "ai" and source not in active_models:
                active_models.add(source)
                yield ResearchStreamEvent(type="model_started", name=source)
            elif token_type == "tool":
                name = str(getattr(token, "name", "") or "internet_search")
                call_id = str(getattr(token, "tool_call_id", "") or f"{source}:{name}")
                active_tools.discard(call_id)
                yield ResearchStreamEvent(type="tool_completed", name=name, call_id=call_id)
        elif chunk_type == "updates" and isinstance(data, dict):
            if "model_request" in data and source in active_models:
                active_models.discard(source)
                yield ResearchStreamEvent(type="model_completed", name=source)
            for node_name in data:
                if node_name in {"model_request", "tools"}:
                    yield ResearchStreamEvent(
                        type="stage",
                        message=f"Research {node_name.replace('_', ' ')} completed",
                    )

    for source in sorted(active_models):
        yield ResearchStreamEvent(type="model_completed", name=source)

    report_path = _require_filesystem_report(recorder)
    recorder.record_event(
        ResearchEvent(
            type="final",
            message="Wrote research report",
            payload={"report_path": str(report_path)},
        )
    )
    yield ResearchStreamEvent(type="report", report_path=report_path)


def _stream_source(namespace: object) -> str:
    if isinstance(namespace, (list, tuple)):
        for segment in namespace:
            value = str(segment)
            if value.startswith("tools:"):
                return value
    return "main"


def run_offline_smoke(question: str, settings: Settings, recorder: RunRecorder) -> Path:
    cleaned_question = question.strip()
    if not cleaned_question:
        raise ValueError("research question cannot be empty")

    try:
        settings.validate_credentials()
    except MissingCredentials as exc:
        readiness = f"blocked: {exc}"
    else:
        readiness = "ready: required credentials are present"

    recorder.record_event(
        ResearchEvent(
            type="offline-smoke",
            message="Created offline smoke report without calling DeepAgents or Tavily",
            payload={"credential_readiness": readiness},
        )
    )

    report = f"""# Offline Smoke Test

This is not a live DeepAgents research result. It verifies that the local
AgentRoom DeepAgent prototype can load configuration, create a run directory,
record events, and write a Markdown report without provider or search
credentials.

## Question

{cleaned_question}

## Runtime Configuration

- Model: {describe_model(settings)}
- Search topic: {settings.search_topic}
- Search max results: {settings.search_max_results}
- Include raw content: {settings.include_raw_content}
- Output directory: {settings.output_dir}
- Credential readiness: {readiness}

## Live Run Command

After credentials are available, run the same question without
`--offline-smoke` to execute the real DeepAgents + Tavily path.
"""
    report_path = recorder.write_report(report)
    recorder.record_event(
        ResearchEvent(
            type="final",
            message="Wrote offline smoke report",
            payload={"report_path": str(report_path)},
        )
    )
    return report_path


def _require_filesystem_report(recorder: RunRecorder) -> Path:
    report_path = recorder.report_path
    if report_path.exists():
        if report_path.read_text(encoding="utf-8").strip():
            return report_path
        raise ValueError("DeepAgent wrote empty report.md")
    raise ValueError("DeepAgent did not write report.md")
