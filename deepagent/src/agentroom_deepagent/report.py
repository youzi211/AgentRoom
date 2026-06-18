from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


@dataclass(frozen=True)
class ResearchEvent:
    type: str
    message: str
    payload: dict[str, Any] = field(default_factory=dict)


class RunRecorder:
    def __init__(self, output_dir: str | Path, run_id: str) -> None:
        self.run_dir = Path(output_dir) / run_id
        self.run_dir.mkdir(parents=True, exist_ok=True)
        self.events_path = self.run_dir / "events.jsonl"

    @property
    def report_path(self) -> Path:
        return self.run_dir / "report.md"

    def record_event(self, event: ResearchEvent) -> None:
        payload = {
            "type": event.type,
            "message": event.message,
            "payload": event.payload,
        }
        with self.events_path.open("a", encoding="utf-8") as handle:
            handle.write(json.dumps(payload, ensure_ascii=False) + "\n")

    def write_report(self, content: str) -> Path:
        cleaned = content.strip()
        if not cleaned:
            raise ValueError("report content cannot be empty")

        report_path = self.report_path
        report_path.write_text(cleaned + "\n", encoding="utf-8")
        return report_path
