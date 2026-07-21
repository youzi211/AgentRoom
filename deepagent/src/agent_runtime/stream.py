from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator
from dataclasses import dataclass

from .events import EventPayload, payload
from .v1 import agent_runtime_pb2


_END = object()


@dataclass(frozen=True)
class _Raised:
    error: Exception


async def backpressured_events(
    source: AsyncIterator[EventPayload],
    *,
    buffer_size: int,
    max_coalesced_bytes: int,
) -> AsyncIterator[EventPayload]:
    """Bound Executor production and coalesce only adjacent output deltas.

    Lifecycle, tool, artifact, and terminal events always wait for queue capacity;
    they are never discarded. At most one additional coalesced output delta is
    retained outside the bounded queue.
    """

    queue: asyncio.Queue[EventPayload | _Raised | object] = asyncio.Queue(buffer_size)

    async def produce() -> None:
        coalesced: EventPayload | None = None
        try:
            async for item in source:
                if item.field == "output_delta":
                    if coalesced is not None:
                        combined = coalesced.message.text + item.message.text
                        if len(combined.encode("utf-8")) <= max_coalesced_bytes:
                            coalesced = payload(
                                "output_delta",
                                agent_runtime_pb2.OutputDeltaEvent(text=combined),
                            )
                            continue
                        await queue.put(coalesced)
                        coalesced = None
                    try:
                        queue.put_nowait(item)
                    except asyncio.QueueFull:
                        coalesced = item
                    continue

                if coalesced is not None:
                    await queue.put(coalesced)
                    coalesced = None
                await queue.put(item)

            if coalesced is not None:
                await queue.put(coalesced)
        except asyncio.CancelledError:
            raise
        except Exception as exc:
            await queue.put(_Raised(exc))
        finally:
            await queue.put(_END)

    producer = asyncio.create_task(produce())
    try:
        while True:
            item = await queue.get()
            if item is _END:
                break
            if isinstance(item, _Raised):
                raise item.error
            yield item
    finally:
        producer.cancel()
        await asyncio.gather(producer, return_exceptions=True)
