from __future__ import annotations

import asyncio
import logging
import signal
from dataclasses import dataclass

import grpc
from grpc_health.v1 import health, health_pb2, health_pb2_grpc

from .config import RuntimeSettings
from .executors import DeepAgentExecutor, FakeExecutor, LLMExecutor
from .registry import ExecutorRegistry
from .service import AgentRuntimeServicer
from .security import install_sensitive_data_filter
from .v1 import agent_runtime_pb2, agent_runtime_pb2_grpc


LOGGER = logging.getLogger(__name__)
SERVICE_NAME = "agentroom.runtime.v1.AgentRuntimeService"


@dataclass
class RuntimeServer:
    settings: RuntimeSettings
    registry: ExecutorRegistry

    def __post_init__(self) -> None:
        self.settings.validate()
        self.servicer = AgentRuntimeServicer(self.settings, self.registry)
        self.health = health.aio.HealthServicer()
        self.server = grpc.aio.server(
            options=self.settings.server_options,
            # Leave one admission slot for the CapacityLimiter so callers see
            # its stable RESOURCE_EXHAUSTED semantics instead of a transport
            # level rejection racing the bounded wait queue.
            maximum_concurrent_rpcs=self.settings.max_concurrency + self.settings.max_pending + 1,
        )
        agent_runtime_pb2_grpc.add_AgentRuntimeServiceServicer_to_server(self.servicer, self.server)
        health_pb2_grpc.add_HealthServicer_to_server(self.health, self.server)
        self.bound_port = 0

    async def start(self) -> int:
        await self.health.set("", health_pb2.HealthCheckResponse.NOT_SERVING)
        await self.health.set(SERVICE_NAME, health_pb2.HealthCheckResponse.NOT_SERVING)
        if self.settings.insecure:
            self.bound_port = self.server.add_insecure_port(self.settings.bind_address)
        else:
            self.bound_port = self.server.add_secure_port(
                self.settings.bind_address,
                self._server_credentials(),
            )
        if self.bound_port == 0:
            raise RuntimeError(f"failed to bind Agent Runtime to {self.settings.bind_address}")
        await self.server.start()
        if len(self.registry) > 0:
            await self.health.set("", health_pb2.HealthCheckResponse.SERVING)
            await self.health.set(SERVICE_NAME, health_pb2.HealthCheckResponse.SERVING)
        return self.bound_port

    async def stop(self) -> None:
        await self.health.enter_graceful_shutdown()
        await self.server.stop(self.settings.shutdown_grace_seconds)
        await self.servicer.cancel_active()
        try:
            await self.servicer.active.wait_empty(timeout=1.0)
        except TimeoutError:
            LOGGER.warning("Timed out waiting for cancelled Agent Runtime calls to clean up")

    async def wait_for_termination(self) -> None:
        await self.server.wait_for_termination()

    def _server_credentials(self) -> grpc.ServerCredentials:
        certificate = self.settings.tls_cert_file.read_bytes()
        private_key = self.settings.tls_key_file.read_bytes()
        root_certificates = (
            self.settings.tls_client_ca_file.read_bytes()
            if self.settings.tls_client_ca_file is not None
            else None
        )
        return grpc.ssl_server_credentials(
            ((private_key, certificate),),
            root_certificates=root_certificates,
            require_client_auth=root_certificates is not None,
        )


def build_registry(settings: RuntimeSettings) -> ExecutorRegistry:
    registry = ExecutorRegistry()
    if settings.enable_fake_executor:
        registry.register(FakeExecutor(agent_runtime_pb2.EXECUTOR_KIND_LLM))
        registry.register(FakeExecutor(agent_runtime_pb2.EXECUTOR_KIND_DEEPAGENT))
    else:
        registry.register(LLMExecutor())
        registry.register(DeepAgentExecutor())
    return registry


async def serve(settings: RuntimeSettings | None = None) -> None:
    effective = settings or RuntimeSettings.from_env()
    runtime = RuntimeServer(effective, build_registry(effective))
    await runtime.start()
    LOGGER.info("Agent Runtime listening", extra={"address": effective.bind_address})

    shutdown = asyncio.Event()
    loop = asyncio.get_running_loop()

    def request_shutdown() -> None:
        shutdown.set()

    for signum in (signal.SIGINT, signal.SIGTERM):
        try:
            loop.add_signal_handler(signum, request_shutdown)
        except NotImplementedError:
            signal.signal(signum, lambda _signum, _frame: loop.call_soon_threadsafe(request_shutdown))

    try:
        await shutdown.wait()
    finally:
        await runtime.stop()


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    install_sensitive_data_filter()
    asyncio.run(serve())
