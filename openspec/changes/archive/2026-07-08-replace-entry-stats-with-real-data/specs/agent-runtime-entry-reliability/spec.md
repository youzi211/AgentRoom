## ADDED Requirements

### Requirement: 入口页统计必须来自真实后端数据
系统 SHALL 通过后端摘要接口向入口页提供真实统计数据，入口页不得使用硬编码业务数字展示“进行中”、“今日会议”、“知识来源”或“Agent 角色”。

#### Scenario: 入口页加载统计摘要
- **WHEN** 普通用户打开入口页
- **THEN** 前端请求非 admin 的入口摘要接口
- **THEN** 统计卡片使用该接口返回的数值渲染

#### Scenario: 统计摘要字段完整
- **WHEN** 后端返回入口摘要
- **THEN** 响应包含 activeRooms、todayRooms、knowledgeDocuments 和 enabledAgents 字段

#### Scenario: 统计加载失败
- **WHEN** 入口摘要接口请求失败
- **THEN** 前端不得展示硬编码的业务统计数字
- **THEN** 前端显示占位、0 或错误态以表明统计不可用

### Requirement: 入口统计口径必须一致
系统 SHALL 在后端统一计算入口页统计口径，避免前端从受限列表或多个接口中推导总量。

#### Scenario: 进行中会议统计
- **WHEN** 后端计算 activeRooms
- **THEN** activeRooms 等于当前 active 状态会议总数

#### Scenario: 今日会议统计
- **WHEN** 后端计算 todayRooms
- **THEN** todayRooms 等于服务器本地日期当天创建的会议总数

#### Scenario: 知识来源统计
- **WHEN** 后端计算 knowledgeDocuments
- **THEN** knowledgeDocuments 等于知识库文档总数

#### Scenario: Agent 角色统计
- **WHEN** 后端计算 enabledAgents
- **THEN** enabledAgents 等于 enabled 状态为 true 的 Agent 总数
