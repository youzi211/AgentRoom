import json
from pathlib import Path

import pytest
from google.protobuf import json_format

from collaboration_runtime.protocol import (
    EventSequenceValidator,
    ProtocolVersionError,
    validate_protocol_version,
)
from collaboration_runtime.v1 import collaboration_runtime_pb2


FIXTURE_PATH = (
    Path(__file__).resolve().parents[2]
    / "proto"
    / "collaboration_runtime"
    / "v1"
    / "testdata"
    / "contract_golden.json"
)


def _fixture():
    return json.loads(FIXTURE_PATH.read_text(encoding="utf-8"))


def _accepted(run_id: str, sequence: int):
    return collaboration_runtime_pb2.CollaborationEvent(
        protocol_version="v1",
        collaboration_run_id=run_id,
        sequence=sequence,
        accepted=collaboration_runtime_pb2.AcceptedEvent(),
    )


def _started(run_id: str, sequence: int):
    return collaboration_runtime_pb2.CollaborationEvent(
        protocol_version="v1",
        collaboration_run_id=run_id,
        sequence=sequence,
        collaboration_started=collaboration_runtime_pb2.CollaborationStartedEvent(),
    )


def test_collaboration_contract_golden_parses_and_validates():
    fixture = _fixture()
    request = json_format.ParseDict(fixture["request"], collaboration_runtime_pb2.ExecuteConversationRequest())

    validate_protocol_version(request.protocol_version)
    assert request.collaboration_run_id == "collaboration_contract"
    assert request.engine == collaboration_runtime_pb2.COLLABORATION_ENGINE_NATIVE

    validator = EventSequenceValidator(request.collaboration_run_id)
    for raw in fixture["events"]:
        event = json_format.ParseDict(raw, collaboration_runtime_pb2.CollaborationEvent())
        validator.validate(event)
    assert validator.terminal_seen


def test_execute_conversation_request_ignores_unknown_binary_field():
    request = collaboration_runtime_pb2.ExecuteConversationRequest(
        protocol_version="v1", collaboration_run_id="collaboration_unknown"
    )
    payload = request.SerializeToString() + bytes((0x98, 0x06, 0x01))

    decoded = collaboration_runtime_pb2.ExecuteConversationRequest.FromString(payload)

    assert decoded.collaboration_run_id == request.collaboration_run_id


def test_collaboration_protocol_rejects_unsupported_version():
    with pytest.raises(ProtocolVersionError):
        validate_protocol_version("v2")


def test_collaboration_event_validator_rejects_out_of_order_and_duplicate_terminal():
    validator = EventSequenceValidator("collaboration_sequence")
    validator.validate(_accepted("collaboration_sequence", 1))
    validator.validate(_started("collaboration_sequence", 3))
    with pytest.raises(ValueError, match="sequence"):
        validator.validate(_started("collaboration_sequence", 2))

    terminal_validator = EventSequenceValidator("collaboration_terminal")
    terminal_validator.validate(_accepted("collaboration_terminal", 1))
    terminal_validator.validate(
        collaboration_runtime_pb2.CollaborationEvent(
            protocol_version="v1",
            collaboration_run_id="collaboration_terminal",
            sequence=2,
            completed=collaboration_runtime_pb2.CompletedEvent(),
        )
    )
    with pytest.raises(ValueError, match="terminal"):
        terminal_validator.validate(
            collaboration_runtime_pb2.CollaborationEvent(
                protocol_version="v1",
                collaboration_run_id="collaboration_terminal",
                sequence=3,
                failed=collaboration_runtime_pb2.FailedEvent(),
            )
        )


def test_collaboration_model_selection_has_no_credential_fields():
    request_json = json.dumps(_fixture()["request"]).lower()
    for forbidden in ("api_key", "apikey", "authorization", "provider_response", "providerresponse"):
        assert forbidden not in request_json

    fields = collaboration_runtime_pb2.ModelSelection.DESCRIPTOR.fields_by_name
    assert "api_key" not in fields
    assert "authorization" not in fields
    assert "provider_response" not in fields
