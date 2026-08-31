## ADDED Requirements

### Requirement: Collaboration Runtime 使用版本化服务端流式协议
系统 SHALL 通过版本化 Protobuf 服务定义 `ExecuteConversation` 服务端流式 RPC；Go 发送一个不可变的多轮协作请求，Python 按顺序返回零个或多个活动事件以及唯一终态。

#### Scenario: 正常多轮协作
- **WHEN** Go 使用受支持的协议版本和唯一 collaboration run ID 调用 `ExecuteConversation`
- **THEN** Python 首先返回已接受或已开始事件
- **THEN** 后续选角、turn、工具、artifact、检查点和终态事件具有严格递增序号
- **THEN** 流以唯一的成功、停止、取消或失败终态结束

#### Scenario: 协议版本不受支持
- **WHEN** 请求声明的协议版本不受 Python 服务支持
- **THEN** 服务在创建引擎或调用模型前以稳定协议错误拒绝请求

### Requirement: 请求使用框架中立的不可变快照
`ExecuteConversationRequest` MUST 使用 AgentRoom 自有类型表达房间、Agent、触发消息、历史、知识、协作策略、执行限制、模型引用和可选引擎检查点，并 MUST NOT 包含 AutoGen 或其他协作框架的序列化对象。

#### Scenario: AutoGen Engine 接收请求
- **WHEN** Engine Registry 将中立请求路由给 AutoGen Engine
- **THEN** AutoGen 适配器在 Python 内部构造框架对象
- **THEN** Go 请求和生成的 Protobuf 代码不依赖 AutoGen 类型

#### Scenario: 请求包含不属于房间的 Agent
- **WHEN** 请求的初始候选或策略引用不在 Agent 快照中的 Agent ID
- **THEN** 服务在启动协作前以无效请求错误拒绝调用

### Requirement: 协作事件具有稳定类型和关联标识
每个 Collaboration Runtime 事件 MUST 包含 collaboration run ID、事件序号和事件类型；Agent turn 相关事件还 MUST 包含稳定 turn ID 与 Agent ID。

#### Scenario: 引擎选择下一位发言者
- **WHEN** 引擎选出下一位合格 Agent
- **THEN** 流返回包含 Agent ID、turn ID 和非敏感选择原因分类的 `speaker_selected` 事件

#### Scenario: Agent turn 成功
- **WHEN** Agent 完成一次可提交回复
- **THEN** 流返回包含完整内容、知识来源、artifact 和非敏感模型审计的 `agent_message_completed` 事件
- **THEN** 同一 turn 不再返回第二个成功完成事件

#### Scenario: 框架产生内部控制消息
- **WHEN** 引擎产生不应进入聊天历史的内部消息
- **THEN** 该消息只能表示为控制或诊断事件
- **THEN** 事件不得伪装成 `agent_message_completed`

### Requirement: Deadline 和取消贯穿整个协作运行
Go SHALL 为每次协作调用设置 deadline 并通过 gRPC 上下文传播取消；Python MUST 在容量等待、选角、Agent turn、模型、工具、事件输出和检查点生成期间观察取消。

#### Scenario: 新的人类消息中断旧运行
- **WHEN** Go 因同一房间的新消息取消活动协作调用
- **THEN** Python 停止选择新的 Agent turn并取消当前可取消工作
- **THEN** Go 将旧运行收敛为取消或与已提交终态保持一致

#### Scenario: 等待容量时被取消
- **WHEN** 请求仍在等待 Python 容量时调用被取消
- **THEN** 服务不创建 Engine 实例或启动模型调用

### Requirement: 引擎检查点是可选且不透明的
协议 SHALL 允许返回带引擎名、引擎版本、状态格式版本和校验摘要的 opaque checkpoint，但 MUST NOT 要求 Go 解析框架内部字段。

#### Scenario: 检查点版本兼容
- **WHEN** 新运行携带与目标引擎兼容的 checkpoint
- **THEN** 引擎可以恢复内部协作状态并继续使用权威业务快照

#### Scenario: 检查点缺失或不兼容
- **WHEN** checkpoint 缺失、损坏或版本不受支持
- **THEN** 引擎从请求携带的 AgentRoom 权威快照重建运行状态
- **THEN** 不得因无法恢复可选 checkpoint 而修改或丢失已持久化聊天历史

### Requirement: 协议限制资源并保护敏感信息
请求、事件、内联 artifact 和 checkpoint MUST 具有可配置大小限制；日志、事件、检查点和错误描述 MUST NOT 包含 API Key、Authorization Header 或未脱敏 Provider 响应。

#### Scenario: checkpoint 超过大小限制
- **WHEN** 引擎生成的 checkpoint 超过配置上限
- **THEN** 服务丢弃该可选 checkpoint并记录非敏感诊断
- **THEN** 已完成的有效 Agent 消息和运行终态不受影响

#### Scenario: Provider 异常包含凭据
- **WHEN** 底层模型或工具异常文本包含请求凭据
- **THEN** Python 在写日志或发送失败事件前完成脱敏
