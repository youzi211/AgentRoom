## Why

AgentRoom 当前把 Agent 对话编排、Go 模型调用和 DeepAgent 子进程管理集中在同一 Go 进程中，导致 Agent 能力迭代受 Go/Python 双运行时与进程启动模型限制，也使长任务的流式进度、取消和独立扩缩容难以形成稳定边界。现在应在继续扩展工具型 Agent 之前，将 Agent 执行抽离为 Python 常驻服务，同时保留 Go 对房间、对话策略、持久化和实时会话的统一控制。

## What Changes

- 新增 Python 常驻 Agent Runtime 服务，以版本化 Protobuf 合约接收单个 Agent turn 的不可变执行快照。
- 新增 Go gRPC Runtime Client，通过服务端流式 RPC 接收运行开始、模型调用、工具调用、artifact、完成和失败事件。
- Go 继续负责响应者选择、Mention Fanout、Guided Dialogue、轮次限制、Agent/Dialogue Run 状态、MySQL 写入和 WebSocket 广播；Python 不直接读写 AgentRoom 业务数据库。
- 将普通 LLM Agent、DeepAgent、Prompt 组合、模型调用、工具调用和 artifact 生成逐步迁移到 Python Executor Registry。
- 为远程执行定义 deadline、客户端取消、标准 gRPC 状态映射、幂等 `run_id`、消息大小限制、流式背压、标准健康检查和优雅停机行为。
- 迁移期保留现有 Go LLM Runtime 和 DeepAgent 子进程 Runtime 作为受控回退；按 Agent/配置灰度切换，验收完成后再移除旧实现。
- 调整 DeepAgent 的并发治理：从 Go 进程内子进程信号量迁移为 Python 服务容量限制和 Go 侧调用并发限制。
- 保持 Model Profile 的选择与解密在 Go 控制面；仅通过私有 gRPC 调用把本次运行所需模型凭据交给 Python 内存，禁止写入日志、配置、事件或 artifact。
- 更新 Docker Compose、健康检查、日志与运行手册，增加独立 `agent-runtime` 服务及其部署配置。
- **BREAKING**：最终切换阶段将不再由 Go 后端直接启动 DeepAgent Python 子进程，原 `DEEPAGENT_COMMAND`、`DEEPAGENT_WORKDIR` 等进程启动配置将被远程 Runtime 地址和服务容量配置取代。

## Capabilities

### New Capabilities

- `grpc-agent-execution-contract`: 定义 Go 到 Python 的版本化 gRPC 服务端流式执行协议、事件序列、错误语义、deadline、取消、健康检查、兼容性和安全约束。
- `python-agent-runtime-service`: 定义 Python 常驻服务的 Executor Registry、普通 LLM/DeepAgent 执行、工具与 artifact 行为、并发隔离和优雅停机要求。
- `remote-agent-run-governance`: 定义 Go 控制面对远程 Agent turn 的调度、幂等提交、状态持久化、实时事件映射、灰度迁移和回退行为。

### Modified Capabilities

- `agent-runtime-entry-reliability`: 将 DeepAgent 的 CLI 参数隔离和 Go 进程内子进程并发要求，替换为远程 Python Runtime 的请求隔离、容量限制、取消和 artifact 等价性要求。

## Impact

- 后端：调整 `backend/internal/agent` 的 Runtime 边界，新增 gRPC Client/Adapter、流式事件消费、错误映射、连接健康和关闭逻辑；Room、Service、Store 和 WebSocket 继续保有业务状态所有权。
- Python：把 `deepagent/` 演进为常驻 `agent-runtime` 服务，新增 Protobuf 生成代码、Executor Registry、普通 LLM Executor、DeepAgent Executor、健康检查和结构化事件。
- 协议：新增共享 `.proto` 定义和 Go/Python 代码生成流程；要求向后兼容的字段演进规则和跨语言契约测试。
- 数据：首个迁移批次复用现有 `agent_runs`、`dialogue_runs` 和 `messages`；为幂等、取消或租约恢复所需的字段另行通过受控迁移增加，Python 不直接访问这些表。
- 安全：Go 保留 `MODEL_CONFIG_ENCRYPTION_KEY` 与 Profile 解密；Python 仅获得单次执行凭据，且 gRPC 服务必须限制在可信私有网络并配置传输保护策略。
- 部署：Docker Compose 从三个常驻服务增加为 `frontend`、`backend`、`agent-runtime`、`mysql` 四个服务；后端健康与就绪检查需区分自身可用和 Agent Runtime 可用。
- 兼容：实现必须协调当前活动变更 `add-model-profile-management`，将其中“DeepAgent 单次子进程环境注入”更新为“Python Runtime 单次执行上下文注入”，同时保持 Profile 解析优先级和密钥不落盘不变。
