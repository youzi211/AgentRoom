"""Tests for ModelExecutionService."""
import asyncio
import os

import pytest

from collaboration_runtime.model_execution import ModelExecutionService, ModelResponse
from agent_runtime.model_config import ModelConfigResolver, ModelConfigPreparationError


class FakeModelClient:
    def __init__(self, response=None):
        self._response = response or ModelResponse(content="test", input_tokens=10, output_tokens=20)
        self._configs = []

    async def complete(self, config, messages, *, cancel_event, timeout_seconds=None):
        self._configs.append(config)
        return self._response


class FakeModelClientFactory:
    def __init__(self, client=None):
        self._client = client or FakeModelClient()
        self._configs = []

    def create(self, config):
        self._configs.append(config)
        return self._client


class FakeSelection:
    profile_id = "test-profile"
    source = "test"
    protocol = "openai"
    model_name = "gpt-4"
    runtime_scope = "collaboration"
    credential_ref = "environment:deepagent"


class EmptySelection:
    profile_id = ""
    source = ""
    protocol = ""
    model_name = ""
    runtime_scope = ""
    credential_ref = ""


def test_model_execution_service_complete():
    resolver = ModelConfigResolver()
    factory = FakeModelClientFactory()
    service = ModelExecutionService(resolver, factory)

    os.environ["MODEL_BASE_URL"] = "https://test.example.com"
    os.environ["MODEL_API_KEY"] = "test-key-123"
    os.environ["MODEL_NAME"] = "gpt-4-test"

    try:
        cancel = asyncio.Event()
        response = asyncio.run(service.complete(FakeSelection(), ("message",), cancel_event=cancel))

        assert response.content == "test"
        assert response.input_tokens == 10
        assert response.output_tokens == 20
        assert len(factory._configs) == 1
        assert factory._configs[0].api_key == "test-key-123"
    finally:
        del os.environ["MODEL_BASE_URL"]
        del os.environ["MODEL_API_KEY"]
        del os.environ["MODEL_NAME"]


def test_model_execution_service_ready():
    resolver = ModelConfigResolver()
    factory = FakeModelClientFactory()
    service = ModelExecutionService(resolver, factory)
    assert service.ready is True

    service2 = ModelExecutionService(None, None)
    assert service2.ready is False


def test_model_execution_service_incomplete_selection():
    resolver = ModelConfigResolver()
    factory = FakeModelClientFactory()
    service = ModelExecutionService(resolver, factory)

    cancel = asyncio.Event()
    with pytest.raises(ModelConfigPreparationError):
        asyncio.run(service.complete(EmptySelection(), (), cancel_event=cancel))
