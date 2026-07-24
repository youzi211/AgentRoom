## MODIFIED Requirements

### Requirement: Python 负责 Prompt、模型和工具执行
Python Runtime SHALL 根据结构化房间、Agent、触发消息、最近消息和知识快照组合 Prompt，并 SHALL 通过共享 ModelGatewayCore 承担 Provider 调用、工具循环、输出校验和 artifact 生成；普通 LLM Executor 与 DeepAgent Executor MUST 使用同一个 Provider Registry，不得直接导入供应商 SDK。

#### Scenario: Agent 使用知识上下文
- **WHEN** 请求包含房间或 Agent 范围的知识分片
- **THEN** Python Prompt Composer 以标明来源的方式加入相关分片
- **THEN** 最终结果返回实际使用的知识来源标识

#### Scenario: 普通 LLM 模型调用
- **WHEN** Registry 将请求路由到普通 LLM Executor
- **THEN** Executor 从 ModelGatewayCore 获取模型并消费统一增量事件
- **THEN** Provider 错误转换为稳定 Agent 失败事件

#### Scenario: DeepAgent 模型调用
- **WHEN** Registry 将请求路由到 DeepAgent Executor
- **THEN** Executor 使用 ModelGatewayCore 提供的模型构造结果执行 DeepAgent
- **THEN** 工具、artifact 和终态事件保持现有 Agent Runtime 合约

#### Scenario: 工具调用失败可恢复
- **WHEN** 工具返回可恢复错误且执行策略允许继续
- **THEN** Python 发送工具失败事件并继续受控执行
- **THEN** 最终模型调用仍经过 ModelGatewayCore

### Requirement: Python Runtime 不直接拥有供应商调用入口
Python Agent Runtime 的 Executor、Prompt、报告和工具模块 MUST NOT 直接创建 OpenAI、Anthropic 或其他供应商客户端；供应商 SDK 依赖只能存在于 Model Gateway Provider Adapter 模块。

#### Scenario: 生产代码依赖扫描
- **WHEN** 对 Python Runtime 的生产源码执行供应商 SDK 引用扫描
- **THEN** 供应商客户端引用仅出现在 Provider Adapter 目录
- **THEN** Executor 模块只依赖网关 Core 或统一模型接口

#### Scenario: 共享模型构造失败
- **WHEN** Provider Adapter 无法构造目标模型
- **THEN** Gateway Core 返回稳定模型配置或 Provider 错误
- **THEN** Executor 不自行切换到另一套供应商调用路径
