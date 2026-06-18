import json

from agentroom_deepagent.report import ResearchEvent, RunRecorder


def test_run_recorder_writes_report_and_events(tmp_path):
    recorder = RunRecorder(output_dir=tmp_path, run_id="run-test")

    recorder.record_event(ResearchEvent(type="plan", message="Created research plan"))
    recorder.record_event(
        ResearchEvent(
            type="search",
            message="Searched official documentation",
            payload={"query": "DeepAgents quickstart"},
        )
    )
    report_path = recorder.write_report("# Report\n\nDone.")

    assert report_path == tmp_path / "run-test" / "report.md"
    assert report_path.read_text(encoding="utf-8") == "# Report\n\nDone.\n"

    events = [
        json.loads(line)
        for line in (tmp_path / "run-test" / "events.jsonl").read_text(encoding="utf-8").splitlines()
    ]
    assert events == [
        {"type": "plan", "message": "Created research plan", "payload": {}},
        {
            "type": "search",
            "message": "Searched official documentation",
            "payload": {"query": "DeepAgents quickstart"},
        },
    ]


def test_run_recorder_rejects_empty_report(tmp_path):
    recorder = RunRecorder(output_dir=tmp_path, run_id="run-test")

    try:
        recorder.write_report("   ")
    except ValueError as exc:
        assert str(exc) == "report content cannot be empty"
    else:
        raise AssertionError("expected empty report to be rejected")
