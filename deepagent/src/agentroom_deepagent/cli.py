from __future__ import annotations

import argparse
import sys
from uuid import uuid4

from agentroom_deepagent.config import MissingCredentials, load_settings
from agentroom_deepagent.report import RunRecorder
from agentroom_deepagent.research import run_offline_smoke, run_research


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Run the AgentRoom DeepAgent research prototype.")
    parser.add_argument("question", help="Research question to investigate.")
    parser.add_argument("--config", default="deepagent.toml", help="Path to deepagent.toml.")
    parser.add_argument("--run-id", default="", help="Optional deterministic run id.")
    parser.add_argument(
        "--offline-smoke",
        action="store_true",
        help="Verify the local CLI/config/report path without calling DeepAgents or Tavily.",
    )
    args = parser.parse_args(argv)

    settings = load_settings(args.config)
    run_id = args.run_id or f"run-{uuid4().hex[:12]}"
    recorder = RunRecorder(settings.output_dir, run_id)

    try:
        if args.offline_smoke:
            report_path = run_offline_smoke(args.question, settings, recorder)
            print(f"Offline smoke report written to {report_path}")
            return 0
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
