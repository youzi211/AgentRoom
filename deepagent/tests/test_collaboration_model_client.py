import asyncio

import pytest

from collaboration_runtime.model_client import (
    CollaborationModelClient,
    CollaborationModelMessage,
    CollaborationModelPurpose,
    CollaborationModelRequest,
    CollaborationModelResponse,
    CollaborationModelUsage,
    FakeCollaborationModelClient,
)
from collaboration_runtime.models import ModelSelection


def model_request(purpose, *, agent_id="", candidate_agent_ids=()):
    return CollaborationModelRequest(
        request_id="model_request_1",
        collaboration_run_id="collaboration_1",
        trace_id="trace_1",
        purpose=purpose,
        model_selection=ModelSelection(
            id="model_1",
            profile_id="profile_1",
            source="test",
            protocol="fake",
            model_name="fake-model",
            runtime_scope="collaboration",
            credential_ref="",
            purpose="agent_turn",
        ),
        messages=(CollaborationModelMessage(role="user", content="Choose or respond"),),
        agent_id=agent_id,
        candidate_agent_ids=candidate_agent_ids,
    )


def test_fake_model_client_supports_selector_and_participant_calls():
    selector_request = model_request(
        CollaborationModelPurpose.SELECTOR,
        candidate_agent_ids=("agent_pm", "agent_architect"),
    )
    participant_request = model_request(
        CollaborationModelPurpose.PARTICIPANT,
        agent_id="agent_pm",
    )
    selector_response = CollaborationModelResponse(content="agent_pm")
    participant_response = CollaborationModelResponse(
        content="Here is the proposal.",
        usage=CollaborationModelUsage(input_tokens=8, output_tokens=4, total_tokens=12),
    )
    client = FakeCollaborationModelClient((selector_response, participant_response))

    async def scenario():
        cancel_event = asyncio.Event()
        return (
            await client.complete(selector_request, cancel_event),
            await client.complete(participant_request, cancel_event),
        )

    responses = asyncio.run(scenario())

    assert isinstance(client, CollaborationModelClient)
    assert responses == (selector_response, participant_response)
    assert client.requests == (selector_request, participant_request)


def test_fake_model_client_observes_cancellation_before_consuming_response():
    request = model_request(CollaborationModelPurpose.SELECTOR)
    expected = CollaborationModelResponse(content="agent_pm")
    client = FakeCollaborationModelClient((expected,))

    async def scenario():
        cancel_event = asyncio.Event()
        cancel_event.set()
        with pytest.raises(asyncio.CancelledError):
            await client.complete(request, cancel_event)

        cancel_event.clear()
        return await client.complete(request, cancel_event)

    response = asyncio.run(scenario())

    assert response == expected
    assert client.requests == (request,)


def test_fake_model_client_can_script_safe_failures():
    request = model_request(CollaborationModelPurpose.PARTICIPANT, agent_id="agent_pm")
    failure = RuntimeError("model gateway unavailable")
    client = FakeCollaborationModelClient((failure,))

    async def scenario():
        with pytest.raises(RuntimeError, match="model gateway unavailable"):
            await client.complete(request, asyncio.Event())

    asyncio.run(scenario())
    assert client.requests == (request,)
