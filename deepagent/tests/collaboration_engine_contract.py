import asyncio

from collaboration_runtime.models import (
    AgentSnapshot,
    CollaborationPolicy,
    CollaborationRequest,
    ExecutionLimits,
    MessageSnapshot,
    ModelSelection,
    RoomSnapshot,
)


TERMINAL_KINDS = {"completed", "stopped", "cancelled", "failed"}


def contract_request(run_id="collaboration_contract", *, credential_ref="environment:deepagent"):
    """Create a contract test request.

    Args:
        run_id: Unique run identifier.
        credential_ref: Credential reference for model resolution.
            Override to test credential resolution failure scenarios.
    """
    return CollaborationRequest(
        protocol_version="v1",
        collaboration_run_id=run_id,
        trace_id="trace_contract",
        engine="native",
        room=RoomSnapshot(id="room_contract", name="Contract room", status="active"),
        agents=(
            AgentSnapshot(
                id="agent_contract",
                name="Contract agent",
                mention="contract",
                role="assistant",
                description="Contract agent",
                system_prompt="Respond",
                runtime="llm",
                model_selection_id="model_contract",
            ),
        ),
        trigger=MessageSnapshot(
            id="message_contract",
            sender_id="human_contract",
            sender_name="Human",
            sender_type="human",
            content="Test",
        ),
        transcript=(),
        knowledge_chunks=(),
        model_selections=(
            ModelSelection(
                id="model_contract",
                profile_id="profile_contract",
                source="test",
                protocol="fake",
                model_name="fake-model",
                runtime_scope="collaboration",
                credential_ref=credential_ref,
                purpose="agent_turn",
            ),
        ),
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
        initial_candidate_agent_ids=("agent_contract",),
    )


async def collect(engine, request=None, *, cancelled=False):
    cancel_event = asyncio.Event()
    if cancelled:
        cancel_event.set()
    return [
        event
        async for event in engine.execute(
            request or contract_request(),
            cancel_event,
        )
    ]


def assert_engine_contract(factory):
    first = factory()
    second = factory()
    assert first is not second
    assert first.name
    assert first.version

    events = asyncio.run(collect(first))
    assert events[0].kind == "collaboration_started"
    assert events[0].kind != "accepted"
    assert events[-1].kind in TERMINAL_KINDS
    assert sum(event.kind in TERMINAL_KINDS for event in events) == 1

    cancelled = asyncio.run(collect(second, cancelled=True))
    assert cancelled[-1].kind == "cancelled"
    assert sum(event.kind in TERMINAL_KINDS for event in cancelled) == 1
