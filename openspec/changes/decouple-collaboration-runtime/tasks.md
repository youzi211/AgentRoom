## 1. 锁定现状与迁移基线

- [ ] 1.1 为 `mention_fanout` 的单 mention、多 mention、Agent-to-Agent mention、轮次限制和失败路径补齐 Go 黄金测试
- [ ] 1.2 为 `guided_dialogue` 的选角顺序、空输出、重复输出、冷却、停止原因和 `dialogue_runs` 审计补齐 Go 黄金测试
- [ ] 1.3 为普通人类消息无 mention 时不触发 Agent 的当前行为建立显式基线测试
- [ ] 1.4 为人类消息先持久化和广播、再异步调度 Agent/Focus 的顺序补齐回归测试
- [ ] 1.5 为 Agent 成功消息与 `agent_run` 终态事务提交、提交失败不广播补齐回归测试
- [ ] 1.6 记录当前 Go/Python Runtime 的取消、超时、停机、artifact 和敏感字段扫描基线

## 2. 建立协作策略与持久化模型

- [ ] 2.1 定义框架中立的 `CollaborationPolicy`、engine、trigger mode、停止原因和默认值，并增加领域验证测试
- [ ] 2.2 实现旧 `DialoguePolicy` 到新协作策略的兼容映射，保证旧房间默认为 `native + mention_only`
- [ ] 2.3 新增 `collaboration_runs` GORM 模型、Store 类型和状态转换，覆盖唯一终态与 interrupted 启动治理
- [ ] 2.4 为 `agent_runs` 增加可空 collaboration run ID、turn index 和 parent message ID，并更新转换与测试 Store
- [ ] 2.5 实现 collaboration run 创建、完成、失败、取消和幂等终态的 MySQL Repository 与事务测试
- [ ] 2.6 更新建房快照、冷加载和房间 DTO，使协作策略在重启后保持一致
- [ ] 2.7 保留旧 `dialogue_runs` 历史读取，增加新旧审计结果兼容查询测试

## 3. 定义 Collaboration Runtime Protobuf 合约

- [ ] 3.1 在版本化 proto 包中定义房间、Agent、触发消息、transcript、知识、模型引用、策略和执行限制快照
- [ ] 3.2 定义 `CollaborationRuntimeService.ExecuteConversation` 服务端流式 RPC 与请求校验字段
- [ ] 3.3 定义 accepted、选角、turn、模型、工具、artifact、handoff、消息完成、checkpoint 和唯一终态事件
- [ ] 3.4 定义稳定的停止/错误分类、事件序号、collaboration run ID、turn ID 和 Agent ID 关联规则
- [ ] 3.5 定义 opaque checkpoint 的 engine/version/format/hash/size 元数据和资源限制
- [ ] 3.6 生成并提交 Go/Python Protobuf 代码，更新生成脚本与生成物一致性检查
- [ ] 3.7 增加 Go/Python 跨语言 JSON/golden 契约测试，覆盖未知字段、版本拒绝、乱序、重复终态和敏感字段
- [ ] 3.8 增加 deadline、取消、请求/事件/artifact/checkpoint 大小限制和 gRPC 状态映射测试

## 4. 建立 Python Collaboration Runtime 基础设施

- [ ] 4.1 创建框架中立的 collaboration 请求、事件、Engine 接口和 `CollaborationEngineRegistry`
- [ ] 4.2 实现独立的 collaboration run 上下文、活动注册表、取消句柄和 run ID 命名空间
- [ ] 4.3 实现 Collaboration Runtime Servicer 的请求验证、事件顺序、唯一终态和异常脱敏
- [ ] 4.4 实现全局协作容量、有界等待队列、每房间互斥和取消时容量释放
- [ ] 4.5 在现有 Python gRPC Server 注册 Collaboration Runtime 与独立健康服务名
- [ ] 4.6 扩展优雅关闭，使 Agent Runtime 与 Collaboration Runtime 分别退出就绪并清理活动调用
- [ ] 4.7 增加协作运行结构化日志和指标，禁止记录 Prompt、凭据、完整 Provider 响应和框架内部状态
- [ ] 4.8 增加 Engine 未注册、服务未就绪、容量耗尽、重复 run ID、重复 room run 和关闭竞态测试

## 5. 实现 Native Collaboration Engine

- [ ] 5.1 在 Python 中实现不依赖 AutoGen 的 Native Engine 骨架，并通过共享 Engine 契约测试
- [ ] 5.2 抽取可被 Collaboration Engine 进程内调用的普通 LLM 与 DeepAgent Executor 接口，禁止 localhost gRPC 回环
- [ ] 5.3 复现显式 mention 的排序、去重、资格校验和多 Agent 初始候选行为
- [ ] 5.4 复现 Agent-to-Agent handoff、自我跟进和每 Agent/总轮数限制
- [ ] 5.5 复现 guided dialogue 的冷却、空输出、重复输出和稳定停止原因
- [ ] 5.6 为 automatic 且无 mention 的模式实现受控默认首发策略，并保证不会默认触发全部 Agent
- [ ] 5.7 映射 Executor 的模型、工具、artifact、知识来源和最终消息事件到中立 Collaboration 事件
- [ ] 5.8 使用黄金场景对比 Native Engine 与旧 Go 路径的发言顺序、消息内容边界、审计和终态

## 6. 建立 Go Collaboration Coordinator 与远程客户端

- [ ] 6.1 定义 Go `CollaborationRuntime` 端口、中立请求/事件类型和测试 Fake
- [ ] 6.2 实现 gRPC Collaboration Runtime Client、TLS、消息大小、deadline、健康检查和连接生命周期
- [ ] 6.3 实现不可变请求快照构建器，包含房间 Agent、mention 候选、transcript、知识、策略和模型引用
- [ ] 6.4 实现事件验证器，检查 run ID、递增序号、turn/Agent 资格、状态转换和唯一终态
- [ ] 6.5 实现 `CollaborationCoordinator` 的每房间活动映射、全局有界调度和新消息抢占
- [ ] 6.6 为每个远程 turn 创建 Agent run，并幂等事务提交最终消息、模型审计、artifact 与 turn 终态
- [ ] 6.7 实现 collaboration run 成功、停止、失败、取消、超时和中断收敛
- [ ] 6.8 在房间关闭、归档、后端停机和 deadline 到达时统一取消活动 collaboration run
- [ ] 6.9 验证提交失败会立即取消远程流并丢弃后续候选事件，不产生未持久化聊天消息
- [ ] 6.10 在组合根中以显式配置装配旧 Go 路径、本地兼容路径或远程 Collaboration Runtime，禁止不确定错误自动换引擎重做

## 7. 统一房间消息触发与实时事件

- [ ] 7.1 将已持久化人类消息的 Agent 调度入口改为统一 Collaboration Coordinator，并保持消息立即广播
- [ ] 7.2 实现 `mention_only` 与 `automatic` 触发模式，保证两者使用同一 collaboration run 主链
- [ ] 7.3 将显式 mention 转换为优先选角信号，而不是选择 fanout/guided 两条独立代码路径
- [ ] 7.4 定义 collaboration started、speaker selected、turn、handoff、cancelled 和 terminal 实时事件
- [ ] 7.5 映射非敏感远程活动到 WebSocket，禁止内部控制消息和 selector Prompt 进入聊天历史
- [ ] 7.6 增加同一房间快速连续人类消息的取消/重启顺序和迟到事件丢弃测试
- [ ] 7.7 增加不同房间并发、队列满、Runtime 不可用时人类消息仍可交付的服务测试

## 8. 更新 API 与前端协作体验

- [ ] 8.1 扩展建房与房间查询 API，暴露受支持的协作 engine、trigger mode 和策略字段
- [ ] 8.2 增加管理员只读的 Collaboration Runtime 能力与就绪状态接口，数据来源不得由前端硬编码
- [ ] 8.3 更新建房/管理界面，支持选择兼容模式或自动协作及允许的灰度引擎
- [ ] 8.4 更新房间前端状态合并，展示协作运行、当前发言者、handoff、取消和终态活动
- [ ] 8.5 确保框架活动不进入普通消息列表，刷新后以 REST 历史和运行审计重新同步
- [ ] 8.6 增加前端 helper 与 Node 测试，覆盖策略 payload、未知事件、迟到事件和权限边界

## 9. 建立框架中立模型端口

- [ ] 9.1 定义供 selector 和参与 Agent 使用的 `CollaborationModelClient` 接口及 Fake 实现
- [ ] 9.2 对接 `unify-model-gateway` 提供的进程内 Model Gateway Core 或等价中立端口
- [ ] 9.3 统一 selector/Agent 调用的 Profile、deadline、取消、usage、能力检查和错误分类
- [ ] 9.4 增加生产依赖扫描，禁止 Collaboration Engine 直接构造 Provider SDK 客户端
- [ ] 9.5 在 Model Gateway 不可用时使 AutoGen Engine 不报告生产就绪，同时保持 Native/单 Agent 路径可用

## 10. 实现隔离的 AutoGen Collaboration Engine

- [ ] 10.1 固定经验证的 Microsoft AutoGen 版本，并将依赖和 import 限制在独立 AutoGen Engine 模块
- [ ] 10.2 实现 AgentRoom Agent ID/名称到 AutoGen Participant 的稳定无冲突映射
- [ ] 10.3 实现普通 LLM 与 DeepAgent Participant Adapter，复用现有 Executor、artifact 和工具事件
- [ ] 10.4 构造受约束的 AutoGen Team 与终止条件，映射总轮数、单 Agent 轮数、空/重复输出和取消
- [ ] 10.5 实现显式 mention 的确定性首发选择，避免额外 selector 调用
- [ ] 10.6 实现无 mention 时从合格 Agent 集合选择单一首发者，并记录非敏感选择活动和 usage
- [ ] 10.7 实现 handoff 目标验证和后续 turn 选择，拒绝房间快照外或超限 Agent
- [ ] 10.8 将 AutoGen participant、manager、tool、handoff、termination 和异常事件映射到中立事件
- [ ] 10.9 实现可选 checkpoint 的 dump/load、版本/hash 校验、大小限制和不兼容重建
- [ ] 10.10 增加 AutoGen Engine 契约测试，覆盖选择、交接、取消、终止、状态恢复、内部消息隔离和敏感字段

## 11. 灰度、回滚与架构清理

- [ ] 11.1 增加 Engine allowlist、AutoGen 开关、协作容量、等待队列、checkpoint 和默认策略配置
- [ ] 11.2 先以 Fake 模型运行 Native/AutoGen 影子对比，记录选角差异、事件差异和停止原因
- [ ] 11.3 在隔离环境运行真实模型评估，记录首响应延迟、总延迟、额外 selector token、费用和成功率
- [ ] 11.4 以房间 allowlist 灰度远程 Native Engine，再灰度 `autogen + automatic`
- [ ] 11.5 验证 AutoGen、远程 Collaboration Runtime 和 Python 服务三级显式回滚均不会重做已开始的 run
- [ ] 11.6 停止为新协作写入旧 `dialogue_runs`，保留历史读取和兼容查询
- [ ] 11.7 在观察期结束后删除或隔离 Go 中重复的 fanout/guided 主链，仅保留显式开发回滚条件

## 12. 文档、安全与交付验证

- [ ] 12.1 更新根 README、`.env.example`、DeepAgent README、Compose 和 Agent Runtime 部署说明
- [ ] 12.2 更新架构文档，说明 Go 控制面、Collaboration Runtime、Agent Runtime、Model Gateway 和 MySQL 所有权
- [ ] 12.3 记录 AutoGen 固定版本、升级流程、checkpoint 兼容、灰度指标和回滚手册
- [ ] 12.4 运行凭据、Prompt、Provider 响应、AutoGen 状态、日志、事件和 artifact 敏感信息扫描
- [ ] 12.5 运行 Python Engine/Runtime/AutoGen 单元与集成测试
- [ ] 12.6 运行 `go -C backend test ./...`、`go -C backend vet ./...` 和 `go -C backend build ./cmd/server`
- [ ] 12.7 运行前端相关 Node 测试和 `npm --prefix frontend run build`
- [ ] 12.8 运行 Go/Python gRPC、真实 MySQL、取消竞态、停机和端到端房间协作测试
- [ ] 12.9 运行生产依赖扫描、`git diff --check` 和 `openspec validate decouple-collaboration-runtime --strict`
