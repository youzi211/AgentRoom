## 1. 基线与变更协调

- [x] 1.1 运行并记录当前 `go -C backend test ./...`、`go -C backend vet ./...`、`go -C backend build ./cmd/server`、Python `pytest` 和前端构建基线，保存失败项与环境前提。
- [x] 1.2 为 Mention Fanout、Guided Dialogue、普通 LLM、DeepAgent artifact、超时和取消建立现状行为清单，并把对应现有测试映射到新 Runtime 验收项。
- [x] 1.3 协调活动变更 `add-model-profile-management`，把 DeepAgent“单次子进程环境注入”更新为“单次 gRPC 执行上下文注入”，保持 Profile 解析优先级、审计和密钥不落盘要求不变。
- [x] 1.4 确认 `deepagent/` 在第一阶段保留目录名并新增常驻服务包，记录最终重命名为 `agent-runtime/` 的退出条件，避免协议迁移同时进行无关路径重构。

## 2. Protobuf 合约与生成流程

- [x] 2.1 在 `proto/agent_runtime/v1/` 创建版本化 Protobuf 包，定义 `AgentRuntimeService.ExecuteAgent` 服务端流式 RPC 和 Go/Python 包映射。
- [x] 2.2 定义执行请求中的 `run_id`、trace、Executor 类型、Room/Agent 快照、Trigger、最近消息、知识分片、模型连接和执行限制，并明确所有敏感与非敏感字段。
- [x] 2.3 使用 `oneof` 定义 accepted、模型、工具、output delta、artifact、completed 和 failed 事件，加入单调序号和终态错误码。
- [x] 2.4 定义字段编号保留、未知字段兼容、协议版本拒绝、请求/事件/artifact 大小限制和规范 gRPC Status 映射规则。
- [x] 2.5 为 Go 和 Python 增加固定版本的 gRPC/Protobuf 依赖及生成命令，并提交双方生成代码。
- [x] 2.6 增加生成代码一致性检查，验证重新生成后工作树无差异且已删除字段编号不会被复用。
- [x] 2.7 增加 Go/Python 跨语言序列化契约测试，覆盖完整请求、全部事件类型、未知可选字段和不支持协议版本。

## 3. Python 常驻服务骨架

- [x] 3.1 在现有 Python 项目中建立 `agent_runtime` 服务包、配置加载、gRPC 异步 Server 和独立可执行入口，确保导入服务模块不会启动网络监听。
- [x] 3.2 实现标准 `grpc.health.v1.Health`，启动未完成时报告非服务状态，Executor Registry 就绪后报告 `SERVING`。
- [x] 3.3 实现 `Executor` 接口与 Registry，按稳定 `executor_kind` 路由请求，并在未知类型时于调用模型前返回明确错误。
- [x] 3.4 实现每 `run_id` 独立的 `RunContext` 和活动运行注册表，隔离模型配置、临时目录、工具状态、事件序号和取消信号。
- [x] 3.5 实现重复 `run_id` 防护，验证同一运行活动期间第二个请求不会启动第二次 Executor。
- [x] 3.6 实现统一 Event Writer，保证 accepted 首发、序号单调、只允许一个 completed/failed 终态且终态后拒绝发送事件。
- [x] 3.7 实现总并发信号量、DeepAgent 专属信号量和有界等待队列，确保等待期间响应 deadline 与取消。
- [x] 3.8 实现事件输出背压和可合并进度策略，确保 accepted、artifact、completed、failed 等关键事件不被丢弃。
- [x] 3.9 实现请求、事件和内联 artifact 的显式字节限制，超限时返回 `RESOURCE_EXHAUSTED` 而不截断内容。
- [x] 3.10 实现 Fake Executor，能够确定性产生成功、应用失败、工具事件、artifact、延迟、乱序测试输入和取消响应。
- [x] 3.11 增加 Python 单元测试，覆盖服务启动、健康状态、Registry、重复 run、事件顺序、唯一终态、容量、背压、超限、deadline 和取消。
- [x] 3.12 实现 SIGTERM/SIGINT 优雅停机：先切换健康状态、拒绝新请求、等待宽限期、取消剩余 Run 并关闭资源，并用测试验证顺序。

## 4. Go gRPC Client 与 Runtime 边界

- [x] 4.1 为后端增加 Agent Runtime 传输配置，支持显式 `local|grpc`、地址、超时、消息限制、不安全开发模式和 TLS 文件校验，迁移期默认保持 `local`。
- [x] 4.2 扩展内部 `AgentRuntime` 接口以接收 `AgentEventObserver`，让现有本地 Runtime 发出最小生命周期事件并保持现有最终响应行为。
- [x] 4.3 实现长生命周期 gRPC ClientConn 的创建、复用和关闭，禁止为每个 Agent turn 重复 Dial。
- [x] 4.4 实现 `RemotePythonRuntime`，把内部 `AgentRuntimeRequest` 映射为 Protobuf 请求并持续读取服务端事件直到终态或传输结束。
- [x] 4.5 实现 Go 侧事件校验，拒绝错误 `run_id`、序号倒退、重复 accepted、重复终态和终态后事件。
- [x] 4.6 实现 gRPC Status 到稳定内部错误的映射，区分参数、认证、容量、不可用、取消、超时、协议和已接受后的应用失败。
- [x] 4.7 实现活动 `run_id -> cancel` 注册表，并从 deadline、房间关闭、归档、后端停机和显式取消路径结束对应 gRPC 调用。
- [x] 4.8 将模型、工具和进度事件映射为脱敏 `realtime.Activity`，确认非终态事件不会创建普通聊天消息。
- [x] 4.9 为普通 LLM 和 DeepAgent 配置独立默认 deadline，并验证 Python 不能把执行延长到 Go Context deadline 之后。
- [x] 4.10 增加 Go 单元测试，使用 Fake gRPC Server 覆盖成功流、失败终态、EOF 无终态、错误 run、乱序、重复终态、取消、超时、容量和连接中断。

## 5. Run 持久化与幂等终态

- [x] 5.1 选择并记录最终消息与 Agent Run 的数据库关联方案，优先使用可空 `messages.agent_run_id` 唯一约束或等价 `agent_runs.result_message_id` 约束。
- [x] 5.2 增加 GORM 模型、参考 SQL 和受控迁移，使旧消息保持兼容并确保同一 Agent Run 不能关联两条最终消息。
- [x] 5.3 扩展领域 Message/Store 契约，携带 Agent Run 关联但不向不需要的公开 API 泄露内部字段。
- [x] 5.4 实现事务式 `CommitAgentRunSuccess`，在同一 MySQL 事务中插入最终消息、保存非敏感模型审计并条件更新 Run 为 succeeded。
- [x] 5.5 实现失败、取消、超时和中断的条件终态更新，确保只有一个非终态到终态转换能够成功。
- [x] 5.6 调整 Runner，仅在成功事务提交后把 Agent 消息追加到 Room 并广播；提交失败时广播系统错误而不展示不可恢复成功消息。
- [x] 5.7 实现后端启动时遗留 `running` Run 收敛，将没有本地活动调用的旧记录标记为 interrupted/failed，且不自动重新调用 Python。
- [x] 5.8 增加 Repository 和 Service 测试，覆盖重复成功、完成与取消竞争、事务回滚、旧消息兼容和遗留 Run 收敛。

## 6. Python 普通 LLM Agent 迁移

- [x] 6.1 把现有 Go Prompt Composer 行为整理为跨语言黄金样例，覆盖系统 Prompt、Agent Prompt、Trigger、最近消息和知识来源。
- [x] 6.2 在 Python 实现结构化 Prompt Composer，并用黄金样例验证迁移前后关键上下文和安全边界等价。
- [x] 6.3 在 Python 实现 OpenAI-compatible 模型 Adapter，按单次请求创建模型客户端并支持 deadline、取消和稳定错误分类。
- [x] 6.4 实现普通 LLM Executor，把模型生命周期、output delta、最终内容、usage 和安全模型元数据转换为统一 AgentEvent。
- [x] 6.5 实现模型响应内容校验和脱敏，确保 Provider 原始响应、完整 Prompt 和 API Key 不进入事件或日志。
- [x] 6.6 实现知识来源回传，使 Go 保存的 Agent 消息继续携带实际使用的 room/agent 文档来源。
- [x] 6.7 增加 Python 测试，覆盖不同 Profile 并发隔离、认证失败、限流、Provider 超时、客户端取消、输出为空和凭据清理。
- [x] 6.8 增加 Go 到 Python 的普通 LLM 集成测试，验证 Model Resolver 优先级、运行审计、最终消息和 WebSocket Activity。
- [x] 6.9 在集成环境显式将普通 LLM Agent 切换到 `grpc`，对比本地 Runtime 的成功、失败、延迟和上下文结果，不启用自动回退。

## 7. DeepAgent 迁移

- [x] 7.1 将现有 DeepAgent CLI 的研究核心抽取为可由服务调用的库接口，同时保留 CLI 入口用于迁移期测试。
- [x] 7.2 实现 DeepAgent Executor，使用请求内容字段接收问题，确保以 `--` 开头的文本不能改变服务或 Executor 控制参数。
- [x] 7.3 把 Tavily 搜索、模型调用和研究阶段映射为统一 tool/model/progress 事件，并对外部错误执行脱敏。
- [x] 7.4 把 `report.md` 和必要运行产物适配为统一 Markdown artifact，保持 Mention Fanout 与 Guided Dialogue 的现有消息结构。
- [x] 7.5 将 DeepAgent 并发治理迁移到 Python 专属容量，验证超额等待有界、取消生效且不会启动超限任务。
- [x] 7.6 确保每个 DeepAgent Run 使用独立工作目录和模型配置，运行结束后清理临时凭据且不污染其他并发 Run。
- [x] 7.7 增加 DeepAgent 单元测试，覆盖离线 smoke、Tavily 错误、模型错误、报告超限、取消、超时、并发隔离和优雅停机。
- [x] 7.8 增加端到端测试，验证普通 Mention、Agent-to-Agent mention 和 Guided Dialogue 都能保存 DeepAgent artifact 与运行审计。
- [x] 7.9 在集成环境切换 DeepAgent 到 gRPC，确认 Go 不再为远程模式启动 Python CLI 子进程。

## 8. 安全、健康与可观测性

- [x] 8.1 实现 Python 服务监听边界，默认不监听非受控外部接口；本机 plaintext 必须通过显式不安全配置启用。
- [x] 8.2 为非本机部署实现并测试 TLS 与服务身份校验配置，启动时拒绝缺失或不可读的必需证书材料。
- [x] 8.3 为 Go/Python 日志和拦截器增加敏感字段过滤，禁止记录完整 gRPC 请求体、认证 metadata、API Key、房间口令和管理员密钥。
- [x] 8.4 增加凭据泄漏测试，在成功、Provider 错误、工具错误、取消和异常堆栈中扫描模型 API Key 不得出现。
- [x] 8.5 保持 `/api/health` 为 Go 核心存活检查，并新增依赖就绪检查以分别报告 MySQL 与 Agent Runtime 状态。
- [x] 8.6 实现 Python 活动/等待 Run、成功、失败、取消、超时、排队时间、执行耗时和 gRPC 状态的结构化指标或等价日志。
- [x] 8.7 使用 `run_id`、room、agent、dialogue run 和 trace 标识贯穿 Go/Python 日志，验证同一执行可以跨服务关联且不包含 Prompt 明文。
- [x] 8.8 演练 Python 未启动、运行中崩溃、健康切换、连接重置和慢消费者，确认核心房间/历史/管理 API 可用且新 Agent Run 快速进入可观察失败。

## 9. Docker 与运行配置

- [x] 9.1 新增 Python Agent Runtime Dockerfile/启动命令，使用锁定依赖构建常驻服务并保留 DeepAgent 所需工具和运行目录。
- [x] 9.2 在 `docker-compose.yml` 增加 `agent-runtime` 服务、内部 gRPC 地址、健康检查、资源/并发配置和 artifact volume，默认不映射宿主机端口。
- [x] 9.3 调整 backend 镜像和 Compose 依赖，使迁移期 local 模式仍可回退，最终阶段再移除 backend 内不再需要的 Python/DeepAgent 运行依赖。
- [x] 9.4 更新 `.env.example`，记录 Runtime 传输、gRPC 地址、deadline、容量、消息限制、TLS 和优雅停机配置，并标明开发/生产安全默认值。
- [ ] 9.5 验证四服务 Compose 冷启动、健康等待、正常停机、Python 重启和 backend 重启，不产生永久 running Run 或重复 Agent 消息。

## 10. 灰度、回滚与旧实现退出

- [x] 10.1 实现显式 local/grpc Runtime 选择并验证远程流不确定失败时不会自动调用本地 Runtime 重做同一 `run_id`。
- [x] 10.2 编写灰度清单：只影响新 Run、切换前处理活动 Run、监控失败率/排队/耗时、保留数据库兼容字段和明确回滚负责人。
- [x] 10.3 在开发和集成环境把默认传输切为 grpc，完成普通 LLM 与 DeepAgent 的功能、并发和长任务回归。
- [ ] 10.4 执行回滚演练：停止接收新远程 Run、等待或取消活动 Run、切回 local、验证已完成远程 Run 不会重做。
- [ ] 10.5 达到约定稳定期后，删除 Go Agent LLM Runtime 和 DeepAgent 子进程 Adapter，但保留 Focus/Minutes 仍依赖的 Go LLM 能力。
- [ ] 10.6 删除旧 `DEEPAGENT_COMMAND`、`DEEPAGENT_WORKDIR` 等进程启动配置和 backend 镜像内无用依赖，并验证无生产代码引用。
- [ ] 10.7 在远程普通 LLM 与 DeepAgent 均稳定后，将 `deepagent/` 到 `agent-runtime/` 的顶层目录重命名作为独立机械步骤执行并更新路径引用。

## 11. 文档与最终验证

- [x] 11.1 更新根 README、后端 README、Python Runtime README 和部署说明，覆盖本地启动、TLS、健康、故障排查、灰度和回滚。
- [x] 11.2 更新 `docs/architecture`，把 DeepAgent 子进程拓扑替换为 Go 控制面 + Python gRPC 执行面，并记录状态、密钥、取消和单实例边界。
- [x] 11.3 更新数据与实时文档，说明 Agent 成功消息的事务提交、Run 唯一关联和遗留 running 收敛语义。
- [ ] 11.4 运行 Go 全量测试、vet、build 和 gofmt 检查，确保新 gRPC 依赖与生成代码通过。
- [x] 11.5 运行 Python 全量 pytest、离线 smoke、生成代码检查和并发/取消测试，确保测试不调用真实外部模型或 Tavily，除非显式标记为 live。
- [x] 11.6 运行相关前端 Node 测试和生产构建，确认新增 Activity/就绪展示没有破坏现有房间与管理页面。
- [ ] 11.7 构建全部 Docker 镜像并完成 Compose 端到端测试，验证普通 LLM、DeepAgent、artifact、取消、Python 故障和回滚路径。
- [x] 11.8 运行 OpenSpec 严格校验，确认所有新/修改规格都有测试映射，随后记录实现验证命令与结果。
