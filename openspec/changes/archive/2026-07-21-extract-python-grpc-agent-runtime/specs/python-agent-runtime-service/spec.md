## ADDED Requirements

### Requirement: Python 以常驻服务托管 Agent Executor
系统 SHALL 提供独立常驻 Python Agent Runtime 服务，并通过 Executor Registry 将请求路由到普通 LLM Agent 或 DeepAgent Executor，而不是由 Go 为每次执行启动 Python CLI 子进程。

#### Scenario: 普通 LLM Agent 执行
- **WHEN** 请求指定普通 LLM Executor
- **THEN** Registry 选择普通 LLM Agent 实现并返回统一 Agent 事件流

#### Scenario: DeepAgent 执行
- **WHEN** 请求指定 DeepAgent Executor
- **THEN** Registry 选择 DeepAgent 实现并在同一 gRPC 协议上返回工具、artifact 和终态事件

#### Scenario: Executor 类型未知
- **WHEN** 请求指定未注册的 Executor 类型
- **THEN** 服务在调用模型或工具前返回稳定的不支持错误

### Requirement: 单次执行上下文相互隔离
Python 服务 MUST 为每个 `run_id` 建立独立执行上下文；Prompt、临时模型配置、工具状态、取消状态和 artifact 工作区不得在并发运行之间共享可变数据。

#### Scenario: 两个 Agent 使用不同模型 Profile
- **WHEN** 两次并发执行携带不同的模型地址、模型名和凭据
- **THEN** 每个 Executor 只使用所属请求的模型配置
- **THEN** 任一执行的日志和结果不包含另一执行的数据

#### Scenario: 同一 run_id 重复到达
- **WHEN** 服务在首次请求仍活动时收到相同 `run_id` 的第二个请求
- **THEN** 服务拒绝并发重复执行或返回可识别的已有运行状态
- **THEN** 不启动第二次模型调用

### Requirement: Python 不拥有 AgentRoom 业务数据库
Python Runtime MUST NOT 直接读取或写入 AgentRoom 的 rooms、messages、agents、model profiles、agent runs 或 dialogue runs；执行所需业务上下文 SHALL 由 Go 作为请求快照提供。

#### Scenario: Executor 需要最近消息和知识
- **WHEN** Agent 执行需要对话历史和知识分片
- **THEN** Executor 使用请求携带的最近消息和知识快照
- **THEN** Executor 不建立到 AgentRoom MySQL 的连接

#### Scenario: Executor 产生最终回复
- **WHEN** Python 完成回复和 artifact 生成
- **THEN** Python 通过流返回结果
- **THEN** 只有 Go 负责保存消息和运行审计

### Requirement: Python 负责 Prompt、模型和工具执行
Python Runtime SHALL 根据结构化房间、Agent、触发消息、最近消息和知识快照组合 Prompt，并 SHALL 承担模型 Provider 调用、工具循环、输出校验和 artifact 生成。

#### Scenario: Agent 使用知识上下文
- **WHEN** 请求包含 Room 或 Agent 范围的知识分片
- **THEN** Python Prompt Composer 以标明来源的方式加入相关分片
- **THEN** 最终结果返回实际使用的知识来源标识

#### Scenario: 工具调用失败但 Executor 可以恢复
- **WHEN** 某个工具返回可恢复错误且执行策略允许继续
- **THEN** Python 发送工具失败事件并继续受控执行
- **THEN** 最终终态明确此次运行成功或失败

### Requirement: artifact 与现有消息语义兼容
Python Runtime SHALL 返回现有 Agent 消息能够持久化的 artifact 标识、类型、标题、文件名、MIME 类型和内容，并 MUST 在超出内联限制时返回资源超限或外部引用，而不得静默截断。

#### Scenario: DeepAgent 生成 Markdown 报告
- **WHEN** DeepAgent 成功生成报告
- **THEN** 完成事件包含可由 Go 保存到 Agent 消息的 Markdown artifact
- **THEN** Mention Fanout 与 Guided Dialogue 使用相同 artifact 结构

#### Scenario: artifact 超过内联限制
- **WHEN** artifact 内容超过协议允许的内联大小
- **THEN** Python 不发送不完整的 artifact
- **THEN** 运行返回明确的超限结果或已配置的外部 artifact 引用

### Requirement: Python 服务实施有界并发和背压
Python Runtime MUST 对总执行数和高成本 DeepAgent 执行数设置有界容量，并 SHALL 在等待队列、模型调用和事件输出上实施背压，保证终态事件不会因非关键进度事件堆积而丢失。

#### Scenario: DeepAgent 达到并发上限
- **WHEN** 活动 DeepAgent 数达到配置上限
- **THEN** 新请求在有界队列等待或以资源耗尽错误结束
- **THEN** 服务不启动超过上限的 DeepAgent 执行

#### Scenario: Go 客户端读取事件变慢
- **WHEN** Go 暂时不能及时消费进度事件
- **THEN** Python 对事件生产施加背压或合并可合并进度
- **THEN** started、artifact、completed 和 failed 等关键事件保持有序且不被丢弃

### Requirement: 模型凭据只在单次运行内存中存在
Python Runtime MUST NOT 把请求携带的模型 API Key 写入环境文件、TOML、报告、事件、异常文本或持久缓存，并 SHALL 在运行结束后释放对应执行上下文。

#### Scenario: 模型调用成功后清理上下文
- **WHEN** 一次 Agent 执行成功、失败、取消或超时结束
- **THEN** 服务释放该次模型客户端和临时凭据引用
- **THEN** 后续执行不能读取该次凭据

#### Scenario: Provider 错误包含请求敏感值
- **WHEN** Provider 或工具异常文本包含模型凭据
- **THEN** Python 在记录日志或发送失败事件前完成脱敏

### Requirement: 服务优雅停机不接受半初始化运行
Python Runtime SHALL 在启动完成前保持不就绪，并在停机期间拒绝新请求、等待受限时间、取消剩余工作并关闭模型和工具资源。

#### Scenario: 容器滚动停止
- **WHEN** Python 容器收到终止信号
- **THEN** 服务先从健康就绪状态退出
- **THEN** 新执行不再被接受
- **THEN** 活动执行在宽限期结束前完成或收到取消
