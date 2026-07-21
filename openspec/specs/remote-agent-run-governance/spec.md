# remote-agent-run-governance Specification

## Purpose
TBD - created by archiving change extract-python-grpc-agent-runtime. Update Purpose after archive.
## Requirements
### Requirement: Go 保持对话编排和业务状态所有权
Go 控制面 SHALL 继续选择响应 Agent、执行 Mention Fanout 与 Guided Dialogue 策略、限制轮次、创建运行记录、持久化消息并广播实时事件；Python 仅执行 Go 指定的单个 Agent turn。

#### Scenario: Mention Fanout 触发多个 Agent
- **WHEN** 人类消息提及多个可响应 Agent
- **THEN** Go 按现有策略为每个 Agent 分别创建和调用一个远程执行
- **THEN** Python 不自行选择额外房间 Agent

#### Scenario: Agent 回复提及另一个 Agent
- **WHEN** 策略允许 Agent-to-Agent mention 且 Python 返回的回复提及另一个 Agent
- **THEN** Go 在保存首个回复后决定是否调度后续 turn
- **THEN** Python 不直接启动下一位 Agent

### Requirement: Go 构建不可变远程执行快照
Go SHALL 在调用 Python 前解析房间 Agent 快照、触发消息、最近消息、知识分片、执行限制和有效 Model Profile，并 MUST 只发送该次 turn 所需的数据。

#### Scenario: 已有房间 Agent 绑定具体 Profile
- **WHEN** 房间 Agent 快照包含有效 Profile ID
- **THEN** Go 按现有解析优先级解密该 Profile
- **THEN** 请求携带该次调用所需模型配置且不携带主加密密钥

#### Scenario: 知识检索失败
- **WHEN** Go 无法检索知识分片但现有行为允许无知识继续
- **THEN** 请求携带空知识集合并继续执行
- **THEN** Python 不尝试直接查询 AgentRoom 数据库作为补偿

### Requirement: Go 将远程事件映射为审计和实时活动
Go SHALL 验证远程事件的 `run_id`、顺序和类型，并将运行、模型、工具、artifact 和终态事件映射为 Agent Run 审计与 WebSocket Activity；最终 Agent 消息只在成功终态通过 Go 创建。

#### Scenario: 收到工具进度事件
- **WHEN** Go 收到有效的工具开始或完成事件
- **THEN** Go 为对应房间广播不含凭据的 Agent Activity
- **THEN** Go 不把工具事件保存为普通聊天消息

#### Scenario: 收到成功终态
- **WHEN** Go 收到成功终态且该 run 尚未提交结果
- **THEN** Go 保存一条 Agent 消息及其知识来源和 artifacts
- **THEN** Go 把 Agent Run 标记为成功并广播完成活动

#### Scenario: 收到乱序或错误 run_id 的事件
- **WHEN** 流返回不属于当前调用或事件序号倒退的事件
- **THEN** Go 终止该流并把运行标记为协议失败
- **THEN** Go 不保存该事件携带的最终消息

### Requirement: 运行结果按 run_id 幂等提交
Go MUST 确保一个 `run_id` 最多提交一个终态和一条最终 Agent 消息，并 SHALL 忽略或拒绝终态后的重复事件。

#### Scenario: 成功终态被重复处理
- **WHEN** 相同成功终态因重连、重试或内部重复消费再次到达
- **THEN** Go 不创建第二条 Agent 消息
- **THEN** Agent Run 保持首次已提交终态

#### Scenario: 完成与取消竞争
- **WHEN** 成功终态和取消请求并发发生
- **THEN** 只有一个状态转换成功成为最终状态
- **THEN** 数据库和 WebSocket 不展示互相矛盾的两个终态

### Requirement: 活动远程运行可被统一取消
Go SHALL 维护活动 `run_id` 到调用取消函数的受控映射，并 MUST 在房间关闭、服务停机、deadline 到达或显式取消时结束对应 gRPC 调用。

#### Scenario: 房主关闭正在运行的会议
- **WHEN** 房间关闭且仍有活动 Agent turn
- **THEN** Go 取消对应 gRPC 调用
- **THEN** 运行最终被记录为取消或已在竞争中成功完成

#### Scenario: 后端开始优雅停机
- **WHEN** Go 后端进入停机流程
- **THEN** Go 停止创建新的远程执行并取消或等待活动调用
- **THEN** gRPC ClientConn 在活动调用处理后关闭

### Requirement: Runtime 切换显式且不自动重复执行
系统 SHALL 提供显式配置在迁移期选择本地或远程 Runtime；远程调用出现不确定传输错误后 MUST NOT 自动回退到本地执行同一 `run_id`，避免重复模型调用和重复回复。

#### Scenario: 灰度启用远程 Runtime
- **WHEN** 运维配置指定使用 Python gRPC Runtime
- **THEN** Go 的 Runtime Registry 将目标 Agent turn 交给远程 Adapter
- **THEN** 其他房间和 API 行为保持兼容

#### Scenario: 远程流在模型可能已启动后断开
- **WHEN** Go 收到 `UNAVAILABLE` 或连接重置且无法证明 Python 未开始执行
- **THEN** Go 把本次运行标记为远程执行中断
- **THEN** Go 不自动调用本地 Runtime 重做该 run

#### Scenario: 运维回滚到本地 Runtime
- **WHEN** 运维显式恢复本地 Runtime 配置并重启或重新加载服务
- **THEN** 后续新 run 使用本地 Runtime
- **THEN** 已产生终态的远程 run 不被重新执行

### Requirement: 首个版本明确后端重启语义
在引入持久任务领取和租约恢复之前，系统 MUST NOT 自动恢复被后端重启中断的远程执行；启动治理 SHALL 将可识别的遗留运行收敛为中断状态，而不是让其永久保持 running。

#### Scenario: 后端重启前存在 running 的远程 Agent Run
- **WHEN** 新后端进程启动并发现无本地活动调用对应的遗留 running 记录
- **THEN** 系统把该记录标记为中断失败或取消
- **THEN** 系统不自动重新调用 Python

### Requirement: Agent Runtime 不可用不使核心会议服务失活
Go SHALL 区分核心 API 存活状态与 Agent Runtime 就绪状态；Python 不可用时房间读取、历史、管理和非 Agent 操作仍可服务，但新的 Agent 执行 MUST 快速失败并给出可观察状态。

#### Scenario: Python 健康检查失败
- **WHEN** Python Runtime 不健康但 Go 和 MySQL 正常
- **THEN** 核心健康端点继续报告 Go 进程存活
- **THEN** 依赖就绪状态明确报告 Agent Runtime 不可用
- **THEN** 新 Agent run 不在无界队列中等待
