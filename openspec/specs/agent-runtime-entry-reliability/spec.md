# agent-runtime-entry-reliability Specification

## Purpose
TBD - created by archiving change fix-agent-runtime-entry-bugs. Update Purpose after archive.
## Requirements
### Requirement: Runtime artifacts persist in all dialogue modes
The system SHALL persist and expose runtime-produced message artifacts for agent replies generated through both mention fanout and guided dialogue modes.

#### Scenario: Guided dialogue stores DeepAgent report artifact
- **WHEN** a guided dialogue agent runtime returns a response with a Markdown report artifact
- **THEN** the saved agent message includes that artifact in its `artifacts` collection

#### Scenario: Mention fanout continues storing runtime artifacts
- **WHEN** a mention fanout agent runtime returns a response with one or more artifacts
- **THEN** the saved agent message includes those artifacts unchanged except for normal defaulting of missing artifact IDs or filenames

### Requirement: Entry page uses non-admin recent room discovery
The system SHALL allow the entry page to retrieve a limited list of recent active rooms without requiring admin credentials.

#### Scenario: Ordinary user loads entry page recent rooms
- **WHEN** the entry page requests recent active rooms without an admin API key
- **THEN** the backend returns a successful response containing only safe public room summary fields

#### Scenario: Public room summary matches entry page fields
- **WHEN** the entry page renders a recent room returned by the public listing response
- **THEN** room name, room ID, dialogue mode, agent count, passcode status, and room status can be read from fields present in the response

#### Scenario: Admin room listing stays protected
- **WHEN** `ADMIN_API_KEY` is configured and a request calls the admin room listing route without the admin key
- **THEN** the backend rejects that admin listing request

### Requirement: Entry page statistics use real backend data
The system SHALL provide real entry page statistics through a backend summary endpoint, and the entry page SHALL NOT use hardcoded business values for active rooms, today's rooms, knowledge sources, or Agent roles.

#### Scenario: Ordinary user loads entry summary
- **WHEN** an ordinary user opens the entry page
- **THEN** the frontend requests a non-admin entry summary endpoint
- **THEN** the statistics card renders values returned by that endpoint

#### Scenario: Entry summary response fields are complete
- **WHEN** the backend returns the entry summary
- **THEN** the response contains `activeRooms`, `todayRooms`, `knowledgeDocuments`, and `enabledAgents`

#### Scenario: Entry summary load fails
- **WHEN** the entry summary endpoint request fails
- **THEN** the frontend does not display hardcoded business statistics
- **THEN** the frontend displays a placeholder or unavailable state for the statistics

### Requirement: Entry statistics are calculated consistently
The system SHALL calculate entry page statistic totals in the backend, avoiding frontend derivation from limited lists or multiple unrelated endpoints.

#### Scenario: Active room total
- **WHEN** the backend calculates `activeRooms`
- **THEN** `activeRooms` equals the total number of rooms with active status

#### Scenario: Today's room total
- **WHEN** the backend calculates `todayRooms`
- **THEN** `todayRooms` equals the total number of rooms created on the current server-local date

#### Scenario: Knowledge document total
- **WHEN** the backend calculates `knowledgeDocuments`
- **THEN** `knowledgeDocuments` equals the total number of knowledge documents

#### Scenario: Enabled Agent total
- **WHEN** the backend calculates `enabledAgents`
- **THEN** `enabledAgents` equals the total number of Agents whose enabled status is true

### Requirement: 远程 DeepAgent 请求内容与控制字段隔离
系统 SHALL 把用户控制的 DeepAgent 问题作为 Protobuf 内容字段传递，并 MUST 由 Python Executor 以数据而非命令行选项解释该字段。

#### Scenario: 问题以选项样式文本开头
- **WHEN** DeepAgent 问题以 `--` 或其他类似控制参数的文本开头
- **THEN** 该文本保持为问题内容
- **THEN** 它不能改变 Python 服务启动参数、Executor 选择或运行限制

### Requirement: 远程 DeepAgent 执行并发受服务容量约束
系统 SHALL 在 Python 常驻 Runtime 中限制 DeepAgent 并发，并 MUST 让等待容量的调用观察 gRPC deadline 和取消。

#### Scenario: 多个 DeepAgent 请求超过容量
- **WHEN** 活动 DeepAgent 数达到配置上限
- **THEN** 超额请求在有界队列等待或收到资源耗尽错误
- **THEN** Python 不启动超过配置上限的 DeepAgent 执行

#### Scenario: 容量等待期间调用取消
- **WHEN** 等待 DeepAgent 容量的 gRPC 调用被取消或超时
- **THEN** 该请求退出等待
- **THEN** 该请求不再启动 DeepAgent Executor

