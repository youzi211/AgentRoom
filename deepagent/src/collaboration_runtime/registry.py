from __future__ import annotations

from collections.abc import Callable

from .engine import CollaborationEngine


EngineFactory = Callable[[], CollaborationEngine]


class CollaborationEngineNotFound(LookupError):
    """Raised before acceptance when an Engine is unknown or disabled."""


class CollaborationEngineRegistry:
    def __init__(self) -> None:
        self._factories: dict[str, EngineFactory] = {}

    def register(self, name: str, factory: EngineFactory) -> None:
        normalized = self._normalize(name)
        if normalized in self._factories:
            raise ValueError(f"Collaboration Engine {normalized!r} is already registered")
        self._factories[normalized] = factory

    def resolve(self, name: str) -> CollaborationEngine:
        normalized = self._normalize(name)
        try:
            factory = self._factories[normalized]
        except KeyError as exc:
            raise CollaborationEngineNotFound(
                f"Collaboration Engine {normalized!r} is not configured"
            ) from exc
        engine = factory()
        if self._normalize(engine.name) != normalized:
            raise ValueError(
                f"Collaboration Engine factory for {normalized!r} produced {engine.name!r}"
            )
        return engine

    def names(self) -> tuple[str, ...]:
        return tuple(sorted(self._factories))

    def __len__(self) -> int:
        return len(self._factories)

    @staticmethod
    def _normalize(name: str) -> str:
        normalized = name.strip().lower()
        if not normalized:
            raise ValueError("Collaboration Engine name is required")
        return normalized
