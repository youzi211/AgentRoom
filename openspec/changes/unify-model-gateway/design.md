## Context

当前模型调用分散在四条路径：Go `internal/llm` 通过 LangChainGo 调用 OpenAI-compatible 接口；Python 普通 LLM Executor 直接使用 OpenAI SDK；DeepAgent 通过 `ChatOpenAI` 或 `ChatAnthropic` 独立构建模型；模型 Profile 连接测试由 Go 手工请求 `/chat/completions`。模型 Profile 虽然保存 `protocol`，但解析结果未携带该字段，远程 Agent 请求又固定写入 OpenAI Chat Completions，导致协议配置没有真正贯穿调用链。

Go 后端目前是 AgentRoom 业务权威，负责 Profile 存储与密钥解密、Agent/房间绑定、运行审计、权限和调度；Python 已经是常驻 Agent 执行服务，负责普通 LLM、DeepAgent、工具、流式事件和 artifact。改造必须保持这一所有权边界，不允许 Python 直接访问 AgentRoom MySQL，也不能让模型调用重新进入用户消息交付关键路径。

目标部署形态如下：

```text
Go Backend（控制面）
├── ModelProfileService / ModelResolver
├── Focus / Minutes
├── Agent Orchestrator
├── ModelGatewayClient ───────────────┐
└── AgentRuntimeClient ───────────┐   │
                                  │   │ gRPC
Python Process                    │   │
├── AgentRuntimeService <─────────┘   │
│   ├── LLMExecutor ───────────┐      │
│   └── DeepAgentExecutor ─────┤      │
└── ModelGatewayService <──────┼──────┘
    └── ModelGatewayCore <─────┘
        └── ProviderRegistry
            ├── OpenAICompatibleAdapter
            └── FutureProviderAdapter
```

Model Gateway 与 Agent Runtime 在逻辑上独立，初期共享 Python 进程、gRPC Server、健康检查和关闭生命周期。Agent Executor 在进程内调用 `ModelGatewayCore`，不经过回环 gRPC；Go 系统能力通过 `ModelGatewayService` 调用同一个 Core。

## Goals / Non-Goals

**Goals:**

- 所有生产模型调用、流式模型调用和连接测试经过统一 Model Gateway。
- 保持 Go 对 Profile 解析、密钥、权限、审计和业务编排的权威。
- 让普通 Python LLM 与 DeepAgent 共享 Provider Registry、模型构造和错误归一化。
- 建立协议中立、版本化、可取消、可观测的 Go/Python gRPC 合约。
- 保持已有 Agent Runtime 事件、artifact、消息持久化和 WebSocket 行为兼容。
- 首阶段保持现有数据库结构、Runtime Scope、默认 Profile 和环境变量兜底行为。
- 将新增协议的修改面限制在 Provider Adapter、协议注册和契约测试。

**Non-Goals:**

- 本次不同时接入 Anthropic、Gemini 或其他新的正式生产协议。
- 本次不实现模型负载均衡、自动故障转移、成本路由、语义缓存或跨 Profile 重试。
- 本次不把 Model Gateway 部署为第三个独立进程，也不引入外部 LiteLLM 类服务。
- 本次不把 `go/deepagent` Runtime Scope 迁移为 `agent_chat/focus/minutes/research` 用途绑定。
- 本次不改变普通会议用户权限，不允许其查看或修改模型 Profile。
- 本次不扩展多模态输入、批量推理或供应商专有高级参数。

## Decisions

### 1. Model Gateway 是独立逻辑服务，初期与 Python Agent Runtime 同进程部署

Python 进程同时注册 `AgentRuntimeService` 和 `ModelGatewayService`，两者依赖同一个 `ModelGatewayCore`，但不得相互冒充或复用不相关 RPC。这样可以直接复用现有常驻服务、TLS、容量控制和 Python 模型生态，又不会把 Focus、Minutes 等系统能力定义成 Agent Runtime 的附属功能。

备选方案一是在 Go 内实现网关。它能让明文凭据留在 Go，但需要在 Go 重建多供应商流式、工具和结构化输出适配，并让 DeepAgent 反向调用 Go，当前成本更高。备选方案二是立即部署独立网关进程，会增加服务发现、发布和健康治理复杂度，首阶段收益不足。逻辑独立、同进程部署为后续拆分保留稳定合约。

### 2. Go 管理控制面，Python 网关只管理单次模型执行

Go 的 `ModelResolver` 继续按显式 Profile、Runtime 默认 Profile、环境变量兜底的顺序解析配置，并在调用前解密 API Key。解析结果增加 `Protocol`，然后作为单次 `ModelConnection` 发送给 Python。Python 不查询 Profile 数据库、不持久化凭据，也不决定业务默认模型。

这样保留现有数据所有权和轮换语义。凭据跨进程传输沿用现有受保护 gRPC 边界：非本机部署必须使用 TLS 和服务身份校验；开发环境不安全连接必须显式启用。请求、日志、事件、trace、错误和 artifact 均不得序列化 API Key。

### 3. 新增协议中立的 Model Gateway gRPC 合约

新增版本化 Protobuf 服务：

```text
ModelGatewayService
├── Generate(ModelRequest) -> ModelResponse
├── Stream(ModelRequest) -> stream ModelEvent
└── Probe(ProbeRequest) -> ProbeResponse
```

`ModelRequest` 使用内部中立模型，不复用 OpenAI 请求结构，至少包含：`protocol_version`、`request_id`、`trace_id`、`purpose`、单次 `ModelConnection`、有序消息、`response_format`、执行限制和可选工具定义。首阶段 `response_format` 支持文本和 JSON 对象；工具字段用于保持合约可演进，但只在 Provider 与调用方均声明支持时使用。

`ModelEvent` 使用有序序号表达 started、text delta、usage、completed 和 failed。`Generate` 在服务端复用同一 Core 聚合结果；不要求 Go 调用方自行消费流以完成 Focus、Minutes 等简单调用。gRPC 状态码只表达协议、认证、容量和传输失败；已开始的 Provider 业务错误使用稳定错误结构返回。

备选方案是只提供 `Stream` 并由所有客户端聚合。该方案协议更少，但会把流状态机和错误处理复制到每个简单 Go 消费方，因此保留 unary 与 streaming 两种表面、共享同一核心实现。

### 4. Provider Registry 以协议为键并显式声明能力

`ProviderRegistry` 按规范化 `protocol` 查找 Provider Adapter。Adapter 必须提供模型构造、流式调用和探测能力，并声明 `streaming`、`json_object`、`tools` 等能力。网关在发起外部请求前验证所需能力；不支持时返回稳定的 capability 错误，而不是静默降级。

首个 `OpenAICompatibleAdapter` 覆盖当前 `openai_chat_completions`。同协议下接入新的模型 ID 或兼容 Endpoint 只需创建 Profile，不修改代码。全新协议必须新增 Adapter、注册项、能力声明和共享契约测试，但不得修改 Focus、Minutes、Agent Orchestrator 或 Profile Resolver。

### 5. 普通 LLM 与 DeepAgent 共享 BaseChatModel 构造入口

Provider Adapter 的核心构造结果采用 LangChain `BaseChatModel`。普通 LLM Executor 通过统一模型的 `astream`/`ainvoke` 执行；DeepAgent 将同一工厂产生的模型对象传入 `create_deep_agent(model=...)`。Gateway Core 负责把 LangChain 消息块、usage、finish 状态和异常转换为内部事件。

这会移除普通 LLM Executor 对 OpenAI Python SDK 的直接依赖，也避免 DeepAgent 维护第二套协议判断。若未来某协议只能由原生 SDK 支持，可由对应 Adapter 封装成 `BaseChatModel` 或在 Adapter 内实现等价接口，但不得让 Executor 直接依赖供应商 SDK。

备选方案是网关暴露本地 OpenAI-compatible HTTP 端点，再让 DeepAgent 使用 `ChatOpenAI` 回调网关。该方案引入同进程网络回环并把内部合约再次限制为 OpenAI 形状，因此不采用。

### 6. Go 使用兼容适配器迁移现有 llm.Client 消费方

Go 新建 `GatewayClient` 实现现有 `llm.Client` 和 `llm.JSONClient`，内部完成 Profile 解析、`ModelRequest` 映射和错误转换。Focus 与 Minutes 保持当前依赖接口和调用方式，只在装配层替换实现。模型 Profile 连接测试调用 `Probe`，删除手工 HTTP 请求。

普通 Agent 在 gRPC 模式下继续通过 `AgentRuntimeService` 执行，Agent Runtime 内部再调用共享 Gateway Core；它不需要从 Go 发起第二次网关 RPC。本地非 gRPC Runtime 在过渡期可以使用旧客户端，但迁移验收后必须删除或改为显式开发兼容模式，避免生产中继续存在两套模型协议实现。

### 7. 第一阶段保留 Runtime Scope，协议在创建后不可修改

现有 Profile 的 `go/deepagent` Scope、默认值和 Agent 绑定继续有效。Go 系统能力仍使用 `go` 默认 Profile，DeepAgent 仍使用 `deepagent` Profile。此次只让两个 Scope 的调用最终进入同一网关，不同时改变模型选择策略。

管理端创建 Profile 时从后端返回的受支持协议列表中选择协议。协议决定 Endpoint 规范化、凭据和能力语义，因此已创建 Profile 不允许直接切换协议；管理员需要新建 Profile并迁移绑定。前端不得独立维护一份与网关可能漂移的协议枚举。

后续可以单独提出用途绑定变更，将 Runtime Scope 替换为 `agent_chat`、`focus_analysis`、`meeting_minutes`、`deep_research`，但这不是网关统一的前置条件。

### 8. 可观测性使用非敏感统一字段

每次网关调用记录 `request_id`、`trace_id`、`purpose`、`protocol`、`profile_id`、`model_name`、耗时、结果分类和 token usage；不得记录消息正文、API Key、Authorization Header 或完整 Provider 响应。Agent Runtime 将网关模型 started/completed/failed 映射回现有 Agent 事件，Go 继续负责持久化 Agent run 的 Profile 和模型审计。

Focus、Minutes 和 Probe 只记录结构化运维日志，首阶段不新增数据库调用明细表。

### 9. 容量、取消和关闭由共享服务生命周期治理

Go 为每次 `Generate`、`Stream` 和 `Probe` 设置 deadline；Python 在容量等待、Provider 调用和事件输出期间观察取消。Model Gateway 使用有界并发，不能为每次调用创建无界后台任务。服务关闭时两个 gRPC 服务先退出就绪状态、停止接收新请求，再在宽限期内完成或取消活动调用。

Gateway 不在一次调用内自动切换 Profile，也不对非幂等流式调用做跨 Provider 重试。Provider Adapter 可遵守明确的单 Endpoint 短暂重试策略，但首阶段保持现有不重试行为，避免重复计费和输出。

## Risks / Trade-offs

- [Focus 和 Minutes 新增对 Python 服务可用性的依赖] -> 使用健康检查、明确的不可用错误和现有业务降级；Minutes 保留本地确定性 fallback，Focus 失败时推进分析游标且不阻塞消息。
- [LangChain 不同 Provider 的流式块、usage 和异常不完全一致] -> Provider Adapter 承担归一化，并用同一套契约测试覆盖文本、JSON、空输出、usage、取消和错误。
- [API Key 需要从 Go 传入 Python] -> 仅通过受保护的单次 gRPC 请求传输，禁止持久化和日志序列化，并保留跨语言敏感字段测试。
- [同进程部署使 Agent Runtime 与网关共享故障域] -> 服务合约和 Core 保持独立，未来可直接拆分部署；首阶段通过独立健康服务名和容量指标区分故障。
- [只实现一种真实协议可能产生 OpenAI 形状的抽象] -> 内部合约避免使用供应商字段，以假 Provider 和能力拒绝测试验证 Adapter 边界；第二协议另立 change 接入。
- [保留 Runtime Scope 会延续配置重复] -> 明确作为兼容阶段，网关完成后再单独设计用途默认绑定，避免本次同时承担数据库和 UI 重构。
- [本地旧 Runtime 形成旁路] -> 迁移任务包含生产装配扫描和依赖约束测试，确保默认路径没有供应商直连；旧路径只允许显式开发兼容并设置移除条件。

## Migration Plan

1. 扩展 `ResolvedModelConfig` 和现有 Agent Runtime 映射，使 `protocol` 端到端保真；保持旧 OpenAI 值作为已有数据兼容值。
2. 新增 Model Gateway Protobuf、生成 Go/Python代码和跨语言契约测试，不切换生产流量。
3. 在 Python 实现 Gateway Core、Provider Registry 和 OpenAI-compatible Adapter；先迁移普通 LLM Executor，再迁移 DeepAgent 模型构造。
4. 注册独立 `ModelGatewayService`，补齐健康检查、容量、取消、敏感字段清理和优雅关闭。
5. 在 Go 实现网关客户端和 `llm.Client` 兼容适配器，迁移 Focus、Minutes 和连接测试；保持业务接口不变。
6. 更新模型管理 API 与前端协议选择，协议选项来源于后端网关能力，不允许编辑已有 Profile 的协议。
7. 运行 Go、Python、前端、gRPC 集成和 OpenSpec 严格校验；扫描生产代码，确认 Provider SDK 和供应商 HTTP 请求只存在于网关 Adapter。
8. 删除或隔离旧 Go 直连与本地 DeepAgent 独立模型路径，更新 README、架构和部署文档。

部署时先发布向后兼容的 Python 服务，再发布 Go 客户端切换。回滚时优先回滚 Go，使调用恢复旧路径；数据库无破坏性变更，旧 Profile 仍保持 OpenAI-compatible 值。若 Python 网关上线但 Go 未切换，新增服务保持空闲且不影响现有 Agent Runtime。

## Open Questions

- Runtime Scope 用途化、协议专用高级参数和独立网关进程拆分均留给后续 change，不阻塞本次实现。
- 首阶段不承诺供应商专有 JSON Schema 和工具并行语义；内部合约保留字段，只有能力明确支持时才启用。
