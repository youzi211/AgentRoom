## ADDED Requirements

### Requirement: 所有生产模型调用经过统一网关
系统 SHALL 通过 Model Gateway 提供文本生成、流式生成和连接探测能力，Focus、会议纪要、普通 LLM Agent、DeepAgent 以及模型连接测试 MUST NOT 在网关 Provider Adapter 之外直接调用供应商 SDK 或供应商 HTTP 接口。

#### Scenario: Go 系统能力调用网关
- **WHEN** Focus 或会议纪要需要生成模型响应
- **THEN** Go 通过 ModelGatewayClient 调用网关
- **THEN** 业务服务不创建供应商模型客户端

#### Scenario: Python Agent Executor 调用网关
- **WHEN** 普通 LLM Executor 或 DeepAgent Executor 执行模型步骤
- **THEN** Executor 使用共享 ModelGatewayCore 或其 Provider Registry
- **THEN** Executor 不直接导入供应商 SDK

### Requirement: 网关按协议注册并路由 Provider Adapter
Model Gateway SHALL 按规范化 `protocol` 选择 Provider Adapter，并为每个 Adapter 声明 `streaming`、`json_object` 和 `tools` 等能力；请求所需能力不受支持时 MUST 返回稳定的能力错误。

#### Scenario: OpenAI-compatible Profile 路由
- **WHEN** 请求的协议为 `openai_chat_completions`
- **THEN** 网关选择 OpenAI-compatible Adapter
- **THEN** 该 Adapter 使用请求中的 Endpoint、模型名和单次凭据执行调用

#### Scenario: 不支持的协议
- **WHEN** 请求协议没有已注册 Adapter
- **THEN** 网关在发起供应商请求前返回协议不支持错误
- **THEN** 不产生供应商调用

#### Scenario: 请求能力不受支持
- **WHEN** 请求要求 JSON 或工具能力而目标 Adapter 未声明该能力
- **THEN** 网关返回能力不支持错误
- **THEN** 不静默降级为可能改变语义的普通文本调用

### Requirement: 普通 LLM 与 DeepAgent 共享模型构造入口
Python 网关 SHALL 为普通 LLM Executor 和 DeepAgent Executor 提供同一个 Provider Adapter 模型构造入口；普通 LLM MUST 通过该模型的流式或非流式接口执行，DeepAgent MUST 将该模型传给其 Agent 构造器。

#### Scenario: 普通 LLM 使用共享模型
- **WHEN** LLM Executor 执行一次文本请求
- **THEN** Executor 从 Gateway Core 获取 BaseChatModel
- **THEN** 输出块和 usage 被转换为统一网关事件

#### Scenario: DeepAgent 使用共享模型
- **WHEN** DeepAgent Executor 开始研究执行
- **THEN** Executor 从同一 Gateway Core 获取 BaseChatModel
- **THEN** DeepAgent 不再单独根据协议创建 ChatOpenAI 或 ChatAnthropic

### Requirement: 网关返回统一响应和错误语义
网关 SHALL 将供应商响应转换为统一文本、usage、完成状态和错误结构，并 MUST 对 Provider 异常进行脱敏；空输出、超限、超时、认证失败、限流和不可用状态必须可被调用方区分。

#### Scenario: 流式调用成功
- **WHEN** Provider 返回一个或多个文本增量并正常结束
- **THEN** 网关返回有序增量事件、usage 事件和 completed 事件
- **THEN** completed 事件包含非敏感模型元数据

#### Scenario: Provider 认证失败
- **WHEN** Provider 返回认证或权限错误
- **THEN** 网关返回稳定的模型认证错误
- **THEN** 错误中不包含 API Key 或 Authorization Header

#### Scenario: 模型返回空输出
- **WHEN** Provider 正常响应但没有有效文本
- **THEN** 网关返回 output_invalid 错误
- **THEN** 不返回伪造的成功内容

### Requirement: 网关调用使用单次凭据和执行隔离
网关 SHALL 只在单次请求和执行上下文中使用 `ModelConnection`，MUST NOT 将 API Key 写入环境文件、配置文件、持久化缓存、日志、事件或 artifact，并 MUST 在请求结束后释放客户端和凭据引用。

#### Scenario: 单次调用结束
- **WHEN** 模型调用成功、失败、取消或超时结束
- **THEN** 网关释放该调用的模型客户端和临时凭据引用
- **THEN** 后续调用不能读取该次调用的凭据

#### Scenario: Provider 异常包含敏感值
- **WHEN** Provider 异常文本包含 API Key 或认证 Header
- **THEN** 网关在日志和错误事件输出前完成脱敏

### Requirement: 网关实施有界并发和取消
Model Gateway SHALL 对 Provider 调用实施有界并发和背压，并 MUST 在 Go deadline、客户端取消或服务关闭时停止对应模型调用；网关不得为每个请求创建无界后台任务。

#### Scenario: 达到并发上限
- **WHEN** 活跃模型调用达到配置上限
- **THEN** 新请求进入有界等待或返回资源耗尽错误
- **THEN** 服务不创建超过上限的 Provider 调用

#### Scenario: 调用被取消
- **WHEN** Go 取消 gRPC 上下文或房间关闭取消运行
- **THEN** 网关停止对应 Provider 调用和事件输出
- **THEN** 网关释放该调用占用的容量

### Requirement: 网关提供非敏感调用审计字段
网关 SHALL 为每次调用记录 `request_id`、`trace_id`、`purpose`、`protocol`、`profile_id`、`model_name`、耗时、结果分类和 usage；MUST NOT 记录消息正文、API Key 或完整 Provider 响应。

#### Scenario: 成功调用审计
- **WHEN** 一次模型调用成功
- **THEN** 日志和返回元数据包含用途、协议、Profile、模型和 usage
- **THEN** 日志不包含消息正文和凭据

#### Scenario: 失败调用审计
- **WHEN** 一次模型调用失败
- **THEN** 日志记录稳定的失败分类和耗时
- **THEN** 日志不包含供应商原始敏感响应
