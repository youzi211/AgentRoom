# realtime-message-delivery Specification

## Purpose
确保会议 room 中用户消息在持久化成功后立即交付，同时将焦点分析与消息交付隔离，并可靠推进分析游标。

## Requirements

### Requirement: 持久化消息立即进入实时交付

系统 SHALL 在用户消息成功持久化并追加到房间状态后广播现有 `message` 事件，并且 MUST NOT 等待焦点分析完成后才广播该事件。

#### Scenario: 焦点分析响应缓慢
- **WHEN** 用户消息持久化成功且该消息触发的焦点 LLM 调用持续未返回
- **THEN** 房间客户端在焦点调用完成前收到该用户消息的 `message` 事件

#### Scenario: 消息持久化失败
- **WHEN** 用户消息写入持久化存储失败
- **THEN** 系统不向房间广播该消息的成功 `message` 事件
- **THEN** 系统不为该消息调度焦点分析

### Requirement: 焦点分析与消息交付异步隔离

系统 SHALL 通过独立的有界后台调度器执行焦点分析，并 MUST 保证调度排队、模型调用、超时或失败不会阻塞对应用户消息的实时广播。

#### Scenario: 焦点分析成功
- **WHEN** 已广播的用户消息触发后台焦点分析且分析返回有效非空结果
- **THEN** 系统通过现有 `focus_update` 事件单独广播新的焦点集合
- **THEN** 焦点更新的完成时序不改变此前用户消息的交付结果

#### Scenario: 同一房间分析尚未完成
- **WHEN** 同一房间已有 queued 或 in-flight 的焦点分析且新消息继续到达
- **THEN** 系统不为该房间并发启动第二个焦点分析
- **THEN** 新消息被保留用于该房间后续满足阈值的分析快照

#### Scenario: 后台调度器达到容量
- **WHEN** 焦点后台调度器已经达到其有界容量
- **THEN** 用户消息仍在持久化成功后被广播
- **THEN** 系统不会通过创建无界 goroutine 来绕过容量限制

### Requirement: 焦点分析终态推进游标

系统 MUST 在一次焦点分析尝试进入终态后，将该房间分析游标单调推进到任务捕获的目标消息位置，无论结果为成功、模型错误、超时、响应解析失败或空结果。

#### Scenario: 模型调用或响应解析失败
- **WHEN** 焦点分析的模型调用失败、超时或返回无法解析的响应
- **THEN** 系统不发布新的 `focus_update` 事件
- **THEN** 系统将分析游标推进到本次任务的目标消息位置并清除 in-flight 状态

#### Scenario: 焦点结果为空
- **WHEN** 焦点分析成功完成但没有得到有效焦点项
- **THEN** 系统保留已有焦点集合且不发布空的 `focus_update` 事件
- **THEN** 系统将分析游标推进到本次任务的目标消息位置

#### Scenario: 失败后新增消息不足阈值
- **WHEN** 一次失败或空结果分析已经推进游标且随后新增消息数量尚未达到焦点分析阈值
- **THEN** 系统不重复分析已完成尝试的同一批消息

#### Scenario: 旧任务迟到完成
- **WHEN** 某个目标消息位置较旧的焦点任务在较新状态之后返回
- **THEN** 系统不得回退该房间的分析游标或用旧结果覆盖较新的焦点状态

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
