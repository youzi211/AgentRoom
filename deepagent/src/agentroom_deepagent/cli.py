from __future__ import annotations

import argparse
import json
import sys
from uuid import uuid4

from agentroom_deepagent.config import CustomEndpoint, MissingCredentials, Settings, load_settings
from agentroom_deepagent.report import RunRecorder
from agentroom_deepagent.research import run_offline_smoke, run_research


def _read_model_config_stdin() -> dict | None:
    """Read model config JSON from stdin if available."""
    if sys.stdin.isatty():
        return None
    try:
        raw = sys.stdin.read()
        if not raw.strip():
            return None
        return json.loads(raw)
    except (json.JSONDecodeError, OSError):
        return None


def _apply_model_config_override(settings: Settings, model_config: dict) -> Settings:
    """Override settings with model config from stdin.

    This replaces environment-variable-based credential passing with an
    explicit stdin pipe, reducing credential exposure in process listings.
    """
    custom = CustomEndpoint(
        enabled=True,
        protocol=model_config.get("protocol") or settings.custom.protocol or "openai",
        base_url=model_config.get("base_url") or settings.custom.base_url,
        api_key=model_config.get("api_key") or settings.custom.api_key,
        model_name=model_config.get("model_name") or settings.custom.model_name,
    )
    return Settings(
        config_path=settings.config_path,
        env_path=settings.env_path,
        model_name=custom.model_name or settings.model_name,
        search_max_results=settings.search_max_results,
        search_topic=settings.search_topic,
        include_raw_content=settings.include_raw_content,
        output_dir=settings.output_dir,
        stream_updates=settings.stream_updates,
        custom=custom,
        env=settings.env,
    )


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Run the AgentRoom DeepAgent research prototype.")
    parser.add_argument("question", nargs="?", default="", help="Research question to investigate.")
    parser.add_argument("--config", default="deepagent.toml", help="Path to deepagent.toml.")
    parser.add_argument("--run-id", default="", help="Optional deterministic run id.")
    parser.add_argument(
        "--model-config-stdin",
        action="store_true",
        help="Read model config (protocol, base_url, model_name, api_key) as JSON from stdin.",
    )
    parser.add_argument(
        "--offline-smoke",
        action="store_true",
        help="Verify the local CLI/config/report path without calling DeepAgents or Tavily.",
    )
    args = parser.parse_args(argv)

    settings = load_settings(args.config)

    if args.model_config_stdin:
        model_config = _read_model_config_stdin()
        if model_config is not None:
            settings = _apply_model_config_override(settings, model_config)

    run_id = args.run_id or f"run-{uuid4().hex[:12]}"
    recorder = RunRecorder(settings.output_dir, run_id)

    try:
        if args.offline_smoke:
            report_path = run_offline_smoke(args.question, settings, recorder)
            print(f"Offline smoke report written to {report_path}")
            return 0
        if not args.question:
            print("Error: research question is required", file=sys.stderr)
            return 2
        report_path = run_research(args.question, settings, recorder)
    except MissingCredentials as exc:
        print(f"Configuration error: {exc}", file=sys.stderr)
        print(f"Config file: {settings.config_path}", file=sys.stderr)
        print(f"Env file: {settings.env_path} ({'found' if settings.env_path.exists() else 'not found'})", file=sys.stderr)
        return 2
    except Exception as exc:
        print(f"Research run failed: {exc}", file=sys.stderr)
        return 1

    print(f"Report written to {report_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
