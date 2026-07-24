## Context

AgentRoom 当前由 Go `Runner` 同时承担房间协作规划与单 Agent turn 治理：

```text
Human message
  -> Go mention detection
  -> mention_fanout OR guided_dialogue
  -> Go selects one Agent
  -> local Runtime OR Python AgentRuntimeService.ExecuteAgent
  -> Go persists one Agent message and schedules the next turn
```

这种结构保证 Go 拥有房间、权限、消息和审计，但也把触发、选角、交接、停止与具体执行方式绑定在 `internal/agent`。无 mention 的普通消息不会自然触发协作，`mention_fanout` 与 `guided_dialogue` 还维护两条不同主链。若直接把 AutoGen 的 `SelectorGroupChat`、`Swarm` 或状态对象嵌入当前 Runner，Go 会依赖 Python 框架概念，后续版本升级或更换引擎将再次触及业务层。

当前 Python Agent Runtime 已具备常驻 gRPC Server、Executor Registry、容量、取消、事件背压、健康检查和优雅关闭，适合作为 Collaboration Runtime 的承载进程。Go 仍是唯一业务控制面，Python 不访问 AgentRoom MySQL。活动变更 `unify-model-gateway` 将统一模型 Provider 调用；本设计与其保持协议和实现独立，但 AutoGen 生产启用依赖框架中立模型端口就绪。

目标拓扑：

```text
Browser
  -> Go AgentRoom control plane
       |-- MySQL business truth
       |-- REST / WebSocket
       |-- CollaborationCoordinator
       |      `-- CollaborationRuntimeClient
       `-- ModelGatewayClient
                    |
                    | gRPC
                    v
       Python service process
       |-- AgentRuntimeService
       |     `-- ExecutorRegistry
       |-- CollaborationRuntimeService
       |     `-- CollaborationEngineRegistry
       |            |-- NativeCollaborationEngine
       |            `-- AutoGenCollaborationEngine
       `-- ModelGatewayCore / framework-neutral model port
```

## Goals / Non-Goals

**Goals:**

- 用一次统一的 collaboration run 表达有 mention、无 mention、多人交接和受限自主讨论。
- 将多 Agent 选角、交接和终止规划移到可替换 Collaboration Engine 后面。
- 保持 Go 对房间、权限、业务消息、运行审计、幂等提交和实时广播的所有权。
- 建立版本化、框架中立、可取消、可观测的 Go/Python Collaboration Runtime 合约。
- 保留 Native Engine 作为行为基线与回滚路径，并以 AutoGen Engine 验证扩展边界。
- 让 AutoGen 依赖、类型、状态与升级影响局限在单一 Python 模块。
- 保持人类消息“先持久化并立即广播，再异步协作”的低延迟语义。
- 为每房间串行运行、全局容量、人类中断和失败收敛建立确定性规则。

**Non-Goals:**

- 本变更不把房间、消息、Agent 配置或 Model Profile 的数据所有权迁移到 Python。
- 本变更不让 AutoGen 内部消息、Team 对象或状态格式成为公共 API 或数据库领域模型。
- 本变更不实现跨 Go 实例的分布式房间锁、durable queue、租约恢复或自动任务接管。
- 本变更不同时引入新的模型 Provider、自动故障转移、成本路由或语义缓存。
- 本变更不允许普通用户管理 Model Profile 或执行任意工具。
- 本变更不保证旧 collaboration run 在后端或 Python 重启后自动续跑；首阶段统一收敛为 interrupted。
- 本变更不立即删除现有 `ExecuteAgent`、本地 Runtime 或旧 Dialogue 审计读取路径。

## Decisions

### 1. 以 Collaboration Runtime 作为独立执行边界，而不是把 AutoGen 嵌入 Go Runner

Go 新增 `CollaborationRuntime` 端口，面向中立的请求与事件；远程实现使用 gRPC，测试使用 Fake，迁移期可保留本地兼容实现。Python 暴露逻辑独立的 `CollaborationRuntimeService`，并由 `CollaborationEngineRegistry` 选择 Native 或 AutoGen Engine。

备选方案是在 Go Runner 中调用一个“路由 Agent”后继续执行现有 fanout。它能较快解决无 mention 不回复，但不会统一多轮状态、交接、终止、框架状态和替换边界，因此不采用。

备选方案是让 Go 直接理解 AutoGen Team 与消息类型。Go/Python 之间将产生框架耦合，版本升级需要修改 Protobuf 和业务层，因此不采用。

### 2. Go 保持控制面所有权，Python 只拥有一次运行内的协作状态

Go 负责：

- 校验房间访问与生命周期；
- 持久化人类消息；
- 创建 collaboration run 与 Agent run 审计；
- 解析不可变房间 Agent 快照、知识和允许的模型引用；
- 验证远程事件并幂等提交最终 Agent 消息；
- 映射 WebSocket 活动；
- 取消、deadline、停机与回滚。

Python 负责：

- 从允许的 Agent 集合选择发言者；
- 构造 Engine/Team 与运行内上下文；
- 执行 Agent turn、工具和交接；
- 执行终止条件；
- 输出中立事件与可选 checkpoint；
- 清理运行内资源。

Python 不查询房间当前状态。请求开始后的权限、Agent 与策略快照保持不可变；新的人类消息通过取消旧 run并创建新 run体现状态变化。

### 3. 使用服务端流式 `ExecuteConversation`，首阶段不采用双向长会话

新增版本化 RPC：

```text
CollaborationRuntimeService
  `-- ExecuteConversation(ExecuteConversationRequest)
        -> stream CollaborationEvent
```

请求包含：

- `protocol_version`
- `collaboration_run_id`、`trace_id`
- 房间与触发消息快照
- 有序 Agent 快照与初始 mention 候选
- 有界 transcript 与知识上下文
- Collaboration Policy
- 每个 Agent 的框架中立模型引用/连接
- 执行限制与显式 engine
- 可选 opaque checkpoint

事件包含：

- accepted / collaboration_started
- speaker_selected
- agent_turn_started
- model/tool/progress/output activity
- handoff_requested
- agent_message_completed
- checkpoint
- completed / stopped / cancelled / failed

双向流可以让 Go 对每个 turn 提交后回送 ACK，但会把 AutoGen 的连续 `run_stream` 拆成分布式状态机，并显著增加重连与半提交语义。首阶段由 Python 在单次流内推进 Team；Go 按事件提交消息。一旦提交失败，Go 立即取消流并丢弃之后的候选事件。可能已经发生的额外模型调用是该简化方案的代价，但不会形成错误业务消息。

若未来必须在每个 turn 提交后才能继续，可新增版本化的 bidi 协议，而不是改变 v1 语义。

### 4. 用新的 collaboration run 统一触发与多轮治理

每条符合策略的人类消息最多创建一个 collaboration run。显式 mention 是初始选角的高优先级输入，不再决定是否进入另一条执行主链。

统一流程：

```text
persist and broadcast human message
  -> cancel previous room collaboration
  -> create collaboration run
  -> build immutable snapshot
  -> execute selected Engine
  -> validate stream
  -> commit each accepted Agent message
  -> finish collaboration run
```

房间策略增加：

- `engine`: `native` / `autogen`
- `trigger_mode`: `mention_only` / `automatic`
- `max_turns`
- `max_turns_per_agent`
- `allow_agent_handoff`
- `allow_self_followup`
- `cooldown_ms`
- 重复与空输出停止规则

旧 `DialoguePolicy` 字段在迁移期映射到新策略。旧房间默认 `native + mention_only`，避免无意增加模型调用；灰度房间可选择 `autogen + automatic`。验证完成后再单独决定新房间默认值。

### 5. 每房间单活动 run，人类消息具有抢占优先级

Go `CollaborationCoordinator` 维护 `room_id -> active run/cancel` 映射。新的人类消息成功持久化并广播后：

1. 标记旧 run 不再接受新 turn；
2. 取消旧 gRPC Context；
3. 等待旧 run 在受限时间内收敛；
4. 为新消息启动 run。

Python 同时执行每房间互斥检查，防止 Go 进程内竞态或错误客户端启动两个 Team。不同房间可并发，但受全局 Engine 容量和等待队列限制。

备选方案是把人类消息排在旧 run 之后。它会让 Agent 自主讨论阻塞人的新意图，与会议产品的人类优先原则冲突，因此不采用。

### 6. 新增 collaboration run 审计，复用 agent_runs 表达具体 turn

新增 `collaboration_runs` 业务记录，至少包含：

- ID、room ID、root human message ID
- engine、engine version、policy version
- status、stop reason、turn count
- created/started/completed timestamps
- sanitized error

`agent_runs` 增加可空 `collaboration_run_id`、`turn_index` 和 `parent_message_id`，继续承担每次 Agent turn 的模型与结果审计。现有 `dialogue_runs` 保留历史只读语义；不在本变更中破坏性重命名或批量迁移旧记录。API 查询层可以在迁移期合并展示两种历史记录。

框架 checkpoint 首阶段不作为业务恢复前提。协议支持 checkpoint，但生产默认可以丢弃；若启用持久化，只保存带 engine/version/hash 且通过大小限制的 opaque 字节，不允许包含凭据。后端或 Python 重启时，活动 run 统一标记为 interrupted，不自动续跑。

### 7. Native Engine 在 Python 中复现当前策略并复用内部 Executor

`NativeCollaborationEngine` 是不依赖 AutoGen 的确定性实现，用于：

- 复现显式 mention 顺序；
- 复现 Agent-to-Agent mention 交接；
- 复现 `guided_dialogue` 的轮次、冷却、重复和空输出规则；
- 为无 mention 的 automatic 模式提供受控的默认首发选择策略；
- 作为共享 Engine 契约测试的基线。

Engine 在进程内调用共享 Executor 接口，而不是通过 localhost gRPC 回调 `AgentRuntimeService`。普通 LLM 与 DeepAgent 仍由现有 Executor 实现完成单 turn；Collaboration Engine 只负责协作。

备选方案是在 Go 保留 Native Engine、Python 只实现 AutoGen。这样会保留两套协作状态机，无法验证 Engine Registry 的替换边界，因此只作为短期灰度回滚路径，不作为目标结构。

### 8. AutoGen Engine 首阶段采用 SelectorGroupChat 风格的受约束选角

AutoGen 当前 AgentChat 提供 Team、流式运行、终止条件、取消和状态管理；`SelectorGroupChat` 支持模型选角，`Swarm` 支持显式 handoff。首阶段采用单一受约束 Team 适配器：

- 有有效 mention 时，首位发言者确定性选择，不为首发额外调用 selector 模型；
- 无 mention 时，selector 从允许集合选择一位首发 Agent；
- 后续 handoff 和 selector 结果都要经过 AgentRoom Policy Validator；
- 每次只允许一个 Agent turn 进入可提交状态；
- AutoGen 内部 manager/control 消息不进入房间聊天历史；
- AgentRoom ID 与 AutoGen 内部安全名称保持稳定映射。

不同时混用多个 Team 实现暴露给 Go。若 `Swarm` 的 handoff 语义在验证后更适合，可在 AutoGen 适配器内部替换，不改变 Collaboration Runtime 合约。

AutoGen 版本必须精确锁定，并由契约测试和状态兼容测试保护。任何 `autogen_*` import 只能出现在 `agent_runtime/collaboration/engines/autogen/` 或对应测试。

### 9. AutoGen 参与者通过适配器复用现有 Agent Executor

房间可能同时包含普通 `llm` Agent 与 `deepagent` Agent。AutoGen Engine 不假设所有参与者都是原生 `AssistantAgent`；它创建 AgentRoom Participant Adapter，根据 Agent 快照的 runtime 调用内部 Executor 接口，并将执行事件映射回 AutoGen/中立事件。

这样 DeepAgent artifact、工具活动、输出限制和模型审计仍复用现有实现。AutoGen 负责 Team 协作，不重新实现 DeepAgent。

### 10. 所有 selector 与 Agent 模型调用通过框架中立模型端口

AutoGen Engine 定义 `CollaborationModelClient` 适配器。目标实现调用 `ModelGatewayCore` 或其同进程端口；它负责：

- Profile/协议映射；
- 流式与非流式调用；
- usage、错误和取消归一化；
- 凭据生命周期；
- Provider 能力校验。

AutoGen Agent、Team 和 selector 不得直接创建 OpenAI、Anthropic 或其他 Provider SDK 客户端。`unify-model-gateway` 未就绪时，AutoGen Engine 只允许 Fake/集成测试或显式非生产实验，不报告生产就绪。Native Engine 与现有 Agent Runtime 可继续服务。

### 11. 框架状态只作为可丢弃优化，不成为业务事实

权威状态顺序：

```text
MySQL messages / room agent snapshots / policies / runs
  > current Go in-memory active mapping
  > optional Engine checkpoint
```

AutoGen checkpoint 包含：

- engine=`autogen`
- pinned package version
- adapter state version
- hash/size
- opaque payload

恢复时先验证版本与摘要，再由适配器加载；不兼容则从请求 transcript 重建 Team。不得因为 checkpoint 无法恢复而修改、删除或重复生成已持久化消息。

### 12. 实时层区分聊天消息与协作活动

继续使用现有 `message` 事件承载已提交的人类和 Agent 消息。新增或扩展 activity 事件表达：

- collaboration started/finished
- speaker selected
- Agent turn started/finished
- handoff
- tool/model progress
- cancelled/failed

活动事件不包含 Prompt、API Key、完整 Provider 响应或 AutoGen 内部推理。流式 delta 可以作为短暂 UI 状态，但刷新后不保证恢复，最终完整消息仍以 MySQL 提交为准。

### 13. 健康、容量和配置按逻辑服务分离

Python 同一进程注册两个健康服务名：

- `agentroom.runtime.v1.AgentRuntimeService`
- `agentroom.collaboration.v1.CollaborationRuntimeService`

两个服务共享 gRPC Server、TLS 和关闭信号，但拥有独立 Registry 就绪状态、活动运行命名空间和容量指标。建议新增：

- `COLLABORATION_RUNTIME_ENABLED`
- `COLLABORATION_ENGINE_ALLOWLIST`
- `COLLABORATION_MAX_CONCURRENCY`
- `COLLABORATION_MAX_PENDING`
- `COLLABORATION_AUTOGEN_ENABLED`
- `COLLABORATION_CHECKPOINT_MAX_BYTES`

Go 增加目标地址、TLS、deadline、最大请求/事件和默认引擎配置。配置未启用时不注册远程协作路径。

## Risks / Trade-offs

- [AutoGen API 与状态格式变化较快] → 精确锁定版本，所有依赖封装在适配器目录，并用 Engine 契约、事件映射和 checkpoint 兼容测试保护。
- [SelectorGroupChat 增加额外模型调用、延迟和费用] → mention 首发走确定性选择；限制每轮 selector 调用；记录非敏感 usage；保留 Native Engine。
- [服务端流没有逐 turn 提交 ACK] → Go 提交失败立即取消并丢弃后续事件；Python 不直接广播或持久化；若实际故障成本不可接受，再设计 v2 双向协议。
- [同一 Python 进程扩大故障域] → 两个逻辑服务、Registry 和健康状态独立；目标合约允许未来拆分进程。
- [AutoGen 内部消息误入聊天历史] → 仅 `agent_message_completed` 可提交；管理、规划与 selector 消息只映射为受限 activity。
- [可选 checkpoint 可能含敏感或不可兼容内容] → 默认不持久化；启用时实施大小、版本、摘要和敏感字段契约；恢复失败直接重建。
- [Native Engine Python 重写与现有 Go 行为漂移] → 先建立黄金测试和共享场景；影子执行对比选角与终止结果；旧 Go 路径保留到观察期结束。
- [DeepAgent 无法直接作为普通 AutoGen AssistantAgent] → 使用 Participant Adapter 调用现有 Executor，不重写 DeepAgent 工具与 artifact 逻辑。
- [Model Gateway 与 Collaboration Runtime 并行开发造成临时适配] → 把模型端口定义为独立接口；AutoGen 生产就绪显式依赖 Gateway，不引入临时 Provider 直连。
- [每房间单活动 run 在高频发言下频繁取消] → 人类消息优先；加入短暂收敛窗口和稳定取消；不自动重试已可能调用模型的 run。
- [新增审计模型增加查询复杂度] → 保留 agent_runs 为 turn 事实，collaboration_runs 只记录父运行；旧 dialogue_runs 保持只读兼容。

## Migration Plan

1. 为当前 `mention_fanout`、`guided_dialogue`、人类消息交付、Agent run 提交、取消和 WebSocket 活动补齐黄金测试。
2. 新增 collaboration run 数据模型、Store 契约、幂等状态转换和 Go `CollaborationCoordinator`，先不切换现有生产流量。
3. 新增 Collaboration Runtime Protobuf、Go/Python 生成代码和跨语言契约测试。
4. 在 Python 注册逻辑独立的 Collaboration Runtime、Engine Registry、容量、健康检查和优雅关闭。
5. 实现 Native Engine，复用内部 Executor；通过黄金场景验证与现有 Go 策略等价。
6. 在 Go 增加远程 Collaboration Runtime Client 和事件治理，以显式灰度配置让测试房间使用 `native` 远程引擎。
7. 更新前端与 API，展示 collaboration activity，并允许管理员或建房策略选择兼容触发模式与引擎。
8. 完成或接入框架中立 Model Gateway 端口；在其就绪前保持 AutoGen 生产状态不可用。
9. 固定 AutoGen 版本，实现 AutoGen Engine、Participant Adapter、selector、handoff、终止和事件映射。
10. 使用 Fake Model/Tool 运行 AutoGen 契约测试，再在隔离环境执行真实模型对比，评估选角正确率、额外 token、延迟、取消和重复输出。
11. 以房间 allowlist 灰度 `autogen + automatic`；Native Engine 与旧 Go 路径继续作为显式回滚选项。
12. 观察期满足成功率、延迟、费用和一致性阈值后，停止创建新的旧 `dialogue_runs`，但保留历史读取。
13. 更新 README、架构、部署、环境变量和运维回滚文档；运行 Go、Python、前端、gRPC、MySQL 集成与 OpenSpec 严格验证。

回滚顺序：

1. 停止为新房间/运行选择 AutoGen Engine；
2. 切回远程 Native Engine；
3. 若 Collaboration Runtime 整体不可用，显式恢复旧 Go 协作路径；
4. 不重做已经进入终态或可能已调用模型的 run；
5. 保留新增审计数据和向后兼容字段，不执行破坏性数据库回滚。

## Open Questions

- 首次启用 automatic 模式时，新房间默认使用 AutoGen 还是继续要求管理员显式开启，需要真实延迟和费用数据后决定。
- AutoGen selector 使用专门的低成本 Profile，还是复用当前发言 Agent/Profile，需要与用途化 Model Profile 后续设计协调。
- 首阶段 checkpoint 是否完全禁用持久化，还是仅在测试环境验证恢复能力，取决于 AutoGen 固定版本状态内容审计。
- 是否需要在 v2 引入双向流和逐 turn commit ACK，取决于影子运行中数据库提交失败后额外模型调用的实际影响。
- 旧 `dialogue_runs` 的历史 API 是长期兼容还是最终统一迁移到 collaboration run 视图，可在灰度完成后单独决定。
