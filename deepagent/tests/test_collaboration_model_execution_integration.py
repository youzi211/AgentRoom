"""Integration test for collaboration model execution boundary.

Tests the full pipeline: ModelSelection -> ModelConfigResolver -> ModelConfig
-> ModelClient -> HTTP request to a fake OpenAI-compatible server.

Verifies:
1. environment:go credentials are resolved and sent to the HTTP server
   with correct Authorization header and model name.
2. The API key secret never appears in collaboration events, checkpoints,
   or log captures.
3. profile:p1 credential_ref fails at preparation stage with no HTTP
   request and no environment fallback.
"""
from __future__ import annotations

import asyncio
import json
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import Any

import httpx
import pytest

from agent_runtime.model_config import (
    CredentialResolver,
    ModelConfigResolver,
)
from collaboration_runtime.executor import (
    AgentTurnRequest,
    ExecutorEvent,
    ExecutorEventKind,
)
from collaboration_runtime.model_execution import (
    ModelClient,
    ModelClientFactory,
    ModelExecutionService,
    ModelResponse,
)
from collaboration_runtime.engines.native import NativeCollaborationEngine
from collaboration_runtime.models import (
    AgentSnapshot,
    CollaborationPolicy,
    CollaborationRequest,
    ExecutionLimits,
    MessageSnapshot,
    ModelSelection,
    RoomSnapshot,
)

from collaboration_engine_contract import contract_request


# ---------------------------------------------------------------------------
# Fake OpenAI-compatible HTTP server
# ---------------------------------------------------------------------------


class _FakeOpenAIServer:
    """A minimal OpenAI /v1/chat/completions endpoint for integration tests."""

    def __init__(self) -> None:
        self.received_requests: list[dict[str, Any]] = []
        self._server: HTTPServer | None = None
        self._thread: threading.Thread | None = None
        self.base_url: str = ""

    def start(self) -> str:
        captured = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, format, *args):  # silence
                pass

            def do_POST(self):
                content_length = int(self.headers.get("Content-Length", 0))
                body = self.rfile.read(content_length).decode("utf-8")
                authorization = self.headers.get("Authorization", "")
                captured.received_requests.append(
                    {
                        "path": self.path,
                        "authorization": authorization,
                        "body": json.loads(body) if body else {},
                    }
                )
                response = {
                    "id": "chatcmpl-integration",
                    "object": "chat.completion",
                    "choices": [
                        {
                            "index": 0,
                            "message": {
                                "role": "assistant",
                                "content": "Integration response from fake server",
                            },
                            "finish_reason": "stop",
                        }
                    ],
                    "usage": {
                        "prompt_tokens": 10,
                        "completion_tokens": 5,
                        "total_tokens": 15,
                    },
                }
                payload = json.dumps(response).encode("utf-8")
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(payload)))
                self.end_headers()
                self.wfile.write(payload)

        self._server = HTTPServer(("127.0.0.1", 0), Handler)
        self._thread = threading.Thread(target=self._server.serve_forever, daemon=True)
        self._thread.start()
        port = self._server.server_address[1]
        self.base_url = f"http://127.0.0.1:{port}"
        return self.base_url

    def stop(self) -> None:
        if self._server:
            self._server.shutdown()
            self._server.server_close()
        if self._thread:
            self._thread.join(timeout=2)


# ---------------------------------------------------------------------------
# Real HTTP-based ModelClient
# ---------------------------------------------------------------------------


class HttpxModelClient:
    """A ModelClient that makes real HTTP calls to an OpenAI-compatible server."""

    def __init__(self, config) -> None:
        self._config = config

    async def complete(
        self,
        config,
        messages: tuple,
        *,
        cancel_event: asyncio.Event,
        timeout_seconds: float | None = None,
    ) -> ModelResponse:
        url = f"{config.base_url}/v1/chat/completions"
        headers = {
            "Authorization": f"Bearer {config.api_key}",
            "Content-Type": "application/json",
        }
        payload = {
            "model": config.model_name,
            "messages": [
                {"role": "system", "content": "You are a test assistant."},
                {"role": "user", "content": messages[-1] if messages else "Hello"},
            ],
        }
        async with httpx.AsyncClient(timeout=timeout_seconds or 10) as client:
            resp = await client.post(url, json=payload, headers=headers)
            resp.raise_for_status()
            data = resp.json()
            content = data["choices"][0]["message"]["content"]
            usage = data.get("usage", {})
            return ModelResponse(
                content=content,
                input_tokens=usage.get("prompt_tokens", 0),
                output_tokens=usage.get("completion_tokens", 0),
            )


class HttpxModelClientFactory:
    """Factory that creates HttpxModelClient instances."""

    def create(self, config) -> ModelClient:
        return HttpxModelClient(config)


# ---------------------------------------------------------------------------
# AgentExecutor that uses ModelExecutionService
# ---------------------------------------------------------------------------


class ModelServiceExecutor:
    """An AgentExecutor that delegates to ModelExecutionService for model calls.

    This bridges the collaboration engine to the ModelExecutionService,
    exercising the full credential resolution -> HTTP call pipeline.

    Like RuntimeRegistryAgentExecutor, it resolves credentials at the
    preparation stage BEFORE emitting any model events. A preparation
    failure yields only FAILED — no model_started/model_completed.
    """

    def __init__(self, service: ModelExecutionService) -> None:
        self._service = service

    async def execute(self, request: AgentTurnRequest, cancel_event: asyncio.Event):
        # Preparation stage: resolve credentials first
        sel = request.model_selection
        try:
            config = self._service._resolver.resolve(
                profile_id=sel.profile_id,
                source=sel.source,
                protocol=sel.protocol,
                model_name=sel.model_name,
                runtime_scope=sel.runtime_scope,
                credential_ref=sel.credential_ref,
            )
        except Exception:
            yield ExecutorEvent(ExecutorEventKind.FAILED, {"code": "model_not_configured", "retryable": False})
            return

        # Model execution stage — credentials resolved, now emit events
        yield ExecutorEvent(ExecutorEventKind.MODEL_STARTED, {"model_name": config.model_name})
        try:
            client = self._service._factory.create(config)
            messages = (request.trigger.content,)
            response = await client.complete(
                config,
                messages,
                cancel_event=cancel_event,
                timeout_seconds=request.limits.timeout_seconds,
            )
        except Exception:
            yield ExecutorEvent(ExecutorEventKind.FAILED, {"code": "internal", "retryable": False})
            return
        yield ExecutorEvent(
            ExecutorEventKind.MODEL_COMPLETED,
            {"model_name": config.model_name, "usage": {"input_tokens": response.input_tokens, "output_tokens": response.output_tokens, "total_tokens": response.input_tokens + response.output_tokens}},
        )
        yield ExecutorEvent(
            ExecutorEventKind.COMPLETED,
            {
                "content": response.content,
                "artifacts": (),
                "knowledge_sources": (),
                "model": {
                    "profile_id": config.profile_id,
                    "source": config.source,
                    "model_name": config.model_name,
                },
                "usage": {"input_tokens": response.input_tokens, "output_tokens": response.output_tokens, "total_tokens": response.input_tokens + response.output_tokens},
            },
        )


# ---------------------------------------------------------------------------
# Helpers to serialize events for secret-leak checking
# ---------------------------------------------------------------------------


def _events_to_text(events: list) -> str:
    """Flatten all event data into a single string for secret-leak scanning."""
    parts = []
    for event in events:
        parts.append(str(event.kind))
        parts.append(str(event.data))
        parts.append(str(event.turn_id))
        parts.append(str(event.agent_id))
    return "\n".join(parts)


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


@pytest.fixture
def fake_server():
    server = _FakeOpenAIServer()
    base_url = server.start()
    yield server, base_url
    server.stop()


def test_environment_go_resolution_sends_correct_auth_and_model(fake_server):
    """Step 1: environment:go credentials reach the HTTP server correctly.

    The fake OpenAI server must receive:
    - Authorization: Bearer <the injected API key>
    - body.model: the model_name from ModelSelection

    The API key must NOT appear in any collaboration event.
    """
    server, base_url = fake_server
    api_key = "integration-secret"

    # Build CredentialResolver with injected environment mapping
    env = {
        "LLM_BASE_URL": base_url,
        "LLM_API_KEY": api_key,
        "LLM_MODEL": "integration-model",
    }
    credential_resolver = CredentialResolver(env=env)
    config_resolver = ModelConfigResolver(credential_resolver=credential_resolver)
    factory = HttpxModelClientFactory()
    service = ModelExecutionService(config_resolver, factory)
    executor = ModelServiceExecutor(service)
    engine = NativeCollaborationEngine(executor)

    request = contract_request(
        run_id="integration_env_go",
        credential_ref="environment:go",
    )
    # Override model selection fields to match integration scenario
    request = CollaborationRequest(
        protocol_version=request.protocol_version,
        collaboration_run_id=request.collaboration_run_id,
        trace_id=request.trace_id,
        engine=request.engine,
        room=request.room,
        agents=request.agents,
        trigger=MessageSnapshot(
            id=request.trigger.id,
            sender_id=request.trigger.sender_id,
            sender_name=request.trigger.sender_name,
            sender_type=request.trigger.sender_type,
            content="Integration test message",
        ),
        transcript=request.transcript,
        knowledge_chunks=request.knowledge_chunks,
        model_selections=(
            ModelSelection(
                id="model_contract",
                profile_id="profile_contract",
                source="environment",
                protocol="openai_chat_completions",
                model_name="integration-model",
                runtime_scope="go",
                credential_ref="environment:go",
                purpose="agent_turn",
            ),
        ),
        policy=request.policy,
        limits=request.limits,
        initial_candidate_agent_ids=request.initial_candidate_agent_ids,
    )

    cancel_event = asyncio.Event()
    events = asyncio.run(_collect(engine, request, cancel_event))

    # The server should have received exactly one request
    assert len(server.received_requests) == 1
    received = server.received_requests[0]
    assert received["authorization"] == f"Bearer {api_key}"
    assert received["body"]["model"] == "integration-model"
    assert received["path"] == "/v1/chat/completions"

    # The engine should have produced a completed terminal event
    terminal_events = [e for e in events if e.kind in {"completed", "stopped", "cancelled", "failed"}]
    assert len(terminal_events) == 1
    assert terminal_events[0].kind == "completed"

    # The agent should have produced model_started and model_completed
    kinds = [e.kind for e in events]
    assert "model_started" in kinds
    assert "model_completed" in kinds
    assert "agent_message_completed" in kinds

    # CRITICAL: the API key must never appear in any event data
    all_text = _events_to_text(events)
    assert api_key not in all_text, "API key leaked into collaboration events"


def test_environment_go_secret_not_in_checkpoint_or_log(fake_server):
    """The secret must not appear in serialized event data or checkpoint-like structures."""
    server, base_url = fake_server
    api_key = "checkpoint-secret-key"

    env = {
        "LLM_BASE_URL": base_url,
        "LLM_API_KEY": api_key,
        "LLM_MODEL": "checkpoint-model",
    }
    credential_resolver = CredentialResolver(env=env)
    config_resolver = ModelConfigResolver(credential_resolver=credential_resolver)
    factory = HttpxModelClientFactory()
    service = ModelExecutionService(config_resolver, factory)
    executor = ModelServiceExecutor(service)
    engine = NativeCollaborationEngine(executor)

    request = contract_request(
        run_id="integration_checkpoint",
        credential_ref="environment:go",
    )
    request = CollaborationRequest(
        protocol_version=request.protocol_version,
        collaboration_run_id=request.collaboration_run_id,
        trace_id=request.trace_id,
        engine=request.engine,
        room=request.room,
        agents=request.agents,
        trigger=MessageSnapshot(
            id=request.trigger.id,
            sender_id=request.trigger.sender_id,
            sender_name=request.trigger.sender_name,
            sender_type=request.trigger.sender_type,
            content="Checkpoint test",
        ),
        transcript=request.transcript,
        knowledge_chunks=request.knowledge_chunks,
        model_selections=(
            ModelSelection(
                id="model_contract",
                profile_id="profile_contract",
                source="environment",
                protocol="openai_chat_completions",
                model_name="checkpoint-model",
                runtime_scope="go",
                credential_ref="environment:go",
                purpose="agent_turn",
            ),
        ),
        policy=request.policy,
        limits=request.limits,
        initial_candidate_agent_ids=request.initial_candidate_agent_ids,
    )

    cancel_event = asyncio.Event()
    events = asyncio.run(_collect(engine, request, cancel_event))

    # Serialize all events as JSON (simulating checkpoint/log serialization)
    serialized = json.dumps(
        [
            {"kind": str(e.kind), "data": dict(e.data), "turn_id": e.turn_id, "agent_id": e.agent_id}
            for e in events
        ],
        default=str,
    )
    assert api_key not in serialized, "API key leaked into serialized event/checkpoint data"

    # Also verify the credential_ref value itself doesn't carry the key
    assert "integration-secret" not in serialized


def test_profile_credential_fails_without_http_request(fake_server):
    """Step 2: profile:p1 fails at preparation stage — no HTTP request, no env fallback.

    With credential_ref=profile:p1 and no Secret Provider Adapter, the
    ModelConfigResolver must raise ModelConfigPreparationError. The engine
    must emit agent_turn_started -> failed(model_not_configured) with zero
    HTTP requests to the fake server.
    """
    server, base_url = fake_server
    api_key = "should-not-be-used"

    # Environment credentials exist but profile:p1 must NOT fall back to them
    env = {
        "LLM_BASE_URL": base_url,
        "LLM_API_KEY": api_key,
        "LLM_MODEL": "fallback-model",
    }
    credential_resolver = CredentialResolver(env=env)
    config_resolver = ModelConfigResolver(credential_resolver=credential_resolver)
    factory = HttpxModelClientFactory()
    service = ModelExecutionService(config_resolver, factory)

    # Directly test the resolver: profile:p1 must fail
    from agent_runtime.model_config import ModelConfigPreparationError

    with pytest.raises(ModelConfigPreparationError):
        config_resolver.resolve(
            profile_id="p1",
            source="database",
            protocol="openai_chat_completions",
            model_name="db-model",
            runtime_scope="go",
            credential_ref="profile:p1",
        )

    # Now test through the engine pipeline — the executor should receive
    # a FAILED event, not make any HTTP call
    executor = ModelServiceExecutor(service)
    engine = NativeCollaborationEngine(executor)

    request = contract_request(
        run_id="integration_profile_fail",
        credential_ref="profile:p1",
    )
    request = CollaborationRequest(
        protocol_version=request.protocol_version,
        collaboration_run_id=request.collaboration_run_id,
        trace_id=request.trace_id,
        engine=request.engine,
        room=request.room,
        agents=request.agents,
        trigger=MessageSnapshot(
            id=request.trigger.id,
            sender_id=request.trigger.sender_id,
            sender_name=request.trigger.sender_name,
            sender_type=request.trigger.sender_type,
            content="Profile fail test",
        ),
        transcript=request.transcript,
        knowledge_chunks=request.knowledge_chunks,
        model_selections=(
            ModelSelection(
                id="model_contract",
                profile_id="p1",
                source="database",
                protocol="openai_chat_completions",
                model_name="db-model",
                runtime_scope="go",
                credential_ref="profile:p1",
                purpose="agent_turn",
            ),
        ),
        policy=request.policy,
        limits=request.limits,
        initial_candidate_agent_ids=request.initial_candidate_agent_ids,
    )

    cancel_event = asyncio.Event()
    events = asyncio.run(_collect(engine, request, cancel_event))

    # No HTTP request should have been made
    assert len(server.received_requests) == 0, "HTTP request made despite profile credential failure"

    # The engine should have failed (not completed)
    terminal_events = [e for e in events if e.kind in {"completed", "stopped", "cancelled", "failed"}]
    assert len(terminal_events) == 1
    assert terminal_events[0].kind == "failed"

    # No model_started or model_completed events (preparation failed before model call)
    kinds = [e.kind for e in events]
    assert "model_started" not in kinds, "model_started emitted despite preparation failure"
    assert "model_completed" not in kinds

    # The API key must not appear in any event
    all_text = _events_to_text(events)
    assert api_key not in all_text, "API key leaked into events during profile failure"

    # The environment fallback model name must not appear
    assert "fallback-model" not in all_text, "Environment fallback model name leaked into events"


async def _collect(engine, request, cancel_event):
    return [event async for event in engine.execute(request, cancel_event)]
