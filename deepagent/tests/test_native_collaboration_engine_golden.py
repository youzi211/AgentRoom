import asyncio
import json
from dataclasses import replace
from pathlib import Path

from collaboration_engine_contract import contract_request
from collaboration_runtime.executor import ExecutorEvent, ExecutorEventKind
from collaboration_runtime.engines import NativeCollaborationEngine
from collaboration_runtime.models import AgentSnapshot, ModelSelection


GOLDEN_PATH = (
    Path(__file__).resolve().parents[2]
    / "proto"
    / "collaboration_runtime"
    / "v1"
    / "testdata"
    / "native_engine_golden.json"
)


def test_native_engine_matches_legacy_go_golden_scenarios():
    suite = json.loads(GOLDEN_PATH.read_text(encoding="utf-8"))

    for case in suite["cases"]:
        events = asyncio.run(_execute_case(case))
        expected = case["expected"]
        assert [
            event.agent_id for event in events if event.kind == "speaker_selected"
        ] == expected["speaker_ids"], case["name"]
        completed = [
            event for event in events if event.kind == "agent_message_completed"
        ]
        assert len(completed) == len(expected["messages"]), case["name"]
        for event, message in zip(completed, expected["messages"], strict=True):
            assert event.agent_id == message["agent_id"], case["name"]
            assert event.data["content"] == message["content"], case["name"]
            assert _first_id(event.data["artifacts"], "id") == message.get(
                "artifact_id", ""
            ), case["name"]
            assert _first_id(
                event.data["knowledge_sources"], "document_id"
            ) == message.get("knowledge_document_id", ""), case["name"]
            assert {
                key: event.data["model"][key]
                for key in ("profile_id", "source", "model_name")
            } == message["model"], case["name"]
        terminal = events[-1]
        assert terminal.kind == expected["terminal"]["kind"], case["name"]
        assert terminal.data["reason"] == expected["terminal"]["reason"], case["name"]
        assert terminal.data["turn_count"] == expected["terminal"]["turn_count"], case[
            "name"
        ]
        for response in case["responses"]:
            activity_text = response.get("activity_text", "")
            if activity_text:
                assert all(
                    event.data.get("content") != activity_text for event in completed
                ), case["name"]


async def _execute_case(case):
    base = contract_request(run_id=f"golden_{case['name']}")
    agents = tuple(
        AgentSnapshot(
            id=item["id"],
            name=item["name"],
            mention=f"@{item['name']}",
            role=f"{item['name']} role",
            description="Golden scenario agent",
            system_prompt=f"You are {item['name']}.",
            runtime="deepagent",
            model_selection_id=f"model-{item['id']}",
        )
        for item in case["agents"]
    )
    models = tuple(
        ModelSelection(
            id=f"model-{item['id']}",
            profile_id=f"profile-{item['id']}",
            source="database",
            protocol="fake",
            model_name="research-model",
            runtime_scope="collaboration",
                credential_ref="",
                purpose="agent_turn",
        )
        for item in case["agents"]
    )
    policy = case["policy"]
    request = replace(
        base,
        agents=agents,
        model_selections=models,
        trigger=replace(base.trigger, content=case["trigger"]),
        initial_candidate_agent_ids=tuple(case["initial_candidate_agent_ids"]),
        policy=replace(
            base.policy,
            max_turns=policy["max_turns"],
            max_turns_per_agent=policy["max_turns_per_agent"],
        ),
    )
    executor = GoldenExecutor(case["responses"])
    events = [
        event
        async for event in NativeCollaborationEngine(executor).execute(
            request, asyncio.Event()
        )
    ]
    assert executor.calls == len(case["responses"]), case["name"]
    return events


class GoldenExecutor:
    def __init__(self, responses):
        self._responses = responses
        self.calls = 0

    async def execute(self, turn, _cancel_event):
        response = self._responses[self.calls]
        self.calls += 1
        assert turn.agent.id == response["agent_id"]
        if activity_text := response.get("activity_text"):
            yield ExecutorEvent(
                ExecutorEventKind.OUTPUT_DELTA,
                data={"text": activity_text},
            )
        artifact = _artifact(response)
        if artifact:
            yield ExecutorEvent(
                ExecutorEventKind.ARTIFACT_READY,
                data={"artifact": artifact},
            )
        yield ExecutorEvent(
            ExecutorEventKind.COMPLETED,
            data={
                "content": response["content"],
                "artifacts": (artifact,) if artifact else (),
                "knowledge_sources": _knowledge_sources(response),
                "model": response["model"],
            },
        )


def _artifact(response):
    artifact_id = response.get("artifact_id", "")
    if not artifact_id:
        return None
    return {
        "id": artifact_id,
        "type": "markdown_report",
        "title": "Report",
        "file_name": f"{artifact_id}.md",
        "mime_type": "text/markdown",
        "content": f"# {artifact_id}".encode(),
        "external_uri": "",
    }


def _knowledge_sources(response):
    document_id = response.get("knowledge_document_id", "")
    if not document_id:
        return ()
    return (
        {
            "document_id": document_id,
            "document_name": f"{document_id}.md",
            "scope": "room",
        },
    )


def _first_id(items, key):
    return items[0][key] if items else ""
