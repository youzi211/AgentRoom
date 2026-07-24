## 1. 锁定现状与协议传播边界

- [ ] 1.1 为当前 Go 直连、Python LLM Executor、DeepAgent 模型构造和 Profile 连接测试补充调用路径基线测试，确保迁移前行为可比较
- [ ] 1.2 将 `Protocol` 加入 `ResolvedModelConfig` 和环境兜底配置，并更新数据库 Profile、显式 Profile、默认 Profile 的解析测试
- [ ] 1.3 移除远程 Agent 请求中固定的 OpenAI 协议值，完整映射解析后的 protocol，并增加 Go/Python 请求契约断言
- [ ] 1.4 定义首阶段 protocol 规范化规则与 `openai_chat_completions` 兼容默认值，覆盖空值、未知值和旧 Profile 数据

## 2. 建立 Model Gateway Protobuf 合约

- [ ] 2.1 在版本化 proto 包中定义协议中立的 ModelConnection、ModelMessage、ResponseFormat、ModelRequest、ModelResponse、ModelEvent、ProbeRequest 和 ProbeResponse
- [ ] 2.2 定义 `ModelGatewayService.Generate`、`Stream` 和 `Probe` RPC，以及稳定错误码、usage、能力和终态结构
- [ ] 2.3 生成并提交 Go/Python Protobuf 代码，更新生成脚本和生成物一致性检查
- [ ] 2.4 增加 Go/Python 跨语言契约测试，覆盖版本拒绝、未知可选字段、事件序号、唯一终态和敏感字段
- [ ] 2.5 增加 deadline、取消、资源耗尽和 gRPC 状态映射测试

## 3. 实现 Python Model Gateway Core

- [ ] 3.1 创建独立 `model_gateway` 模块，定义 Gateway Core、Provider Adapter 协议、Provider Registry 和能力声明
- [ ] 3.2 实现协议中立请求校验、用途和响应格式校验、Adapter 查找及调用前能力检查
- [ ] 3.3 实现统一 started、delta、usage、completed、failed 事件和非流式响应聚合
- [ ] 3.4 实现 Provider 异常到稳定错误码的映射，覆盖认证、权限、限流、超时、连接失败、空输出和输出超限
- [ ] 3.5 实现单次凭据上下文、异常脱敏、客户端释放和禁止敏感字段进入日志或事件的测试
- [ ] 3.6 实现有界并发、容量等待、取消传播和活动调用清理

## 4. 实现首个 Provider Adapter

- [ ] 4.1 实现 `OpenAICompatibleAdapter`，从 ModelConnection 构建 LangChain BaseChatModel
- [ ] 4.2 实现文本与 JSON 对象调用、流式文本块、usage 和 finish 状态归一化
- [ ] 4.3 实现统一 Probe 最小请求，不在 Adapter 外拼装 `/chat/completions`
- [ ] 4.4 建立 Provider Adapter 共享契约测试，覆盖成功、JSON、空输出、认证失败、限流、超时、取消和脱敏
- [ ] 4.5 使用假 Provider Adapter 验证新协议注册不需要修改 Gateway Core 或业务消费方

## 5. 迁移 Python Agent Executor

- [ ] 5.1 将普通 LLM Executor 从 OpenAI SDK 直连迁移到 ModelGatewayCore，并保持现有 Agent 流式事件、Think Block 过滤和 usage 语义
- [ ] 5.2 将 DeepAgent 模型构造迁移到共享 Provider Registry，将 Gateway 产生的 BaseChatModel 传入 `create_deep_agent`
- [ ] 5.3 删除 Executor 和 `agentroom_deepagent` 中重复的协议选择与 ChatOpenAI/ChatAnthropic 构造分支
- [ ] 5.4 更新普通 LLM 与 DeepAgent 测试，验证两者使用同一 Adapter、并发调用隔离且 artifact/工具事件不回归
- [ ] 5.5 增加生产 Python 源码依赖扫描，确保供应商客户端引用仅存在于 Provider Adapter 模块

## 6. 暴露并治理 ModelGatewayService

- [ ] 6.1 在现有 Python gRPC Server 中注册独立 ModelGatewayService，并让 AgentRuntimeService 与其共享 Gateway Core 而不使用回环 RPC
- [ ] 6.2 为 Generate、Stream 和 Probe 实现请求校验、错误返回、deadline 和取消处理
- [ ] 6.3 将 Model Gateway 纳入健康检查、就绪状态、结构化日志、容量指标和优雅停机流程
- [ ] 6.4 扩展服务安全测试，验证 TLS/显式本机不安全模式、消息大小限制及两个服务均不泄露凭据

## 7. 迁移 Go 模型消费方

- [ ] 7.1 在 Go `internal/llm` 中实现 ModelGatewayClient，支持 Generate、Stream、Probe 和统一错误映射
- [ ] 7.2 实现现有 `llm.Client`、`llm.JSONClient` 兼容适配器，让 Focus 和 Minutes 保持业务接口不变
- [ ] 7.3 更新服务装配，使 gRPC 模式下 Focus、Minutes 和其他 Go 系统能力使用 ModelGatewayClient
- [ ] 7.4 将 ModelProfileService 连接测试改为 Gateway Probe，删除手工 HTTP Chat Completions 请求与相关 HTTP 客户端依赖
- [ ] 7.5 增加 Go 集成测试，覆盖 Focus JSON、Minutes 文本、Profile Probe、不可用降级、deadline 和敏感错误映射
- [ ] 7.6 验证普通 Agent 继续通过 AgentRuntimeService 执行且不会从 Go 发起重复的 Model Gateway RPC

## 8. 更新协议管理与前端

- [ ] 8.1 增加受支持模型协议/能力的管理员只读 API，数据来源于网关注册能力而非前端硬编码
- [ ] 8.2 更新 Profile 创建校验，使 protocol 必须受支持，并禁止更新已有 Profile 的 protocol
- [ ] 8.3 更新草稿和已保存 Profile Probe payload，使测试携带实际协议并保持密钥不回显
- [ ] 8.4 在模型管理页增加协议选择、列表协议标识和已有 Profile 只读协议展示
- [ ] 8.5 更新前端模型 Profile helper 与 Node 测试，覆盖协议加载、创建 payload、编辑不可变和 Probe payload
- [ ] 8.6 保持普通用户无权访问协议能力与 Profile 管理 API，并补充后端安全测试

## 9. 清理旁路并更新文档

- [ ] 9.1 移除默认生产装配中的 Go LangChain/OpenAI 直连客户端和 Python OpenAI SDK 直连路径
- [ ] 9.2 将本地非 gRPC Runtime 删除或隔离为显式开发兼容模式，并记录其移除条件和不可用于生产的约束
- [ ] 9.3 增加 Go/Python 生产源码扫描，确保供应商 SDK 和供应商 HTTP 请求只存在于网关 Provider Adapter
- [ ] 9.4 更新 `README.md`、`.env.example`、DeepAgent README 和架构文档，说明控制面/执行面、双 gRPC 服务、TLS、健康检查和回滚方式
- [ ] 9.5 记录 Runtime Scope 继续保留的兼容决策，并为后续用途化模型绑定单独列出非阻塞改造项

## 10. 端到端验证与交付

- [ ] 10.1 运行 Python 网关、普通 LLM、DeepAgent、服务生命周期和安全测试
- [ ] 10.2 运行 `go -C backend test ./...`、`go -C backend vet ./...` 和 `go -C backend build ./cmd/server`
- [ ] 10.3 运行前端模型管理相关 Node 测试和 `npm --prefix frontend run build`
- [ ] 10.4 运行 Go 到 Python 集成测试，验证 Generate、Stream、Probe、普通 Agent、DeepAgent、Focus 和 Minutes 均经过同一 Gateway Core
- [ ] 10.5 执行凭据泄露扫描、`git diff --check` 和 `openspec validate unify-model-gateway --strict`
- [ ] 10.6 按先 Python 后 Go 的顺序完成本地启动验收，验证健康检查、模型连接测试、普通 Agent 回复、DeepAgent artifact、Focus 和会议纪要
