import asyncio
from pathlib import Path

import pytest

from collaboration_runtime.models import EngineEvent, EventKind
from collaboration_runtime.registry import (
    CollaborationEngineNotFound,
    CollaborationEngineRegistry,
)


class FakeEngine:
    name = "native"
    version = "fake-v1"

    async def execute(self, request, cancel_event: asyncio.Event):
        yield EngineEvent(EventKind.ACCEPTED, data={"run_id": request.collaboration_run_id})


def test_registry_creates_an_isolated_engine_for_each_resolution():
    registry = CollaborationEngineRegistry()
    registry.register("native", FakeEngine)

    first = registry.resolve("NATIVE")
    second = registry.resolve("native")

    assert isinstance(first, FakeEngine)
    assert isinstance(second, FakeEngine)
    assert first is not second
    assert registry.names() == ("native",)


def test_registry_rejects_duplicate_unknown_and_mismatched_engines():
    registry = CollaborationEngineRegistry()
    registry.register("native", FakeEngine)

    with pytest.raises(ValueError, match="already registered"):
        registry.register("NATIVE", FakeEngine)
    with pytest.raises(CollaborationEngineNotFound, match="not configured"):
        registry.resolve("autogen")

    registry.register("wrong", FakeEngine)
    with pytest.raises(ValueError, match="produced"):
        registry.resolve("wrong")


def test_framework_neutral_modules_do_not_import_framework_or_transport_types():
    import collaboration_runtime.engine as engine_module
    import collaboration_runtime.models as models_module
    import collaboration_runtime.registry as registry_module

    sources = (
        Path(engine_module.__file__).read_text(encoding="utf-8")
        + Path(models_module.__file__).read_text(encoding="utf-8")
        + Path(registry_module.__file__).read_text(encoding="utf-8")
    ).lower()
    for forbidden in ("autogen", "grpc", "protobuf", "mysql", "openai", "anthropic"):
        assert forbidden not in sources
