import argparse
import asyncio

from agent_runtime.config import RuntimeSettings
from agent_runtime.executors import DeepAgentExecutor, LLMExecutor
from agent_runtime.model_adapter import ModelAdapterError, ModelCompletion, ModelDelta
from agent_runtime.registry import ExecutorRegistry
from agent_runtime.server import RuntimeServer
from agent_runtime.v1 import agent_runtime_pb2
from agentroom_deepagent.research import ResearchStreamEvent


class DeterministicIntegrationAdapter:
    async def stream(self, request, messages):
        assert request.model.api_key
        assert len(messages) == 2
        assert request.prompt_context.trigger_content in messages[1].content
        if "[fail]" in request.trigger.content:
            raise ModelAdapterError(
                agent_runtime_pb2.RUN_ERROR_CODE_MODEL_AUTHENTICATION_FAILED,
                "deterministic integration failure",
            )
        if "[delay]" in request.trigger.content:
            await asyncio.sleep(1)
        yield ModelDelta("python ")
        yield ModelDelta("integration response")
        yield ModelCompletion(
            "python integration response",
            input_tokens=12,
            output_tokens=3,
            total_tokens=15,
        )


async def deterministic_research(question, settings, recorder):
    assert question.startswith("--") or question == "cross-service research"
    assert settings.custom.api_key
    yield ResearchStreamEvent(type="model_started", name="main")
    yield ResearchStreamEvent(type="tool_started", name="internet_search", call_id="search-1")
    yield ResearchStreamEvent(type="tool_completed", name="internet_search", call_id="search-1")
    yield ResearchStreamEvent(type="model_completed", name="main")
    report_path = recorder.write_report(f"# Integration Research\n\nQuestion: {question}")
    yield ResearchStreamEvent(type="report", report_path=report_path)


async def serve(port: int) -> None:
    settings = RuntimeSettings(
        host="127.0.0.1",
        port=port,
        insecure=True,
        shutdown_grace_seconds=0.1,
    )
    registry = ExecutorRegistry(
        [
            LLMExecutor(DeterministicIntegrationAdapter()),
            DeepAgentExecutor(deterministic_research),
        ]
    )
    runtime = RuntimeServer(settings, registry)
    await runtime.start()
    print("READY", flush=True)
    try:
        await asyncio.Event().wait()
    finally:
        await runtime.stop()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, required=True)
    args = parser.parse_args()
    asyncio.run(serve(args.port))


if __name__ == "__main__":
    main()
