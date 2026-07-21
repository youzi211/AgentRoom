## 改造前基线（2026-07-16）

### 工具版本

- Go：`go1.26.4 windows/amd64`
- Node.js：`v24.12.0`
- npm：`11.6.2`
- uv：`0.11.27`
- Python：`3.11.14`

### 验证结果

| 验证 | 结果 | 备注 |
| --- | --- | --- |
| `go -C backend test ./...` | 通过 | 所有 backend 测试包通过；Go telemetry 尝试写受限用户目录并输出权限提示，但命令退出码为 0 |
| `go -C backend vet ./...` | 通过 | 无 vet 诊断；同样存在非代码的 Go telemetry 权限提示 |
| `go -C backend build ./cmd/server` | 通过 | 编译产物输出到仓库内临时目录后成功，验证后已删除；向 `C:\tmp` 写产物在当前沙箱被拒绝 |
| `uv run pytest`（`deepagent/`） | 通过，22 tests | `UV_CACHE_DIR` 需指向工作区；pytest 缓存目录有权限警告，不影响测试结果 |
| `npm --prefix frontend run build` | 通过 | Vite 生产构建完成，2543 modules transformed |

基线验证未调用真实模型或 Tavily。后续回归应把 uv 缓存保持在工作区，并允许关闭 pytest cache provider 以避免与产品行为无关的权限警告。

## 现状测试到远程 Runtime 验收项的映射

| 行为 | 现有主要测试 | 远程 Runtime 必须保持/新增的验收 |
| --- | --- | --- |
| Mention Fanout | `mention_test.go`、`mention_fanout_followup_test.go`、`activity_events_test.go` | Go 继续选择响应者；每位响应者单独创建远程 run；Agent-to-Agent mention 仍由 Go 决定后续 turn |
| Guided Dialogue | `dialogue_phase2_test.go`、`prompt_context_test.go`、`activity_events_test.go` | 最大轮次、自跟随、去重、Provider 错误停止和 dialogue activity 保持不变；Python 只执行一个 turn |
| 普通 LLM Agent | `prompt_context_test.go`、`runner_knowledge_test.go`、`runtime_registry_test.go`、`llm/client_test.go` | Python Prompt Composer 与 Go 黄金样例一致；Profile 解析仍由 Go 完成；知识来源和模型审计保持 |
| DeepAgent artifact | `runtime_deepagent_test.go`、`TestGuidedDialoguePreservesDeepAgentRuntimeArtifacts`、`TestDownloadMessageArtifactReturnsPersistedReport` | gRPC completed/artifact 事件必须在两种 Dialogue Mode 中保存为相同消息 artifact |
| DeepAgent 内容隔离 | `TestDeepAgentRuntimeSeparatesQuestionFromCLIOptions` | 以 `--` 开头的问题只能进入 Protobuf content 字段，不能影响 Executor 或服务控制字段 |
| 模型凭据脱敏 | `TestDeepAgentRuntimeInjectsResolvedModelAndRedactsSecretFromReportAndErrors`、Model Profile API/Service tests、`test_offline_outputs_do_not_persist_injected_model_api_key` | gRPC 请求体不得被日志记录；事件、错误、报告、数据库非密钥字段不得出现明文 Key |
| 并发、超时和取消 | `TestDeepAgentRuntimeConcurrencyLimitWaitsAndHonorsCancellation`、`agent_response_queue_test.go`、Dialogue Provider error tests | Python 总容量和 DeepAgent 专属容量有界；等待、模型、工具和流发送均观察 gRPC deadline/cancel |
| Runtime 选择 | `runtime_registry_test.go`、`TestAgentRunUsesRegisteredDeepAgentRuntime` | 显式 local/grpc 选择；不确定远程失败不自动切回本地重做同一 run |
| 运行活动与审计 | `activity_events_test.go`、`server_activity_test.go` | Go 验证 `run_id`/sequence，将中间事件映射为 activity，并只提交一次最终消息和终态 |

## 目录迁移决策

第一阶段保留顶层 `deepagent/`，在其中新增 `agent_runtime` Python 包和常驻服务入口。只有普通 LLM Executor、DeepAgent Executor、Docker、灰度和回滚验收全部完成，且 Go 不再启动 DeepAgent CLI 后，才把顶层目录重命名为 `agent-runtime/`。目录重命名必须是独立机械变更，不与协议或执行语义修改混在同一批次。

## 与模型 Profile 变更的协调

`add-model-profile-management` 已完成的子进程环境注入是当前 local Runtime 的过渡实现。目标状态改为：Go 继续解析、解密和审计 Profile；grpc 模式下仅把单次 `ModelConnection` 放入受保护的 ExecuteAgent 请求；Python 只在对应 RunContext 内存中使用并在结束后释放。旧子进程实现保留到灰度回滚窗口结束，不作为远程失败后的自动回退。

## Agent Run 最终消息关联

实现采用可空的 `messages.agent_run_id`，并对该列增加唯一索引和指向
`agent_runs.id` 的外键。旧消息保持兼容，关联字段不通过公开 JSON 暴露。
`CommitAgentRunSuccess` 先锁定 Run，再在同一事务中插入唯一最终消息、保存
非敏感模型审计，并把 Run 从 `running` 更新为 `succeeded`。

## 普通 LLM grpc 集成验收

`TestGoToPythonLLMRuntimeIntegration` 显式使用 `grpc` transport，启动测试目录中的
确定性 Python LLM 服务（不访问外部 Provider），覆盖成功、应用失败和 Go deadline。
跨语言 Prompt 黄金样例同时由 Go 和 Python Composer 校验；远程应用失败直接返回，
不会把同一 `run_id` 交给 local Runtime 重做。
## DeepAgent gRPC migration

- `agentroom_deepagent.research.stream_research` is the service-facing async library API; the existing CLI remains a thin migration and smoke entrypoint.
- `DeepAgentExecutor` consumes only `trigger.content`, strips the addressed Agent mention, and never parses RPC content as CLI flags. Deep Agents `updates/messages` are mapped to model, tool, and bounded progress events.
- Every run receives its own `RunContext` directory and immutable request-derived model settings. `report.md` is read into a Markdown artifact before context cleanup; request API keys are cleared during cleanup.
- Python owns total and DeepAgent-specific capacity. Tests cover excess-work rejection, cancellation, timeout classification, graceful shutdown, concurrent configuration isolation, provider error redaction, and artifact limits.
- The real Go-to-Python process integration exercises both LLM and DeepAgent executors over the same long-lived gRPC service. Go does not invoke the DeepAgent CLI per remote turn.
- Runner end-to-end tests cover a human Mention, an Agent-to-Agent Mention, and Guided Dialogue while preserving Markdown artifacts and model audit fields.

## Security and readiness

- Python Runtime defaults to loopback and requires explicit `AGENT_RUNTIME_INSECURE=true` for plaintext. TLS deployments validate certificate/key/CA readability before bind; Go validates CA and optional client identity and uses the standard gRPC Health service.
- Go `/api/health` is a liveness endpoint. `/api/ready` independently reports MySQL and Agent Runtime readiness and returns `503` when either dependency is unavailable.
- Both runtimes emit structured run lifecycle telemetry containing only `run_id`, room/agent/dialogue/trace IDs, executor, outcome, queue and duration fields. Logging filters redact environment credentials and request/provider errors are converted to safe messages.
- Tests cover credential scans, exception-stack redaction, readiness transitions, service health changes, connection reset/unavailable behavior, slow consumers, cancellation and bounded execution capacity.

## Containers

- Added `deepagent/Dockerfile` for the locked, non-root Python Runtime image and a gRPC HealthCheck module.
- Compose now starts an internal-only `agent-runtime` service with an artifact/work volume, explicit development plaintext, bounded capacity, and backend dependency on Runtime health. Backend keeps its Python/DeepAgent dependencies during the migration window so `AGENT_RUNTIME_TRANSPORT=local` remains a supported rollback.
- Root `.env.example` documents Runtime transport, deadlines, limits, TLS paths, capacity and shutdown settings. Docker Compose cold-start verification remains pending because Docker is unavailable in the current environment.

## Rollout and rollback

- Added `docs/architecture/agent-runtime-rollout.md` with explicit local/grpc selection, staged rollout observations, ownership roles, and a no-duplicate-run rollback drill. Existing Go integration tests verify remote application failures do not fall back to local execution.

## Final validation snapshot

- Go: `go test ./...`, `go vet ./...`, `go build ./cmd/server` passed with a workspace-local `GOCACHE`; the newly touched Go files are `gofmt` clean. A full `gofmt -l cmd internal` still reports pre-existing unrelated files, so they were not rewritten.
- Python: 69 tests passed, offline smoke passed, and `check-agent-runtime-proto.ps1` passed.
- Frontend: 82 Node tests and `npm run build` passed.
- OpenSpec strict validation passed for this change.
- Remaining environment/stability gates: Docker Compose cold-start/rollback cannot run because Docker is unavailable here; old Runtime removal and `deepagent/` rename remain intentionally pending until the documented stable observation period.
