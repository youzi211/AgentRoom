import datetime

from google.protobuf import duration_pb2 as _duration_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class CollaborationEngine(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    COLLABORATION_ENGINE_UNSPECIFIED: _ClassVar[CollaborationEngine]
    COLLABORATION_ENGINE_NATIVE: _ClassVar[CollaborationEngine]
    COLLABORATION_ENGINE_AUTOGEN: _ClassVar[CollaborationEngine]

class TriggerMode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    TRIGGER_MODE_UNSPECIFIED: _ClassVar[TriggerMode]
    TRIGGER_MODE_MENTION_ONLY: _ClassVar[TriggerMode]
    TRIGGER_MODE_AUTOMATIC: _ClassVar[TriggerMode]

class SenderType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SENDER_TYPE_UNSPECIFIED: _ClassVar[SenderType]
    SENDER_TYPE_HUMAN: _ClassVar[SenderType]
    SENDER_TYPE_AGENT: _ClassVar[SenderType]
    SENDER_TYPE_SYSTEM: _ClassVar[SenderType]

class CollaborationStopReason(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    COLLABORATION_STOP_REASON_UNSPECIFIED: _ClassVar[CollaborationStopReason]
    COLLABORATION_STOP_REASON_COMPLETED: _ClassVar[CollaborationStopReason]
    COLLABORATION_STOP_REASON_MAX_TURNS: _ClassVar[CollaborationStopReason]
    COLLABORATION_STOP_REASON_MAX_TURNS_PER_AGENT: _ClassVar[CollaborationStopReason]
    COLLABORATION_STOP_REASON_EMPTY_OUTPUT: _ClassVar[CollaborationStopReason]
    COLLABORATION_STOP_REASON_DUPLICATE_OUTPUT: _ClassVar[CollaborationStopReason]
    COLLABORATION_STOP_REASON_NO_ELIGIBLE_AGENT: _ClassVar[CollaborationStopReason]
    COLLABORATION_STOP_REASON_CANCELLED: _ClassVar[CollaborationStopReason]
    COLLABORATION_STOP_REASON_DEADLINE_EXCEEDED: _ClassVar[CollaborationStopReason]
    COLLABORATION_STOP_REASON_INTERRUPTED: _ClassVar[CollaborationStopReason]
    COLLABORATION_STOP_REASON_ENGINE_FAILURE: _ClassVar[CollaborationStopReason]
    COLLABORATION_STOP_REASON_PROTOCOL_ERROR: _ClassVar[CollaborationStopReason]

class CollaborationErrorCode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    COLLABORATION_ERROR_CODE_UNSPECIFIED: _ClassVar[CollaborationErrorCode]
    COLLABORATION_ERROR_CODE_INVALID_REQUEST: _ClassVar[CollaborationErrorCode]
    COLLABORATION_ERROR_CODE_UNSUPPORTED_VERSION: _ClassVar[CollaborationErrorCode]
    COLLABORATION_ERROR_CODE_ENGINE_UNAVAILABLE: _ClassVar[CollaborationErrorCode]
    COLLABORATION_ERROR_CODE_RESOURCE_EXHAUSTED: _ClassVar[CollaborationErrorCode]
    COLLABORATION_ERROR_CODE_DUPLICATE_RUN: _ClassVar[CollaborationErrorCode]
    COLLABORATION_ERROR_CODE_ROOM_BUSY: _ClassVar[CollaborationErrorCode]
    COLLABORATION_ERROR_CODE_MODEL_NOT_CONFIGURED: _ClassVar[CollaborationErrorCode]
    COLLABORATION_ERROR_CODE_MODEL_AUTHENTICATION_FAILED: _ClassVar[CollaborationErrorCode]
    COLLABORATION_ERROR_CODE_MODEL_RATE_LIMITED: _ClassVar[CollaborationErrorCode]
    COLLABORATION_ERROR_CODE_MODEL_TIMEOUT: _ClassVar[CollaborationErrorCode]
    COLLABORATION_ERROR_CODE_TOOL_FAILED: _ClassVar[CollaborationErrorCode]
    COLLABORATION_ERROR_CODE_OUTPUT_INVALID: _ClassVar[CollaborationErrorCode]
    COLLABORATION_ERROR_CODE_CHECKPOINT_INVALID: _ClassVar[CollaborationErrorCode]
    COLLABORATION_ERROR_CODE_PROTOCOL_ERROR: _ClassVar[CollaborationErrorCode]
    COLLABORATION_ERROR_CODE_CANCELLED: _ClassVar[CollaborationErrorCode]
    COLLABORATION_ERROR_CODE_DEADLINE_EXCEEDED: _ClassVar[CollaborationErrorCode]
    COLLABORATION_ERROR_CODE_INTERNAL: _ClassVar[CollaborationErrorCode]
COLLABORATION_ENGINE_UNSPECIFIED: CollaborationEngine
COLLABORATION_ENGINE_NATIVE: CollaborationEngine
COLLABORATION_ENGINE_AUTOGEN: CollaborationEngine
TRIGGER_MODE_UNSPECIFIED: TriggerMode
TRIGGER_MODE_MENTION_ONLY: TriggerMode
TRIGGER_MODE_AUTOMATIC: TriggerMode
SENDER_TYPE_UNSPECIFIED: SenderType
SENDER_TYPE_HUMAN: SenderType
SENDER_TYPE_AGENT: SenderType
SENDER_TYPE_SYSTEM: SenderType
COLLABORATION_STOP_REASON_UNSPECIFIED: CollaborationStopReason
COLLABORATION_STOP_REASON_COMPLETED: CollaborationStopReason
COLLABORATION_STOP_REASON_MAX_TURNS: CollaborationStopReason
COLLABORATION_STOP_REASON_MAX_TURNS_PER_AGENT: CollaborationStopReason
COLLABORATION_STOP_REASON_EMPTY_OUTPUT: CollaborationStopReason
COLLABORATION_STOP_REASON_DUPLICATE_OUTPUT: CollaborationStopReason
COLLABORATION_STOP_REASON_NO_ELIGIBLE_AGENT: CollaborationStopReason
COLLABORATION_STOP_REASON_CANCELLED: CollaborationStopReason
COLLABORATION_STOP_REASON_DEADLINE_EXCEEDED: CollaborationStopReason
COLLABORATION_STOP_REASON_INTERRUPTED: CollaborationStopReason
COLLABORATION_STOP_REASON_ENGINE_FAILURE: CollaborationStopReason
COLLABORATION_STOP_REASON_PROTOCOL_ERROR: CollaborationStopReason
COLLABORATION_ERROR_CODE_UNSPECIFIED: CollaborationErrorCode
COLLABORATION_ERROR_CODE_INVALID_REQUEST: CollaborationErrorCode
COLLABORATION_ERROR_CODE_UNSUPPORTED_VERSION: CollaborationErrorCode
COLLABORATION_ERROR_CODE_ENGINE_UNAVAILABLE: CollaborationErrorCode
COLLABORATION_ERROR_CODE_RESOURCE_EXHAUSTED: CollaborationErrorCode
COLLABORATION_ERROR_CODE_DUPLICATE_RUN: CollaborationErrorCode
COLLABORATION_ERROR_CODE_ROOM_BUSY: CollaborationErrorCode
COLLABORATION_ERROR_CODE_MODEL_NOT_CONFIGURED: CollaborationErrorCode
COLLABORATION_ERROR_CODE_MODEL_AUTHENTICATION_FAILED: CollaborationErrorCode
COLLABORATION_ERROR_CODE_MODEL_RATE_LIMITED: CollaborationErrorCode
COLLABORATION_ERROR_CODE_MODEL_TIMEOUT: CollaborationErrorCode
COLLABORATION_ERROR_CODE_TOOL_FAILED: CollaborationErrorCode
COLLABORATION_ERROR_CODE_OUTPUT_INVALID: CollaborationErrorCode
COLLABORATION_ERROR_CODE_CHECKPOINT_INVALID: CollaborationErrorCode
COLLABORATION_ERROR_CODE_PROTOCOL_ERROR: CollaborationErrorCode
COLLABORATION_ERROR_CODE_CANCELLED: CollaborationErrorCode
COLLABORATION_ERROR_CODE_DEADLINE_EXCEEDED: CollaborationErrorCode
COLLABORATION_ERROR_CODE_INTERNAL: CollaborationErrorCode

class RoomSnapshot(_message.Message):
    __slots__ = ("id", "name", "status")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    status: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., status: _Optional[str] = ...) -> None: ...

class AgentSnapshot(_message.Message):
    __slots__ = ("id", "name", "mention", "role", "description", "system_prompt", "runtime", "model_reference_id", "tool_names")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    MENTION_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    SYSTEM_PROMPT_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_FIELD_NUMBER: _ClassVar[int]
    MODEL_REFERENCE_ID_FIELD_NUMBER: _ClassVar[int]
    TOOL_NAMES_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    mention: str
    role: str
    description: str
    system_prompt: str
    runtime: str
    model_reference_id: str
    tool_names: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., mention: _Optional[str] = ..., role: _Optional[str] = ..., description: _Optional[str] = ..., system_prompt: _Optional[str] = ..., runtime: _Optional[str] = ..., model_reference_id: _Optional[str] = ..., tool_names: _Optional[_Iterable[str]] = ...) -> None: ...

class MessageSnapshot(_message.Message):
    __slots__ = ("id", "sender_id", "sender_name", "sender_type", "content", "created_at", "collaboration_run_id", "turn_index", "parent_message_id")
    ID_FIELD_NUMBER: _ClassVar[int]
    SENDER_ID_FIELD_NUMBER: _ClassVar[int]
    SENDER_NAME_FIELD_NUMBER: _ClassVar[int]
    SENDER_TYPE_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    COLLABORATION_RUN_ID_FIELD_NUMBER: _ClassVar[int]
    TURN_INDEX_FIELD_NUMBER: _ClassVar[int]
    PARENT_MESSAGE_ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    sender_id: str
    sender_name: str
    sender_type: SenderType
    content: str
    created_at: _timestamp_pb2.Timestamp
    collaboration_run_id: str
    turn_index: int
    parent_message_id: str
    def __init__(self, id: _Optional[str] = ..., sender_id: _Optional[str] = ..., sender_name: _Optional[str] = ..., sender_type: _Optional[_Union[SenderType, str]] = ..., content: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., collaboration_run_id: _Optional[str] = ..., turn_index: _Optional[int] = ..., parent_message_id: _Optional[str] = ...) -> None: ...

class KnowledgeChunk(_message.Message):
    __slots__ = ("id", "document_id", "document_name", "scope", "scope_id", "chunk_index", "content")
    ID_FIELD_NUMBER: _ClassVar[int]
    DOCUMENT_ID_FIELD_NUMBER: _ClassVar[int]
    DOCUMENT_NAME_FIELD_NUMBER: _ClassVar[int]
    SCOPE_FIELD_NUMBER: _ClassVar[int]
    SCOPE_ID_FIELD_NUMBER: _ClassVar[int]
    CHUNK_INDEX_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    id: str
    document_id: str
    document_name: str
    scope: str
    scope_id: str
    chunk_index: int
    content: str
    def __init__(self, id: _Optional[str] = ..., document_id: _Optional[str] = ..., document_name: _Optional[str] = ..., scope: _Optional[str] = ..., scope_id: _Optional[str] = ..., chunk_index: _Optional[int] = ..., content: _Optional[str] = ...) -> None: ...

class ModelReference(_message.Message):
    __slots__ = ("id", "profile_id", "source", "protocol", "model_name", "runtime_scope")
    ID_FIELD_NUMBER: _ClassVar[int]
    PROFILE_ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_SCOPE_FIELD_NUMBER: _ClassVar[int]
    id: str
    profile_id: str
    source: str
    protocol: str
    model_name: str
    runtime_scope: str
    def __init__(self, id: _Optional[str] = ..., profile_id: _Optional[str] = ..., source: _Optional[str] = ..., protocol: _Optional[str] = ..., model_name: _Optional[str] = ..., runtime_scope: _Optional[str] = ...) -> None: ...

class CollaborationPolicySnapshot(_message.Message):
    __slots__ = ("version", "engine", "trigger_mode", "max_turns", "max_turns_per_agent", "allow_agent_handoff", "allow_self_followup", "cooldown", "stop_on_empty_output", "stop_on_repeated_output")
    VERSION_FIELD_NUMBER: _ClassVar[int]
    ENGINE_FIELD_NUMBER: _ClassVar[int]
    TRIGGER_MODE_FIELD_NUMBER: _ClassVar[int]
    MAX_TURNS_FIELD_NUMBER: _ClassVar[int]
    MAX_TURNS_PER_AGENT_FIELD_NUMBER: _ClassVar[int]
    ALLOW_AGENT_HANDOFF_FIELD_NUMBER: _ClassVar[int]
    ALLOW_SELF_FOLLOWUP_FIELD_NUMBER: _ClassVar[int]
    COOLDOWN_FIELD_NUMBER: _ClassVar[int]
    STOP_ON_EMPTY_OUTPUT_FIELD_NUMBER: _ClassVar[int]
    STOP_ON_REPEATED_OUTPUT_FIELD_NUMBER: _ClassVar[int]
    version: str
    engine: CollaborationEngine
    trigger_mode: TriggerMode
    max_turns: int
    max_turns_per_agent: int
    allow_agent_handoff: bool
    allow_self_followup: bool
    cooldown: _duration_pb2.Duration
    stop_on_empty_output: bool
    stop_on_repeated_output: bool
    def __init__(self, version: _Optional[str] = ..., engine: _Optional[_Union[CollaborationEngine, str]] = ..., trigger_mode: _Optional[_Union[TriggerMode, str]] = ..., max_turns: _Optional[int] = ..., max_turns_per_agent: _Optional[int] = ..., allow_agent_handoff: _Optional[bool] = ..., allow_self_followup: _Optional[bool] = ..., cooldown: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., stop_on_empty_output: _Optional[bool] = ..., stop_on_repeated_output: _Optional[bool] = ...) -> None: ...

class ExecutionLimits(_message.Message):
    __slots__ = ("timeout", "max_output_bytes", "max_artifact_bytes", "max_tool_steps", "max_request_bytes", "max_event_bytes", "max_checkpoint_bytes")
    TIMEOUT_FIELD_NUMBER: _ClassVar[int]
    MAX_OUTPUT_BYTES_FIELD_NUMBER: _ClassVar[int]
    MAX_ARTIFACT_BYTES_FIELD_NUMBER: _ClassVar[int]
    MAX_TOOL_STEPS_FIELD_NUMBER: _ClassVar[int]
    MAX_REQUEST_BYTES_FIELD_NUMBER: _ClassVar[int]
    MAX_EVENT_BYTES_FIELD_NUMBER: _ClassVar[int]
    MAX_CHECKPOINT_BYTES_FIELD_NUMBER: _ClassVar[int]
    timeout: _duration_pb2.Duration
    max_output_bytes: int
    max_artifact_bytes: int
    max_tool_steps: int
    max_request_bytes: int
    max_event_bytes: int
    max_checkpoint_bytes: int
    def __init__(self, timeout: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., max_output_bytes: _Optional[int] = ..., max_artifact_bytes: _Optional[int] = ..., max_tool_steps: _Optional[int] = ..., max_request_bytes: _Optional[int] = ..., max_event_bytes: _Optional[int] = ..., max_checkpoint_bytes: _Optional[int] = ...) -> None: ...

class OpaqueCheckpoint(_message.Message):
    __slots__ = ("engine", "engine_version", "format_version", "sha256", "size_bytes", "payload")
    ENGINE_FIELD_NUMBER: _ClassVar[int]
    ENGINE_VERSION_FIELD_NUMBER: _ClassVar[int]
    FORMAT_VERSION_FIELD_NUMBER: _ClassVar[int]
    SHA256_FIELD_NUMBER: _ClassVar[int]
    SIZE_BYTES_FIELD_NUMBER: _ClassVar[int]
    PAYLOAD_FIELD_NUMBER: _ClassVar[int]
    engine: CollaborationEngine
    engine_version: str
    format_version: str
    sha256: str
    size_bytes: int
    payload: bytes
    def __init__(self, engine: _Optional[_Union[CollaborationEngine, str]] = ..., engine_version: _Optional[str] = ..., format_version: _Optional[str] = ..., sha256: _Optional[str] = ..., size_bytes: _Optional[int] = ..., payload: _Optional[bytes] = ...) -> None: ...

class ConversationSnapshot(_message.Message):
    __slots__ = ("room", "agents", "trigger", "transcript", "knowledge_chunks", "model_references", "policy", "limits", "initial_candidate_agent_ids")
    ROOM_FIELD_NUMBER: _ClassVar[int]
    AGENTS_FIELD_NUMBER: _ClassVar[int]
    TRIGGER_FIELD_NUMBER: _ClassVar[int]
    TRANSCRIPT_FIELD_NUMBER: _ClassVar[int]
    KNOWLEDGE_CHUNKS_FIELD_NUMBER: _ClassVar[int]
    MODEL_REFERENCES_FIELD_NUMBER: _ClassVar[int]
    POLICY_FIELD_NUMBER: _ClassVar[int]
    LIMITS_FIELD_NUMBER: _ClassVar[int]
    INITIAL_CANDIDATE_AGENT_IDS_FIELD_NUMBER: _ClassVar[int]
    room: RoomSnapshot
    agents: _containers.RepeatedCompositeFieldContainer[AgentSnapshot]
    trigger: MessageSnapshot
    transcript: _containers.RepeatedCompositeFieldContainer[MessageSnapshot]
    knowledge_chunks: _containers.RepeatedCompositeFieldContainer[KnowledgeChunk]
    model_references: _containers.RepeatedCompositeFieldContainer[ModelReference]
    policy: CollaborationPolicySnapshot
    limits: ExecutionLimits
    initial_candidate_agent_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, room: _Optional[_Union[RoomSnapshot, _Mapping]] = ..., agents: _Optional[_Iterable[_Union[AgentSnapshot, _Mapping]]] = ..., trigger: _Optional[_Union[MessageSnapshot, _Mapping]] = ..., transcript: _Optional[_Iterable[_Union[MessageSnapshot, _Mapping]]] = ..., knowledge_chunks: _Optional[_Iterable[_Union[KnowledgeChunk, _Mapping]]] = ..., model_references: _Optional[_Iterable[_Union[ModelReference, _Mapping]]] = ..., policy: _Optional[_Union[CollaborationPolicySnapshot, _Mapping]] = ..., limits: _Optional[_Union[ExecutionLimits, _Mapping]] = ..., initial_candidate_agent_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class ExecuteConversationRequest(_message.Message):
    __slots__ = ("protocol_version", "collaboration_run_id", "trace_id", "engine", "snapshot", "checkpoint")
    PROTOCOL_VERSION_FIELD_NUMBER: _ClassVar[int]
    COLLABORATION_RUN_ID_FIELD_NUMBER: _ClassVar[int]
    TRACE_ID_FIELD_NUMBER: _ClassVar[int]
    ENGINE_FIELD_NUMBER: _ClassVar[int]
    SNAPSHOT_FIELD_NUMBER: _ClassVar[int]
    CHECKPOINT_FIELD_NUMBER: _ClassVar[int]
    protocol_version: str
    collaboration_run_id: str
    trace_id: str
    engine: CollaborationEngine
    snapshot: ConversationSnapshot
    checkpoint: OpaqueCheckpoint
    def __init__(self, protocol_version: _Optional[str] = ..., collaboration_run_id: _Optional[str] = ..., trace_id: _Optional[str] = ..., engine: _Optional[_Union[CollaborationEngine, str]] = ..., snapshot: _Optional[_Union[ConversationSnapshot, _Mapping]] = ..., checkpoint: _Optional[_Union[OpaqueCheckpoint, _Mapping]] = ...) -> None: ...

class Usage(_message.Message):
    __slots__ = ("input_tokens", "output_tokens", "total_tokens")
    INPUT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_TOKENS_FIELD_NUMBER: _ClassVar[int]
    input_tokens: int
    output_tokens: int
    total_tokens: int
    def __init__(self, input_tokens: _Optional[int] = ..., output_tokens: _Optional[int] = ..., total_tokens: _Optional[int] = ...) -> None: ...

class ModelAudit(_message.Message):
    __slots__ = ("model_reference_id", "profile_id", "source", "model_name")
    MODEL_REFERENCE_ID_FIELD_NUMBER: _ClassVar[int]
    PROFILE_ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    model_reference_id: str
    profile_id: str
    source: str
    model_name: str
    def __init__(self, model_reference_id: _Optional[str] = ..., profile_id: _Optional[str] = ..., source: _Optional[str] = ..., model_name: _Optional[str] = ...) -> None: ...

class KnowledgeSource(_message.Message):
    __slots__ = ("document_id", "document_name", "scope")
    DOCUMENT_ID_FIELD_NUMBER: _ClassVar[int]
    DOCUMENT_NAME_FIELD_NUMBER: _ClassVar[int]
    SCOPE_FIELD_NUMBER: _ClassVar[int]
    document_id: str
    document_name: str
    scope: str
    def __init__(self, document_id: _Optional[str] = ..., document_name: _Optional[str] = ..., scope: _Optional[str] = ...) -> None: ...

class Artifact(_message.Message):
    __slots__ = ("id", "type", "title", "file_name", "mime_type", "content", "external_uri")
    ID_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    FILE_NAME_FIELD_NUMBER: _ClassVar[int]
    MIME_TYPE_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_URI_FIELD_NUMBER: _ClassVar[int]
    id: str
    type: str
    title: str
    file_name: str
    mime_type: str
    content: bytes
    external_uri: str
    def __init__(self, id: _Optional[str] = ..., type: _Optional[str] = ..., title: _Optional[str] = ..., file_name: _Optional[str] = ..., mime_type: _Optional[str] = ..., content: _Optional[bytes] = ..., external_uri: _Optional[str] = ...) -> None: ...

class CollaborationFailure(_message.Message):
    __slots__ = ("code", "message", "retryable")
    CODE_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    RETRYABLE_FIELD_NUMBER: _ClassVar[int]
    code: CollaborationErrorCode
    message: str
    retryable: bool
    def __init__(self, code: _Optional[_Union[CollaborationErrorCode, str]] = ..., message: _Optional[str] = ..., retryable: _Optional[bool] = ...) -> None: ...

class AcceptedEvent(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CollaborationStartedEvent(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SpeakerSelectedEvent(_message.Message):
    __slots__ = ("reason_category",)
    REASON_CATEGORY_FIELD_NUMBER: _ClassVar[int]
    reason_category: str
    def __init__(self, reason_category: _Optional[str] = ...) -> None: ...

class AgentTurnStartedEvent(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ModelStartedEvent(_message.Message):
    __slots__ = ("model_reference_id",)
    MODEL_REFERENCE_ID_FIELD_NUMBER: _ClassVar[int]
    model_reference_id: str
    def __init__(self, model_reference_id: _Optional[str] = ...) -> None: ...

class ModelCompletedEvent(_message.Message):
    __slots__ = ("model_reference_id", "usage")
    MODEL_REFERENCE_ID_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    model_reference_id: str
    usage: Usage
    def __init__(self, model_reference_id: _Optional[str] = ..., usage: _Optional[_Union[Usage, _Mapping]] = ...) -> None: ...

class ToolStartedEvent(_message.Message):
    __slots__ = ("tool_call_id", "tool_name", "input_summary")
    TOOL_CALL_ID_FIELD_NUMBER: _ClassVar[int]
    TOOL_NAME_FIELD_NUMBER: _ClassVar[int]
    INPUT_SUMMARY_FIELD_NUMBER: _ClassVar[int]
    tool_call_id: str
    tool_name: str
    input_summary: str
    def __init__(self, tool_call_id: _Optional[str] = ..., tool_name: _Optional[str] = ..., input_summary: _Optional[str] = ...) -> None: ...

class ToolCompletedEvent(_message.Message):
    __slots__ = ("tool_call_id", "tool_name", "output_summary")
    TOOL_CALL_ID_FIELD_NUMBER: _ClassVar[int]
    TOOL_NAME_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_SUMMARY_FIELD_NUMBER: _ClassVar[int]
    tool_call_id: str
    tool_name: str
    output_summary: str
    def __init__(self, tool_call_id: _Optional[str] = ..., tool_name: _Optional[str] = ..., output_summary: _Optional[str] = ...) -> None: ...

class ToolFailedEvent(_message.Message):
    __slots__ = ("tool_call_id", "tool_name", "failure")
    TOOL_CALL_ID_FIELD_NUMBER: _ClassVar[int]
    TOOL_NAME_FIELD_NUMBER: _ClassVar[int]
    FAILURE_FIELD_NUMBER: _ClassVar[int]
    tool_call_id: str
    tool_name: str
    failure: CollaborationFailure
    def __init__(self, tool_call_id: _Optional[str] = ..., tool_name: _Optional[str] = ..., failure: _Optional[_Union[CollaborationFailure, _Mapping]] = ...) -> None: ...

class OutputDeltaEvent(_message.Message):
    __slots__ = ("text",)
    TEXT_FIELD_NUMBER: _ClassVar[int]
    text: str
    def __init__(self, text: _Optional[str] = ...) -> None: ...

class ArtifactReadyEvent(_message.Message):
    __slots__ = ("artifact",)
    ARTIFACT_FIELD_NUMBER: _ClassVar[int]
    artifact: Artifact
    def __init__(self, artifact: _Optional[_Union[Artifact, _Mapping]] = ...) -> None: ...

class HandoffRequestedEvent(_message.Message):
    __slots__ = ("target_agent_id", "reason_category")
    TARGET_AGENT_ID_FIELD_NUMBER: _ClassVar[int]
    REASON_CATEGORY_FIELD_NUMBER: _ClassVar[int]
    target_agent_id: str
    reason_category: str
    def __init__(self, target_agent_id: _Optional[str] = ..., reason_category: _Optional[str] = ...) -> None: ...

class AgentMessageCompletedEvent(_message.Message):
    __slots__ = ("content", "artifacts", "knowledge_sources", "model", "usage")
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    ARTIFACTS_FIELD_NUMBER: _ClassVar[int]
    KNOWLEDGE_SOURCES_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    content: str
    artifacts: _containers.RepeatedCompositeFieldContainer[Artifact]
    knowledge_sources: _containers.RepeatedCompositeFieldContainer[KnowledgeSource]
    model: ModelAudit
    usage: Usage
    def __init__(self, content: _Optional[str] = ..., artifacts: _Optional[_Iterable[_Union[Artifact, _Mapping]]] = ..., knowledge_sources: _Optional[_Iterable[_Union[KnowledgeSource, _Mapping]]] = ..., model: _Optional[_Union[ModelAudit, _Mapping]] = ..., usage: _Optional[_Union[Usage, _Mapping]] = ...) -> None: ...

class CheckpointEvent(_message.Message):
    __slots__ = ("checkpoint",)
    CHECKPOINT_FIELD_NUMBER: _ClassVar[int]
    checkpoint: OpaqueCheckpoint
    def __init__(self, checkpoint: _Optional[_Union[OpaqueCheckpoint, _Mapping]] = ...) -> None: ...

class CompletedEvent(_message.Message):
    __slots__ = ("turn_count", "reason")
    TURN_COUNT_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    turn_count: int
    reason: CollaborationStopReason
    def __init__(self, turn_count: _Optional[int] = ..., reason: _Optional[_Union[CollaborationStopReason, str]] = ...) -> None: ...

class StoppedEvent(_message.Message):
    __slots__ = ("turn_count", "reason")
    TURN_COUNT_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    turn_count: int
    reason: CollaborationStopReason
    def __init__(self, turn_count: _Optional[int] = ..., reason: _Optional[_Union[CollaborationStopReason, str]] = ...) -> None: ...

class CancelledEvent(_message.Message):
    __slots__ = ("turn_count", "reason")
    TURN_COUNT_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    turn_count: int
    reason: CollaborationStopReason
    def __init__(self, turn_count: _Optional[int] = ..., reason: _Optional[_Union[CollaborationStopReason, str]] = ...) -> None: ...

class FailedEvent(_message.Message):
    __slots__ = ("turn_count", "reason", "failure")
    TURN_COUNT_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    FAILURE_FIELD_NUMBER: _ClassVar[int]
    turn_count: int
    reason: CollaborationStopReason
    failure: CollaborationFailure
    def __init__(self, turn_count: _Optional[int] = ..., reason: _Optional[_Union[CollaborationStopReason, str]] = ..., failure: _Optional[_Union[CollaborationFailure, _Mapping]] = ...) -> None: ...

class CollaborationEvent(_message.Message):
    __slots__ = ("protocol_version", "collaboration_run_id", "sequence", "occurred_at", "turn_id", "agent_id", "accepted", "collaboration_started", "speaker_selected", "agent_turn_started", "model_started", "model_completed", "tool_started", "tool_completed", "tool_failed", "output_delta", "artifact_ready", "handoff_requested", "agent_message_completed", "checkpoint", "completed", "stopped", "cancelled", "failed")
    PROTOCOL_VERSION_FIELD_NUMBER: _ClassVar[int]
    COLLABORATION_RUN_ID_FIELD_NUMBER: _ClassVar[int]
    SEQUENCE_FIELD_NUMBER: _ClassVar[int]
    OCCURRED_AT_FIELD_NUMBER: _ClassVar[int]
    TURN_ID_FIELD_NUMBER: _ClassVar[int]
    AGENT_ID_FIELD_NUMBER: _ClassVar[int]
    ACCEPTED_FIELD_NUMBER: _ClassVar[int]
    COLLABORATION_STARTED_FIELD_NUMBER: _ClassVar[int]
    SPEAKER_SELECTED_FIELD_NUMBER: _ClassVar[int]
    AGENT_TURN_STARTED_FIELD_NUMBER: _ClassVar[int]
    MODEL_STARTED_FIELD_NUMBER: _ClassVar[int]
    MODEL_COMPLETED_FIELD_NUMBER: _ClassVar[int]
    TOOL_STARTED_FIELD_NUMBER: _ClassVar[int]
    TOOL_COMPLETED_FIELD_NUMBER: _ClassVar[int]
    TOOL_FAILED_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_DELTA_FIELD_NUMBER: _ClassVar[int]
    ARTIFACT_READY_FIELD_NUMBER: _ClassVar[int]
    HANDOFF_REQUESTED_FIELD_NUMBER: _ClassVar[int]
    AGENT_MESSAGE_COMPLETED_FIELD_NUMBER: _ClassVar[int]
    CHECKPOINT_FIELD_NUMBER: _ClassVar[int]
    COMPLETED_FIELD_NUMBER: _ClassVar[int]
    STOPPED_FIELD_NUMBER: _ClassVar[int]
    CANCELLED_FIELD_NUMBER: _ClassVar[int]
    FAILED_FIELD_NUMBER: _ClassVar[int]
    protocol_version: str
    collaboration_run_id: str
    sequence: int
    occurred_at: _timestamp_pb2.Timestamp
    turn_id: str
    agent_id: str
    accepted: AcceptedEvent
    collaboration_started: CollaborationStartedEvent
    speaker_selected: SpeakerSelectedEvent
    agent_turn_started: AgentTurnStartedEvent
    model_started: ModelStartedEvent
    model_completed: ModelCompletedEvent
    tool_started: ToolStartedEvent
    tool_completed: ToolCompletedEvent
    tool_failed: ToolFailedEvent
    output_delta: OutputDeltaEvent
    artifact_ready: ArtifactReadyEvent
    handoff_requested: HandoffRequestedEvent
    agent_message_completed: AgentMessageCompletedEvent
    checkpoint: CheckpointEvent
    completed: CompletedEvent
    stopped: StoppedEvent
    cancelled: CancelledEvent
    failed: FailedEvent
    def __init__(self, protocol_version: _Optional[str] = ..., collaboration_run_id: _Optional[str] = ..., sequence: _Optional[int] = ..., occurred_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., turn_id: _Optional[str] = ..., agent_id: _Optional[str] = ..., accepted: _Optional[_Union[AcceptedEvent, _Mapping]] = ..., collaboration_started: _Optional[_Union[CollaborationStartedEvent, _Mapping]] = ..., speaker_selected: _Optional[_Union[SpeakerSelectedEvent, _Mapping]] = ..., agent_turn_started: _Optional[_Union[AgentTurnStartedEvent, _Mapping]] = ..., model_started: _Optional[_Union[ModelStartedEvent, _Mapping]] = ..., model_completed: _Optional[_Union[ModelCompletedEvent, _Mapping]] = ..., tool_started: _Optional[_Union[ToolStartedEvent, _Mapping]] = ..., tool_completed: _Optional[_Union[ToolCompletedEvent, _Mapping]] = ..., tool_failed: _Optional[_Union[ToolFailedEvent, _Mapping]] = ..., output_delta: _Optional[_Union[OutputDeltaEvent, _Mapping]] = ..., artifact_ready: _Optional[_Union[ArtifactReadyEvent, _Mapping]] = ..., handoff_requested: _Optional[_Union[HandoffRequestedEvent, _Mapping]] = ..., agent_message_completed: _Optional[_Union[AgentMessageCompletedEvent, _Mapping]] = ..., checkpoint: _Optional[_Union[CheckpointEvent, _Mapping]] = ..., completed: _Optional[_Union[CompletedEvent, _Mapping]] = ..., stopped: _Optional[_Union[StoppedEvent, _Mapping]] = ..., cancelled: _Optional[_Union[CancelledEvent, _Mapping]] = ..., failed: _Optional[_Union[FailedEvent, _Mapping]] = ...) -> None: ...
