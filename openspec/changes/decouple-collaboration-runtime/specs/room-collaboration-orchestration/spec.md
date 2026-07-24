## ADDED Requirements

### Requirement: 人类消息统一创建协作运行
系统 SHALL 在活动房间中的人类消息成功持久化后，按照房间协作策略创建唯一的 collaboration run，并 MUST 使用同一条执行主链处理有 mention 和无 mention 的消息。

#### Scenario: 普通消息没有显式 mention
- **WHEN** 用户在启用自动协作的活动房间发送没有显式 mention 的消息
- **THEN** 系统创建 collaboration run，并允许协作引擎从房间 Agent 快照中选择首位发言者

#### Scenario: 消息包含显式 mention
- **WHEN** 用户消息显式 mention 一个或多个可响应 Agent
- **THEN** 系统把这些 Agent 作为高优先级选角信号传给同一个 collaboration run
- **THEN** 系统不切换到独立的 fanout 执行主链

#### Scenario: 房间使用兼容的仅 mention 策略
- **WHEN** 房间在灰度期显式配置为仅 mention 且消息没有 mention
- **THEN** 系统不创建 collaboration run
- **THEN** 人类消息仍按正常流程持久化和广播

### Requirement: 每个房间最多存在一个活动协作运行
系统 MUST 串行治理同一房间的 collaboration run，并 SHALL 允许不同房间在全局容量范围内并发执行。

#### Scenario: 同一房间的新消息到达
- **WHEN** 房间仍有活动 collaboration run 且新的有效人类消息已持久化
- **THEN** 系统取消或收敛旧运行，等待其进入唯一终态后再启动新运行
- **THEN** 新消息优先于旧运行尚未开始的后续 Agent turn

#### Scenario: 不同房间同时发言
- **WHEN** 两个房间各自不存在活动 collaboration run 且同时收到人类消息
- **THEN** 系统可在全局容量允许时并发启动两个运行

### Requirement: 协作轮次由统一策略约束
系统 SHALL 对所有 collaboration run 统一执行最大总轮数、单 Agent 最大轮数、允许的 Agent 集合、重复输出、空输出、deadline、交接和终止条件，不得由具体引擎绕过房间策略。

#### Scenario: 引擎选择超过单 Agent 限制的发言者
- **WHEN** 协作引擎请求某 Agent 执行超过房间策略允许的最大轮数
- **THEN** 运行治理拒绝该选角并以受控终止原因结束或选择其他合格 Agent

#### Scenario: Agent 返回重复或空输出
- **WHEN** Agent turn 返回空内容或命中协作策略定义的近期重复条件
- **THEN** 系统不提交该内容为成功聊天消息
- **THEN** collaboration run 以稳定的停止原因结束

#### Scenario: 达到最大总轮数
- **WHEN** 已提交 Agent turn 数达到房间策略上限
- **THEN** 系统停止选择下一位 Agent，并将 collaboration run 标记为达到轮次限制

### Requirement: Go 保持协作业务事实所有权
Go 控制面 MUST 创建和提交 collaboration run、Agent run、最终聊天消息与终态审计，并 SHALL 仅把协作规划和受约束执行委托给 Collaboration Runtime。

#### Scenario: Agent turn 成功
- **WHEN** Collaboration Runtime 返回一个有效的 Agent 消息完成事件
- **THEN** Go 以幂等事务提交 Agent 消息、对应 Agent run 终态和非敏感模型审计
- **THEN** 事务提交后才向房间广播最终聊天消息

#### Scenario: Collaboration Runtime 返回框架内部消息
- **WHEN** 远程事件包含规划说明、内部提示或框架控制消息
- **THEN** Go 不把该内容直接持久化为用户可见聊天消息
- **THEN** 只有中立合约明确标记的 Agent 最终消息可进入房间历史

### Requirement: 协作引擎选择显式且可回滚
系统 SHALL 为新 collaboration run 显式记录所选引擎、引擎版本和策略版本，并 MUST NOT 在一个运行已经可能调用模型后自动切换引擎重做。

#### Scenario: 灰度房间选择 AutoGen 引擎
- **WHEN** 房间或部署灰度配置选择 AutoGen Engine
- **THEN** 新 collaboration run 记录该引擎与版本并只通过该引擎执行

#### Scenario: 远程引擎流中断
- **WHEN** AutoGen Engine 在可能已开始模型调用后发生传输中断
- **THEN** 系统把运行记录为中断或失败
- **THEN** 系统不使用 Native Engine 自动重做同一 collaboration run
