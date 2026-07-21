## Context

AgentRoom 当前是单实例 Go 模块化单体：`backend/cmd/server/main.go` 装配 API、Room Manager、RoomService、Agent Runner、Runtime Registry、模型 Resolver 和 MySQL Store。人类消息先持久化，再进入 Room 内存并通过 WebSocket Hub 广播；Agent turn 由进程内有界队列触发，`agent.Runner` 根据 Mention Fanout 或 Guided Dialogue 策略选择响应者并调用 `AgentRuntime`。

现有 Runtime 有两条执行路径：

- 普通 `llm` Agent 由 Go `LLMAgentRuntime` 调用 OpenAI-compatible 模型；
- `deepagent` Agent 由 Go `DeepAgentRuntime` 为每个 turn 启动 Python/`uv` 子进程，等待报告并把 artifact 返回给 Runner。

这个结构已经通过 `AgentRuntime` 和 `RuntimeRegistry` 形成可替换边界，但 `Respond` 当前只返回最终结果，不能承载远程执行过程事件；DeepAgent 的子进程启动、并发信号量、环境变量注入和文件收集也仍属于 Go 进程职责。随着工具调用、长时间研究任务和更多 Python Agent 能力增加，继续扩展该边界会让 Go 同时承担控制面和执行面复杂度。

本变更采用 Python 常驻服务和 gRPC 服务端流式 RPC。官方 gRPC 能力满足当前需要：客户端发送单一请求并持续接收有序响应；Go 调用上下文可承载 deadline 和取消；标准 Health Checking 可用于服务就绪和连接健康。设计仍受以下约束：

- Go 必须继续拥有 Room、Dialogue Policy、消息和 Run 的业务状态；
- Python 不得成为第二个房间状态源或直接访问 AgentRoom MySQL；
- 当前产品仍以单实例 Go 后端为主要部署范围；
- Model Profile 由 Go 管理和解密，Python 只获得单次调用凭据；
- 迁移不能一次性删除现有 Go/子进程 Runtime，必须支持显式回滚；
- 当前活动变更 `add-model-profile-management` 对 DeepAgent 的“子进程环境注入”描述需要在实施前协调为远程执行上下文注入。

目标部署拓扑：

```text
Browser
  | REST + WebSocket
  v
frontend/nginx
  |
  v
Go backend (control plane)
  |-- Room / Hub / API / orchestration / MySQL writes
  |-- Model Profile resolution and secret decryption
  `-- gRPC ExecuteAgent(request) -> stream AgentEvent
          |
          v
Python agent-runtime (execution plane)
  |-- LLM Agent Executor
  |-- DeepAgent Executor
  |-- Prompt / models / tools / artifacts
  `-- no AgentRoom DB access
```

## Goals / Non-Goals

**Goals:**

- 将普通 LLM Agent 和 DeepAgent 的执行实现统一到 Python 常驻服务。
- 以“单个 Agent turn”为稳定远程调用边界，保持 Go 对跨 Agent 对话的确定性治理。
- 通过 gRPC 服务端流返回运行、模型、工具、输出、artifact 和终态事件。
- 让 deadline、取消、容量限制、健康检查、优雅停机和错误分类成为第一版契约。
- 保持 MySQL 单写入方，Go 统一提交 Agent 消息、Run 状态和 WebSocket Activity。
- 保持现有 Model Profile 选择优先级、运行审计和凭据不落盘语义。
- 支持本地/远程 Runtime 显式切换、分阶段灰度和安全回滚。
- 建立 Protobuf 兼容和 Go/Python 跨语言契约测试。

**Non-Goals:**

- 本变更不把 Room Manager、WebSocket Hub、Dialogue Policy 或会议生命周期迁到 Python。
- 本变更不让 Python 直接读写 AgentRoom MySQL。
- 第一版不引入 RabbitMQ、Kafka、NATS、Celery 或 Redis Streams。
- 第一版不承诺 Go 后端重启后自动恢复未完成 Agent turn；遗留 `running` 记录只收敛为中断状态。
- 第一版不实现跨多个 Go 后端实例的房间所有权和实时广播。
- 第一版不把 FocusService 和 MinutesService 迁到 Python；它们继续使用现有 Go 模型能力。
- 第一版不引入对象存储；artifact 继续以内联内容持久化，超过限制则明确失败。对象存储引用作为后续扩展。
- 第一版不要求把 token delta 直接展示到聊天消息；流式进度先用于 Agent Activity，最终消息仍在成功终态一次性提交。
- 本变更不更改产品级身份、租户和角色模型。

## Decisions

### 1. Go 是控制面，Python 是单 Turn 执行面

Go 继续负责：

- 选择响应 Agent；
- Mention Fanout 和 Guided Dialogue；
- 最大自主轮次、每 Agent 最大轮次和禁止自跟随等策略；
- 创建 `agent_runs` / `dialogue_runs`；
- 获取最近消息与知识分片；
- Model Profile 解析和解密；
- 取消、终态提交、消息持久化和 WebSocket 广播。

Python 负责：

- Prompt 组合；
- 模型 Provider 调用；
- 单 Agent 内部工具循环；
- DeepAgent 研究；
- 输出校验、artifact 生成和细粒度执行事件。

Python 完成一个 turn 后把结果交还 Go；如果回复触发另一个 Agent，仍由 Go 决定是否创建下一 turn。

**选择理由：** 该边界与现有 `AgentRuntime.Respond` 接近，迁移范围可控，并且避免 Go/Python 双方同时持有房间状态机。

**未选择的方案：** 把整段多 Agent 对话交给 Python。该方案会让 Python 需要恢复房间、轮次、消息提交和取消状态，形成第二个业务状态源，也会削弱实时审计和回滚能力。

### 2. 使用服务端流式 RPC，不使用双向流

第一版协议定义：

```proto
package agentroom.runtime.v1;

service AgentRuntimeService {
  rpc ExecuteAgent(ExecuteAgentRequest) returns (stream AgentEvent);
}
```

另实现标准 `grpc.health.v1.Health`，不自定义重复的健康协议。

Go 在调用开始时提供完整不可变请求；Python 只向 Go 发送事件。业务取消通过 Go 取消 gRPC Context 实现，第一版不增加独立 `CancelRun` RPC。Go 维护活动 `run_id -> cancel` 映射，使房间关闭、deadline 和服务停机都能结束同一调用。

**选择理由：** 单个 turn 的输入在开始前已经由 Go 构建完成，无需执行中反复修改；服务端流比双向流更容易定义顺序、背压、终态和重试语义。

**未选择的方案：**

- Unary RPC：无法自然承载分钟级任务的进度和工具事件；
- 双向流：允许执行中追加控制消息，但会显著增加状态机和半关闭语义；当前只有取消需要，而取消可由 Context 满足；
- HTTP + SSE：调试简单，但跨语言代码生成、deadline、状态码和标准健康检查不如 gRPC 一致。

### 3. Protobuf 合约按 v1 包版本演进

共享协议位于：

```text
proto/agent_runtime/v1/agent_runtime.proto
```

请求主要字段：

```text
protocol_version
run_id
trace_id
executor_kind
room_snapshot
agent_snapshot
trigger_message
recent_messages
knowledge_chunks
model_connection
execution_limits
```

`model_connection` 包含本次调用需要的协议、Base URL、模型名和 API Key，但不包含 Profile 密文、主加密密钥、管理员密钥或房间口令。

事件使用 `oneof` 表达：

```text
accepted
model_started / model_completed
tool_started / tool_completed / tool_failed
output_delta
artifact_ready
completed
failed
```

每个事件包含 `run_id`、单调递增 `sequence` 和服务端时间。成功或失败为应用终态；终态后不得继续发送。请求在接受前发生的验证、认证、容量或协议错误使用 gRPC Status；请求接受后的模型/工具错误尽可能使用 `failed` 终态，Go 不解析 Python 错误字符串。

生成策略：

- 提交 Go 和 Python 生成代码，保证普通构建不依赖开发机临时安装生成器；
- 提供固定版本的生成命令；
- CI 重新生成并检查工作树无差异；
- 删除字段只保留编号并标记 `reserved`；
- 第一版不改变现有公开 REST/WebSocket DTO。

### 4. Go Runtime 接口增加事件观察能力

把当前只返回最终结果的接口演进为概念上的：

```go
type AgentRuntime interface {
    Name() string
    Execute(
        ctx context.Context,
        request AgentRuntimeRequest,
        observer AgentEventObserver,
    ) (AgentRuntimeResponse, error)
}
```

`AgentEventObserver` 由 Runner/Service 提供，负责：

- 验证当前 `run_id`；
- 验证事件序号；
- 把非终态事件转换为 `realtime.Activity`；
- 过滤敏感内容；
- 不直接创建聊天消息。

迁移期本地 Go Runtime 和旧 DeepAgent Runtime 可以只发最小 `accepted/completed/failed` 事件，以便共用新接口。`RemotePythonRuntime` 持有长生命周期 gRPC ClientConn，把 Protobuf 事件转换为内部事件。

**未选择的方案：** 让 Remote Runtime 直接访问 `room.Room` 并广播。这样会把业务副作用重新塞进传输 Adapter，难以测试事件校验和终态提交。

### 5. Python 服务采用异步 gRPC Server 和 Executor Registry

Python Runtime 目录从现有 `deepagent/` 演进，建议最终结构：

```text
agent-runtime/
├── pyproject.toml
├── uv.lock
├── src/agent_runtime/
│   ├── server.py
│   ├── contracts/
│   ├── registry.py
│   ├── context.py
│   ├── prompts/
│   ├── executors/
│   │   ├── base.py
│   │   ├── llm_agent.py
│   │   └── deep_research.py
│   ├── providers/
│   ├── tools/
│   ├── artifacts/
│   ├── security/
│   └── telemetry/
└── tests/
```

为降低迁移噪音，实施初期可以保留物理目录名 `deepagent/`，先新增 `agent_runtime` Python 包；待 Go 已完全切换远程服务后再单独重命名顶层目录。

每个请求建立独立 `RunContext`，包含取消、模型连接、临时工作区和事件序号。Registry 按 `executor_kind` 选择 Executor。服务级信号量限制总并发，DeepAgent 使用独立更严格的信号量；等待必须响应 Context 取消。服务不缓存带 API Key 的模型客户端，也不建立 AgentRoom MySQL 连接。

### 6. Go 在调用前构建完整上下文和解析模型

Runner 保持现有顺序：

1. 创建 `agent_run`；
2. 获取最近消息；
3. 由 KnowledgeService 检索 room/agent scope 分片；
4. 根据房间 Agent 快照解析 Model Profile；
5. 构建不可变远程请求；
6. 调用 Python 并消费事件；
7. 提交成功或失败终态。

Prompt 的字符串渲染迁到 Python，但 Go 继续决定哪些消息、知识和 Agent 快照可以进入上下文。这样 Python 无需数据库权限，也不会绕过 Room 的有界上下文策略。

Model Profile 解析仍遵循现有优先级：具体快照 Profile、对应 Runtime 数据库默认值、环境迁移兜底。Go 解密后把单次凭据放入请求，`MODEL_CONFIG_ENCRYPTION_KEY` 永不进入 Python。

FocusService 和 MinutesService 暂不迁移，因此 Go `llm` 包在本变更完成后仍可能保留，不能因为 Agent Runtime 迁移就直接删除。

### 7. Go 是唯一终态提交方，并增加事务幂等约束

当前 `persistAndBroadcast` 对 Agent/System 消息是 best-effort，且 Agent 消息没有稳定的 Agent Run 唯一关联。远程执行后，连接中断和重复终态更容易出现，因此成功提交必须升级为事务语义。

建议增加：

- `messages.agent_run_id` 可空字段；
- 对非空 `agent_run_id` 的唯一约束，或等价的 `agent_runs.result_message_id` 唯一关联；
- Store 端口 `CommitAgentRunSuccess(ctx, runID, message, metadata)`，在一个 MySQL 事务中插入消息并把 Run 从 `running` 更新为 `succeeded`；
- `FinishAgentRunFailure` 使用条件更新，只允许非终态转入一个终态；
- 启动时把没有本地活动调用的遗留 `running` Run 收敛为 `interrupted`/`failed`，不自动重跑。

只有事务成功后，Go 才把最终消息追加到 Room 并广播。非终态 Activity 不要求 durable outbox，仍允许只实时可见；最终回复和 Run 终态必须一致。

**选择理由：** 单靠进程内 `run_id` map 无法跨重启保证幂等，远程边界需要数据库约束作为最终防线。

**未选择的方案：** 在第一版实现完整持久任务、租约和自动重试。该能力值得后续建设，但与服务拆分同时引入会扩大失败状态空间。

### 8. 不自动重试不确定执行，也不自动本地回退

错误分为：

```text
调用前确定失败：参数错误、未认证、明确未接受、健康检查失败
调用中应用失败：模型、工具、输出校验失败
调用中不确定失败：连接重置、UNAVAILABLE、后端进程退出
取消/超时：CANCELLED、DEADLINE_EXCEEDED
协议失败：run_id 错误、序号倒退、重复终态、缺少终态
```

第一版不对 Agent 模型调用做自动重试；尤其在调用中不确定失败时，不自动切换本地 Runtime，因为 Python 可能已经调用模型或生成副作用。运维回滚通过显式配置影响后续新 Run。

Runtime 选择配置建议：

```text
AGENT_RUNTIME_TRANSPORT=local|grpc
AGENT_RUNTIME_GRPC_ADDRESS=agent-runtime:50051
AGENT_RUNTIME_GRPC_INSECURE=false
AGENT_RUNTIME_GRPC_SERVER_NAME=...
AGENT_RUNTIME_GRPC_CA_FILE=...
AGENT_RUNTIME_GRPC_CLIENT_CERT_FILE=...
AGENT_RUNTIME_GRPC_CLIENT_KEY_FILE=...
```

迁移期默认保持 `local`，集成环境和灰度环境显式设为 `grpc`；验收完成后再把默认值改为 `grpc`。不设计隐式“远程失败就本地执行”开关。

### 9. Deadline、取消与服务停机使用同一生命周期

Go 为普通 LLM 和 DeepAgent 保留不同的运行超时默认值，并将 deadline 写入 Context；Python 不自行延长该 deadline。Go 的活动 Run Registry 只管理取消函数和非持久连接状态，不成为任务事实源。

取消来源：

- Context deadline；
- 房间关闭或归档；
- Go 优雅停机；
- 后续显式取消 API；
- gRPC 连接终止。

Python 的模型/工具 Adapter 必须接收取消信号。对于不能立即取消的第三方 SDK，Python 至少停止启动后续步骤、停止发送非终态事件并在调用返回后丢弃过期结果。

Python 停机顺序：

1. Health 状态切为 `NOT_SERVING`；
2. 停止接受新 `ExecuteAgent`；
3. 在宽限期内等待活动运行；
4. 取消剩余 Run；
5. 关闭工具、模型客户端和 gRPC Server。

Go 停机顺序：

1. 停止接受新 Agent Job；
2. 取消或等待活动远程 Run；
3. 提交可确定终态；
4. 关闭 gRPC ClientConn；
5. 关闭 HTTP Server 和 Store。

### 10. 健康、就绪和连接生命周期分离

Go 维护一个长生命周期 gRPC ClientConn，不为每个 turn 重复 Dial。后台健康观察使用标准 Health Checking；单次执行仍以实际 RPC 结果为准。

健康语义：

- `/api/health`：Go 进程存活，不因 Python 短暂不可用而失败；
- `/api/ready`：报告 MySQL 与 Agent Runtime 依赖就绪状态，可返回非 2xx；
- Python gRPC Health：Registry 初始化完成且接受新 Run 才为 `SERVING`；
- 前端、房间历史和管理功能在 Python 不可用时继续工作；新 Agent Run 快速失败并产生可见系统状态。

Docker Compose 中 `agent-runtime` 不默认暴露宿主机端口；Go 通过内部服务名连接。开发环境可以显式启用 plaintext，非本机环境要求 TLS 与服务身份验证。gRPC 拦截器不得记录完整请求体或认证 metadata。

### 11. 流式背压和大小边界显式配置

Python 不建立无界事件队列。关键事件包括 accepted、artifact、completed 和 failed，不得丢弃；高频 output delta 或细粒度进度允许合并，但必须保持单调序号和语义顺序。

第一版建议采用以下默认策略，具体数值可在实施前通过现有数据样本校准：

- 请求总大小和单事件大小设置显式上限，不依赖库默认值；
- 内联 artifact 使用较小的独立上限；
- 超限返回 `RESOURCE_EXHAUSTED`，不静默截断；
- Go 最近消息和知识分片在构造请求前执行现有数量限制和新增字节预算；
- Python 工具日志只返回摘要，不把完整网页、模型原始响应或二进制数据当作进度事件。

对象存储与 artifact 外部引用暂不进入第一版；如果真实 DeepAgent 报告经常超过内联限制，再单独设计。

### 12. 可观测性以 run_id 贯穿两端

Go 和 Python 的结构化日志都包含：

```text
run_id
room_id
agent_id
dialogue_run_id（如有）
trace_id
executor_kind
event_type
duration_ms
safe model name / profile id
```

不得记录 API Key、完整 Prompt、房间口令、管理员密钥或未经脱敏的 Provider 错误。首版至少提供以下指标或等价日志聚合：

- 当前活动/等待 Run；
- 按 Executor 的成功、失败、取消、超时数；
- 排队时间和执行耗时；
- gRPC 状态码；
- 事件流协议失败数；
- artifact 大小和超限数。

### 13. 协调活动 Model Profile 变更

`add-model-profile-management` 当前规格要求 DeepAgent 通过每次子进程环境获得 `MODEL_*`。本变更应用前需要同步其尚未归档工件或实现分支：

- 保持 Profile 选择、加密、默认值和审计要求不变；
- 把“子进程环境注入”改为“单次远程请求内存注入”；
- Python 不把凭据写入 `.env`、TOML、报告或事件；
- `agent_runs` 继续记录 Profile ID、来源和模型名，不记录明文 Key；
- 测试从子进程环境断言迁移为 gRPC 请求/服务脱敏断言。

如果该活动变更先完成并归档，本变更需把 `runtime-model-resolution` 作为 Modified Capability 补充 delta；如果本变更先实施，则两者必须在同一合并序列中消除冲突。

## Risks / Trade-offs

- **[远程边界增加延迟和故障点]** → 复用长生命周期 ClientConn；Python 常驻避免每次进程冷启动；健康检查和快速失败避免无界等待。
- **[连接中断时无法判断模型是否已调用]** → 不自动重试或本地回退；用 `run_id`、终态条件更新和事务提交避免重复消息。
- **[模型密钥穿过网络]** → Go 保留解密权；生产使用 TLS/服务身份校验；不记录请求体；Python 仅在 RunContext 内存持有并在结束时释放。
- **[Python 并发任务相互污染]** → 每 Run 独立上下文、模型客户端和工作区；禁止模块级可变执行状态；增加并发隔离测试。
- **[高频事件造成内存压力]** → 有界队列、gRPC 流控、可合并进度、关键事件不可丢、显式消息大小限制。
- **[Go/Python Protobuf 漂移]** → 单一 proto 源、提交生成代码、固定生成版本、跨语言契约测试和 CI 无差异检查。
- **[两套 Runtime 并存增加维护成本]** → 并存只用于迁移；定义退出门槛和删除任务，不长期提供隐式自动回退。
- **[现有 DeepAgent 文件结构与新服务命名冲突]** → 先在原目录增加服务包，稳定后再做单独目录重命名，避免一次改动过大。
- **[第一版不恢复中断任务]** → 启动时明确标记遗留 Run 为 interrupted；后续以持久任务、租约和 at-least-once 执行为独立变更。
- **[Agent 最终消息从 best-effort 改为事务提交会改变失败体验]** → 这是远程执行可靠性所需的有意收紧；持久化失败时广播系统错误，不展示无法恢复的成功消息。
- **[TLS/mTLS 增加部署复杂度]** → 本机 Compose 允许显式 insecure 且不暴露端口；生产提供清晰证书配置和启动前校验。

## Migration Plan

### Phase 0：基线与冲突协调

1. 记录现有 Mention Fanout、Guided Dialogue、普通 LLM、DeepAgent、artifact、超时和取消测试基线。
2. 协调 `add-model-profile-management` 中 DeepAgent 子进程注入语义。
3. 确认现有 Go/Python 依赖和 Docker 构建可重复。

**退出条件：** 当前测试通过；两个活动变更对 Profile/凭据所有权没有冲突。

### Phase 1：协议和跨语言契约

1. 增加 `proto/agent_runtime/v1/agent_runtime.proto`。
2. 固定 Go/Python 生成工具版本并提交生成代码。
3. 实现 Python Fake Executor 和标准 Health Service。
4. 增加 Go 客户端契约测试：正常事件、失败、乱序、超限、deadline、取消。

**退出条件：** 不调用真实模型即可完成端到端 gRPC 流测试；协议生成检查无差异。

### Phase 2：Go 远程 Runtime Adapter

1. 扩展内部 Runtime 接口以支持 Event Observer。
2. 实现长生命周期 gRPC ClientConn、错误映射和活动 Run 取消表。
3. Runner 验证事件并映射 Agent Activity。
4. 增加事务式成功提交和终态条件更新；处理遗留 running Run。
5. 保持默认 `AGENT_RUNTIME_TRANSPORT=local`。

**退出条件：** 使用 Fake Python 服务时，现有两种 Dialogue Mode、artifact 和失败审计测试通过。

### Phase 3：迁移普通 LLM Agent

1. 把 Prompt Composer 和普通 LLM Provider 调用实现到 Python。
2. 由 Go 解析 Profile 并传递单次模型连接。
3. 对比本地 Go Runtime 与 Python Runtime 的上下文、输出和审计元数据。
4. 在集成环境显式启用远程普通 LLM Executor。

**退出条件：** 普通 Agent 在真实或受控模型端点上完成成功、认证失败、限流、超时和取消测试；无凭据泄漏。

### Phase 4：迁移 DeepAgent

1. 将现有 DeepAgent CLI 核心抽为可由常驻服务调用的库接口。
2. 把 Tavily、运行工作区、报告和事件适配到统一 Executor/Event 模型。
3. 把并发限制从 Go 子进程信号量迁到 Python 总容量和 DeepAgent 专属容量。
4. 验证 Mention Fanout 与 Guided Dialogue 的 artifact 等价性。

**退出条件：** Python 服务不再需要 Go 启动 DeepAgent CLI；报告 artifact、取消、超时和并发限制测试通过。

### Phase 5：部署、安全和可观测性

1. 增加独立 `agent-runtime` Dockerfile/Compose 服务和持久 artifact volume。
2. 默认不暴露 Python gRPC 宿主机端口。
3. 增加本机 insecure 显式配置与生产 TLS 配置校验。
4. 增加 Go `/api/ready`、Python标准健康检查、结构化日志和指标。
5. 更新 `.env.example`、README 和架构文档。

**退出条件：** 四服务 Compose 冷启动、滚动停止、Python 故障和恢复演练通过。

### Phase 6：灰度切换与旧 Runtime 退出

1. 在开发/测试环境把传输默认切为 `grpc`。
2. 运行回归、并发和长任务测试，观察失败率、排队和执行耗时。
3. 在生产或目标部署环境以配置切换新 Run；不迁移正在运行的本地 Run。
4. 达到稳定期后删除 Go Agent LLM Runtime 和 DeepAgent 子进程 Adapter；保留 Focus/Minutes 所需 Go LLM 能力。
5. 移除 `DEEPAGENT_COMMAND`、`DEEPAGENT_WORKDIR` 等旧配置并更新运维说明。

**退出条件：** 新 Runtime 覆盖所有 Agent 类型；旧 Runtime 无调用；回滚窗口结束后再删除旧代码。

### Rollback

- 只对后续新 Run 生效：把 `AGENT_RUNTIME_TRANSPORT` 显式改回 `local` 并重启/滚动 Go 后端。
- 回滚前取消或等待当前远程 Run，不把同一 `run_id` 交给本地 Runtime 重做。
- 数据库新增字段保持向后兼容且可空，回滚代码忽略它们；不在紧急回滚中删除列或唯一约束。
- Python 服务可以继续运行但不再接收请求，确认无活动 Run 后再停止。
- 如果已经删除旧 Runtime，则代码级回滚需要回到删除前发布版本；因此旧 Runtime 删除只能在独立、可逆的最终阶段完成。

## Open Questions

以下问题不改变总体边界，但应在对应 Phase 开始前固定默认值：

1. 内联 artifact 和请求总字节上限应根据现有最大 DeepAgent 报告与知识上下文样本确定；默认必须显式配置且不得依赖 gRPC 库默认值。
2. 生产部署采用单向 TLS + 内部共享服务凭据，还是 mTLS；默认建议 mTLS，但需结合现有证书分发能力确认。
3. `output_delta` 第一版只进入后端观测，还是同时扩展 WebSocket/UI 展示；默认只用于观测，避免引入草稿消息持久化语义。
4. 顶层目录何时从 `deepagent/` 重命名为 `agent-runtime/`；默认在远程普通 LLM 和 DeepAgent 都稳定后作为单独机械变更执行。
5. 持久任务、租约和自动恢复是否紧接本变更立项；默认不阻塞本次改造，但应在需要多实例或任务恢复前完成。
