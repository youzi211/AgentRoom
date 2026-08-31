import asyncio

import pytest

from collaboration_runtime.capacity import (
    CollaborationCapacityExceeded,
    CollaborationCapacityLimiter,
    CollaborationRoomBusy,
)


def run(coro):
    return asyncio.run(coro)


def test_capacity_allows_different_rooms_and_rejects_same_room():
    async def scenario():
        capacity = CollaborationCapacityLimiter(max_concurrency=2, max_pending=1)
        async with capacity.slot("room_one"):
            async with capacity.slot("room_two"):
                assert await capacity.active() == 2
                assert await capacity.room_count() == 2
                with pytest.raises(CollaborationRoomBusy):
                    async with capacity.slot("room_one"):
                        pass
        assert await capacity.active() == 0
        assert await capacity.room_count() == 0

    run(scenario())


def test_capacity_bounds_waiting_queue():
    async def scenario():
        capacity = CollaborationCapacityLimiter(max_concurrency=1, max_pending=1)
        release = asyncio.Event()

        async def occupy(room_id):
            async with capacity.slot(room_id):
                await release.wait()

        first = asyncio.create_task(occupy("room_one"))
        while await capacity.active() != 1:
            await asyncio.sleep(0)
        second = asyncio.create_task(occupy("room_two"))
        while await capacity.pending() != 1:
            await asyncio.sleep(0)

        with pytest.raises(CollaborationCapacityExceeded):
            async with capacity.slot("room_three"):
                pass

        release.set()
        await asyncio.gather(first, second)
        assert await capacity.active() == 0
        assert await capacity.pending() == 0
        assert await capacity.room_count() == 0

    run(scenario())


def test_cancelling_waiter_releases_queue_and_room_reservation():
    async def scenario():
        capacity = CollaborationCapacityLimiter(max_concurrency=1, max_pending=1)
        release = asyncio.Event()

        async def occupy(room_id):
            async with capacity.slot(room_id):
                await release.wait()

        first = asyncio.create_task(occupy("room_one"))
        while await capacity.active() != 1:
            await asyncio.sleep(0)
        waiting = asyncio.create_task(occupy("room_waiting"))
        while await capacity.pending() != 1:
            await asyncio.sleep(0)

        waiting.cancel()
        with pytest.raises(asyncio.CancelledError):
            await waiting
        assert await capacity.pending() == 0
        assert await capacity.room_count() == 1

        release.set()
        await first
        async with capacity.slot("room_waiting"):
            assert await capacity.active() == 1

    run(scenario())
