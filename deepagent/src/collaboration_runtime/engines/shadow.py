from __future__ import annotations

import asyncio
from dataclasses import dataclass

from ..models import CollaborationRequest, EngineEvent


@dataclass(frozen=True)
class ShadowComparison:
    speaker_differences: tuple[tuple[str, tuple[str, ...], tuple[str, ...]], ...]
    event_differences: tuple[tuple[int, str, str], ...]
    stop_reason_difference: tuple[str, str] | None


async def compare_engines(native_engine, candidate_engine, request: CollaborationRequest) -> ShadowComparison:
    native_events, candidate_events = await asyncio.gather(
        _collect(native_engine, request),
        _collect(candidate_engine, request),
    )
    native_speakers = _agent_ids(native_events, "speaker_selected")
    candidate_speakers = _agent_ids(candidate_events, "speaker_selected")
    speaker_differences = ()
    if native_speakers != candidate_speakers:
        speaker_differences = (("speaker_selected", native_speakers, candidate_speakers),)

    event_differences = tuple(
        (index, native_kind, candidate_kind)
        for index, (native_kind, candidate_kind) in enumerate(
            zip(_kinds(native_events), _kinds(candidate_events)),
            start=1,
        )
        if native_kind != candidate_kind
    )
    if len(native_events) != len(candidate_events):
        event_differences = event_differences + ((
            min(len(native_events), len(candidate_events)) + 1,
            "<missing>" if len(native_events) < len(candidate_events) else "<extra>",
            "<missing>" if len(candidate_events) < len(native_events) else "<extra>",
        ),)

    native_reason = _terminal_reason(native_events)
    candidate_reason = _terminal_reason(candidate_events)
    stop_reason_difference = None
    if native_reason != candidate_reason:
        stop_reason_difference = (native_reason, candidate_reason)

    return ShadowComparison(
        speaker_differences=speaker_differences,
        event_differences=event_differences,
        stop_reason_difference=stop_reason_difference,
    )


async def _collect(engine, request: CollaborationRequest) -> tuple[EngineEvent, ...]:
    cancel_event = asyncio.Event()
    events = []
    async for event in engine.execute(request, cancel_event):
        events.append(event)
    return tuple(events)


def _kinds(events: tuple[EngineEvent, ...]) -> tuple[str, ...]:
    return tuple(event.kind for event in events)


def _agent_ids(events: tuple[EngineEvent, ...], kind: str) -> tuple[str, ...]:
    return tuple(event.agent_id for event in events if event.kind == kind)


def _terminal_reason(events: tuple[EngineEvent, ...]) -> str:
    for event in reversed(events):
        if event.kind in {"completed", "stopped", "cancelled", "failed"}:
            return str(event.data.get("reason", event.kind))
    return "<missing>"
