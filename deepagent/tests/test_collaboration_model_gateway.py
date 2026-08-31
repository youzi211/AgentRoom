import asyncio
import time
from dataclasses import replace

import pytest

from collaboration_runtime.model_client import (
    CollaborationModelClient,
    CollaborationModelError,
    CollaborationModelErrorCode,
    CollaborationModelMessage,
    CollaborationModelPurpose,
    CollaborationModelRequest,
)
from collaboration_runtime.model_gateway import (
    ModelGatewayCollaborationModelClient,
    ModelGatewayCapabilities,
    ModelGatewayCore,
    ModelGatewayError,
    ModelGatewayErrorCode,
    ModelGatewayMessage,
    ModelGatewayRequest,
    ModelGatewayResponse,
    ModelGatewayUsage,
)
from collaboration_runtime.models import ModelReference


class RecordingModelGatewayCore:
    def __init__(self, response, *, capabilities=ModelGatewayCapabilities()):
        self.response = response
        self.model_capabilities = capabilities
        self.calls = []

    def ready(self):
        return True

    def capabilities(self, request):
        return self.model_capabilities

    async def generate(self, request, cancel_event):
        self.calls.append((request, cancel_event))
        if isinstance(self.response, BaseException):
            raise self.response
        return self.response


class BlockingModelGatewayCore(RecordingModelGatewayCore):
    def __init__(self):
        super().__init__(ModelGatewayResponse(content="unused"))
        self.started = None
        self.cancelled = False

    async def generate(self, request, cancel_event):
        self.calls.append((request, cancel_event))
        self.started = asyncio.Event()
        self.started.set()
        try:
            await asyncio.Event().wait()
        finally:
            self.cancelled = True


def model_request(purpose, *, agent_id="", candidate_agent_ids=()):
    return CollaborationModelRequest(
        request_id="model_request_1",
        collaboration_run_id="collaboration_1",
        trace_id="trace_1",
        purpose=purpose,
        model_reference=ModelReference(
            id="model_reference_1",
            profile_id="profile_1",
            source="room_agent",
            protocol="openai_chat_completions",
            model_name="example-model",
            runtime_scope="collaboration",
        ),
        messages=(
            CollaborationModelMessage(role="system", content="Coordinate safely"),
            CollaborationModelMessage(
                role="user",
                content="Choose or respond",
                name="human",
            ),
        ),
        agent_id=agent_id,
        candidate_agent_ids=candidate_agent_ids,
    )


def test_gateway_adapter_maps_selector_request_and_usage():
    gateway_response = ModelGatewayResponse(
        content="agent_pm",
        usage=ModelGatewayUsage(input_tokens=9, output_tokens=2, total_tokens=11),
    )
    core = RecordingModelGatewayCore(gateway_response)
    client = ModelGatewayCollaborationModelClient(core)
    request = model_request(
        CollaborationModelPurpose.SELECTOR,
        candidate_agent_ids=("agent_pm", "agent_architect"),
    )

    async def scenario():
        cancel_event = asyncio.Event()
        response = await client.complete(request, cancel_event)
        return response, cancel_event

    response, cancel_event = asyncio.run(scenario())

    assert isinstance(client, CollaborationModelClient)
    assert response.content == "agent_pm"
    assert response.usage.input_tokens == 9
    assert response.usage.output_tokens == 2
    assert response.usage.total_tokens == 11
    assert isinstance(core, ModelGatewayCore)
    gateway_request, observed_cancel_event = core.calls[0]
    assert gateway_request == ModelGatewayRequest(
        request_id="model_request_1",
        trace_id="trace_1",
        purpose="selector",
        profile_id="profile_1",
        source="room_agent",
        protocol="openai_chat_completions",
        model_name="example-model",
        runtime_scope="collaboration",
        messages=(
            ModelGatewayMessage(role="system", content="Coordinate safely"),
            ModelGatewayMessage(
                role="user",
                content="Choose or respond",
                name="human",
            ),
        ),
        candidate_agent_ids=("agent_pm", "agent_architect"),
    )
    assert observed_cancel_event is cancel_event


def test_gateway_adapter_maps_participant_identity():
    core = RecordingModelGatewayCore(ModelGatewayResponse(content="Proposal"))
    client = ModelGatewayCollaborationModelClient(core)
    request = model_request(
        CollaborationModelPurpose.PARTICIPANT,
        agent_id="agent_pm",
    )

    response = asyncio.run(client.complete(request, asyncio.Event()))

    assert response.content == "Proposal"
    assert core.calls[0][0].purpose == "participant"
    assert core.calls[0][0].agent_id == "agent_pm"


def test_gateway_adapter_does_not_call_core_when_already_cancelled():
    core = RecordingModelGatewayCore(ModelGatewayResponse(content="unused"))
    client = ModelGatewayCollaborationModelClient(core)

    async def scenario():
        cancel_event = asyncio.Event()
        cancel_event.set()
        with pytest.raises(asyncio.CancelledError):
            await client.complete(
                model_request(CollaborationModelPurpose.SELECTOR),
                cancel_event,
            )

    asyncio.run(scenario())
    assert core.calls == []


def test_gateway_adapter_rejects_incomplete_profile_and_missing_capability():
    request = model_request(CollaborationModelPurpose.SELECTOR)
    incomplete = replace(
        request,
        model_reference=replace(request.model_reference, profile_id=""),
    )
    core = RecordingModelGatewayCore(ModelGatewayResponse(content="unused"))
    client = ModelGatewayCollaborationModelClient(core)

    with pytest.raises(CollaborationModelError) as profile_error:
        asyncio.run(client.complete(incomplete, asyncio.Event()))
    assert profile_error.value.code is CollaborationModelErrorCode.NOT_CONFIGURED

    core = RecordingModelGatewayCore(
        ModelGatewayResponse(content="unused"),
        capabilities=ModelGatewayCapabilities(text_generation=False),
    )
    client = ModelGatewayCollaborationModelClient(core)
    with pytest.raises(CollaborationModelError) as capability_error:
        asyncio.run(
            client.complete(
                model_request(CollaborationModelPurpose.PARTICIPANT),
                asyncio.Event(),
            )
        )
    assert capability_error.value.code is CollaborationModelErrorCode.NOT_CONFIGURED
    assert core.calls == []


@pytest.mark.parametrize(
    ("gateway_code", "model_code", "retryable", "message"),
    (
        (
            ModelGatewayErrorCode.NOT_CONFIGURED,
            CollaborationModelErrorCode.NOT_CONFIGURED,
            False,
            "Model Profile is not configured",
        ),
        (
            ModelGatewayErrorCode.AUTHENTICATION_FAILED,
            CollaborationModelErrorCode.AUTHENTICATION_FAILED,
            False,
            "Model authentication failed",
        ),
        (
            ModelGatewayErrorCode.RATE_LIMITED,
            CollaborationModelErrorCode.RATE_LIMITED,
            True,
            "Model request was rate limited",
        ),
        (
            ModelGatewayErrorCode.TIMEOUT,
            CollaborationModelErrorCode.TIMEOUT,
            True,
            "Model request timed out",
        ),
        (
            ModelGatewayErrorCode.INTERNAL,
            CollaborationModelErrorCode.INTERNAL,
            False,
            "Model request failed",
        ),
    ),
)
def test_gateway_adapter_maps_stable_errors(gateway_code, model_code, retryable, message):
    core = RecordingModelGatewayCore(
        ModelGatewayError(gateway_code, retryable=retryable)
    )
    client = ModelGatewayCollaborationModelClient(core)

    with pytest.raises(CollaborationModelError) as caught:
        asyncio.run(
            client.complete(
                model_request(CollaborationModelPurpose.PARTICIPANT),
                asyncio.Event(),
            )
        )

    assert caught.value.code is model_code
    assert caught.value.retryable is retryable
    assert str(caught.value) == message


def test_gateway_adapter_enforces_deadline_and_active_cancellation():
    async def deadline_scenario():
        core = BlockingModelGatewayCore()
        client = ModelGatewayCollaborationModelClient(core)
        request = replace(
            model_request(CollaborationModelPurpose.SELECTOR),
            deadline_monotonic=time.monotonic() + 0.01,
        )
        with pytest.raises(CollaborationModelError) as caught:
            await client.complete(request, asyncio.Event())
        return core, caught.value

    deadline_core, deadline_error = asyncio.run(deadline_scenario())
    assert deadline_error.code is CollaborationModelErrorCode.TIMEOUT
    assert deadline_core.cancelled

    async def cancellation_scenario():
        core = BlockingModelGatewayCore()
        client = ModelGatewayCollaborationModelClient(core)
        cancel_event = asyncio.Event()
        task = asyncio.create_task(
            client.complete(
                model_request(CollaborationModelPurpose.PARTICIPANT),
                cancel_event,
            )
        )
        while core.started is None:
            await asyncio.sleep(0)
        cancel_event.set()
        with pytest.raises(asyncio.CancelledError):
            await task
        return core

    cancellation_core = asyncio.run(cancellation_scenario())
    assert cancellation_core.cancelled


def test_gateway_adapter_derives_total_usage_when_gateway_omits_it():
    core = RecordingModelGatewayCore(
        ModelGatewayResponse(
            content="Proposal",
            usage=ModelGatewayUsage(input_tokens=5, output_tokens=3),
        )
    )
    client = ModelGatewayCollaborationModelClient(core)

    response = asyncio.run(
        client.complete(
            model_request(CollaborationModelPurpose.PARTICIPANT),
            asyncio.Event(),
        )
    )

    assert response.usage.total_tokens == 8


def test_gateway_adapter_rejects_invalid_usage_as_stable_internal_error():
    core = RecordingModelGatewayCore(
        ModelGatewayResponse(
            content="Proposal",
            usage=ModelGatewayUsage(input_tokens=-1),
        )
    )
    client = ModelGatewayCollaborationModelClient(core)

    with pytest.raises(CollaborationModelError) as caught:
        asyncio.run(
            client.complete(
                model_request(CollaborationModelPurpose.PARTICIPANT),
                asyncio.Event(),
            )
        )

    assert caught.value.code is CollaborationModelErrorCode.INTERNAL
    assert str(caught.value) == "Model request failed"
