# ADR-001：分离控制面、协作编排、Agent 执行与模型执行

- 状态：Accepted
- 日期：2026-09-01
- 决策范围：Go Backend、Python Agent Runtime、Native/AutoGen Collaboration Engine、所有模型消费者

## 背景

AgentRoom 已形成 Go 控制面与 Python 执行面的基本拓扑：Go 拥有房间、Agent、模型 Profile、消息、运行审计和 MySQL；Python Runtime 承载 Collaboration Engine 与 Agent Executor。

当前 Remote Collaboration 路径存在一个未闭合边界：Go 能决定 Agent 使用哪个模型，但传给 Python 的 `ModelReference` 只有模型身份元数据；Python `RuntimeRegistryAgentExecutor` 又直接构造缺少凭据的执行请求，最终由 `LLMExecutor` 在 `model_started` 之后发现 API Key 为空并返回 `model_not_configured`。

如果只为当前错误补传 API Key，或让 Native、AutoGen、DeepAgent 各自读取环境变量，会逐步形成多套模型配置体系，并使协作编排层依赖凭据基础设施。未来 AutoGen Selector 也会重复这个问题。

## 决策

AgentRoom 将模型选择、协作编排、Agent 执行和模型执行定义为四个独立职责层：

```text
Control Plane
    ↓
Collaboration
    ↓
Agent Execution
    ↓
Model Execution
```

### 1. Go Control Plane：决定使用哪个模型

Go 根据房间 Agent 快照、显式 Profile、Runtime scope 和默认策略产生稳定的 `ModelSelection`。`ModelSelection` 描述模型身份和凭据引用，但不包含可执行凭据。

概念字段包括：

```text
ModelSelection
├── profile_id
├── provider / protocol
├── model_name
├── credential_ref
├── runtime_scope
└── purpose            # participant / selector / system 等
```

Go 仍是 AgentRoom 业务数据和模型选择的权威所有者。Python Runtime 不查询 AgentRoom MySQL，也不自行改变 Go 已作出的模型选择。

### 2. Collaboration Engine：只负责协作决策

Native、AutoGen 以及未来 Engine 只负责：

- 选择下一位 Agent；
- 产生 `AgentTurn`；
- 应用轮次、handoff、cooldown 和终止策略；
- 将 Agent 执行结果映射为协作事件。

Collaboration Engine 不知道 `CredentialResolver`、Secret Manager、API Key 或 Provider Client 的存在，也不构造 `ModelConfig`。

```text
Collaboration Engine
    ↓
AgentTurn + ModelSelection
    ↓
AgentExecutor
```

### 3. Agent Execution：准备一次可执行的 Agent turn

`RuntimeRegistryAgentExecutor` 是 Collaboration 与具体 Executor 之间的适配边界。它接收 `AgentTurn` 和 `ModelSelection`，通过统一模型配置基础设施得到完整 `ModelConfig`，再选择具体 Executor。

```text
RuntimeRegistryAgentExecutor
├── ModelConfigResolver.resolve(ModelSelection)
└── ExecutorRegistry.resolve(agent_runtime)
        ├── LLMExecutor
        └── DeepExecutor
```

Agent Execution 层负责准备执行，但不决定业务上使用哪个模型，也不实现 Provider 调用。

### 4. Model Execution：统一产生并使用完整配置

`ModelConfigResolver` 是从选择态到可执行态的唯一入口：

```python
class ModelConfigResolver(Protocol):
    async def resolve(self, selection: ModelSelection) -> ModelConfig:
        ...
```

它组合以下职责：

```text
ModelConfigResolver
├── CredentialResolver
├── endpoint / profile resolution
├── protocol adaptation
├── timeout and defaults
├── organization-level overrides
└── final validation
```

`CredentialResolver` 只负责通过不透明的 `credential_ref` 获取 secret：

```python
class CredentialResolver(Protocol):
    async def resolve(self, credential_ref: str) -> ModelCredential:
        ...
```

本地开发提供 `EnvironmentCredentialResolver`；生产环境通过 Adapter 对接受控的 Secret Manager。具体 Secret Provider 不进入 Collaboration Engine 或 Model Consumer。

完整的 `ModelConfig` 是短生命周期执行对象，包含 Provider 调用需要的 endpoint、protocol、model name、credential、timeout 和经过合并验证的参数。它不得进入协作状态、checkpoint、事件、消息或持久化审计。

## 统一模型消费者边界

所有模型消费者都必须经过同一条不变量：

```text
ModelSelection
    ↓
ModelConfigResolver
    ↓
完整 ModelConfig
    ↓
Model Consumer
```

Model Consumer 包括但不限于：

- 普通 `LLMExecutor`；
- `DeepExecutor`；
- AutoGen Participant；
- AutoGen Selector；
- 未来 Coding Agent、ReAct Agent 和系统级模型调用。

消费者不必全部经过 `LLMExecutor`，但必须复用 `ModelConfigResolver`。AutoGen Selector 通过 `ModelExecutionService` 获取 AutoGen-compatible Model Client：

```text
AutoGen Selector
    ↓
ModelExecutionService
    ↓
ModelConfigResolver
    ↓
ModelClientFactory
    ↓
AutoGen-compatible Model Client
```

Native 与 AutoGen 必须共用同一 Agent Execution / Model Execution 边界，不得为框架便利建立第二套模型配置体系。

## 强制不变量

1. Go 是 AgentRoom 数据的唯一权威所有者。
2. Python Runtime 不直接访问 AgentRoom MySQL。
3. Go Control Plane 决定使用哪个模型并产生 `ModelSelection`。
4. Collaboration Engine 不持有或解析模型凭据。
5. Go 不通过 Collaboration 请求向 Runtime 传递明文模型凭据。
6. 任意 Model Consumer 不得直接访问环境变量、Vault、Secret Manager、数据库或 `credential_ref`。
7. 获得凭据的唯一合法路径是 `ModelConfigResolver → CredentialResolver`。
8. `credential_ref` 不得进入具体 Executor 或 Model Client。
9. Executor 和 Model Client 只能接收完整、已验证的 `ModelConfig`。
10. API Key 不得进入 Collaboration State、Event、Checkpoint、Message、Artifact、日志或持久化审计。
11. LLM、DeepAgent、AutoGen Participant 和 AutoGen Selector 必须复用统一模型配置基础设施。
12. Agent 消息只有经过 Go 校验并事务提交后才成为正式记录；`output_delta` 永远不是正式聊天记录。

核心 Code Review 规则是：

> No model consumer may obtain credentials directly.

## 事件语义

模型配置解析属于 Agent execution preparation，而不是模型执行：

```text
agent_turn_started
    ↓
agent_execution_preparing       # 可为内部状态，不要求公开事件
    ↓
resolve ModelConfig
    ↓
model_started
    ↓
output_delta*
    ↓
model_completed
    ↓
agent_message_completed
```

只有在 `ModelConfig` 已成功解析、验证并准备发起 Provider 调用后，才能产生 `model_started`。

以下错误属于 preparation failure：

- `credential_not_found`；
- `credential_access_denied`；
- `credential_provider_unavailable`；
- `invalid_model_selection`；
- `invalid_model_config`。

它们由 Agent Execution 层映射成稳定、脱敏的运行失败；Executor 内部对空 credential、endpoint 或 model name 的检查只作为防御性验证，不承担正常配置发现职责。

## 运行时数据流

```text
Go Control Plane
│
├── Room / Agent Policy
├── Model Profile
└── ModelSelection
      │
      │ gRPC
      ▼
Python Execution Plane
│
├── Collaboration Engine
│      └── AgentTurn
│             │
│             ▼
├── RuntimeRegistryAgentExecutor
│      ├── ModelConfigResolver
│      │       ├── CredentialResolver
│      │       └── ModelConfig
│      └── ExecutorRegistry
│              ├── LLMExecutor
│              └── DeepExecutor
│
└── Event Stream
       │
       ▼
Go Coordinator
       ├── Validation
       ├── Transaction
       ├── Audit
       └── Broadcast
```

## 实施拆分

### 变更一：Introduce unified model execution boundary

- 新增 `ModelSelection` 与 `ModelConfig`；
- 新增 `ModelConfigResolver` 与 `CredentialResolver`；
- 实现 `EnvironmentCredentialResolver`；
- 为生产 Secret Manager 保留 Adapter 端口；
- 将模型配置错误提前到 `model_started` 之前；
- 为解析优先级、错误分类、脱敏和生命周期编写契约测试。

### 变更二：Eliminate credential/config bypasses

- `LLMExecutor` 改为只接受完整 `ModelConfig`；
- `DeepExecutor` 改为只接受完整 `ModelConfig`；
- Participant Adapter 不自行读取环境变量；
- AutoGen Selector 必须通过 `ModelExecutionService`；
- 禁止 Engine、Executor 和 AutoGen Client Factory 直接解析 `credential_ref`；
- 增加静态架构测试或依赖扫描，防止重新引入旁路。

生产 Secret Manager 接入和现有数据库加密凭据迁移可在统一端口稳定后独立实施，不改变本 ADR 的职责边界。

## 迁移与兼容

- 现有 Go local Runtime 和 DeepAgent 子进程属于迁移期兼容路径；继续保留时也应通过各自执行面内的 `ModelConfigResolver`，不得成为绕过不变量的永久例外。
- 现有 `profile_id` 保持模型选择与审计身份；新增 `credential_ref` 是对 Secret Provider 的不透明引用，两者不得混为同一概念。
- Remote Collaboration 不因配置解析失败自动回退 Legacy，以避免同一 run 重复执行或重复提交。
- 协作请求只携带 `ModelSelection`；明文 `ModelConfig` 不跨越 Go/Python Collaboration 合约。

## 被否决的方案

### Go 在 Collaboration gRPC 请求中传递 API Key

虽然改动最小，但会把明文凭据扩散到协作请求、序列化、日志与抓包边界，并使 Go 承担 Runtime 凭据交付职责，因此否决。

### Native / AutoGen Engine 直接调用 CredentialResolver

会让协作编排依赖模型基础设施，并迫使每种 Engine 重复实现配置准备流程，因此否决。

### 每个 Executor 或框架自行读取环境变量

会形成 LLM、DeepAgent、AutoGen Participant、Selector 多套配置与错误语义，因此否决。

### 强制所有消费者经过 LLMExecutor

AutoGen Selector、DeepAgent 等消费者的执行形态不同。统一点应是 `ModelConfigResolver`，不是某个具体 Executor，因此否决。

## 影响

正面影响：

- 协作策略与模型基础设施解耦；
- Native、AutoGen 和未来 Engine 可以共享执行边界；
- 凭据访问路径唯一、可审计、可测试；
- 配置错误在模型调用前暴露，事件语义更准确；
- Secret Provider、协议和 Provider Client 可以独立替换。

成本与约束：

- Go/Python 合约需要引入稳定的 `ModelSelection`；
- Python Runtime 需要新的 Resolver 与 Secret Provider Adapter；
- DeepAgent 和 AutoGen 现有配置旁路必须迁移；
- 本地环境、生产 Secret Manager 和现有数据库凭据需要明确迁移策略；
- Runtime 必须确保 `ModelConfig` 生命周期短且不会被日志、checkpoint 或异常对象保留。

## 不在本 ADR 范围内

- Secret Manager 产品选型；
- 多租户凭据授权模型；
- Provider 出站网络与 SSRF 策略；
- AutoGen Selector 的具体 Prompt 与选角算法；
- `/api/ready` typed-nil 修复。该问题属于 Composition Root 的独立实现缺陷。
