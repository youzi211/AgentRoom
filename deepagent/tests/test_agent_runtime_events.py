import pytest

from agent_runtime.events import EventSequenceError, EventWriter, ResourceLimitError, payload
from agent_runtime.v1 import agent_runtime_pb2


def writer(**overrides):
    values = {
        "max_event_bytes": 1024,
        "max_artifact_bytes": 128,
        "max_output_bytes": 128,
    }
    values.update(overrides)
    return EventWriter("run_events", **values)


def test_event_writer_requires_accepted_and_one_terminal():
    events = writer()
    with pytest.raises(EventSequenceError, match="accepted"):
        events.write(payload("output_delta", agent_runtime_pb2.OutputDeltaEvent(text="early")))

    accepted = events.accepted()
    completed = events.write(payload("completed", agent_runtime_pb2.CompletedEvent(content="done")))

    assert accepted.sequence == 1
    assert completed.sequence == 2
    assert events.terminal
    with pytest.raises(EventSequenceError, match="terminal"):
        events.write(payload("output_delta", agent_runtime_pb2.OutputDeltaEvent(text="late")))


def test_event_writer_rejects_oversized_artifact_without_truncation():
    events = writer(max_artifact_bytes=4)
    events.accepted()
    with pytest.raises(ResourceLimitError, match="artifact"):
        events.write(
            payload(
                "artifact_ready",
                agent_runtime_pb2.ArtifactReadyEvent(
                    artifact=agent_runtime_pb2.Artifact(content=b"too large")
                ),
            )
        )
