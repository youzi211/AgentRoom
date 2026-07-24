## Why

当前系统分别在 Go、Python 普通 LLM Executor、DeepAgent 和模型连接测试中实现模型调用，协议选择、请求转换、错误处理和连接测试没有统一边界。新增模型或新协议会同时修改多个语言和业务模块，因此需要建立独立模型网关，让所有模型流量通过统一入口，同时保持 Go 对配置、权限、审计和业务编排的控制权。

## What Changes

- 新增独立的 Model Gateway 逻辑边界，统一提供非流式生成、流式生成和模型连接探测能力。
- Model Gateway 初期与常驻 Python Agent Runtime 同进程部署，但使用独立服务合约和模块边界，允许后续单独部署。
- Go 侧新增网关客户端适配器，让 Focus、会议纪要及其他 Go 系统能力不再直接创建供应商模型客户端。
- Python 普通 LLM Executor 与 DeepAgent Executor 共享同一个 Provider Registry 和模型构造入口，不再各自选择供应商 SDK。
- 模型 Profile 的 `protocol` 从持久化层贯穿解析结果、gRPC 请求、Provider 路由、连接测试和审计元数据；移除远程请求中的固定 OpenAI 协议值。
- 管理端允许管理员在创建 Profile 时从网关声明的受支持协议中选择协议；协议创建后不可直接修改，普通会议用户仍不能管理模型配置。
- 模型连接测试改由网关 `Probe` 能力执行，不再由 Go 手工拼装 `/chat/completions` 请求。
- 首阶段保留现有 `go` 与 `deepagent` Runtime Scope、默认 Profile 和环境变量兜底语义，以兼容已有数据和前端绑定；用途化默认模型不在本次范围内。
- 首阶段正式支持现有 OpenAI-compatible Chat Completions 协议，并通过 Provider Adapter 契约与测试保证后续协议只在网关层扩展。
- 逐步停用 Go LangChain/OpenAI 直连、Python OpenAI SDK 直连和本地 DeepAgent 子进程中的独立模型构造路径；迁移完成后，网关 Provider Adapter 之外的生产代码不得直接调用模型供应商。

## Capabilities

### New Capabilities

- `model-gateway-service`: 定义统一模型请求、Provider Adapter 注册与路由、能力校验、流式事件、错误归一化、敏感信息保护及同进程复用边界。
- `grpc-model-gateway-contract`: 定义 Go 与 Python 之间版本化的 `Generate`、`Stream`、`Probe` gRPC 合约、取消传播、错误语义和跨语言兼容要求。

### Modified Capabilities

- `model-profile-management`: 管理员可选择网关支持的模型协议，且已保存和草稿 Profile 的连接测试统一由网关执行。
- `runtime-model-resolution`: 解析结果必须携带模型协议，Go 系统能力、普通 Agent 和 DeepAgent 均使用解析后的配置通过统一网关调用模型，同时保持现有解析优先级与 Runtime Scope 兼容语义。
- `python-agent-runtime-service`: 普通 LLM Executor 与 DeepAgent Executor 必须复用独立 Model Gateway Core，不再直接拥有供应商调用实现。

## Impact

- Protobuf：新增版本化 Model Gateway 服务、请求、响应、流式事件、能力与安全错误类型；现有 Agent Runtime 合约仅做兼容性字段修正。
- Go 后端：`internal/llm`、模型解析、模型 Profile 连接测试、服务装配、Focus、会议纪要及相关测试。
- Python 服务：`agent_runtime/model_adapter.py`、普通 LLM Executor、DeepAgent 模型构造、服务注册、配置、安全清理和契约测试。
- 前端：模型 Profile 创建表单、协议选项加载、草稿连接测试 payload、协议展示和对应测试。
- 数据：首阶段不新增数据库表，不移除 `runtime_scope`；已有 `protocol` 字段继续使用，无破坏性迁移。
- 部署：Python Runtime 将同时暴露 Agent Runtime 与 Model Gateway gRPC 服务；现有 TLS、消息大小、健康检查和优雅停机配置需要覆盖两个服务。
- 依赖：供应商 SDK 或 LangChain Provider 包只能由 Python 网关 Provider Adapter 依赖；不引入新的外部网关服务。
