## ADDED Requirements

### Requirement: Runtime 按确定优先级解析模型配置
系统 SHALL 按房间快照绑定、运行时数据库默认值和迁移期环境配置的顺序解析模型，并在所有来源均不可用时返回明确未配置错误。

#### Scenario: 房间 Agent 有快照 Profile
- **WHEN** Agent 运行请求携带有效的房间快照 Profile ID
- **THEN** Resolver 使用该 Profile且不查询其他默认值

#### Scenario: 旧房间没有 Profile ID
- **WHEN** 迁移前房间的 Agent 快照没有 Profile ID且对应 Runtime 存在启用的数据库默认 Profile
- **THEN** Resolver 使用该数据库默认 Profile

#### Scenario: 数据库没有运行时默认 Profile
- **WHEN** 对应 Runtime 没有可用数据库默认 Profile但现有环境配置完整
- **THEN** Resolver 使用环境配置作为迁移兜底

#### Scenario: 所有配置来源缺失
- **WHEN** Agent 无有效快照绑定、无数据库默认 Profile且无完整环境配置
- **THEN** 系统返回模型未配置错误且不启动模型调用

### Requirement: Go Agent 每次运行使用解析后的 Profile
Go LLM Agent Runtime SHALL 在每次响应生成时解析 Agent 的有效 Profile，并使用该 Profile 的 Base URL、模型 ID 和解密 API Key创建 OpenAI-compatible Chat Completions 客户端。

#### Scenario: 两个 Go Agent 使用不同 Profile
- **WHEN** 同一房间的两个 Go Agent 分别绑定不同 Profile并先后被触发
- **THEN** 每个 Agent 的请求发送到各自 Profile 指定的 Base URL 和模型

#### Scenario: 管理员轮换 API Key
- **WHEN** 管理员更新某个被房间快照引用的 Profile API Key
- **THEN** 该 Agent 下一次运行使用更新后的密钥而无需重启后端

### Requirement: Go 系统能力共用 Go 默认 Profile
焦点提取和会议纪要生成 MUST 使用当前启用的 Go 默认 Profile，并且首版 MUST NOT 为这些能力建立独立模型绑定。

#### Scenario: 更换 Go 默认 Profile
- **WHEN** 管理员把另一个 Go Profile 设为默认值
- **THEN** 后续焦点提取和会议纪要调用使用新的 Go 默认 Profile
- **THEN** 现有房间 Agent 的具体快照绑定不发生变化

### Requirement: DeepAgent 使用单次执行模型上下文
DeepAgent Runtime SHALL 解析有效 DeepAgent Profile，并仅在本次执行上下文中提供 OpenAI-compatible 模型协议、Base URL、模型 ID 和 API Key，不得把数据库模型凭据写回 Python 配置文件。local Runtime 可以把上下文映射为单次子进程环境；gRPC Runtime MUST 把上下文放入受保护的单次执行请求。

#### Scenario: DeepAgent 使用专用 Profile
- **WHEN** 绑定专用 DeepAgent Profile 的 Agent 被触发
- **THEN** Go 后端构造只属于该次 run 的协议、Base URL、模型 ID 和 API Key
- **THEN** Python Runtime 仅从该次执行上下文使用这些值且不修改模型文件配置

#### Scenario: DeepAgent 执行结束
- **WHEN** DeepAgent 执行成功、失败、取消或超时结束
- **THEN** 本次解密模型配置不被写入 DeepAgent `.env`、TOML、报告或事件日志

### Requirement: Agent 运行记录实际模型选择
系统 SHALL 在 Agent 运行记录中保存实际使用的 Profile ID和模型名快照，并且 MUST NOT 保存 API Key或其他明文凭据。

#### Scenario: Agent 调用成功
- **WHEN** Agent 使用数据库 Profile 完成一次模型调用
- **THEN** 对应 `agent_run` 记录该 Profile ID和调用时模型名

#### Scenario: Agent 使用环境兜底
- **WHEN** Agent 通过迁移期环境配置完成调用
- **THEN** 运行记录标识环境兜底来源并保存非敏感模型名
