## ADDED Requirements

### Requirement: Python 通过 Engine Registry 托管可替换协作引擎
Python Collaboration Runtime SHALL 通过框架中立的 Engine Registry 按显式引擎标识选择实现，首阶段 MUST 支持 Native Engine 和 AutoGen Engine。

#### Scenario: 选择已注册引擎
- **WHEN** 请求指定已注册且已启用的引擎
- **THEN** Registry 创建该引擎的独立运行实例并传入中立请求

#### Scenario: 引擎未知或未启用
- **WHEN** 请求指定未注册或被部署配置禁用的引擎
- **THEN** 服务在调用模型或工具前返回稳定的不支持错误

### Requirement: Engine 实现遵守共享协作契约
每个 Engine MUST 通过相同接口输出选角、turn、工具、artifact、检查点和唯一终态事件，并 MUST 遵守请求给定的 Agent 集合、协作策略、执行限制与取消信号。

#### Scenario: Native 与 AutoGen 共享契约测试
- **WHEN** 共享契约测试分别运行 Native Engine 与 AutoGen Engine
- **THEN** 两个实现均满足事件顺序、唯一终态、轮次限制、取消和敏感信息要求

#### Scenario: 引擎试图选择房间外 Agent
- **WHEN** 引擎返回不属于请求 Agent 快照的发言者
- **THEN** Collaboration Runtime 拒绝该选择并返回稳定失败终态

### Requirement: Collaboration Runtime 不拥有 AgentRoom 业务数据库
Collaboration Runtime 和所有 Engine MUST NOT 直接读写 AgentRoom 的 rooms、messages、agents、model profiles、agent runs 或 collaboration runs；执行所需上下文 SHALL 完全来自 Go 提供的快照和框架中立模型端口。

#### Scenario: 引擎需要对话历史
- **WHEN** 引擎准备下一位发言者或 Agent Prompt
- **THEN** 引擎使用请求快照和本次运行中已接受的 turn
- **THEN** 引擎不建立到 AgentRoom MySQL 的连接

#### Scenario: 引擎产生最终消息
- **WHEN** 引擎完成 Agent turn
- **THEN** 引擎通过事件返回候选结果
- **THEN** 只有 Go 决定是否持久化并广播该消息

### Requirement: 每次协作运行相互隔离
Python MUST 为每个 collaboration run 建立独立 Engine 实例、取消状态、临时工具状态、模型引用和 checkpoint 上下文，不得在并发运行间共享可变对话状态。

#### Scenario: 两个房间并发使用不同策略
- **WHEN** 两个 collaboration run 携带不同 Agent、策略和模型配置并发执行
- **THEN** 任一运行的选角、Prompt、日志和结果不包含另一运行的数据

#### Scenario: 同一 run ID 重复到达
- **WHEN** 首次请求仍活动时收到相同 collaboration run ID
- **THEN** 服务拒绝第二次执行
- **THEN** 不创建第二套 Team 或模型调用

### Requirement: 协作服务实施有界容量和每房间互斥
Python SHALL 对活动 collaboration run、等待队列和高成本 Engine 设置有界容量，并 MUST 拒绝同一房间的并发活动运行。

#### Scenario: 全局容量已满
- **WHEN** 新运行无法进入有界等待队列
- **THEN** 服务以 `RESOURCE_EXHAUSTED` 结束调用
- **THEN** 不创建无界后台任务

#### Scenario: 同一房间已有活动运行
- **WHEN** 新请求到达时同一房间仍存在活动 run
- **THEN** 服务拒绝并发启动，除非旧调用的取消与清理已完成

### Requirement: Collaboration Runtime 具有独立就绪和优雅关闭状态
Python 进程 SHALL 为 Collaboration Runtime 注册可区分的 gRPC 健康服务名；停止时 MUST 先退出就绪状态，再在宽限期内完成或取消活动 Engine。

#### Scenario: Agent Runtime 正常但协作引擎未初始化
- **WHEN** 单 Agent Executor Registry 已就绪但 Collaboration Engine Registry 初始化失败
- **THEN** Agent Runtime 健康状态可以保持就绪
- **THEN** Collaboration Runtime 健康状态报告不可用

#### Scenario: 进程收到终止信号
- **WHEN** Python 服务进入优雅关闭
- **THEN** 两个 Runtime 均停止接收新请求
- **THEN** 活动协作运行在宽限期内完成或收到取消

### Requirement: 远程失败不得触发隐式跨引擎重做
Collaboration Runtime MUST 在一次调用开始后固定引擎实现；不确定传输错误、框架异常或模型错误 MUST NOT 自动改用另一引擎执行同一 collaboration run。

#### Scenario: AutoGen Engine 初始化后流断开
- **WHEN** 调用在 AutoGen Team 可能已经开始执行后中断
- **THEN** 服务和 Go 控制面不使用 Native Engine 自动重做
- **THEN** 后续重试必须使用新的 collaboration run ID 和显式业务决策
