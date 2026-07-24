## ADDED Requirements

### Requirement: Model Gateway 使用版本化 gRPC 服务
系统 SHALL 提供版本化 Protobuf `ModelGatewayService`，至少包含 `Generate`、`Stream` 和 `Probe` RPC；Go 与 Python 客户端和服务端 MUST 使用生成代码并通过跨语言契约测试。

#### Scenario: 生成请求使用受支持版本
- **WHEN** Go 使用受支持的协议版本调用 `Generate`
- **THEN** Python 返回统一 ModelResponse
- **THEN** 请求包含 request_id、trace_id 和 purpose

#### Scenario: 不受支持的版本
- **WHEN** 请求声明的 Model Gateway 协议版本不受支持
- **THEN** 服务在调用 Provider 前以 INVALID_ARGUMENT 拒绝请求
- **THEN** 不启动模型调用

### Requirement: ModelRequest 使用协议中立字段
`ModelRequest` SHALL 携带单次 `ModelConnection`、有序消息、响应格式、用途和执行限制；请求 MUST 传递 `protocol`，不得将 OpenAI 专有请求字段作为跨语言公共模型。

#### Scenario: 请求携带解析后的 Profile
- **WHEN** Go 根据 Profile 完成模型解析
- **THEN** 请求包含 Profile ID、来源、协议、Endpoint、模型名和单次凭据
- **THEN** Python 只使用本次请求的连接信息

#### Scenario: JSON 请求
- **WHEN** Go 请求 JSON 对象响应
- **THEN** ModelRequest 的 response_format 明确声明 JSON
- **THEN** 网关根据 Provider 能力决定支持或返回能力错误

### Requirement: Generate 和 Stream 共享调用语义
`Generate` SHALL 返回完整响应，`Stream` SHALL 返回有序增量和终态事件；两者 MUST 共享相同的请求校验、Provider 路由、错误映射和敏感字段规则。

#### Scenario: Generate 成功
- **WHEN** Provider 返回完整非流式内容
- **THEN** Generate 返回 content、usage、model metadata 和 finish 状态

#### Scenario: Stream 正常结束
- **WHEN** Provider 返回流式内容并正常结束
- **THEN** Stream 先返回 started 和零个或多个 delta
- **THEN** Stream 以 completed 或 failed 终态结束

### Requirement: ModelEvent 具有稳定顺序和终态
每个 Stream 事件 MUST 包含 request_id、sequence 和 event_type；sequence 必须严格递增，且一次调用只能有一个 completed 或 failed 终态。

#### Scenario: 事件顺序正确
- **WHEN** Go 消费一次正常 Stream
- **THEN** 后续事件的 sequence 大于前一事件
- **THEN** completed 之后不再出现应用事件

#### Scenario: Provider 失败
- **WHEN** Provider 在流式调用中失败
- **THEN** 服务返回包含稳定 error_code、message 和 retryable 的 failed 终态
- **THEN** failed 事件不包含 Provider 凭据或未脱敏原始响应

### Requirement: gRPC deadline 和取消贯穿模型调用
Go 客户端 SHALL 为每次 RPC 设置 deadline；Python 服务 MUST 在容量等待、Provider 调用和事件发送期间观察取消，并将取消、超时和资源耗尽映射为稳定语义。

#### Scenario: deadline 超时
- **WHEN** RPC 超过 Go deadline
- **THEN** Go 结束流并将结果映射为 deadline exceeded
- **THEN** Python 停止对应 Provider 调用

#### Scenario: 等待容量时取消
- **WHEN** 请求在并发队列中等待期间被取消
- **THEN** 请求不启动 Executor 或 Provider 调用
- **THEN** 网关释放排队项

### Requirement: Probe 使用统一网关连接测试
`Probe` SHALL 使用与正式调用相同的协议路由、Endpoint 规范化、凭据脱敏和能力检查，并返回成功状态、延迟、响应模型和稳定错误分类。

#### Scenario: Profile 探测成功
- **WHEN** 管理员探测已保存且启用的 Profile
- **THEN** Probe 使用该 Profile 的协议和模型发起最小请求
- **THEN** 返回成功、延迟和非敏感响应模型信息

#### Scenario: 草稿探测失败
- **WHEN** 管理员探测未保存的错误配置
- **THEN** Probe 返回连接、认证或模型错误分类
- **THEN** 响应和日志不包含草稿 API Key

### Requirement: gRPC 合约保持跨语言向后兼容
Protobuf MUST 不复用已删除字段编号，新增字段必须可选或具有兼容默认值；Go 客户端面对未知可选事件字段仍 SHALL 正常处理已知终态。

#### Scenario: 新增可选字段
- **WHEN** Python 返回旧 Go 客户端未知的可选字段
- **THEN** Go 仍能解析已知事件并完成本次调用
