# Agent Runtime 与模型架构

## 当前远程执行拓扑

`AGENT_RUNTIME_TRANSPORT=grpc` 时，Go 是控制面，长驻 Python 服务是单 Turn 执行面：

```text
Go Runner
  -> 选择 Agent / 执行 Mention 与 Guided Dialogue 策略
  -> 创建 agent_run，检索历史和知识，解析并解密 Model Profile
  -> RemotePythonRuntime.ExecuteAgent(不可变快照)
       -> Python Executor Registry
            |-- LLMExecutor
            `-- DeepAgentExecutor -> Tavily / report.md
  <- accepted / model / tool / progress / artifact / terminal events
  -> 事务提交最终消息、artifact、模型审计和 succeeded 终态
  -> WebSocket Activity / 最终消息广播
```

Python 不读取 AgentRoom MySQL，不选择其他房间 Agent，也不持久化业务终态。每个请求使用独立 RunContext、模型客户端和工作目录；API Key 只存在于受保护的 gRPC 请求与该上下文内存，结束时清理。Go 保持长生命周期 ClientConn，并用同一 Context 传播 deadline、房间关闭、归档和停机取消。

Runtime Registry 的当前映射：

| Agent Runtime | `local` | `grpc` | Model scope |
| --- | --- | --- | --- |
| `llm` | `LLMAgentRuntime` | Python `LLMExecutor` | `go` |
| `deepagent` | `DeepAgentRuntime` 子进程回退 | Python `DeepAgentExecutor` | `deepagent` |

远程流发生 `UNAVAILABLE`、连接重置或协议错误时不会自动调用 local Runtime 重做同一 `run_id`。Python 总容量和 DeepAgent 专属容量负责执行限流；超过内联限制的 artifact 明确失败，不截断。旧 Go LLM Agent Runtime、DeepAgent 子进程 Adapter 和 `DEEPAGENT_*` 配置仅在稳定观察期结束前保留用于显式回滚。

主要代码：

- `proto/agent_runtime/v1/agent_runtime.proto`
- `backend/internal/agent/runtime_remote.go`
- `deepagent/src/agent_runtime/service.py`
- `deepagent/src/agent_runtime/executors/llm.py`
- `deepagent/src/agent_runtime/executors/deepagent.py`
- `docs/architecture/agent-runtime-rollout.md`

[返回架构索引](README.md)

本文描述 Agent 触发、对话编排、Go/DeepAgent Runtime、模型 Profile 和密钥边界。这里必须区分两个名称空间：

- **Agent Runtime**：`llm` 或 `deepagent`；
- **Model Profile scope**：`go` 或 `deepagent`。

`llm` Agent 使用 `go` scope Profile；`deepagent` Agent 使用 `deepagent` scope Profile。

## 1. Agent 配置与房间快照

全局 `agents` 表保存可编辑 Agent 配置，包括：

- 名称与 mention；
- 角色、描述和 system prompt；
- enabled 状态和排序；
- Agent Runtime；
- 可空的显式 `model_profile_id`。

创建房间时，`AgentService.ResolveForRoom` 选择启用 Agent，并生成写入 `room_agents` 的副本：

```text
global Agent
  |
  +-- explicit model_profile_id exists
  |      `-- validate compatible scope and snapshot that ID
  |
  `-- no explicit model_profile_id
         `-- query current DB default for the Agent runtime
                `-- snapshot concrete ID when available
```

房间固定 Agent Prompt、Runtime 和 Profile ID。之后全局 Agent 改名、改 Prompt、改 Runtime 或改绑 Profile，都只影响新房间。

关键代码：

- `backend/internal/service/agent_service.go`
- `backend/internal/model/types.go`
- `backend/internal/store/mysql/rooms_repo.go`
- `backend/internal/store/mysql/models.go`

## 2. 触发与 mention 规则

`agent.Runner.HandleHumanMessage` 是入口。默认原则是：人类消息没有显式 mention 时，不触发 Agent。

Mention 检测会：

- 兼容半角 `@` 和全角 `＠`；
- 忽略 `@` 后的空白；
- 使用大小写无关匹配；
- 按 mention 在文本中的出现顺序排序；
- 按 Agent ID 去重。

当前实现基于规范化后的子串搜索，不是带严格词边界的 parser。修改 mention 规则时必须验证相似 Agent 名称和中英文输入。

关键代码：

- `backend/internal/agent/mention.go`
- `backend/internal/agent/runner.go`

## 3. Dialogue Policy

每个房间持有经过 `WithDefaults()` 规范化的 `DialoguePolicy`。Runner 根据 Policy 分为两条路径。

### 3.1 `mention_fanout`

```text
human message
  -> resolve explicitly mentioned Agents
  -> call each Agent in mention order
  -> inspect Agent reply for explicit handoff mentions
  -> enqueue eligible follow-up Agents
  -> stop at per-Agent and autonomous-turn limits
```

特点：

- 初始被提及的多个 Agent 串行回复；
- 可选允许 Agent-to-Agent mention；
- self follow-up 和每 Agent 次数受限制；
- `MaxAutonomousTurns` 只统计由 Agent 消息触发的后续回复；
- 不创建 `dialogue_runs`，但每次调用都有 `agent_runs`。

实现位于 `backend/internal/agent/runner.go` 的 fan-out 路径。

### 3.2 `guided_dialogue`

```text
human trigger
  -> create dialogue_run
  -> seed pending speakers from mentions
  -> choose next eligible speaker
  -> create one agent_run
  -> execute Runtime
  -> reject empty or duplicate dialogue output
  -> persist message with dialogueRunID / turnIndex / parentMessageID
  -> detect explicit handoff mentions
  -> repeat within bounded policy
  -> finish dialogue_run with final stop status
```

控制项包括：

- `MaxAutonomousTurns`
- `MaxTurnsPerAgent`
- `AllowAgentToAgentMentions`
- `AllowSelfFollowup`
- `CooldownMs`
- 重复内容和空回复停止条件

Guided Dialogue 创建整段 `dialogue_runs` 审计，每一轮仍创建单独 `agent_runs`。

关键代码：

- `backend/internal/model/dialogue.go`
- `backend/internal/agent/dialogue.go`
- `backend/internal/agent/prompt_context.go`
- `backend/internal/agent/prompt_composer.go`

当前 `ResponseStrategy` 会进入 Prompt Context，但实际 speaker 选择仍主要由 pending 顺序和 eligibility 决定；不要假设它已经对应多种调度算法。

## 4. Runner 执行链

单次 Agent turn 的主要顺序：

```text
select Agent
  -> create agent_run(status=running)
  -> broadcast agent_activity(started)
  -> retrieve room + Agent knowledge
  -> read recent room messages
  -> compose PromptContext
  -> RuntimeRegistry.Resolve(agent.Runtime)
  -> Runtime.Respond
  -> update actual model audit
  -> success: persist/broadcast Agent message
     failure: persist/broadcast sanitized System message
  -> finish agent_run
  -> broadcast agent_activity(finished)
```

Runner 会清理模型输出中的 `<think>` / `<thinking>` 内容，再持久化可见回复。

主要文件：

- `backend/internal/agent/runner.go`
- `backend/internal/agent/prompt_context.go`
- `backend/internal/agent/prompt_composer.go`
- `backend/internal/agent/sanitize.go`

### 4.1 知识上下文

Runner 检索：

- 房间 scope 知识：所有房间 Agent 可用；
- Agent scope 知识：仅对应 Agent 可用。

检索结果进入 Prompt，并被转换为确定性的 `KnowledgeSources` 消息元数据；来源不依赖模型自行声明。

### 4.2 运行审计

`agent_runs` 记录：

- room、Agent 和 trigger message；
- running / succeeded / failed / timeout；
- 实际 Profile ID；
- 配置来源 `database` 或 `environment`；
- 实际模型名；
- 清洗后的错误和完成时间。

它不记录 API Key。常规 `agent_activity` WebSocket 事件也不包含 Base URL、Profile ID、模型名或密钥。

## 5. Runtime Registry

`AgentRuntime` 接口接收一次执行所需的 Agent、触发消息和 Prompt Context，返回：

- 可见回复内容；
- 可选 artifact；
- 实际模型 metadata。

Runtime Registry 当前注册：

| Agent Runtime | 实现 | Model scope |
| --- | --- | --- |
| `llm` | `LLMAgentRuntime` | `go` |
| `deepagent` | `DeepAgentRuntime` | `deepagent` |

未知或未注册 Runtime 返回 `ErrRuntimeNotConfigured`，由 Runner 转成房间 System 消息。

关键代码：`backend/internal/agent/runtime.go`。

## 6. Go LLM Runtime

执行流程：

```text
PromptContext
  -> compose chat messages
  -> create timeout context
  -> ModelResolver.Resolve(scope=go, roomAgent.ModelProfileID)
  -> llm.NewClient(resolved BaseURL, ModelName, APIKey)
  -> OpenAI-compatible Chat Completions
  -> StripThinkBlocks
  -> Runtime response + model metadata
```

每次调用重新查询数据库 Profile 并构造 Client，不长期缓存解密后的 API Key。

主要代码：

- `backend/internal/agent/runtime_llm.go`
- `backend/internal/llm/client.go`

### 6.1 系统级 Go 模型调用

Focus 提取和会议纪要不属于某个房间 Agent，因此通过 `llm.ResolvingClient` 每次解析当前 `go` scope 默认 Profile；没有数据库默认时才走 Go 环境迁移兜底。

这意味着切换 Go 默认 Profile 会立即影响后续 Focus 和 Minutes 调用，但不会改变已快照具体 Profile ID 的房间 Agent。

## 7. DeepAgent Runtime

DeepAgent 是 backend 进程按次启动的 Python 子进程，而非长期服务。

```text
Go DeepAgentRuntime
  -> resolve deepagent Profile
  -> acquire concurrency semaphore
  -> exec:
       uv run deepagent-research
         --config <config>
         --run-id <agentRunID>
         -- <question>
  -> child writes runs/<runID>/report.md and events.jsonl
  -> Go reads report.md
  -> attach report as MessageArtifact
  -> persist artifact content in messages.artifacts_json
```

关键代码：

- `backend/internal/agent/runtime_deepagent.go`
- `deepagent/src/agentroom_deepagent/cli.py`
- `deepagent/src/agentroom_deepagent/research.py`
- `deepagent/src/agentroom_deepagent/report.py`

### 7.1 并发和超时

- `DEEPAGENT_CONCURRENCY` 控制每个 backend 进程的子进程并发；
- 默认值为 1；
- 总超时包含等待 semaphore 的时间；
- `exec.CommandContext` 在超时或取消时终止子进程。

### 7.2 Artifact 边界

原始报告同时存在：

1. `deepagent/runs/<runID>/report.md` 或容器 volume；
2. MySQL `messages.artifacts_json` 中的正文副本。

WebSocket 消息不直接发送 artifact 正文；浏览器通过下载 API 获取持久化内容。

## 8. Model Profile 控制面

`model_profiles` 保存运行时模型连接配置：

- name；
- runtime scope：`go` / `deepagent`；
- protocol（当前为 OpenAI-compatible）；
- API Base URL；
- model name；
- API Key 密文和 hint；
- enabled / default；
- timestamps。

管理 API 支持：

- 列表、创建、更新、删除；
- 设置 Runtime 默认；
- 密钥保留、替换和清除；
- 对已保存或草稿配置做最小连接测试。

Profile 被默认槽位、全局 Agent 或 `room_agents` 快照引用时，删除受到约束。

关键代码：

- `backend/internal/model/model_profile.go`
- `backend/internal/service/model_profiles.go`
- `backend/internal/api/model_profile_handlers.go`
- `backend/internal/store/mysql/model_profiles_repo.go`

## 9. 模型解析优先级

`ModelResolver.Resolve(scope, explicitProfileID)` 的行为不是“任何失败都向下回退”，而是：

```text
explicitProfileID present?
  |
  +-- yes -> load exact DB Profile
  |          |-- missing       -> hard failure
  |          |-- disabled      -> hard failure
  |          |-- wrong scope   -> hard failure
  |          |-- decrypt error -> hard failure
  |          `-- valid         -> source=database
  |
  `-- no -> load DB default for scope
             |-- query error    -> hard failure
             |-- invalid row    -> hard failure
             |-- valid          -> source=database
             `-- not found      -> environment fallback
                                      |-- complete -> source=environment
                                      `-- incomplete -> not configured
```

环境映射：

| Scope | Base URL | Model | API Key |
| --- | --- | --- | --- |
| `go` | `LLM_BASE_URL` | `LLM_MODEL` | `LLM_API_KEY` |
| `deepagent` | `MODEL_BASE_URL` | `MODEL_NAME` | `MODEL_API_KEY` |

环境兜底在 backend 启动时读入 Resolver map；数据库 Profile 则按每次调用动态读取。

### 9.1 快照与轮换语义

```text
room_agents.model_profile_id = stable selection
model_profiles fields         = dynamically read connection content
```

因此：

- 修改全局 Agent 绑定：旧房间不变；
- 切换 Runtime 默认值：有具体快照 ID 的旧房间不变；
- 更新同一 Profile 的 Base URL、模型名或 API Key：引用该 ID 的旧房间下一次调用使用新内容；
- 显式 Profile 失效：旧房间报错，不静默切换到其他凭据。

## 10. API Key 加密和响应边界

Profile API Key 由 `SecretCipher` 使用：

- AES-256-GCM；
- base64 编码的 32 字节 `MODEL_CONFIG_ENCRYPTION_KEY`；
- 每次随机 nonce；
- Profile ID 作为附加认证数据（AAD）；
- 版本化密文格式。

MySQL 保存密文和不可逆 hint。管理响应只返回 `hasAPIKey` 和 hint，不返回明文或可识别密文。主密钥丢失或误换后，已有密文不能恢复，必须为受影响 Profile 重新录入 API Key。

主密钥必须：

- 不进入数据库；
- 由部署 Secret Store 管理；
- 跨重启、升级、恢复和 backend 副本保持一致；
- 纳入加密备份和灾难恢复流程。

## 11. DeepAgent 的实际信任边界

设计目标是仅将本次解析出的 `MODEL_PROTOCOL`、`MODEL_BASE_URL`、`MODEL_NAME` 和 `MODEL_API_KEY` 追加给子进程，并且不把模型密钥写入 argv、TOML、报告或事件。

但当前 `DeepAgentRuntime` 以 `os.Environ()` 构造子进程环境，再追加运行配置。这意味着 Python 子进程实际上还会继承 backend 的其他环境变量，例如数据库 DSN、管理员密钥、模型主密钥和 Tavily key。

因此当前安全模型必须视为：

> DeepAgent Python 代码及其依赖与 Go backend 处于相同高权限信任域；它不是凭据隔离的沙箱。

后续如需降低信任，应改为显式环境 allowlist，并进一步考虑独立容器、低权限网络策略和任务协议。

## 12. Provider 与 SSRF 边界

Profile Base URL 是管理员输入。后端会在连接测试和真实调用时主动访问该 URL，并可能携带保存的 Provider API Key。因此模型 Profile 管理权限同时隐含：

- 服务端出站网络访问能力；
- 修改目标后向新目标发送 Provider key 的能力；
- 选择 HTTP 时明文传输 Provider key 的能力。

当前这依赖管理员完全可信。若管理员边界扩大或转为公网产品，需要增加 HTTPS 策略、私网地址限制、DNS/IP 校验和出站网络控制。

## 13. 修改检查表

修改 Agent 或模型链路时至少检查：

- Agent Runtime 名称与 Profile scope 是否正确映射；
- 建房快照是否保存具体 Profile ID；
- 显式 Profile 错误是否仍禁止回退；
- API 响应、日志、错误、artifact、argv 是否包含密钥；
- `agent_runs` 是否记录实际 Profile/source/model；
- Go 与 DeepAgent 路径是否都有测试；
- Focus/Minutes 是否仍使用当前 Go 默认 Profile；
- `backend/internal/tests/teststore` 是否支持新增审计字段；
- `.env.example`、README 和 DeepAgent README 是否同步。

## 14. 相关架构

- [后端架构](backend.md)
- [数据、实时与会议生命周期](data-realtime-lifecycle.md)
- [前端与部署](frontend-and-deployment.md)
