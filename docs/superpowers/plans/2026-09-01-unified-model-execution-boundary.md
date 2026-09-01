# Unified Model Execution Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 ADR-001 固化的 `ModelSelection → ModelConfigResolver → ModelConfig` 不变量落实到 Go Control Plane、Python Collaboration Runtime、LLM/DeepAgent Executor 与 AutoGen Selector，并在 `model_started` 前完成配置解析和脱敏失败映射。

**Architecture:** Go 只生成不含明文凭据的 `ModelSelection`；Python Agent Execution 层先通过统一的 `ModelConfigResolver` 组合模型元数据与 `CredentialResolver`，再把短生命周期的完整 `ModelConfig` 交给 Executor 或 Model Client。Native 与 AutoGen Engine 只携带选择态数据。第一阶段本地 Resolver 只解析 `environment:go` 和 `environment:deepagent`；`profile:<id>` 保持为生产 Secret Provider 的不透明引用，未配置 Adapter 时必须在 preparation 阶段失败，不能降级成环境凭据。

**Tech Stack:** Go 1.x、Python 3.12、gRPC/Protobuf、pytest、Go testing、OpenAI Python SDK、Microsoft AutoGen、MySQL（仅 Go Control Plane）

**Spec:** [`ADR-001-control-collaboration-agent-model-execution-boundaries.md`](../../architecture/decisions/ADR-001-control-collaboration-agent-model-execution-boundaries.md)

## Global Constraints

- [ ] 保持 Go 为房间、Agent、Model Profile 和消息的唯一数据权威；Python Runtime 不访问 AgentRoom MySQL。
- [ ] Collaboration gRPC 请求只包含 `ModelSelection`，不得包含 API key、Authorization header 或完整 `ModelConfig`。
- [ ] `credential_ref` 只允许被 `ModelConfigResolver` 和 `CredentialResolver` 处理，不得进入 Engine、Executor、Provider Client、事件、checkpoint 或日志。
- [ ] `ModelConfig` 是一次调用内的短生命周期对象，不序列化、不持久化、不进入审计对象。
- [ ] `profile:<profile_id>` 与 `environment:<runtime_scope>` 是不同凭据身份；禁止 database selection 隐式使用环境凭据。
- [ ] 第一阶段支持的环境引用固定为 `environment:go`（`LLM_*`）与 `environment:deepagent`（`MODEL_*`）；未知引用返回 `credential_not_found`。
- [ ] Native 和 AutoGen 在同一 run 内不得因为配置失败回退到 legacy engine。
- [ ] 修改 Protobuf 后只运行生成脚本，不手工编辑 `*.pb.go` 或 `*_pb2*.py`。
- [ ] 不修改或暂存当前用户已有的 frontend、截图、`.mcp.json` 和 `design-system/` 工作树改动。

## Target File Map

```text
proto/collaboration_runtime/v1/collaboration_runtime.proto
    Collaboration 边界的 ModelSelection、purpose 与 selector selection

backend/internal/model/model_profile.go
backend/internal/service/collaboration_scheduler.go
backend/internal/collaboration/{types,snapshot,mapping}.go
    Go 产生、校验并映射稳定 ModelSelection；不传 API key

deepagent/src/agent_runtime/model_config.py
    ModelConfig、ModelConfigResolver、CredentialResolver、环境 Adapter、准备期错误

deepagent/src/agent_runtime/model_adapter.py
deepagent/src/agent_runtime/executors/{base,llm,deepagent}.py
deepagent/src/agent_runtime/{context,service,server}.py
    Executor 只接收完整 ModelConfig；服务边界负责 preparation

deepagent/src/collaboration_runtime/{models,mapping,executor,agent_executor}.py
deepagent/src/collaboration_runtime/engines/{native,autogen}.py
    Engine 只转交 ModelSelection；RuntimeRegistryAgentExecutor 解析完整配置

deepagent/src/collaboration_runtime/model_execution.py
deepagent/src/collaboration_runtime/{model_client,model_gateway}.py
    AutoGen Selector 复用相同 Resolver 和 Model Client Factory

backend/internal/tests/**
deepagent/tests/**
    合约、行为、事件顺序、安全边界和端到端回归
```

---

## Task 1: Replace Collaboration ModelReference with ModelSelection

**Files:**

- Modify: `proto/collaboration_runtime/v1/collaboration_runtime.proto`
- Modify: `backend/internal/model/model_profile.go`
- Modify: `backend/internal/config/env.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/service/collaboration_scheduler.go`
- Modify: `backend/internal/collaboration/types.go`
- Modify: `backend/internal/collaboration/snapshot.go`
- Modify: `backend/internal/collaboration/mapping.go`
- Modify: `backend/internal/tests/service/collaboration_scheduler_test.go`
- Modify: `backend/internal/tests/config/collaboration_runtime_config_test.go`
- Modify: `backend/internal/tests/collaboration/snapshot_test.go`
- Modify: `backend/internal/tests/collaborationproto/contract_test.go`
- Regenerate: `backend/internal/collaboration/proto/v1/*.pb.go`
- Regenerate: `deepagent/src/collaboration_runtime/v1/*_pb2*.py`

- [ ] **Step 1: Write failing Go contract tests for selection semantics**

Add assertions that the descriptor exposes `ModelSelection` with non-secret fields, including `credential_ref` and `purpose`, and that no forbidden credential fields exist:

```go
fields := (&collaborationruntimev1.ModelSelection{}).ProtoReflect().Descriptor().Fields()
requireField(t, fields, "credential_ref")
requireField(t, fields, "purpose")
for _, forbidden := range []protoreflect.Name{"api_key", "authorization", "provider_response"} {
    if fields.ByName(forbidden) != nil {
        t.Fatalf("ModelSelection exposes %q", forbidden)
    }
}
```

Update scheduler tests to require these exact mappings:

```text
environment source + go scope        -> environment:go
environment source + deepagent scope -> environment:deepagent
database source + profile p1         -> profile:p1
participant agent selection          -> purpose=participant
AutoGen selector selection            -> purpose=selector
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```powershell
go -C backend test ./internal/tests/service ./internal/tests/collaboration ./internal/tests/collaborationproto
```

Expected: compile or assertion failures because `ModelSelection`, `credential_ref`, `purpose`, and selector selection are not present.

- [ ] **Step 3: Define the source-of-truth selection types**

Replace the collaboration-domain `ModelReference` with:

```go
type ModelPurpose string

const (
    ModelPurposeParticipant ModelPurpose = "participant"
    ModelPurposeSelector    ModelPurpose = "selector"
)

type ModelSelection struct {
    ID            string
    ProfileID     string
    Source        string
    Protocol      string
    ModelName     string
    CredentialRef string
    RuntimeScope  string
    Purpose       ModelPurpose
}
```

Rename `AgentSnapshot.ModelReferenceID` to `ModelSelectionID`, `ConversationSnapshot.ModelReferences` to `ModelSelections`, and retain model audit event field names only where wire/audit compatibility is required. Do not put endpoint or secret material into the selection.

- [ ] **Step 4: Make Go produce opaque credential references**

Add a small pure helper in `collaboration_scheduler.go`:

```go
func credentialRef(resolved ResolvedModelConfig, scope string) (string, error) {
    switch strings.TrimSpace(resolved.Source) {
    case "environment":
        return "environment:" + scope, nil
    case "database":
        if strings.TrimSpace(resolved.ProfileID) == "" {
            return "", ErrInvalidRemoteCollaboration
        }
        return "profile:" + resolved.ProfileID, nil
    default:
        return "", ErrInvalidRemoteCollaboration
    }
}
```

For AutoGen runs, resolve one additional selector selection in the Go Control Plane. Add `COLLABORATION_SELECTOR_MODEL_PROFILE_ID` to `CollaborationRuntimeConfig` and pass it through `RemoteCollaborationConfig`; use it when present, otherwise resolve the normal `go` default. Give the result a distinct ID and `purpose=selector`. Native requests must not carry an unused selector selection.

- [ ] **Step 5: Change the Protobuf contract without leaking credentials**

Define `ModelPurpose`, `ModelSelection`, `AgentSnapshot.model_selection_id`, `ConversationSnapshot.model_selections`, and `ConversationSnapshot.selector_model_selection_id`. Preserve existing field numbers wherever a field is semantically renamed, and allocate only new numbers for `credential_ref`, `purpose`, and selector ID.

- [ ] **Step 6: Regenerate both language bindings**

Run:

```powershell
bash scripts/generate-collaboration-runtime-proto.sh
bash scripts/check-collaboration-runtime-proto.sh
```

Expected: generation and freshness check both exit 0.

- [ ] **Step 7: Update Go builders/mappers and make tests GREEN**

Update snapshot validation so every participant selection has a non-empty `credential_ref`, matching runtime scope, and `purpose=participant`; selector selection must exist only for AutoGen and have `purpose=selector`.

Run:

```powershell
go -C backend test ./internal/tests/service ./internal/tests/collaboration ./internal/tests/collaborationproto
```

Expected: all focused Go tests pass.

- [ ] **Step 8: Commit the contract slice**

```powershell
git add proto/collaboration_runtime/v1/collaboration_runtime.proto backend/internal/model/model_profile.go backend/internal/config/env.go backend/cmd/server/main.go backend/internal/service/collaboration_scheduler.go backend/internal/collaboration backend/internal/tests/service/collaboration_scheduler_test.go backend/internal/tests/config/collaboration_runtime_config_test.go backend/internal/tests/collaboration backend/internal/tests/collaborationproto deepagent/src/collaboration_runtime/v1
git commit -m "feat: 引入协作模型选择合约"
```

---

## Task 2: Implement the Unified Python ModelConfigResolver

**Files:**

- Create: `deepagent/src/agent_runtime/model_config.py`
- Modify: `deepagent/src/agent_runtime/config.py`
- Create: `deepagent/tests/test_model_config_resolver.py`
- Modify: `deepagent/tests/test_agent_runtime_security.py`

- [ ] **Step 1: Write resolver unit tests first**

Cover:

- `environment:go` resolves endpoint from `LLM_BASE_URL` and secret from `LLM_API_KEY`;
- `environment:deepagent` resolves endpoint from `MODEL_BASE_URL` and secret from `MODEL_API_KEY`;
- the model name and protocol come from the control-plane selection;
- missing secret returns `credential_not_found`;
- unknown `profile:*` returns `credential_not_found`, never an environment fallback;
- malformed selection returns `invalid_model_selection`;
- empty endpoint/model/credential after merging returns `invalid_model_config`;
- exceptions and `repr()` never contain the secret.

Use injected mappings, not process-global environment, in unit tests.

- [ ] **Step 2: Run the new tests and verify RED**

Run:

```powershell
uv --directory deepagent run pytest tests/test_model_config_resolver.py tests/test_agent_runtime_security.py -q
```

Expected: import failure for `agent_runtime.model_config`.

- [ ] **Step 3: Add immutable execution-domain types and ports**

Implement:

```python
@dataclass(frozen=True)
class ModelSelection:
    id: str
    profile_id: str
    source: str
    protocol: str
    model_name: str
    credential_ref: str
    runtime_scope: str
    purpose: str

@dataclass(frozen=True, repr=False)
class ModelCredential:
    api_key: str

@dataclass(frozen=True, repr=False)
class ModelConfig:
    profile_id: str
    source: str
    protocol: str
    base_url: str
    model_name: str
    credential: ModelCredential
    timeout_seconds: float

class CredentialResolver(Protocol):
    async def resolve(self, credential_ref: str) -> ModelCredential: ...

class ModelConfigResolver(Protocol):
    async def resolve(self, selection: ModelSelection) -> ModelConfig: ...
```

Use a concrete implementation name such as `DefaultModelConfigResolver` so the protocol remains the consumer-facing port.

- [ ] **Step 4: Implement preparation failures and environment adapters**

Use a stable internal enum:

```python
class ModelPreparationErrorCode(StrEnum):
    CREDENTIAL_NOT_FOUND = "credential_not_found"
    CREDENTIAL_ACCESS_DENIED = "credential_access_denied"
    CREDENTIAL_PROVIDER_UNAVAILABLE = "credential_provider_unavailable"
    INVALID_MODEL_SELECTION = "invalid_model_selection"
    INVALID_MODEL_CONFIG = "invalid_model_config"
```

`EnvironmentCredentialResolver` and `EnvironmentEndpointResolver` may read the environment only during infrastructure construction. They must use exact lookup tables keyed by credential/runtime reference; do not derive arbitrary environment variable names from user-controlled text.

- [ ] **Step 5: Parse settings without exposing secret values**

Add non-secret timeout/default settings to `RuntimeSettings`; keep API keys out of dataclass `repr`, validation messages, and telemetry. Build resolver adapters from an injected `Mapping[str, str]` so tests never mutate global environment.

- [ ] **Step 6: Run resolver and security tests GREEN**

```powershell
uv --directory deepagent run pytest tests/test_model_config_resolver.py tests/test_agent_runtime_security.py -q
```

Expected: all tests pass.

- [ ] **Step 7: Commit the resolver foundation**

```powershell
git add deepagent/src/agent_runtime/model_config.py deepagent/src/agent_runtime/config.py deepagent/tests/test_model_config_resolver.py deepagent/tests/test_agent_runtime_security.py
git commit -m "feat: 新增统一模型配置解析器"
```

---

## Task 3: Make LLM and DeepAgent Executors Accept Only ModelConfig

**Files:**

- Modify: `deepagent/src/agent_runtime/executors/base.py`
- Modify: `deepagent/src/agent_runtime/executors/llm.py`
- Modify: `deepagent/src/agent_runtime/executors/deepagent.py`
- Modify: `deepagent/src/agent_runtime/model_adapter.py`
- Modify: `deepagent/src/agent_runtime/context.py`
- Modify: `deepagent/src/agent_runtime/service.py`
- Modify: `deepagent/src/agent_runtime/server.py`
- Modify: `deepagent/tests/test_llm_executor.py`
- Modify: `deepagent/tests/test_deepagent_executor.py`
- Modify: `deepagent/tests/test_agent_runtime_service.py`

- [ ] **Step 1: Change tests to pass complete ModelConfig explicitly**

The public Executor protocol should become:

```python
class Executor(Protocol):
    kind: int

    def execute(
        self,
        run: RunContext,
        model_config: ModelConfig,
    ) -> AsyncIterator[EventPayload]: ...
```

Add assertions that `LLMExecutor` and `DeepAgentExecutor` do not inspect `request.model.api_key`, `credential_ref`, `LLM_API_KEY`, or `MODEL_API_KEY`.

- [ ] **Step 2: Verify executor tests RED**

```powershell
uv --directory deepagent run pytest tests/test_llm_executor.py tests/test_deepagent_executor.py tests/test_agent_runtime_service.py -q
```

Expected: signature and adapter expectation failures.

- [ ] **Step 3: Refactor OpenAIModelAdapter to accept ModelConfig**

Use:

```python
async def stream(
    self,
    model_config: ModelConfig,
    limits: ExecutionLimits,
    messages: list[ChatMessage],
) -> AsyncIterator[ModelDelta | ModelCompletion]:
```

The adapter may defensively reject an empty complete config, but it must not discover configuration or resolve credentials. Preserve the existing provider-error-to-safe-runtime-error mapping.

- [ ] **Step 4: Refactor both concrete Executors**

`LLMExecutor` emits `model_started` only after it receives a complete config. `DeepAgentExecutor._request_settings` uses the supplied `ModelConfig` for its custom endpoint and may still obtain the unrelated `TAVILY_API_KEY` through the search-tool configuration path.

Do not place `ModelConfig` on protobuf requests or emitted events. Audit events copy only `profile_id`, `source`, and `model_name`.

- [ ] **Step 5: Preserve the standalone Agent Runtime as a compatibility adapter**

The existing single-Agent gRPC request still contains a full `ModelConnection`. At the `AgentRuntimeServicer` boundary, convert it into a request-scoped selection plus request-scoped profile/credential adapters, then invoke the same `DefaultModelConfigResolver` before `executor.execute(...)`. This is the only legacy ingress; concrete Executors must not know where the secret came from.

Preparation failure flow:

```text
accepted
  -> resolve ModelConfig
  -> failed(model_not_configured / executor_unavailable)
```

Successful flow:

```text
accepted
  -> resolve ModelConfig
  -> model_started
  -> output_delta*
  -> model_completed
  -> completed
```

- [ ] **Step 6: Prove configuration errors occur before model_started**

Add service tests for missing key, missing endpoint, and invalid protocol. Assert the event list contains `accepted, failed` and no `model_started`.

- [ ] **Step 7: Run focused tests GREEN**

```powershell
uv --directory deepagent run pytest tests/test_llm_executor.py tests/test_deepagent_executor.py tests/test_agent_runtime_service.py -q
```

Expected: all tests pass.

- [ ] **Step 8: Commit the Executor boundary**

```powershell
git add deepagent/src/agent_runtime deepagent/tests/test_llm_executor.py deepagent/tests/test_deepagent_executor.py deepagent/tests/test_agent_runtime_service.py
git commit -m "refactor: 统一执行器模型配置输入"
```

---

## Task 4: Resolve ModelConfig at the Collaboration Agent Execution Boundary

**Files:**

- Modify: `deepagent/src/collaboration_runtime/models.py`
- Modify: `deepagent/src/collaboration_runtime/mapping.py`
- Modify: `deepagent/src/collaboration_runtime/executor.py`
- Modify: `deepagent/src/collaboration_runtime/agent_executor.py`
- Modify: `deepagent/src/collaboration_runtime/engines/_shared.py`
- Modify: `deepagent/src/collaboration_runtime/engines/native.py`
- Modify: `deepagent/src/collaboration_runtime/engines/autogen.py`
- Modify: `deepagent/src/agent_runtime/server.py`
- Modify: `deepagent/tests/test_collaboration_agent_executor.py`
- Modify: `deepagent/tests/test_collaboration_runtime_contract.py`
- Modify: `deepagent/tests/test_collaboration_runtime_server.py`

- [ ] **Step 1: Update collaboration tests to the selection vocabulary**

Replace `ModelReference`/`model_reference` fixtures with `ModelSelection`/`model_selection`. Add three critical tests:

1. the Engine forwards the selection unchanged and never resolves it;
2. `RuntimeRegistryAgentExecutor` calls the resolver exactly once before choosing/running the concrete Executor;
3. a preparation error yields one sanitized `FAILED` event and never yields `MODEL_STARTED`.

- [ ] **Step 2: Run focused tests RED**

```powershell
uv --directory deepagent run pytest tests/test_collaboration_agent_executor.py tests/test_collaboration_runtime_contract.py tests/test_collaboration_runtime_server.py -q
```

Expected: mapping and constructor failures for the renamed types and missing resolver dependency.

- [ ] **Step 3: Map Protobuf ModelSelection into the neutral domain**

Validate `purpose`, `credential_ref`, profile/source/protocol/model identity, runtime scope, and selector linkage during `map_request`. Reject invalid combinations as `INVALID_ARGUMENT`; never log the selection wholesale.

- [ ] **Step 4: Inject ModelConfigResolver into RuntimeRegistryAgentExecutor**

Use constructor injection:

```python
class RuntimeRegistryAgentExecutor:
    def __init__(
        self,
        registry: ExecutorRegistry,
        model_config_resolver: ModelConfigResolver,
        *,
        work_dir: Path,
    ) -> None: ...
```

Execution order must be:

```python
model_config = await self._model_config_resolver.resolve(request.model_selection)
executor = self._registry.resolve(runtime_request.executor_kind)
async for event in executor.execute(run, model_config):
    yield _map_event(event)
```

Catch only `ModelPreparationError` at this boundary and map it to existing public safe categories. Do not pass the internal provider message through. Provider unavailable is retryable; invalid/missing configuration is not.

- [ ] **Step 5: Keep both Engines credential-blind**

Native and AutoGen participant adapters receive `AgentTurnRequest + ModelSelection` and pass them to `AgentExecutor`. They must not import `CredentialResolver`, `DefaultModelConfigResolver`, provider SDKs, or environment APIs.

- [ ] **Step 6: Wire one resolver instance at the composition root**

Build the environment endpoint and credential adapters in `RuntimeServer.__post_init__`, compose one `DefaultModelConfigResolver`, and inject it into the collaboration Executor. The same process-level resolver instance is reused by Native participants and the future AutoGen selector service.

- [ ] **Step 7: Run focused tests GREEN**

```powershell
uv --directory deepagent run pytest tests/test_collaboration_agent_executor.py tests/test_collaboration_runtime_contract.py tests/test_collaboration_runtime_server.py -q
```

Expected: all tests pass, including no-`model_started` preparation failures.

- [ ] **Step 8: Commit the collaboration boundary**

```powershell
git add deepagent/src/collaboration_runtime deepagent/src/agent_runtime/server.py deepagent/tests/test_collaboration_agent_executor.py deepagent/tests/test_collaboration_runtime_contract.py deepagent/tests/test_collaboration_runtime_server.py
git commit -m "refactor: 在协作执行边界解析模型配置"
```

---

## Task 5: Route AutoGen Selector Through ModelExecutionService

**Files:**

- Create: `deepagent/src/collaboration_runtime/model_execution.py`
- Modify: `deepagent/src/collaboration_runtime/model_client.py`
- Modify: `deepagent/src/collaboration_runtime/model_gateway.py`
- Modify: `deepagent/src/collaboration_runtime/engines/autogen.py`
- Modify: `deepagent/src/agent_runtime/server.py`
- Create: `deepagent/tests/test_model_execution_service.py`
- Modify: `deepagent/tests/test_collaboration_model_client.py`
- Modify: `deepagent/tests/test_collaboration_model_gateway.py`
- Modify: `deepagent/tests/test_autogen_collaboration_engine.py`

- [ ] **Step 1: Write service and AutoGen selector tests first**

Test that:

- `ModelExecutionService.complete` resolves the selector `ModelSelection` through the same resolver instance used by participants;
- `ModelClientFactory` receives only `ModelConfig`, messages, timeout, and cancellation state;
- `_CollaborationModelChatClient` uses `request.selector_model_selection`, not a placeholder profile;
- deterministic mentioned-speaker selection still avoids a selector model call;
- missing selector selection keeps AutoGen non-ready;
- selector errors are sanitized and mapped to the existing collaboration failure taxonomy.

- [ ] **Step 2: Run tests and verify RED**

```powershell
uv --directory deepagent run pytest tests/test_model_execution_service.py tests/test_collaboration_model_client.py tests/test_collaboration_model_gateway.py tests/test_autogen_collaboration_engine.py -q
```

Expected: missing service and continued placeholder-selection assertions.

- [ ] **Step 3: Implement the framework-neutral model execution service**

Define narrow ports:

```python
class ModelClient(Protocol):
    async def complete(self, messages, *, cancel_event, timeout_seconds) -> ModelResponse: ...

class ModelClientFactory(Protocol):
    def create(self, config: ModelConfig) -> ModelClient: ...

class ModelExecutionService:
    async def complete(self, selection, messages, *, cancel_event, timeout_seconds):
        config = await self._resolver.resolve(selection)
        client = self._factory.create(config)
        return await client.complete(messages, cancel_event=cancel_event, timeout_seconds=timeout_seconds)
```

The factory must not receive `credential_ref` and must not read environment variables.

- [ ] **Step 4: Replace the placeholder AutoGen selector profile**

Remove `_ModelRequestAdapter`'s empty `ModelReference`. Bind the request-scoped selector selection when constructing `_CollaborationModelChatClient`. Keep AutoGen-specific types confined to `engines/autogen.py`; keep Provider SDK construction behind `ModelClientFactory`.

- [ ] **Step 5: Make readiness reflect real dependencies**

AutoGen is ready only when all are true:

```text
feature flag enabled
engine allowlisted
selector selection present
ModelExecutionService ready
selection is resolvable by the configured profile/credential adapters
```

Readiness may probe non-secret metadata availability, but must not fetch or log the credential during health checks.

- [ ] **Step 6: Run selector tests GREEN**

```powershell
uv --directory deepagent run pytest tests/test_model_execution_service.py tests/test_collaboration_model_client.py tests/test_collaboration_model_gateway.py tests/test_autogen_collaboration_engine.py -q
```

Expected: all tests pass.

- [ ] **Step 7: Commit selector execution reuse**

```powershell
git add deepagent/src/collaboration_runtime/model_execution.py deepagent/src/collaboration_runtime/model_client.py deepagent/src/collaboration_runtime/model_gateway.py deepagent/src/collaboration_runtime/engines/autogen.py deepagent/src/agent_runtime/server.py deepagent/tests
git commit -m "feat: 复用统一模型服务执行 AutoGen Selector"
```

---

## Task 6: Eliminate Remaining Credential and Config Bypasses

**Files:**

- Modify: `deepagent/src/agent_runtime/executors/deepagent.py`
- Modify: `backend/internal/agent/runtime_deepagent.go`
- Modify: `backend/internal/tests/agent/runtime_deepagent_test.go`
- Modify: `deepagent/tests/test_collaboration_dependency_boundaries.py`
- Modify: `deepagent/tests/test_agent_runtime_security.py`
- Modify: `backend/internal/tests/collaborationproto/contract_test.go`

- [ ] **Step 1: Add static architecture tests**

Scan production sources and fail on these dependencies:

```text
collaboration_runtime/engines/** -> agent_runtime.model_config
collaboration_runtime/engines/** -> os.environ / getenv
agent_runtime/executors/**        -> credential_ref
model client factory             -> credential_ref / LLM_API_KEY / MODEL_API_KEY
checkpoint/event serializers     -> api_key / authorization / credential
```

Allow `TAVILY_API_KEY` only in the DeepAgent search-tool configuration path; it is not a model credential.

- [ ] **Step 2: Verify architecture tests RED**

```powershell
uv --directory deepagent run pytest tests/test_collaboration_dependency_boundaries.py tests/test_agent_runtime_security.py -q
go -C backend test ./internal/tests/collaborationproto
```

Expected: the DeepAgent model environment bypass and any stale ModelReference code are reported.

- [ ] **Step 3: Remove the local DeepAgent subprocess model-secret environment channel**

For the Go legacy subprocess path, pass the already-resolved complete model config through a request-scoped stdin payload, not command arguments, environment variables, or a disk file. Add a `--model-config-stdin` CLI mode that parses once into the same Python `ModelConfig` type and never writes or logs the payload.

Go sketch:

```go
payload, err := json.Marshal(modelConfigPayload{
    Protocol: model.ModelProtocolOpenAIChatCompletions,
    BaseURL: resolved.BaseURL,
    ModelName: resolved.ModelName,
    APIKey: resolved.APIKey,
})
if err != nil {
    return AgentRuntimeResponse{}, fmt.Errorf("encode model config: %w", err)
}
defer clear(payload)
cmd.Stdin = bytes.NewReader(payload)
```

Immediately clear the temporary byte slice after `cmd.Run`. Tests must assert the secret is absent from `cmd.Env`, process arguments, output, report content, and error text.

- [ ] **Step 4: Remove stale bypass helpers**

Delete any consumer-side `NewFromEnv`, empty-key normal-path checks, placeholder AutoGen profile, or direct `request.model.api_key` access that is no longer used. Retain defensive validation only on complete `ModelConfig`.

- [ ] **Step 5: Run boundary tests GREEN**

```powershell
uv --directory deepagent run pytest tests/test_collaboration_dependency_boundaries.py tests/test_agent_runtime_security.py tests/test_deepagent_executor.py -q
go -C backend test ./internal/tests/agent ./internal/tests/collaborationproto
```

Expected: all tests pass and no credential bypass remains in scanned production paths.

- [ ] **Step 6: Commit bypass elimination**

```powershell
git add backend/internal/agent/runtime_deepagent.go backend/internal/tests/agent/runtime_deepagent_test.go deepagent/src deepagent/tests backend/internal/tests/collaborationproto/contract_test.go
git commit -m "security: 禁止模型消费者直接获取凭据"
```

---

## Task 7: Verify Event Semantics, Failure Mapping, and No Fallback

**Files:**

- Modify: `deepagent/tests/test_collaboration_agent_executor.py`
- Modify: `deepagent/tests/test_native_collaboration_engine.py`
- Modify: `deepagent/tests/test_autogen_collaboration_engine.py`
- Modify: `backend/internal/tests/collaboration/events_test.go`
- Modify: `backend/internal/tests/service/collaboration_scheduler_test.go`

- [ ] **Step 1: Add event-order contract tests**

Success must match:

```text
agent_turn_started
model_started
output_delta*
model_completed
agent_message_completed
```

Preparation failure must match:

```text
agent_turn_started
failed
```

and must contain no `model_started`, `model_completed`, or committed Agent message.

- [ ] **Step 2: Add stable preparation error mapping tests**

Map internal errors without exposing provider text:

```text
credential_not_found / invalid_model_selection / invalid_model_config
    -> model_not_configured, retryable=false
credential_access_denied
    -> model_authentication_failed, retryable=false
credential_provider_unavailable
    -> engine_unavailable, retryable=true
```

- [ ] **Step 3: Add no-fallback regression tests**

For both Native and AutoGen, assert that a Resolver failure terminates the selected run and never invokes another Engine or legacy Agent runner.

- [ ] **Step 4: Run tests RED, implement minimal mapping, then run GREEN**

```powershell
uv --directory deepagent run pytest tests/test_collaboration_agent_executor.py tests/test_native_collaboration_engine.py tests/test_autogen_collaboration_engine.py -q
go -C backend test ./internal/tests/collaboration ./internal/tests/service
```

Expected after implementation: all tests pass.

- [ ] **Step 5: Commit event semantics**

```powershell
git add deepagent/tests backend/internal/tests/collaboration backend/internal/tests/service/collaboration_scheduler_test.go deepagent/src/collaboration_runtime
git commit -m "test: 固化模型准备阶段事件语义"
```

---

## Task 8: Add a Real Local Integration Test for Environment Resolution

**Files:**

- Create: `deepagent/tests/test_collaboration_model_execution_integration.py`
- Reuse: `deepagent/tests/integration_llm_server.py`
- Modify: `deepagent/tests/test_collaboration_runtime_server.py`

- [ ] **Step 1: Write an integration test around the fake OpenAI-compatible HTTP server**

Construct a Collaboration request with:

```text
source=environment
runtime_scope=go
credential_ref=environment:go
protocol=openai_chat_completions
model_name=integration-model
```

Inject `LLM_BASE_URL=<local test server>` and `LLM_API_KEY=integration-secret` into the resolver mapping. Assert the HTTP server receives the expected authorization and model, while the Collaboration request/event stream/checkpoint/log capture never contains the secret.

- [ ] **Step 2: Add the database-profile negative case**

Send the same model identity with `credential_ref=profile:p1` and no Secret Provider Adapter. Assert `agent_turn_started -> failed(model_not_configured)`, no HTTP request, and no environment fallback.

- [ ] **Step 3: Run integration tests GREEN**

```powershell
uv --directory deepagent run pytest tests/test_collaboration_model_execution_integration.py tests/test_collaboration_runtime_server.py -q
```

Expected: both environment success and database-profile fail-closed cases pass.

- [ ] **Step 4: Commit integration coverage**

```powershell
git add deepagent/tests/test_collaboration_model_execution_integration.py deepagent/tests/test_collaboration_runtime_server.py
git commit -m "test: 覆盖协作模型配置端到端解析"
```

---

## Task 9: Update Operator Documentation and Environment Examples

**Files:**

- Modify: `.env.example`
- Modify: `deepagent/.env.example`
- Modify: `README.md`
- Modify: `docs/architecture/README.md`
- Modify: `docs/architecture/agent-runtime-and-models.md`
- Modify: `docs/architecture/collaboration-runtime-baseline.md`
- Modify: `docs/architecture/decisions/ADR-001-control-collaboration-agent-model-execution-boundaries.md`

- [ ] **Step 1: Document supported credential references**

Document exactly:

```text
environment:go        -> LLM_BASE_URL + LLM_API_KEY
environment:deepagent -> MODEL_BASE_URL + MODEL_API_KEY
profile:<id>          -> requires a production CredentialResolver adapter
```

State that `profile:<id>` does not fall back to environment variables and that current encrypted MySQL credentials remain Go-owned until the separate Secret Provider migration.

- [ ] **Step 2: Document selector model ownership**

Explain that Go chooses the AutoGen selector model; AutoGen receives a selector-purpose `ModelSelection`; the Selector invokes `ModelExecutionService`, never a raw SDK or environment variable.

- [ ] **Step 3: Document failure timing and operations**

Add the preparation-stage event diagram, stable error categories, startup/readiness behavior, and the operator action for an unresolved `profile:<id>`.

- [ ] **Step 4: Check examples contain no real secrets**

```powershell
rg -n "sk-[A-Za-z0-9]{8,}|Bearer [A-Za-z0-9]" .env.example deepagent/.env.example README.md docs/architecture
```

Expected: no real credential-like values.

- [ ] **Step 5: Commit documentation**

```powershell
git add .env.example deepagent/.env.example README.md docs/architecture
git commit -m "docs: 补充统一模型执行运维说明"
```

---

## Task 10: Full Verification and Architecture Review

**Files:**

- Verify only; modify only failures directly caused by this change.

- [ ] **Step 1: Verify generated Protobuf is current**

```powershell
bash scripts/check-collaboration-runtime-proto.sh
```

Expected: exit 0 with no generated diff.

- [ ] **Step 2: Run all Python tests**

```powershell
uv --directory deepagent run pytest
```

Expected: all tests pass.

- [ ] **Step 3: Run all Go tests, vet, and build**

```powershell
go -C backend test ./...
go -C backend vet ./...
go -C backend build ./cmd/server
```

Expected: all commands exit 0.

- [ ] **Step 4: Validate Compose configuration**

```powershell
docker compose config
```

Expected: resolved Compose configuration is valid and contains no accidental plaintext credential in committed files.

- [ ] **Step 5: Review the ADR invariants mechanically**

```powershell
rg -n "credential_ref|LLM_API_KEY|MODEL_API_KEY|api_key" deepagent/src backend/internal/collaboration proto/collaboration_runtime
```

Expected review result:

- `credential_ref` appears only in selection, resolver, mapping, and contract code;
- environment model keys appear only in configuration/resolver infrastructure;
- concrete Executors and Provider Client factories do not resolve references or read model environment variables;
- API key fields do not appear in Collaboration Protobuf/state/events/checkpoints.

- [ ] **Step 6: Inspect the final diff without touching unrelated work**

```powershell
git status --short
git diff --check
git log --oneline --decorate -12
```

Expected: only planned backend/runtime/docs changes belong to this feature; the user's pre-existing frontend and local artifact changes remain uncommitted and untouched.

- [ ] **Step 7: Request code review before integration**

Use `superpowers:requesting-code-review` against ADR-001 and this plan. Address only verified findings, rerun the affected focused tests, then rerun the full verification above.

## Explicit Follow-up Outside This Plan

The following is intentionally a separate change after the unified port is stable:

- choose and implement the production Secret Manager product/adapter;
- migrate existing MySQL-encrypted profile secrets to provider-managed `credential_ref` values;
- add multi-tenant secret authorization and rotation policy;
- decide whether the standalone single-Agent gRPC contract should also stop carrying its legacy full `ModelConnection`.

None of these follow-ups may alter the invariant that every model consumer receives only a complete `ModelConfig` produced through `ModelConfigResolver → CredentialResolver`.
