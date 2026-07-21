## Context

AgentRoom 目前在进程启动时从 `LLM_*` 环境变量创建一个 Go OpenAI-compatible 客户端，并把同一客户端交给会议 Agent、焦点提取和会议纪要。DeepAgent 则在独立 Python 子项目中从 `deepagent.toml`、`.env` 和 `MODEL_*` 环境变量加载模型配置。两套配置都脱离管理界面，且 DeepAgent 的凭据与 Go 后端的管理员权限、数据库和审计边界分离。

本变更跨越前端管理页面、Go API/Service/Store、MySQL schema、Agent 房间快照、Go LLM Runtime 与 Python DeepAgent 子进程。实现必须继续支持 OpenAI-compatible `POST /v1/chat/completions`，不得把特定供应商的模型清单写死在代码中；管理员输入模型 ID，系统只验证连接契约。

## Goals / Non-Goals

**Goals:**

- 提供独立的 Agent 配置页和模型配置页，由 Go 后端统一管理 Go 与 DeepAgent 的模型 Profile。
- 支持每个运行时多个 Profile、一个默认 Profile，以及单 Agent 显式覆盖。
- 加密保存 API Key，确保查询响应、日志、活动记录和错误信息不泄露明文凭据。
- 保持现有房间 Agent 快照语义：模型选择在建房时固定，模型连接内容可在运维修改后实时生效。
- 让 Go Agent、焦点提取、会议纪要和 DeepAgent 都通过同一个解析服务获得有效模型配置。
- 保留现有环境变量作为迁移期兜底，使升级部署不必立即在数据库重新录入密钥。

**Non-Goals:**

- 首版不支持 Responses API、Anthropic 原生协议、Azure 专有部署字段或供应商模型自动发现。
- 首版不为焦点提取和会议纪要提供独立模型选择；它们共用 Go 默认 Profile。
- 首版不实现主密钥在线轮换、外部 Vault/KMS 或多租户凭据隔离。
- 首版不改变 Agent 的任务规划、对话编排、Prompt、知识检索或 DeepAgent 研究工具能力。
- 首版不把数据库 Profile 自动回写到 `.env`、`deepagent.toml` 或其他服务器文件。

## Decisions

### 1. 由 Go 后端统一管理单表模型 Profile

新增 `model_profiles` 表，核心字段为：

- `id`、`name`
- `runtime_scope`：`go` 或 `deepagent`
- `protocol`：首版固定为 `openai_chat_completions`
- `base_url`：规范化后以 `/v1` 结尾，不包含 `/chat/completions`
- `model_name`
- `api_key_ciphertext`、`api_key_hint`
- `enabled`、`is_default`
- `created_at`、`updated_at`

同一张表避免 Go 与 Python 各自维护配置源；`runtime_scope` 又阻止 Go Agent 误绑 DeepAgent Profile。设置默认值必须在事务中清除同一 scope 的旧默认值，且只有启用的 Profile 能成为默认值。某个 scope 第一次创建启用 Profile 时，若尚无默认值，系统自动将其设为默认值。

替代方案是分别使用 MySQL 与 DeepAgent TOML/.env，但这会让前端统一、后端分裂，并在 Docker、多实例和密钥轮换时产生配置漂移。外部配置中心安全性更高，但对当前单体部署过重，保留为未来替换 `SecretCipher` 的演进方向。

### 2. API Key 使用版本化 AES-256-GCM 密文

Go 服务从 `MODEL_CONFIG_ENCRYPTION_KEY` 读取 base64 编码的 32 字节主密钥。`SecretCipher` 使用随机 nonce、Profile ID 作为附加认证数据，并保存版本化密文封装，便于未来增加新密钥版本。数据库另存不可逆的显示提示，例如最后四位，但 API 永不返回密文或明文。

创建、替换或测试新 API Key 时必须存在有效主密钥。编辑请求中省略 API Key 表示保留现有密钥；清除密钥使用单独的显式动作。若数据库已存在 Profile 而主密钥缺失或无法解密，模型调用与需要密钥的管理操作明确失败，不得回退到其他 Profile。明文只在单次请求或 DeepAgent RunContext 所需的短生命周期内存中存在。

### 3. 管理 API 由现有管理员鉴权保护

新增 `/api/model-profiles` 管理端点，覆盖列表、创建、更新、设置默认、停用、删除和测试连接。所有读写和测试端点都使用现有 `X-Admin-Key` 管理员保护，因为列表会暴露供应商端点、模型名和密钥提示等运维信息。

连接测试由后端发起。测试请求可以引用已保存 Profile，也可以携带未保存的草稿字段；后端对 `POST <baseURL>/chat/completions` 发送最小消息请求，并只返回成功、延迟、响应模型或已清洗错误。测试请求体、日志与响应不得回显 API Key。首版不依赖 `/v1/models`，因为兼容供应商不一定实现该端点。

### 4. Agent 绑定 Profile，建房时快照解析后的选择

`agents` 和 `room_agents` 增加可空 `model_profile_id`。Agent 配置页只展示与 Agent Runtime 匹配的 Profile：`llm` 映射 `go`，`deepagent` 映射 `deepagent`。保存不匹配绑定时后端拒绝请求；切换 Runtime 时，调用方必须同时清除绑定或提供新 Runtime 的兼容 Profile。

创建房间时，系统把 Agent 显式绑定的 Profile ID，或当时对应 scope 的默认 Profile ID，解析为一个具体 ID并写入 `room_agents`。因此以后修改全局 Agent 的绑定或更换默认 Profile 只影响新房间。Profile 的 Base URL、模型名和 API Key 不复制到房间；现有房间在运行时仍读取该 Profile 当前内容，使密钥轮换和供应商迁移能实时生效。

被 `agents`、`room_agents` 或默认槽位引用的 Profile 不得删除。若被房间快照显式引用的 Profile被停用或无法解密，运行返回明确配置错误，不得静默换成默认模型。

### 5. 使用统一 Resolver 与动态客户端替代启动时单例客户端

新增 `ModelResolver`，按以下优先级解析：

1. 房间 Agent 快照中的具体 `model_profile_id`；
2. 对没有快照 ID 的旧数据或全局 Agent，使用其 Runtime scope 的数据库默认 Profile；
3. 数据库没有可用默认 Profile 时，使用现有环境变量兼容配置；
4. 都不可用时返回“模型未配置”。

显式绑定存在但已停用、缺失或不可解密时停止在第一步并报错，不能降级到默认值。只有“从未指定且没有数据库默认值”的情况才允许环境变量兜底。

Go `LLMAgentRuntime` 在每次执行时通过 Resolver 获得解密后的配置并创建 OpenAI-compatible 客户端，不再持有启动时固定客户端。焦点提取和会议纪要使用一个动态 `DefaultGoClient`，每次调用解析 Go 默认 Profile，因此管理员更新默认连接后无需重启服务。

每个 `agent_run` 保存实际使用的 `model_profile_id` 和 `model_name` 快照用于审计，但不保存 Base URL 中的敏感查询参数、请求正文或 API Key。

### 6. DeepAgent 通过单次执行上下文接收 Profile

DeepAgent Runtime 在取得 `deepagent` scope Profile 后，构造仅供本次执行使用的模型连接：

- `MODEL_PROTOCOL=openai`
- `MODEL_BASE_URL`
- `MODEL_NAME`
- `MODEL_API_KEY`

当前 local Runtime 把这些字段映射到单次子进程环境；目标 gRPC Runtime 把它们映射到受保护的 `ExecuteAgent` 请求并只保存在 Python RunContext 内存。两种路径都无需把数据库配置写入文件，且 Go/Python 的日志、错误包装和活动事件不得包含完整模型连接。DeepAgent 的 Tavily、输出目录、搜索限制等非模型配置仍由 Python Runtime 配置维护，本变更只统一模型连接配置。

### 7. 两个管理页面共享现有工作台导航

管理区提供独立路由：

- `/agents`：保留现有 Agent 列表、角色、Prompt、知识库和启用状态，并新增 Runtime 与模型 Profile 选择。
- `/models`：按“Go 模型”和“DeepAgent 模型”分组展示 Profile，提供新增、编辑、测试连接、设为默认、停用和删除。

模型表单首版只暴露名称、Runtime scope、API Base URL、模型 ID、API Key、启用状态和默认状态。API Key 使用密码输入；已有密钥显示掩码状态而不回填值。页面继续使用 Mantine，不引入新 UI 组件库。

### 8. 环境变量只作为迁移兜底，不自动导入数据库

升级后现有 `LLM_*` 和 DeepAgent `MODEL_*` 配置继续可用。模型配置页在某个 scope 尚无数据库默认值时显示“当前使用环境配置兜底”，但不展示或自动复制环境密钥。管理员创建数据库默认 Profile 后，该 scope 的新调用立即优先使用数据库配置。

不自动导入能避免在用户未设置加密主密钥时把明文环境密钥持久化，也避免启动过程产生隐式数据库写入。

## Risks / Trade-offs

- [主密钥丢失会导致数据库密钥不可恢复] → 部署文档要求将主密钥纳入密钥备份；缺失或错误时明确失败，不覆盖原密文。
- [Profile 内容更新会改变现有房间的实际模型行为] → 这是支持密钥轮换的有意选择；`agent_runs` 记录实际 Profile 和模型名以便追踪。
- [连接测试会产生一次真实模型请求和少量费用] → UI 明确提示，并发送最小请求、设置短超时。
- [动态创建 Go 客户端增加少量开销] → 首版优先正确性；若后续观察到明显开销，可按 Profile ID 与 `updated_at` 做无密钥日志的缓存。
- [被历史房间引用的 Profile 长期无法删除] → 支持停用并隐藏于新绑定选择；保留记录以满足历史快照和审计完整性。
- [DeepAgent Executor 能够读取临时模型密钥] → 这是执行模型调用的必要权限；限制凭据只存在于单次执行上下文，不把密钥写入 argv、文件、日志或事件。
- [兼容供应商对 Chat Completions 细节支持不同] → 首版仅依赖 `model`、`messages` 和 Bearer 鉴权核心，保存前可执行真实连接测试。

## Migration Plan

1. 新增可回滚迁移：创建 `model_profiles`，为 `agents`、`room_agents` 和 `agent_runs` 增加可空模型字段；旧数据保持空值。
2. 部署时配置 `MODEL_CONFIG_ENCRYPTION_KEY`。未配置时服务仍可使用旧环境变量，但拒绝保存数据库 API Key。
3. 部署后端 Resolver、管理 API、动态 Go 客户端和 DeepAgent 单次执行注入；local 模式保留环境映射，gRPC 模式使用请求上下文，同时保留现有环境兜底。
4. 部署模型配置页和扩展后的 Agent 配置页。
5. 管理员通过页面创建并测试 Go 与 DeepAgent Profile，分别设置默认值，再按需为 Agent 绑定专用 Profile。
6. 确认数据库配置稳定后，运维可从旧 `.env` 中移除重复模型凭据；本变更不自动删除旧变量。

回滚时，旧后端会忽略新增表和可空列，并继续读取原环境变量；数据库中的 Profile 密文保留，直到确认无需恢复新版本后再由单独迁移删除。

## Open Questions

无。供应商专有参数、独立系统能力模型、外部 Secret Store 和主密钥在线轮换留待后续变更。
