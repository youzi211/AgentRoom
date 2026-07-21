from pathlib import Path

import pytest
from google.protobuf import json_format

from agent_runtime.protocol import ProtocolVersionError, validate_protocol_version
from agent_runtime.v1 import agent_runtime_pb2


FIXTURE_PATH = (
    Path(__file__).resolve().parents[2]
    / "proto"
    / "agent_runtime"
    / "v1"
    / "testdata"
    / "execute_agent_request.json"
)


def test_execute_agent_request_contract_fixture():
    request = json_format.Parse(FIXTURE_PATH.read_text(encoding="utf-8"), agent_runtime_pb2.ExecuteAgentRequest())

    validate_protocol_version(request.protocol_version)
    assert request.run_id == "run_contract"
    assert request.executor_kind == agent_runtime_pb2.EXECUTOR_KIND_LLM
    assert request.model.api_key == "contract-secret-not-for-logs"


def test_execute_agent_request_ignores_unknown_binary_field():
    request = agent_runtime_pb2.ExecuteAgentRequest(protocol_version="v1", run_id="run_unknown")
    payload = request.SerializeToString() + bytes((0x98, 0x06, 0x01))

    decoded = agent_runtime_pb2.ExecuteAgentRequest.FromString(payload)

    assert decoded.run_id == request.run_id


def test_agent_event_contract_covers_every_payload():
    events = [
        agent_runtime_pb2.AgentEvent(accepted=agent_runtime_pb2.AcceptedEvent()),
        agent_runtime_pb2.AgentEvent(model_started=agent_runtime_pb2.ModelStartedEvent()),
        agent_runtime_pb2.AgentEvent(model_completed=agent_runtime_pb2.ModelCompletedEvent()),
        agent_runtime_pb2.AgentEvent(tool_started=agent_runtime_pb2.ToolStartedEvent()),
        agent_runtime_pb2.AgentEvent(tool_completed=agent_runtime_pb2.ToolCompletedEvent()),
        agent_runtime_pb2.AgentEvent(tool_failed=agent_runtime_pb2.ToolFailedEvent()),
        agent_runtime_pb2.AgentEvent(output_delta=agent_runtime_pb2.OutputDeltaEvent()),
        agent_runtime_pb2.AgentEvent(artifact_ready=agent_runtime_pb2.ArtifactReadyEvent()),
        agent_runtime_pb2.AgentEvent(completed=agent_runtime_pb2.CompletedEvent()),
        agent_runtime_pb2.AgentEvent(failed=agent_runtime_pb2.FailedEvent()),
    ]

    for sequence, event in enumerate(events, start=1):
        event.protocol_version = "v1"
        event.run_id = "run_events"
        event.sequence = sequence
        decoded = agent_runtime_pb2.AgentEvent.FromString(event.SerializeToString())
        assert decoded.WhichOneof("payload") == event.WhichOneof("payload")
        assert decoded.sequence == sequence


def test_unsupported_protocol_version_is_rejected():
    with pytest.raises(ProtocolVersionError):
        validate_protocol_version("v2")
