from __future__ import annotations

import os
import sys

import grpc
from grpc_health.v1 import health_pb2, health_pb2_grpc


def main() -> int:
    address = f"{os.getenv('AGENT_RUNTIME_HEALTH_HOST', '127.0.0.1')}:{os.getenv('AGENT_RUNTIME_PORT', '50051')}"
    channel = grpc.insecure_channel(address)
    try:
        grpc.channel_ready_future(channel).result(timeout=3)
        response = health_pb2_grpc.HealthStub(channel).Check(
            health_pb2.HealthCheckRequest(service="agentroom.runtime.v1.AgentRuntimeService"),
            timeout=2,
        )
        return 0 if response.status == health_pb2.HealthCheckResponse.SERVING else 1
    except Exception:
        return 1
    finally:
        channel.close()


if __name__ == "__main__":
    raise SystemExit(main())
