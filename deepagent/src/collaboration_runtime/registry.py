from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass

from .engine import CollaborationEngine


EngineFactory = Callable[[], CollaborationEngine]
ReadinessCheck = Callable[[], bool]


def _always_ready() -> bool:
    return True


@dataclass(frozen=True)
class EngineRegistration:
    factory: EngineFactory
    ready_when: ReadinessCheck
    version: str = ""


@dataclass(frozen=True)
class CollaborationEngineCapability:
    name: str
    version: str
    enabled: bool
    ready: bool


class CollaborationEngineNotFound(LookupError):
    """Raised before acceptance when an Engine is unknown or disabled."""


class CollaborationEngineNotReady(CollaborationEngineNotFound):
    """Raised before acceptance when an Engine dependency is unavailable."""


class CollaborationEngineRegistry:
    def __init__(self) -> None:
        self._registrations: dict[str, EngineRegistration] = {}

    def register(
        self,
        name: str,
        factory: EngineFactory,
        *,
        ready_when: ReadinessCheck = _always_ready,
        version: str = "",
    ) -> None:
        normalized = self._normalize(name)
        if normalized in self._registrations:
            raise ValueError(f"Collaboration Engine {normalized!r} is already registered")
        self._registrations[normalized] = EngineRegistration(
            factory=factory,
            ready_when=ready_when,
            version=version.strip(),
        )

    def resolve(self, name: str) -> CollaborationEngine:
        normalized = self._normalize(name)
        try:
            registration = self._registrations[normalized]
        except KeyError as exc:
            raise CollaborationEngineNotFound(
                f"Collaboration Engine {normalized!r} is not configured"
            ) from exc
        if not self._is_ready(registration):
            raise CollaborationEngineNotReady(
                f"Collaboration Engine {normalized!r} dependencies are unavailable"
            )
        return self._create(normalized, registration)

    @classmethod
    def _create(
        cls,
        normalized: str,
        registration: EngineRegistration,
    ) -> CollaborationEngine:
        engine = registration.factory()
        if cls._normalize(engine.name) != normalized:
            raise ValueError(
                f"Collaboration Engine factory for {normalized!r} produced {engine.name!r}"
            )
        return engine

    def names(self) -> tuple[str, ...]:
        return tuple(sorted(self._registrations))

    def capabilities(self) -> tuple[CollaborationEngineCapability, ...]:
        capabilities = []
        for name in self.names():
            registration = self._registrations[name]
            version = registration.version
            try:
                ready = self._is_ready(registration)
                if ready:
                    engine = self._create(name, registration)
                    version = str(engine.version).strip()
                    ready = bool(version)
            except Exception:
                ready = False
            capabilities.append(
                CollaborationEngineCapability(
                    name=name,
                    version=version,
                    enabled=True,
                    ready=ready,
                )
            )
        return tuple(capabilities)

    def ready(self) -> bool:
        return any(capability.ready for capability in self.capabilities())

    def __len__(self) -> int:
        return len(self._registrations)

    @staticmethod
    def _is_ready(registration: EngineRegistration) -> bool:
        try:
            return bool(registration.ready_when())
        except Exception:
            return False

    @staticmethod
    def _normalize(name: str) -> str:
        normalized = name.strip().lower()
        if not normalized:
            raise ValueError("Collaboration Engine name is required")
        return normalized
