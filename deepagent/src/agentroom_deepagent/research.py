"""Research orchestration.

Coordinates the research use case: validate credentials, assemble the model +
tool + agent, invoke the agent, and persist the report and event log via the
`RunRecorder`. The agent itself is built by `agent.create_research_agent`; the
model and tool by `models.build_model` and `tools.build_search_tool`.
"""

from __future__ import annotations

from pathlib import Path

from deepagents.backends import FilesystemBackend

from agentroom_deepagent.agent import create_research_agent
from agentroom_deepagent.config import MissingCredentials, Settings
from agentroom_deepagent.models import build_model
from agentroom_deepagent.report import ResearchEvent, RunRecorder
from agentroom_deepagent.tools import build_search_tool


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
