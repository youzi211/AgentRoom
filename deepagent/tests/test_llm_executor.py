import asyncio
from types import SimpleNamespace

import httpx
import pytest
from openai import APITimeoutError, AuthenticationError, RateLimitError

from agent_runtime.context import RunContext
from agent_runtime.executors import LLMExecutor
from agent_runtime.model_adapter import (
    ModelAdapterError,
    ModelCompletion,
    ModelDelta,
    OpenAIModelAdapter,
)
from agent_runtime.v1 import agent_runtime_pb2


def llm_request(run_id="run_llm", *, profile="profile_1", key="secret_1"):
    request = agent_runtime_pb2.ExecuteAgentRequest(
        protocol_version="v1",
        run_id=run_id,
        executor_kind=agent_runtime_pb2.EXECUTOR_KIND_LLM,
        room=agent_runtime_pb2.RoomSnapshot(id="room_1", name="Planning"),
        agent=agent_runtime_pb2.AgentSnapshot(
            id="agent_1",
            name="Planner",
            mention="@Planner",
            role="planner",
            system_prompt="Plan safely.",
        ),
        trigger=agent_runtime_pb2.MessageSnapshot(
            id="message_1",
            sender_name="Alice",
            sender_type=agent_runtime_pb2.SENDER_TYPE_HUMAN,
            content="Make a plan",
        ),
        model=agent_runtime_pb2.ModelConnection(
            protocol="openai_chat_completions",
            base_url="https://model.invalid/v1",
            model_name=f"model_{profile}",
            api_key=key,
            profile_id=profile,
            source="database",
        ),
        knowledge_chunks=[
            agent_runtime_pb2.KnowledgeChunk(
                id="chunk_1",
                document_id="document_1",
                document_name="Plan.md",
                scope="room",
                content="Known fact",
            )
        ],
        limits=agent_runtime_pb2.ExecutionLimits(max_output_bytes=4096),
        prompt_context=agent_runtime_pb2.PromptContextSnapshot(
            room_name="Planning",
            dialogue_mode="mention_fanout",
            trigger_sender="Alice",
            trigger_sender_type=agent_runtime_pb2.SENDER_TYPE_HUMAN,
            trigger_content="Make a plan",
            latest_visible_speaker="Alice",
            latest_visible_speaker_type=agent_runtime_pb2.SENDER_TYPE_HUMAN,
        ),
    )
    request.limits.timeout.FromSeconds(2)
    return request


async def collect_executor(executor, run):
    return [item async for item in executor.execute(run)]


def run(coro):
    return asyncio.run(coro)


class SuccessfulAdapter:
    async def stream(self, request, messages):
        assert messages[0].role == "system"
        yield ModelDelta("safe ")
        yield ModelDelta("answer")
        yield ModelCompletion("safe answer", input_tokens=8, output_tokens=2, total_tokens=10)


def test_llm_executor_emits_lifecycle_output_usage_sources_and_audit(tmp_path):
    async def scenario():
        context = RunContext.create(llm_request(), tmp_path)
        try:
            events = await collect_executor(LLMExecutor(SuccessfulAdapter()), context)
        finally:
            context.cleanup()
        fields = [event.field for event in events]
        assert fields == [
            "model_started",
            "output_delta",
            "output_delta",
            "model_completed",
            "completed",
        ]
        completed = events[-1].message
        assert completed.content == "safe answer"
        assert completed.usage.total_tokens == 10
        assert completed.model.profile_id == "profile_1"
        assert completed.knowledge_sources[0].document_name == "Plan.md"
        assert "secret_1" not in repr(events)
        assert "Plan safely." not in repr(events)

    run(scenario())


@pytest.mark.parametrize(
    ("code", "message"),
    [
        (agent_runtime_pb2.RUN_ERROR_CODE_MODEL_AUTHENTICATION_FAILED, "authentication"),
        (agent_runtime_pb2.RUN_ERROR_CODE_MODEL_RATE_LIMITED, "rate limit"),
        (agent_runtime_pb2.RUN_ERROR_CODE_MODEL_TIMEOUT, "timed out"),
    ],
)
def test_llm_executor_maps_stable_model_failures(tmp_path, code, message):
    class FailedAdapter:
        async def stream(self, request, messages):
            raise ModelAdapterError(code, message)
            yield

    async def scenario():
        context = RunContext.create(llm_request(), tmp_path)
        try:
            events = await collect_executor(LLMExecutor(FailedAdapter()), context)
        finally:
            context.cleanup()
        assert events[-1].field == "failed"
        assert events[-1].message.failure.code == code
        assert events[-1].message.failure.message == message

    run(scenario())


def test_llm_executor_rejects_empty_output(tmp_path):
    class EmptyAdapter:
        async def stream(self, request, messages):
            if False:
                yield

    async def scenario():
        context = RunContext.create(llm_request(), tmp_path)
        try:
            events = await collect_executor(LLMExecutor(EmptyAdapter()), context)
        finally:
            context.cleanup()
        assert events[-1].field == "failed"
        assert events[-1].message.failure.code == agent_runtime_pb2.RUN_ERROR_CODE_OUTPUT_INVALID

    run(scenario())


def test_llm_executor_cancellation_interrupts_provider_iteration(tmp_path):
    started = asyncio.Event()

    class BlockingAdapter:
        async def stream(self, request, messages):
            started.set()
            await asyncio.Event().wait()
            yield

    async def scenario():
        context = RunContext.create(llm_request(), tmp_path)
        task = asyncio.create_task(collect_executor(LLMExecutor(BlockingAdapter()), context))
        await started.wait()
        task.cancel()
        with pytest.raises(asyncio.CancelledError):
            await task
        context.cleanup()
        assert not any(tmp_path.iterdir())

    run(scenario())


def test_concurrent_profiles_keep_request_scoped_credentials_isolated(tmp_path):
    seen = []

    class IsolatedAdapter:
        async def stream(self, request, messages):
            seen.append((request.run_id, request.model.profile_id, request.model.api_key))
            await asyncio.sleep(0)
            yield ModelCompletion(request.model.profile_id)

    async def scenario():
        first = RunContext.create(llm_request("run_1", profile="one", key="key_one"), tmp_path)
        second = RunContext.create(llm_request("run_2", profile="two", key="key_two"), tmp_path)
        try:
            results = await asyncio.gather(
                collect_executor(LLMExecutor(IsolatedAdapter()), first),
                collect_executor(LLMExecutor(IsolatedAdapter()), second),
            )
        finally:
            first.cleanup()
            second.cleanup()
        assert {item for item in seen} == {
            ("run_1", "one", "key_one"),
            ("run_2", "two", "key_two"),
        }
        assert {result[-1].message.content for result in results} == {"one", "two"}
        assert first.request.model.api_key == ""
        assert second.request.model.api_key == ""

    run(scenario())


class FakeOpenAIStream:
    def __init__(self, chunks):
        self.chunks = chunks

    def __aiter__(self):
        return self._iterate()

    async def _iterate(self):
        for chunk in self.chunks:
            yield chunk


class FakeOpenAIClient:
    def __init__(self, captured, chunks=None, error=None):
        self.captured = captured
        self.chunks = chunks or []
        self.error = error
        self.chat = SimpleNamespace(completions=SimpleNamespace(create=self.create))

    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc, traceback):
        self.captured["closed"] = True

    async def create(self, **kwargs):
        self.captured["request"] = kwargs
        if self.error:
            raise self.error
        return FakeOpenAIStream(self.chunks)


def test_openai_adapter_uses_request_connection_disables_retries_and_closes_client():
    captured = {}
    chunks = [
        SimpleNamespace(
            choices=[SimpleNamespace(delta=SimpleNamespace(content="answer"))],
            usage=None,
        ),
        SimpleNamespace(
            choices=[],
            usage=SimpleNamespace(prompt_tokens=4, completion_tokens=1, total_tokens=5),
        ),
    ]

    def factory(**kwargs):
        captured["client"] = kwargs
        return FakeOpenAIClient(captured, chunks=chunks)

    async def scenario():
        items = [item async for item in OpenAIModelAdapter(factory).stream(llm_request(), [])]
        assert isinstance(items[0], ModelDelta)
        assert items[-1].total_tokens == 5

    run(scenario())
    assert captured["client"]["api_key"] == "secret_1"
    assert captured["client"]["base_url"] == "https://model.invalid/v1"
    assert captured["client"]["max_retries"] == 0
    assert captured["closed"]


def test_openai_adapter_filters_split_private_reasoning_from_deltas_and_final_output():
    captured = {}
    chunks = [
        SimpleNamespace(choices=[SimpleNamespace(delta=SimpleNamespace(content="<thi"))], usage=None),
        SimpleNamespace(choices=[SimpleNamespace(delta=SimpleNamespace(content="nk>private secret"))], usage=None),
        SimpleNamespace(choices=[SimpleNamespace(delta=SimpleNamespace(content="</think>Visible"))], usage=None),
        SimpleNamespace(choices=[SimpleNamespace(delta=SimpleNamespace(content=" answer"))], usage=None),
    ]

    def factory(**kwargs):
        return FakeOpenAIClient(captured, chunks=chunks)

    async def scenario():
        items = [item async for item in OpenAIModelAdapter(factory).stream(llm_request(), [])]
        text = "".join(item.text for item in items if isinstance(item, ModelDelta))
        assert text == "Visible answer"
        assert items[-1].content == "Visible answer"
        assert "private secret" not in repr(items)

    run(scenario())


def test_openai_adapter_rejects_think_only_and_oversized_output():
    async def assert_failure(content, request, expected_code):
        captured = {}
        chunks = [SimpleNamespace(choices=[SimpleNamespace(delta=SimpleNamespace(content=content))], usage=None)]

        def factory(**kwargs):
            return FakeOpenAIClient(captured, chunks=chunks)

        with pytest.raises(ModelAdapterError) as error:
            async for _ in OpenAIModelAdapter(factory).stream(request, []):
                pass
        assert error.value.code == expected_code

    async def scenario():
        await assert_failure(
            "<thinking>private</thinking>",
            llm_request(),
            agent_runtime_pb2.RUN_ERROR_CODE_OUTPUT_INVALID,
        )
        limited = llm_request()
        limited.limits.max_output_bytes = 4
        await assert_failure(
            "too long",
            limited,
            agent_runtime_pb2.RUN_ERROR_CODE_RESOURCE_EXHAUSTED,
        )

    run(scenario())


@pytest.mark.parametrize(
    ("error_factory", "expected_code"),
    [
        (
            lambda request: AuthenticationError(
                "secret provider detail",
                response=httpx.Response(401, request=request),
                body=None,
            ),
            agent_runtime_pb2.RUN_ERROR_CODE_MODEL_AUTHENTICATION_FAILED,
        ),
        (
            lambda request: RateLimitError(
                "secret provider detail",
                response=httpx.Response(429, request=request),
                body=None,
            ),
            agent_runtime_pb2.RUN_ERROR_CODE_MODEL_RATE_LIMITED,
        ),
        (
            lambda request: APITimeoutError(request=request),
            agent_runtime_pb2.RUN_ERROR_CODE_MODEL_TIMEOUT,
        ),
    ],
)
def test_openai_adapter_classifies_provider_errors_without_raw_details(error_factory, expected_code):
    captured = {}
    provider_request = httpx.Request("POST", "https://model.invalid/v1/chat/completions")

    def factory(**kwargs):
        return FakeOpenAIClient(captured, error=error_factory(provider_request))

    async def scenario():
        with pytest.raises(ModelAdapterError) as error:
            async for _ in OpenAIModelAdapter(factory).stream(llm_request(), []):
                pass
        assert error.value.code == expected_code
        assert "secret provider detail" not in error.value.safe_message

    run(scenario())
