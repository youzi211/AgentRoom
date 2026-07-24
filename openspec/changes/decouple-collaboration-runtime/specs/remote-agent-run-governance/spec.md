## MODIFIED Requirements

### Requirement: Go 保持对话编排和业务状态所有权
Go 控制面 SHALL 继续决定何时为已持久化人类消息创建 collaboration run、哪些房间和 Agent 有资格参与、使用哪种协作引擎和策略、何时取消运行，并 MUST 负责创建运行记录、幂等提交最终消息、持久化终态和广播实时事件；Go MAY 将一次运行内部受约束的选角、交接和终止规划委托给 Collaboration Runtime，但 Python 不得取得房间、权限或业务数据所有权。

#### Scenario: 人类消息显式 mention 多个 Agent
- **WHEN** 已持久化的人类消息 mention 多个可响应 Agent
- **THEN** Go 创建一个 collaboration run 并把这些 Agent 作为优先候选传给 Collaboration Runtime
- **THEN** Collaboration Runtime 在房间策略限制内安排多个 turn，而不是为同一消息建立另一条 fanout 主链

#### Scenario: 人类消息没有 mention
- **WHEN** 房间启用自动协作且已持久化的人类消息没有 mention
- **THEN** Go 创建 collaboration run 并允许 Collaboration Runtime 从房间 Agent 快照中选择首位发言者
- **THEN** Python 不得选择快照之外的 Agent

#### Scenario: Agent 回复请求交接
- **WHEN** Collaboration Runtime 返回指向另一个房间 Agent 的交接建议
- **THEN** Go 或远程运行治理验证目标仍在不可变快照与策略允许范围内
- **THEN** 只有合法交接才可产生后续 Agent turn

#### Scenario: Collaboration Runtime 完成多轮运行
- **WHEN** Collaboration Runtime 返回成功、停止、取消或失败终态
- **THEN** Go 提交唯一 collaboration run 终态并保留每个已接受 turn 的 Agent run 审计
- **THEN** Python 不直接写入 AgentRoom 数据库或广播房间事件

## ADDED Requirements

### Requirement: Go 验证并提交多轮协作事件
Go SHALL 验证 Collaboration Runtime 事件的 collaboration run ID、事件序号、turn ID、Agent ID、允许的状态转换和唯一终态，并 MUST 仅提交通过验证的 Agent 最终消息。

#### Scenario: 收到合法 Agent 消息完成事件
- **WHEN** 事件属于当前 collaboration run、引用合格 Agent 和活动 turn，且该 turn 尚未提交
- **THEN** Go 以幂等事务提交最终消息、Agent run 终态和模型审计
- **THEN** 提交成功后广播消息与完成活动

#### Scenario: 收到乱序或重复 turn 完成事件
- **WHEN** 事件序号倒退、turn ID 不属于当前运行或同一 turn 已经提交
- **THEN** Go 拒绝重复业务提交并将协议异常记录为非敏感诊断
- **THEN** 不创建第二条 Agent 消息

### Requirement: Go 统一治理房间级协作取消
Go MUST 维护房间 ID、collaboration run ID 与活动远程调用的受控映射，并 SHALL 在新的人类消息、房间关闭、deadline、显式取消或服务停机时传播取消。

#### Scenario: 新的人类消息抢占旧协作
- **WHEN** 同一房间的新消息成功持久化且旧 collaboration run 仍活动
- **THEN** Go 取消旧远程调用并等待其终态收敛
- **THEN** 旧运行不再启动新的 Agent turn

#### Scenario: 房间关闭
- **WHEN** 房主或管理员关闭存在活动协作的房间
- **THEN** Go 取消对应 Collaboration Runtime 调用
- **THEN** 已提交消息保持不变，未提交的候选输出不进入聊天历史
