from __future__ import annotations

from .executors.base import Executor


class ExecutorNotFound(LookupError):
    """Raised before acceptance when an Executor kind is not registered."""


class ExecutorRegistry:
    def __init__(self, executors: list[Executor] | None = None) -> None:
        self._executors: dict[int, Executor] = {}
        for executor in executors or []:
            self.register(executor)

    def register(self, executor: Executor) -> None:
        if executor.kind in self._executors:
            raise ValueError(f"Executor kind {executor.kind} is already registered")
        self._executors[executor.kind] = executor

    def resolve(self, kind: int) -> Executor:
        try:
            return self._executors[kind]
        except KeyError as exc:
            raise ExecutorNotFound(f"Executor kind {kind} is not configured") from exc

    def __len__(self) -> int:
        return len(self._executors)
