## ADDED Requirements

### Requirement: 协作启动不得阻塞人类消息交付
系统 SHALL 在人类消息成功持久化、追加房间状态并广播现有 `message` 事件后，异步创建、取消或调度 collaboration run；选角、引擎初始化、模型调用、远程容量等待和旧运行收敛 MUST NOT 延迟原始消息广播。

#### Scenario: Collaboration Runtime 响应缓慢
- **WHEN** 人类消息已成功持久化，但 Collaboration Runtime 连接、选角或容量等待尚未完成
- **THEN** 房间客户端已经收到该人类消息的 `message` 事件

#### Scenario: 协作调度队列已满
- **WHEN** 后台协作调度达到有界容量
- **THEN** 人类消息仍正常广播
- **THEN** 系统发布非敏感的协作不可用活动或记录可观测失败，不创建无界 goroutine

### Requirement: 新人类消息优先于旧协作活动
系统 MUST 在新的人类消息成功持久化并广播后，请求取消同一房间旧的 collaboration run，并 SHALL 防止旧运行继续产生新的可提交 Agent turn。

#### Scenario: Agent 正在生成时用户继续发言
- **WHEN** 新人类消息已广播且旧 collaboration run 仍在模型生成中
- **THEN** 系统传播旧运行取消并为新消息建立后续协作
- **THEN** 旧运行取消后到达的未提交候选内容不进入聊天历史

#### Scenario: 旧 Agent 消息已在新消息前提交
- **WHEN** 旧运行的 Agent 消息事务在新的人类消息事务之前完成
- **THEN** 已提交 Agent 消息保持可见
- **THEN** 新消息创建新的 collaboration run，不回滚历史消息

### Requirement: 协作活动使用独立实时事件
系统 SHALL 通过独立的 collaboration activity 事件表达运行开始、选角、Agent turn、handoff、取消和终态，并 MUST NOT 用伪造聊天消息承载框架控制状态。

#### Scenario: 引擎选出下一位 Agent
- **WHEN** Go 接受有效的 `speaker_selected` 事件
- **THEN** 房间客户端可收到不含 Prompt、凭据或内部模型推理的协作活动
- **THEN** 消息历史中不新增聊天消息

#### Scenario: 协作运行失败
- **WHEN** collaboration run 在没有可提交 Agent 消息时失败
- **THEN** 客户端收到稳定的协作失败活动或现有安全错误通知
- **THEN** Provider 原始错误和框架堆栈不被广播
