from pathlib import Path

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
