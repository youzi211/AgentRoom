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

class ExecutorKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    EXECUTOR_KIND_UNSPECIFIED: _ClassVar[ExecutorKind]
    EXECUTOR_KIND_LLM: _ClassVar[ExecutorKind]
    EXECUTOR_KIND_DEEPAGENT: _ClassVar[ExecutorKind]

class SenderType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SENDER_TYPE_UNSPECIFIED: _ClassVar[SenderType]
    SENDER_TYPE_HUMAN: _ClassVar[SenderType]
    SENDER_TYPE_AGENT: _ClassVar[SenderType]
    SENDER_TYPE_SYSTEM: _ClassVar[SenderType]

class RunErrorCode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    RUN_ERROR_CODE_UNSPECIFIED: _ClassVar[RunErrorCode]
    RUN_ERROR_CODE_INVALID_REQUEST: _ClassVar[RunErrorCode]
    RUN_ERROR_CODE_MODEL_NOT_CONFIGURED: _ClassVar[RunErrorCode]
    RUN_ERROR_CODE_MODEL_AUTHENTICATION_FAILED: _ClassVar[RunErrorCode]
    RUN_ERROR_CODE_MODEL_RATE_LIMITED: _ClassVar[RunErrorCode]
    RUN_ERROR_CODE_MODEL_TIMEOUT: _ClassVar[RunErrorCode]
    RUN_ERROR_CODE_CANCELLED: _ClassVar[RunErrorCode]
    RUN_ERROR_CODE_TOOL_FAILED: _ClassVar[RunErrorCode]
    RUN_ERROR_CODE_OUTPUT_INVALID: _ClassVar[RunErrorCode]
    RUN_ERROR_CODE_RESOURCE_EXHAUSTED: _ClassVar[RunErrorCode]
    RUN_ERROR_CODE_EXECUTOR_UNAVAILABLE: _ClassVar[RunErrorCode]
    RUN_ERROR_CODE_PROTOCOL_ERROR: _ClassVar[RunErrorCode]
    RUN_ERROR_CODE_INTERNAL: _ClassVar[RunErrorCode]
EXECUTOR_KIND_UNSPECIFIED: ExecutorKind
EXECUTOR_KIND_LLM: ExecutorKind
EXECUTOR_KIND_DEEPAGENT: ExecutorKind
SENDER_TYPE_UNSPECIFIED: SenderType
SENDER_TYPE_HUMAN: SenderType
SENDER_TYPE_AGENT: SenderType
SENDER_TYPE_SYSTEM: SenderType
RUN_ERROR_CODE_UNSPECIFIED: RunErrorCode
RUN_ERROR_CODE_INVALID_REQUEST: RunErrorCode
RUN_ERROR_CODE_MODEL_NOT_CONFIGURED: RunErrorCode
RUN_ERROR_CODE_MODEL_AUTHENTICATION_FAILED: RunErrorCode
RUN_ERROR_CODE_MODEL_RATE_LIMITED: RunErrorCode
RUN_ERROR_CODE_MODEL_TIMEOUT: RunErrorCode
RUN_ERROR_CODE_CANCELLED: RunErrorCode
RUN_ERROR_CODE_TOOL_FAILED: RunErrorCode
RUN_ERROR_CODE_OUTPUT_INVALID: RunErrorCode
RUN_ERROR_CODE_RESOURCE_EXHAUSTED: RunErrorCode
RUN_ERROR_CODE_EXECUTOR_UNAVAILABLE: RunErrorCode
RUN_ERROR_CODE_PROTOCOL_ERROR: RunErrorCode
RUN_ERROR_CODE_INTERNAL: RunErrorCode

class RoomSnapshot(_message.Message):
    __slots__ = ("id", "name", "dialogue_mode")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    DIALOGUE_MODE_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    dialogue_mode: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., dialogue_mode: _Optional[str] = ...) -> None: ...

class AgentSnapshot(_message.Message):
    __slots__ = ("id", "name", "mention", "role", "description", "system_prompt", "runtime", "model_profile_id")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    MENTION_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    SYSTEM_PROMPT_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_FIELD_NUMBER: _ClassVar[int]
    MODEL_PROFILE_ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    mention: str
    role: str
    description: str
    system_prompt: str
    runtime: str
    model_profile_id: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., mention: _Optional[str] = ..., role: _Optional[str] = ..., description: _Optional[str] = ..., system_prompt: _Optional[str] = ..., runtime: _Optional[str] = ..., model_profile_id: _Optional[str] = ...) -> None: ...

class MessageSnapshot(_message.Message):
    __slots__ = ("id", "sender_id", "sender_name", "sender_type", "content", "created_at", "dialogue_run_id", "turn_index", "parent_message_id")
    ID_FIELD_NUMBER: _ClassVar[int]
    SENDER_ID_FIELD_NUMBER: _ClassVar[int]
    SENDER_NAME_FIELD_NUMBER: _ClassVar[int]
    SENDER_TYPE_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    DIALOGUE_RUN_ID_FIELD_NUMBER: _ClassVar[int]
    TURN_INDEX_FIELD_NUMBER: _ClassVar[int]
    PARENT_MESSAGE_ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    sender_id: str
    sender_name: str
    sender_type: SenderType
    content: str
    created_at: _timestamp_pb2.Timestamp
    dialogue_run_id: str
    turn_index: int
    parent_message_id: str
    def __init__(self, id: _Optional[str] = ..., sender_id: _Optional[str] = ..., sender_name: _Optional[str] = ..., sender_type: _Optional[_Union[SenderType, str]] = ..., content: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., dialogue_run_id: _Optional[str] = ..., turn_index: _Optional[int] = ..., parent_message_id: _Optional[str] = ...) -> None: ...

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

class ModelConnection(_message.Message):
    __slots__ = ("protocol", "base_url", "model_name", "api_key", "profile_id", "source")
    PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    BASE_URL_FIELD_NUMBER: _ClassVar[int]
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    API_KEY_FIELD_NUMBER: _ClassVar[int]
    PROFILE_ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    protocol: str
    base_url: str
    model_name: str
    api_key: str
    profile_id: str
    source: str
    def __init__(self, protocol: _Optional[str] = ..., base_url: _Optional[str] = ..., model_name: _Optional[str] = ..., api_key: _Optional[str] = ..., profile_id: _Optional[str] = ..., source: _Optional[str] = ...) -> None: ...

class ExecutionLimits(_message.Message):
    __slots__ = ("timeout", "max_output_bytes", "max_artifact_bytes", "max_tool_steps")
    TIMEOUT_FIELD_NUMBER: _ClassVar[int]
    MAX_OUTPUT_BYTES_FIELD_NUMBER: _ClassVar[int]
    MAX_ARTIFACT_BYTES_FIELD_NUMBER: _ClassVar[int]
    MAX_TOOL_STEPS_FIELD_NUMBER: _ClassVar[int]
    timeout: _duration_pb2.Duration
    max_output_bytes: int
    max_artifact_bytes: int
    max_tool_steps: int
    def __init__(self, timeout: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., max_output_bytes: _Optional[int] = ..., max_artifact_bytes: _Optional[int] = ..., max_tool_steps: _Optional[int] = ...) -> None: ...

class ParticipantSnapshot(_message.Message):
    __slots__ = ("id", "name")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ...) -> None: ...

class PromptContextSnapshot(_message.Message):
    __slots__ = ("room_name", "dialogue_mode", "online_human_participants", "room_agents", "trigger_sender", "trigger_sender_type", "trigger_content", "latest_visible_speaker", "latest_visible_speaker_type", "transcript", "knowledge_chunks", "current_speaker", "root_human_trigger_sender", "root_human_trigger_type", "root_human_trigger_content", "autonomous_turn_index", "max_autonomous_turns", "allow_self_followup", "allow_agent_to_agent_mentions", "max_turns_per_agent", "response_strategy", "eligible_peers")
    ROOM_NAME_FIELD_NUMBER: _ClassVar[int]
    DIALOGUE_MODE_FIELD_NUMBER: _ClassVar[int]
    ONLINE_HUMAN_PARTICIPANTS_FIELD_NUMBER: _ClassVar[int]
    ROOM_AGENTS_FIELD_NUMBER: _ClassVar[int]
    TRIGGER_SENDER_FIELD_NUMBER: _ClassVar[int]
    TRIGGER_SENDER_TYPE_FIELD_NUMBER: _ClassVar[int]
    TRIGGER_CONTENT_FIELD_NUMBER: _ClassVar[int]
    LATEST_VISIBLE_SPEAKER_FIELD_NUMBER: _ClassVar[int]
    LATEST_VISIBLE_SPEAKER_TYPE_FIELD_NUMBER: _ClassVar[int]
    TRANSCRIPT_FIELD_NUMBER: _ClassVar[int]
    KNOWLEDGE_CHUNKS_FIELD_NUMBER: _ClassVar[int]
    CURRENT_SPEAKER_FIELD_NUMBER: _ClassVar[int]
    ROOT_HUMAN_TRIGGER_SENDER_FIELD_NUMBER: _ClassVar[int]
    ROOT_HUMAN_TRIGGER_TYPE_FIELD_NUMBER: _ClassVar[int]
    ROOT_HUMAN_TRIGGER_CONTENT_FIELD_NUMBER: _ClassVar[int]
    AUTONOMOUS_TURN_INDEX_FIELD_NUMBER: _ClassVar[int]
    MAX_AUTONOMOUS_TURNS_FIELD_NUMBER: _ClassVar[int]
    ALLOW_SELF_FOLLOWUP_FIELD_NUMBER: _ClassVar[int]
    ALLOW_AGENT_TO_AGENT_MENTIONS_FIELD_NUMBER: _ClassVar[int]
    MAX_TURNS_PER_AGENT_FIELD_NUMBER: _ClassVar[int]
    RESPONSE_STRATEGY_FIELD_NUMBER: _ClassVar[int]
    ELIGIBLE_PEERS_FIELD_NUMBER: _ClassVar[int]
    room_name: str
    dialogue_mode: str
    online_human_participants: _containers.RepeatedCompositeFieldContainer[ParticipantSnapshot]
    room_agents: _containers.RepeatedCompositeFieldContainer[AgentSnapshot]
    trigger_sender: str
    trigger_sender_type: SenderType
    trigger_content: str
    latest_visible_speaker: str
    latest_visible_speaker_type: SenderType
    transcript: _containers.RepeatedCompositeFieldContainer[MessageSnapshot]
    knowledge_chunks: _containers.RepeatedCompositeFieldContainer[KnowledgeChunk]
    current_speaker: AgentSnapshot
    root_human_trigger_sender: str
    root_human_trigger_type: SenderType
    root_human_trigger_content: str
    autonomous_turn_index: int
    max_autonomous_turns: int
    allow_self_followup: bool
    allow_agent_to_agent_mentions: bool
    max_turns_per_agent: int
    response_strategy: str
    eligible_peers: _containers.RepeatedCompositeFieldContainer[AgentSnapshot]
    def __init__(self, room_name: _Optional[str] = ..., dialogue_mode: _Optional[str] = ..., online_human_participants: _Optional[_Iterable[_Union[ParticipantSnapshot, _Mapping]]] = ..., room_agents: _Optional[_Iterable[_Union[AgentSnapshot, _Mapping]]] = ..., trigger_sender: _Optional[str] = ..., trigger_sender_type: _Optional[_Union[SenderType, str]] = ..., trigger_content: _Optional[str] = ..., latest_visible_speaker: _Optional[str] = ..., latest_visible_speaker_type: _Optional[_Union[SenderType, str]] = ..., transcript: _Optional[_Iterable[_Union[MessageSnapshot, _Mapping]]] = ..., knowledge_chunks: _Optional[_Iterable[_Union[KnowledgeChunk, _Mapping]]] = ..., current_speaker: _Optional[_Union[AgentSnapshot, _Mapping]] = ..., root_human_trigger_sender: _Optional[str] = ..., root_human_trigger_type: _Optional[_Union[SenderType, str]] = ..., root_human_trigger_content: _Optional[str] = ..., autonomous_turn_index: _Optional[int] = ..., max_autonomous_turns: _Optional[int] = ..., allow_self_followup: _Optional[bool] = ..., allow_agent_to_agent_mentions: _Optional[bool] = ..., max_turns_per_agent: _Optional[int] = ..., response_strategy: _Optional[str] = ..., eligible_peers: _Optional[_Iterable[_Union[AgentSnapshot, _Mapping]]] = ...) -> None: ...

class ExecuteAgentRequest(_message.Message):
    __slots__ = ("protocol_version", "run_id", "trace_id", "dialogue_run_id", "executor_kind", "room", "agent", "trigger", "recent_messages", "knowledge_chunks", "model", "limits", "metadata", "prompt_context")
    class MetadataEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    PROTOCOL_VERSION_FIELD_NUMBER: _ClassVar[int]
    RUN_ID_FIELD_NUMBER: _ClassVar[int]
    TRACE_ID_FIELD_NUMBER: _ClassVar[int]
    DIALOGUE_RUN_ID_FIELD_NUMBER: _ClassVar[int]
    EXECUTOR_KIND_FIELD_NUMBER: _ClassVar[int]
    ROOM_FIELD_NUMBER: _ClassVar[int]
    AGENT_FIELD_NUMBER: _ClassVar[int]
    TRIGGER_FIELD_NUMBER: _ClassVar[int]
    RECENT_MESSAGES_FIELD_NUMBER: _ClassVar[int]
    KNOWLEDGE_CHUNKS_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    LIMITS_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    PROMPT_CONTEXT_FIELD_NUMBER: _ClassVar[int]
    protocol_version: str
    run_id: str
    trace_id: str
    dialogue_run_id: str
    executor_kind: ExecutorKind
    room: RoomSnapshot
    agent: AgentSnapshot
    trigger: MessageSnapshot
    recent_messages: _containers.RepeatedCompositeFieldContainer[MessageSnapshot]
    knowledge_chunks: _containers.RepeatedCompositeFieldContainer[KnowledgeChunk]
    model: ModelConnection
    limits: ExecutionLimits
    metadata: _containers.ScalarMap[str, str]
    prompt_context: PromptContextSnapshot
    def __init__(self, protocol_version: _Optional[str] = ..., run_id: _Optional[str] = ..., trace_id: _Optional[str] = ..., dialogue_run_id: _Optional[str] = ..., executor_kind: _Optional[_Union[ExecutorKind, str]] = ..., room: _Optional[_Union[RoomSnapshot, _Mapping]] = ..., agent: _Optional[_Union[AgentSnapshot, _Mapping]] = ..., trigger: _Optional[_Union[MessageSnapshot, _Mapping]] = ..., recent_messages: _Optional[_Iterable[_Union[MessageSnapshot, _Mapping]]] = ..., knowledge_chunks: _Optional[_Iterable[_Union[KnowledgeChunk, _Mapping]]] = ..., model: _Optional[_Union[ModelConnection, _Mapping]] = ..., limits: _Optional[_Union[ExecutionLimits, _Mapping]] = ..., metadata: _Optional[_Mapping[str, str]] = ..., prompt_context: _Optional[_Union[PromptContextSnapshot, _Mapping]] = ...) -> None: ...

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
    __slots__ = ("profile_id", "source", "model_name")
    PROFILE_ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    profile_id: str
    source: str
    model_name: str
    def __init__(self, profile_id: _Optional[str] = ..., source: _Optional[str] = ..., model_name: _Optional[str] = ...) -> None: ...

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

class RunFailure(_message.Message):
    __slots__ = ("code", "message", "retryable")
    CODE_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    RETRYABLE_FIELD_NUMBER: _ClassVar[int]
    code: RunErrorCode
    message: str
    retryable: bool
    def __init__(self, code: _Optional[_Union[RunErrorCode, str]] = ..., message: _Optional[str] = ..., retryable: _Optional[bool] = ...) -> None: ...

class AcceptedEvent(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ModelStartedEvent(_message.Message):
    __slots__ = ("model_name",)
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    model_name: str
    def __init__(self, model_name: _Optional[str] = ...) -> None: ...

class ModelCompletedEvent(_message.Message):
    __slots__ = ("model_name", "usage")
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    model_name: str
    usage: Usage
    def __init__(self, model_name: _Optional[str] = ..., usage: _Optional[_Union[Usage, _Mapping]] = ...) -> None: ...

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
    failure: RunFailure
    def __init__(self, tool_call_id: _Optional[str] = ..., tool_name: _Optional[str] = ..., failure: _Optional[_Union[RunFailure, _Mapping]] = ...) -> None: ...

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

class CompletedEvent(_message.Message):
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

class FailedEvent(_message.Message):
    __slots__ = ("failure",)
    FAILURE_FIELD_NUMBER: _ClassVar[int]
    failure: RunFailure
    def __init__(self, failure: _Optional[_Union[RunFailure, _Mapping]] = ...) -> None: ...

class AgentEvent(_message.Message):
    __slots__ = ("protocol_version", "run_id", "sequence", "occurred_at", "accepted", "model_started", "model_completed", "tool_started", "tool_completed", "tool_failed", "output_delta", "artifact_ready", "completed", "failed")
    PROTOCOL_VERSION_FIELD_NUMBER: _ClassVar[int]
    RUN_ID_FIELD_NUMBER: _ClassVar[int]
    SEQUENCE_FIELD_NUMBER: _ClassVar[int]
    OCCURRED_AT_FIELD_NUMBER: _ClassVar[int]
    ACCEPTED_FIELD_NUMBER: _ClassVar[int]
    MODEL_STARTED_FIELD_NUMBER: _ClassVar[int]
    MODEL_COMPLETED_FIELD_NUMBER: _ClassVar[int]
    TOOL_STARTED_FIELD_NUMBER: _ClassVar[int]
    TOOL_COMPLETED_FIELD_NUMBER: _ClassVar[int]
    TOOL_FAILED_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_DELTA_FIELD_NUMBER: _ClassVar[int]
    ARTIFACT_READY_FIELD_NUMBER: _ClassVar[int]
    COMPLETED_FIELD_NUMBER: _ClassVar[int]
    FAILED_FIELD_NUMBER: _ClassVar[int]
    protocol_version: str
    run_id: str
    sequence: int
    occurred_at: _timestamp_pb2.Timestamp
    accepted: AcceptedEvent
    model_started: ModelStartedEvent
    model_completed: ModelCompletedEvent
    tool_started: ToolStartedEvent
    tool_completed: ToolCompletedEvent
    tool_failed: ToolFailedEvent
    output_delta: OutputDeltaEvent
    artifact_ready: ArtifactReadyEvent
    completed: CompletedEvent
    failed: FailedEvent
    def __init__(self, protocol_version: _Optional[str] = ..., run_id: _Optional[str] = ..., sequence: _Optional[int] = ..., occurred_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., accepted: _Optional[_Union[AcceptedEvent, _Mapping]] = ..., model_started: _Optional[_Union[ModelStartedEvent, _Mapping]] = ..., model_completed: _Optional[_Union[ModelCompletedEvent, _Mapping]] = ..., tool_started: _Optional[_Union[ToolStartedEvent, _Mapping]] = ..., tool_completed: _Optional[_Union[ToolCompletedEvent, _Mapping]] = ..., tool_failed: _Optional[_Union[ToolFailedEvent, _Mapping]] = ..., output_delta: _Optional[_Union[OutputDeltaEvent, _Mapping]] = ..., artifact_ready: _Optional[_Union[ArtifactReadyEvent, _Mapping]] = ..., completed: _Optional[_Union[CompletedEvent, _Mapping]] = ..., failed: _Optional[_Union[FailedEvent, _Mapping]] = ...) -> None: ...
