from pathlib import Path

import pytest

from agentroom_deepagent.cli import main


@pytest.fixture(autouse=True)
def clear_runtime_credentials(monkeypatch):
    for name in (
        "OPENAI_API_KEY",
        "ANTHROPIC_API_KEY",
        "GOOGLE_API_KEY",
        "OPENROUTER_API_KEY",
        "FIREWORKS_API_KEY",
        "BASETEN_API_KEY",
        "OLLAMA_API_KEY",
        "AZURE_OPENAI_API_KEY",
        "CUSTOM_API_KEY",
        "TAVILY_API_KEY",
    ):
        monkeypatch.delenv(name, raising=False)


def write_config(path: Path) -> None:
    path.write_text(
        """
        [model]
        name = "openai:gpt-5.5"

        [runtime]
        output_dir = "runs"
        """,
        encoding="utf-8",
    )


def test_offline_smoke_writes_report_without_provider_or_search_credentials(tmp_path, capsys):
    config_path = tmp_path / "deepagent.toml"
    write_config(config_path)

    exit_code = main(
        [
            "--config",
            str(config_path),
            "--run-id",
            "smoke-test",
            "--offline-smoke",
            "Research whether DeepAgents fits AgentRoom as a web research runtime",
        ]
    )

    assert exit_code == 0
    output = capsys.readouterr().out
    assert "Offline smoke report written to" in output

    report_path = tmp_path / "runs" / "smoke-test" / "report.md"
    assert report_path.exists()
    report = report_path.read_text(encoding="utf-8")
    assert "Offline Smoke Test" in report
    assert "not a live DeepAgents research result" in report
    assert "OPENAI_API_KEY" in report
    assert "TAVILY_API_KEY" in report


def test_real_run_still_rejects_missing_credentials(tmp_path, capsys):
    config_path = tmp_path / "deepagent.toml"
    write_config(config_path)

    exit_code = main(
        [
            "--config",
            str(config_path),
            "--run-id",
            "real-test",
            "Research whether DeepAgents fits AgentRoom as a web research runtime",
        ]
    )

    assert exit_code == 2
    error_output = capsys.readouterr().err
    assert "Configuration error" in error_output
    assert "OPENAI_API_KEY" in error_output
    assert "TAVILY_API_KEY" in error_output
