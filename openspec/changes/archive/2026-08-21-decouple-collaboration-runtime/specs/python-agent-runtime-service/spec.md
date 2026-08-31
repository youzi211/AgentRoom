## MODIFIED Requirements

### Requirement: Python 以常驻服务托管 Agent Executor
系统 SHALL 提供独立常驻 Python 服务，通过 Executor Registry 托管普通 LLM 与 DeepAgent 的单 Agent 执行，并通过逻辑独立的 Engine Registry 托管 Native 与 AutoGen 多 Agent 协作；两个 Runtime MAY 共享进程、传输安全、容量、健康检查和关闭基础设施，但 MUST 使用独立服务合约与运行上下文。

#### Scenario: 普通 LLM Agent 单 turn 执行
- **WHEN** `AgentRuntimeService` 请求指定普通 LLM Executor
- **THEN** Executor Registry 选择普通 LLM 实现并返回统一 Agent 事件流
- **THEN** 不创建 Collaboration Engine

#### Scenario: DeepAgent 单 turn 执行
- **WHEN** `AgentRuntimeService` 请求指定 DeepAgent Executor
- **THEN** Executor Registry 选择 DeepAgent 实现并在同一 Agent Runtime 协议上返回工具、artifact 和终态事件
- **THEN** 不创建 Collaboration Engine

#### Scenario: 多轮房间协作执行
- **WHEN** `CollaborationRuntimeService` 请求指定已注册协作引擎
- **THEN** Engine Registry 创建独立协作运行并返回统一 Collaboration 事件流
- **THEN** 协作服务不冒充或循环调用单 Agent gRPC 服务

#### Scenario: Executor 或 Engine 类型未知
- **WHEN** 请求指定未注册的 Executor 或 Engine
- **THEN** 对应服务在调用模型或工具前返回稳定的不支持错误

#### Scenario: 两个 Runtime 的就绪状态不同
- **WHEN** Executor Registry 正常但 Engine Registry 初始化失败
- **THEN** Agent Runtime 服务可以保持就绪
- **THEN** Collaboration Runtime 服务报告不可用

## ADDED Requirements

### Requirement: 共享基础设施不得混淆运行身份和容量
Python 服务 MUST 分别跟踪 Agent run 与 collaboration run 的活动身份、取消句柄、指标和容量分类，并 SHALL 防止相同字符串 ID 在不同服务命名空间中造成错误取消或事件串线。

#### Scenario: Agent run 与 collaboration run 使用相同文本 ID
- **WHEN** 两个服务收到文本相同但类型不同的活动 run ID
- **THEN** 服务以独立命名空间跟踪两个调用
- **THEN** 取消其中一个调用不影响另一个调用

#### Scenario: Collaboration Engine 产生多个 Agent turn
- **WHEN** 一个 collaboration run 在内部执行多个 Agent turn
- **THEN** 服务把容量和指标关联到父 collaboration run 与各 turn
- **THEN** 不把内部 turn 注册成重复的外部 collaboration run
