## ADDED Requirements

### Requirement: AutoGen 依赖封装在独立 Engine 适配器
系统 SHALL 将 Microsoft AutoGen 依赖、类型、Team 构造和状态转换限制在 AutoGen Engine 模块内；Go、Protobuf、公共 API、Store 和其他 Engine MUST NOT 导入或序列化 AutoGen 类型。

#### Scenario: AutoGen 包升级
- **WHEN** AutoGen API 或状态格式发生版本变化
- **THEN** 修改范围限定在 AutoGen Engine、其模型适配器和契约测试
- **THEN** Go 的 Collaboration Runtime 客户端与业务数据模型保持不变

### Requirement: AgentRoom 快照确定性映射为 AutoGen Team
AutoGen Engine SHALL 根据请求中的稳定 Agent ID、名称、角色、描述、system prompt、工具能力和模型引用构造 Team 参与者，并 MUST 保持 Agent ID 与事件身份的一一映射。

#### Scenario: 房间包含多个不同角色 Agent
- **WHEN** 请求包含多个已排序的 Agent 快照
- **THEN** 适配器为每个可响应 Agent 构造唯一参与者
- **THEN** 返回事件中的 Agent ID 可无歧义映射回 AgentRoom 快照

#### Scenario: Agent 名称不满足框架命名限制
- **WHEN** AgentRoom 名称包含 AutoGen 参与者名称不支持的字符或发生规范化冲突
- **THEN** 适配器使用稳定 Agent ID 生成内部名称
- **THEN** 用户可见名称仍来自 AgentRoom 快照

### Requirement: AutoGen Team 遵守 AgentRoom 的选角优先级
AutoGen Engine MUST 将有效显式 mention 作为初始发言者的确定性优先信号；没有 mention 时 SHALL 使用受约束的选择策略选出首位发言者，并不得默认让全部 Agent 同时回答。

#### Scenario: 消息明确 mention 一个 Agent
- **WHEN** 请求包含一个有效的显式 mention
- **THEN** 对应 Agent 成为首位发言者，不额外调用模型决定首位

#### Scenario: 消息没有 mention
- **WHEN** 请求没有显式 mention 且房间启用自动协作
- **THEN** AutoGen Engine 根据任务、Agent 角色和对话上下文选择一位首发 Agent
- **THEN** 选择结果受房间允许 Agent 集合和执行限制约束

#### Scenario: Agent 请求交接
- **WHEN** 当前 Agent 产生可识别的 handoff 目标
- **THEN** AutoGen Engine 将其转换为中立交接事件并验证目标资格
- **THEN** 不合格目标不会获得下一 turn

### Requirement: AutoGen 终止条件映射为稳定业务终态
AutoGen Engine SHALL 把最大消息数、文本终止、handoff、取消、超时、空输出、重复输出和框架错误转换为 AgentRoom 定义的稳定停止或失败原因。

#### Scenario: 达到房间最大轮数
- **WHEN** AutoGen Team 达到请求定义的最大 Agent turn 数
- **THEN** Engine 结束 Team 并返回达到轮次限制的终态

#### Scenario: 人类消息取消 Team
- **WHEN** Collaboration Runtime 的取消信号被触发
- **THEN** AutoGen Engine 取消 `run` 或 `run_stream`
- **THEN** 不再启动新的模型或工具调用

#### Scenario: 框架返回未识别终止原因
- **WHEN** AutoGen 返回适配器未识别的终止或异常类型
- **THEN** Engine 使用安全的内部失败分类
- **THEN** 不把原始异常或敏感 Provider 内容直接返回给 Go

### Requirement: AutoGen 事件转换保持聊天与控制边界
AutoGen Engine MUST 将参与者最终回复映射为中立 Agent 消息完成事件，并 SHALL 将选角、handoff、模型、工具、规划和内部控制消息映射为非聊天活动事件。

#### Scenario: Team 产生参与者回复
- **WHEN** AutoGen 参与者完成一条用户可见回复
- **THEN** Engine 返回唯一 `agent_message_completed` 事件
- **THEN** 事件使用对应 AgentRoom Agent ID

#### Scenario: Team 产生 GroupChat 管理消息
- **WHEN** AutoGen 内部管理器产生选角或控制消息
- **THEN** Engine 不把该消息作为 Agent 聊天内容提交
- **THEN** 可观测信息仅以受限活动事件返回

### Requirement: AutoGen 状态是可选的版本化检查点
AutoGen Engine SHALL 仅以 opaque checkpoint 暴露可恢复状态，并 MUST 同时携带 AutoGen 固定版本、适配器状态版本和完整性摘要。

#### Scenario: 兼容状态恢复
- **WHEN** 请求提供与当前适配器和 AutoGen 版本兼容的 checkpoint
- **THEN** Engine 恢复 Team 状态并以请求中的权威 transcript 校验恢复边界

#### Scenario: AutoGen 升级导致状态不兼容
- **WHEN** checkpoint 由不兼容的 AutoGen 或适配器版本生成
- **THEN** Engine 忽略该 checkpoint并从 AgentRoom 快照重建 Team
- **THEN** 已持久化消息不被修改或重复生成

### Requirement: AutoGen 通过框架中立模型端口调用模型
AutoGen Engine MUST 通过 AgentRoom 定义的模型端口调用模型，生产配置 SHALL 使用统一 Model Gateway 或等价适配器，并 MUST NOT 在 Team、Agent 或 selector 中直接构造新的 Provider SDK 客户端。

#### Scenario: Selector 需要模型选择发言者
- **WHEN** AutoGen Team 需要模型完成 speaker selection
- **THEN** selector 通过同一个框架中立模型端口调用
- **THEN** 调用遵守 Profile、deadline、usage、错误归一化和凭据保护语义

#### Scenario: Model Gateway 尚未就绪
- **WHEN** 部署没有可用的框架中立模型端口
- **THEN** AutoGen Engine 不进入生产就绪状态
- **THEN** Native Engine 和现有单 Agent Runtime 可按显式配置继续服务
