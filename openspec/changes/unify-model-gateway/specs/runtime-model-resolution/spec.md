## MODIFIED Requirements

### Requirement: Runtime 按确定优先级解析模型配置
系统 SHALL 按房间快照绑定、运行时数据库默认值和迁移期环境配置的顺序解析模型 Profile，并在所有来源不可用时返回明确未配置错误；解析结果 MUST 包含 Profile ID、来源、协议、Base URL、模型名和仅供本次调用使用的凭据。

#### Scenario: 房间 Agent 有快照 Profile
- **WHEN** Agent 运行请求携带有效的房间快照 Profile ID
- **THEN** Resolver 使用该 Profile 的完整协议和连接配置
- **THEN** Resolver 不查询其他默认值

#### Scenario: 旧房间没有 Profile ID
- **WHEN** 迁移前房间 Agent 没有 Profile ID 且对应 Runtime 存在启用的数据库默认 Profile
- **THEN** Resolver 使用该数据库默认 Profile 并返回其 protocol

#### Scenario: 数据库没有 Runtime 默认 Profile
- **WHEN** 对应 Runtime 没有可用数据库默认 Profile 但环境配置完整
- **THEN** Resolver 使用环境配置作为迁移兜底
- **THEN** 该配置被标记为 environment 来源

#### Scenario: 所有配置来源缺失
- **WHEN** Agent 没有有效快照绑定、数据库默认 Profile 或完整环境配置
- **THEN** 系统返回模型未配置错误
- **THEN** 系统不启动 Model Gateway 调用

### Requirement: Go Agent 每次运行使用解析后的 Profile 并通过统一网关
Go LLM Agent Runtime SHALL 在每次响应生成时解析 Agent 的有效 Profile，并将协议、Base URL、模型 ID 和凭据映射为 ModelGatewayClient 请求；Go Runtime MUST NOT 直接创建供应商模型客户端。

#### Scenario: 两个 Go Agent 使用不同 Profile
- **WHEN** 同一房间的两个 Agent 分别绑定不同 Profile 并先后被触发
- **THEN** 每次请求通过网关发送到各自 Profile 指定的协议、Endpoint 和模型

#### Scenario: 管理员轮换 API Key
- **WHEN** 管理员更新某个被房间快照引用的 Profile API Key
- **THEN** 该 Agent 下一次运行通过网关使用更新后的凭据
- **THEN** 不需要重启 Go 或 Python 服务

### Requirement: Go 系统能力共用 Go 默认 Profile 并通过统一网关
Focus 提取和会议纪要生成 MUST 使用当前启用的 Go 默认 Profile，并通过 ModelGatewayClient 调用；系统能力 MUST NOT 创建独立供应商客户端或手工拼装 Provider HTTP 请求。

#### Scenario: 更换 Go 默认 Profile
- **WHEN** 管理员将另一个 Go Profile 设为默认
- **THEN** 后续 Focus、会议纪要和未显式绑定的 Go Agent 使用新 Profile
- **THEN** 已有房间 Agent 的显式快照绑定不发生变化

#### Scenario: Go 网关不可用
- **WHEN** Focus 或会议纪要调用 Model Gateway 超时或不可用
- **THEN** 调用方收到稳定的网关不可用错误
- **THEN** 消息交付和 Agent 调度不因系统能力调用同步阻塞

### Requirement: DeepAgent 使用单次执行模型上下文并通过共享网关
DeepAgent Runtime SHALL 在本次执行上下文中携带解析后的 protocol、Base URL、模型 ID 和凭据，并通过共享 ModelGatewayCore 构造模型；不得把配置写入 Python 配置文件、环境文件或独立 Provider 客户端路径。

#### Scenario: DeepAgent 使用专用 Profile
- **WHEN** 绑定 DeepAgent Profile 的 Agent 被触发
- **THEN** Go 为本次 run 构造包含协议和连接信息的受保护请求
- **THEN** DeepAgent 使用共享 Gateway Core 的模型构造结果

#### Scenario: DeepAgent 执行结束
- **WHEN** DeepAgent 成功、失败、取消或超时结束
- **THEN** 本次模型凭据不被写入 `.env`、TOML、报告、事件或日志
- **THEN** 本次执行上下文被释放
