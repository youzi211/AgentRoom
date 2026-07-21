## Why

AgentRoom 的 Go Agent 与 DeepAgent 目前分别依赖启动环境变量和 Python 子项目配置文件，管理员无法在产品界面中创建、验证、切换或按 Agent 分配模型。这使部署必须修改代码或服务器文件，也容易造成两套运行时的模型与密钥配置漂移。

## What Changes

- 新增独立的“模型配置”管理页面，以 Go AgentRoom 与 DeepAgent 两个运行时分组管理多个 OpenAI-compatible `v1/chat/completions` 模型 Profile。
- 新增模型 Profile 管理 API，支持新增、编辑、启用/停用、测试连接、设置运行时默认 Profile 和受引用约束的删除。
- 使用服务端主密钥加密 API Key 后保存到 MySQL；读取 API 永不返回明文密钥，DeepAgent 执行时由 Go 后端临时注入解密后的运行参数。
- 扩展现有“Agent 配置”页面，使每个 Agent 可选择与其运行时匹配的模型 Profile；未显式选择时使用对应运行时的默认 Profile。
- 房间创建时快照 Agent 选择的 Profile ID；Profile 的连接信息和密钥更新对现有引用实时生效，但 Agent 后续改绑只影响新房间。
- Go Agent 在每次运行时按 Agent 或 Go 默认 Profile 创建模型客户端；焦点提取与会议纪要首版共用 Go 默认 Profile。
- DeepAgent 在每次运行时按 Agent 或 DeepAgent 默认 Profile获得配置，不再要求管理员维护独立的模型 `.env` 或 TOML 密钥。
- 保留现有 `LLM_*` 与 DeepAgent 环境配置作为迁移期启动兜底；数据库已配置 Profile 后优先使用数据库配置。

## Capabilities

### New Capabilities

- `model-profile-management`: 管理 OpenAI-compatible 模型 Profile、运行时分组默认值、加密凭据、连接测试、状态与引用约束。
- `agent-model-binding`: 允许 Agent 绑定与其 Runtime 匹配的模型 Profile，并定义房间快照与配置更新的生效语义。
- `runtime-model-resolution`: 为 Go Agent、系统级 Go LLM 能力和 DeepAgent 解析有效 Profile，并以各自适配方式执行模型调用。

### Modified Capabilities

无。

## Impact

- 后端：新增模型 Profile 领域模型、服务、加密组件、Resolver、API 合约与 MySQL Repository；调整 Agent、房间快照、Go LLM Runtime 和 DeepAgent 单次执行配置注入。当前 local Runtime 使用子进程环境，目标 gRPC Runtime 使用单次请求上下文。
- 数据库：新增 `model_profiles` 表，并为 `agents`、`room_agents` 增加可空的 `model_profile_id`；新增运行时默认值唯一性和引用约束。
- 前端：新增模型配置路由与页面，扩展管理导航和 Agent 编辑表单；继续使用现有 Mantine 组件体系。
- 配置：新增服务端模型密钥加密主密钥；现有 `LLM_*`、`MODEL_*` 配置降级为迁移兜底。
- 运维：Docker/Compose 需要提供加密主密钥，并确保 DeepAgent Worker 只在本次执行上下文中接收解密后的凭据；当前 local 子进程和目标 gRPC 服务均不得持久化该凭据。
