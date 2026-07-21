import asyncio
from pathlib import Path
from types import SimpleNamespace

import pytest

from agentroom_deepagent import research
from agentroom_deepagent.config import CustomEndpoint, Settings
from agentroom_deepagent.report import RunRecorder


def make_settings(tmp_path: Path) -> Settings:
    return Settings(
        config_path=tmp_path / "deepagent.toml",
        env_path=tmp_path / ".env",
        model_name="openai:gpt-5.5",
        search_max_results=5,
        search_topic="general",
        include_raw_content=False,
        output_dir=tmp_path,
        stream_updates=True,
        custom=CustomEndpoint(
            enabled=False,
            protocol="openai",
            base_url="",
            api_key="",
            model_name="",
        ),
        env={
            "OPENAI_API_KEY": "test-openai",
            "TAVILY_API_KEY": "test-tavily",
        },
    )


def test_run_research_uses_deepagents_filesystem_backend_report(monkeypatch, tmp_path):
    settings = make_settings(tmp_path)
    recorder = RunRecorder(output_dir=tmp_path, run_id="run-test")

    monkeypatch.setattr(research, "build_model", lambda _settings: "test-model")
    monkeypatch.setattr(research, "build_search_tool", lambda _settings: object())

    def fake_create_research_agent(model, search_tool, *, backend):
        assert model == "test-model"
        assert backend.__class__.__name__ == "FilesystemBackend"

        class FakeAgent:
            def invoke(self, _payload):
                recorder.report_path.write_text(
                    "# Filesystem Report\n\nThis came from DeepAgents write_file.\n",
                    encoding="utf-8",
                )
                return {"messages": [{"type": "ai", "content": "Do not persist this chat text."}]}

        return FakeAgent()

    monkeypatch.setattr(research, "create_research_agent", fake_create_research_agent)

    report_path = research.run_research("Research the filesystem path", settings, recorder)

    assert report_path == recorder.report_path
    assert report_path.read_text(encoding="utf-8") == (
        "# Filesystem Report\n\nThis came from DeepAgents write_file.\n"
    )


def test_run_research_fails_when_deepagent_does_not_write_report(monkeypatch, tmp_path):
    settings = make_settings(tmp_path)
    recorder = RunRecorder(output_dir=tmp_path, run_id="run-test")

    monkeypatch.setattr(research, "build_model", lambda _settings: "test-model")
    monkeypatch.setattr(research, "build_search_tool", lambda _settings: object())

    class FakeAgent:
        def invoke(self, _payload):
            return {"messages": [{"type": "ai", "content": "Only chat text."}]}

    monkeypatch.setattr(
        research,
        "create_research_agent",
        lambda _model, _search_tool, *, backend: FakeAgent(),
    )

    with pytest.raises(ValueError, match="did not write report.md"):
        research.run_research("Research without writing", settings, recorder)


def test_offline_outputs_do_not_persist_injected_model_api_key(tmp_path):
    injected_secret = "database-injected-secret-never-persist"
    settings = Settings(
        config_path=tmp_path / "deepagent.toml",
        env_path=tmp_path / ".env",
        model_name="database-injected-model",
        search_max_results=1,
        search_topic="general",
        include_raw_content=False,
        output_dir=tmp_path,
        stream_updates=False,
        custom=CustomEndpoint(
            enabled=True,
            protocol="openai",
            base_url="https://database-injected.example/v1",
            api_key=injected_secret,
            model_name="database-injected-model",
        ),
        env={"MODEL_API_KEY": injected_secret, "TAVILY_API_KEY": "test-tavily"},
    )
    recorder = RunRecorder(output_dir=tmp_path, run_id="secret-regression")

    report_path = research.run_offline_smoke("Check secret boundaries", settings, recorder)

    assert injected_secret not in report_path.read_text(encoding="utf-8")
    assert injected_secret not in recorder.events_path.read_text(encoding="utf-8")


def test_stream_research_exposes_async_model_tool_and_report_events(monkeypatch, tmp_path):
    settings = make_settings(tmp_path)
    recorder = RunRecorder(output_dir=tmp_path, run_id="stream-test")
    monkeypatch.setattr(research, "build_model", lambda _settings: "test-model")
    monkeypatch.setattr(research, "build_search_tool", lambda _settings: object())

    class FakeAgent:
        async def astream(self, _payload, **options):
            assert options == {
                "stream_mode": ["updates", "messages", "custom"],
                "subgraphs": True,
                "version": "v2",
            }
            yield {
                "type": "messages",
                "ns": (),
                "data": (
                    SimpleNamespace(
                        type="ai",
                        content="",
                        tool_call_chunks=[{"id": "call-1", "name": "internet_search"}],
                    ),
                    {},
                ),
            }
            yield {
                "type": "messages",
                "ns": (),
                "data": (
                    SimpleNamespace(
                        type="tool",
                        name="internet_search",
                        tool_call_id="call-1",
                        tool_call_chunks=[],
                    ),
                    {},
                ),
            }
            yield {"type": "updates", "ns": (), "data": {"model_request": {}}}
            recorder.write_report("# Async Report")

    monkeypatch.setattr(
        research,
        "create_research_agent",
        lambda _model, _search_tool, *, backend: FakeAgent(),
    )

    async def scenario():
        return [event async for event in research.stream_research("Question", settings, recorder)]

    events = asyncio.run(scenario())
    assert [event.type for event in events] == [
        "stage",
        "tool_started",
        "model_started",
        "tool_completed",
        "model_completed",
        "stage",
        "report",
    ]
    assert events[-1].report_path == recorder.report_path
