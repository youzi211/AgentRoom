"""Framework-neutral multi-Agent collaboration runtime."""

from .engine import CollaborationEngine
from .executor import AgentExecutor, AgentTurnRequest, ExecutorEvent, ExecutorEventKind
from .model_client import (
    CollaborationModelClient,
    CollaborationModelError,
    CollaborationModelErrorCode,
    CollaborationModelMessage,
    CollaborationModelPurpose,
    CollaborationModelRequest,
    CollaborationModelResponse,
    CollaborationModelUsage,
    FakeCollaborationModelClient,
)
from .model_gateway import (
    ModelGatewayCollaborationModelClient,
    ModelGatewayCapabilities,
    ModelGatewayCore,
    ModelGatewayError,
    ModelGatewayErrorCode,
    ModelGatewayMessage,
    ModelGatewayRequest,
    ModelGatewayResponse,
    ModelGatewayUsage,
)
from .models import CollaborationRequest, EngineEvent, EventKind
from .registry import CollaborationEngineRegistry
from .service import CollaborationRuntimeServicer

__all__ = [
    "AgentExecutor",
    "AgentTurnRequest",
    "CollaborationEngine",
    "CollaborationEngineRegistry",
    "CollaborationModelClient",
    "CollaborationModelError",
    "CollaborationModelErrorCode",
    "CollaborationModelMessage",
    "CollaborationModelPurpose",
    "CollaborationModelRequest",
    "CollaborationModelResponse",
    "CollaborationModelUsage",
    "CollaborationRequest",
    "CollaborationRuntimeServicer",
    "EngineEvent",
    "EventKind",
    "ExecutorEvent",
    "ExecutorEventKind",
    "FakeCollaborationModelClient",
    "ModelGatewayCollaborationModelClient",
    "ModelGatewayCapabilities",
    "ModelGatewayCore",
    "ModelGatewayError",
    "ModelGatewayErrorCode",
    "ModelGatewayMessage",
    "ModelGatewayRequest",
    "ModelGatewayResponse",
    "ModelGatewayUsage",
]
