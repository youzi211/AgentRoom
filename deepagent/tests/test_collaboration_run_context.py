import asyncio

from agent_runtime.context import ActiveRunRegistry
from collaboration_runtime.context import (
    ActiveCollaborationRegistry,
    CollaborationRunContext,
    RunNamespace,
)
from collaboration_runtime.models import (
    AgentSnapshot,
    CollaborationPolicy,
    CollaborationRequest,
    ExecutionLimits,
    MessageSnapshot,
    RoomSnapshot,
)


def request(run_id: str = "shared_run") -> CollaborationRequest:
    return CollaborationRequest(
        protocol_version="v1",
        collaboration_run_id=run_id,
        trace_id="trace_test",
        engine="native",
        room=RoomSnapshot(id="room_test", name="Test room", status="active"),
        agents=(
            AgentSnapshot(
                id="agent_test",
                name="Test agent",
                mention="test",
                role="assistant",
                description="Test agent",
                system_prompt="Respond",
                runtime="llm",
                model_selection_id="model_test",
            ),
        ),
        trigger=MessageSnapshot(
            id="message_test",
            sender_id="human_test",
            sender_name="Human",
            sender_type="human",
            content="Test",
        ),
        transcript=(),
        knowledge_chunks=(),
        model_selections=(),
        policy=CollaborationPolicy(
            version="v1",
            engine="native",
            trigger_mode="mention_only",
            max_turns=3,
            max_turns_per_agent=1,
            allow_agent_handoff=True,
            allow_self_followup=False,
            cooldown_seconds=0,
            stop_on_empty_output=True,
            stop_on_repeated_output=True,
        ),
        limits=ExecutionLimits(
            timeout_seconds=30,
            max_output_bytes=1024,
            max_artifact_bytes=1024,
            max_tool_steps=8,
            max_request_bytes=8192,
            max_event_bytes=4096,
            max_checkpoint_bytes=1024,
        ),
    )


def test_collaboration_context_uses_an_independent_run_namespace():
    context = CollaborationRunContext(request())

    assert context.identity.namespace is RunNamespace.COLLABORATION
    assert context.identity.value == "shared_run"
    assert not context.cancel_event.is_set()
    context.cancel()
    assert context.cancel_event.is_set()


def test_active_registry_rejects_duplicate_and_exposes_cancellation_handle():
    async def scenario():
        registry = ActiveCollaborationRegistry()
        first = CollaborationRunContext(request("collaboration_duplicate"))
        duplicate = CollaborationRunContext(request("collaboration_duplicate"))

        handle = await registry.register(first)
        assert handle is not None
        assert await registry.register(duplicate) is None
        assert await registry.count() == 1
        assert await registry.cancel("collaboration_duplicate")
        assert first.cancel_event.is_set()
        assert not await registry.cancel("missing")

        await registry.unregister("collaboration_duplicate")
        await registry.wait_empty(timeout=0.1)
        assert await registry.count() == 0

    asyncio.run(scenario())


def test_same_text_id_does_not_collide_with_agent_runtime_registry():
    async def scenario():
        agent_registry = ActiveRunRegistry()
        collaboration_registry = ActiveCollaborationRegistry()
        context = CollaborationRunContext(request("same_text_id"))

        assert await agent_registry.register("same_text_id")
        assert await collaboration_registry.register(context) is not None
        assert await agent_registry.count() == 1
        assert await collaboration_registry.count() == 1

        await collaboration_registry.cancel_all()
        assert context.cancel_event.is_set()
        assert await agent_registry.count() == 1

        await collaboration_registry.unregister("same_text_id")
        await agent_registry.unregister("same_text_id")

    asyncio.run(scenario())
