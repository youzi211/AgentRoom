## 1. 数据模型与持久化

- [x] 1.1 新增 `model_profiles` 迁移与 GORM 模型，包含 runtime scope、协议、Base URL、模型名、加密密文、密钥提示、启用/默认状态和时间字段，并验证同一 scope 默认值约束
- [x] 1.2 为 `agents`、`room_agents` 增加可空 `model_profile_id`，为 `agent_runs` 增加实际 Profile ID、来源和模型名快照字段，确保旧数据迁移后保持可读
- [x] 1.3 新增模型 Profile domain/store 类型与 MySQL Repository，覆盖列表、按 ID/默认值查询、创建、更新、事务切换默认值、引用计数和受约束删除
- [x] 1.4 扩展 Agent 与房间快照的 MySQL 转换和测试用内存 Store，使模型绑定及运行审计字段能完整往返
- [x] 1.5 为迁移、Repository 默认值唯一性、引用删除冲突和旧数据空字段兼容补充后端测试

## 2. 密钥保护与模型 Profile 服务

- [x] 2.1 实现以 `MODEL_CONFIG_ENCRYPTION_KEY` 为主密钥的版本化 AES-256-GCM `SecretCipher`，使用随机 nonce 和 Profile ID附加认证数据
- [x] 2.2 为密钥加解密、错误主密钥、密文篡改、不同 nonce 和日志/错误不泄密补充单元测试
- [x] 2.3 实现 `ModelProfileService` 的字段校验、Base URL 规范化、Runtime/协议验证、密钥保留/替换/清除和响应脱敏
- [x] 2.4 实现每个 Runtime scope 的默认 Profile 事务规则，包括首个启用 Profile 自动默认、默认切换和禁止无替代停用
- [x] 2.5 实现 Profile 引用检查与删除错误契约，阻止删除被默认槽位、全局 Agent 或房间快照引用的 Profile
- [x] 2.6 实现后端发起的已保存/草稿 Profile 最小 Chat Completions 连接测试，并清洗鉴权、模型、超时和网络错误

## 3. 管理 API

- [x] 3.1 定义模型 Profile 的创建、更新、列表、测试和脱敏响应合约，确保 API 永不序列化密文或明文密钥
- [x] 3.2 注册 `/api/model-profiles` 管理路由，完成 CRUD、设为默认、停用和连接测试 Handler，并应用现有管理员鉴权
- [x] 3.3 扩展 Agent 创建/更新 API 接收可空 `modelProfileID`，在 Service 层验证 Agent Runtime 与 Profile scope 匹配
- [x] 3.4 为模型 Profile API 的管理员保护、脱敏、默认切换、草稿测试、删除冲突及 Agent 不兼容绑定补充黑盒测试

## 4. 模型解析与 Go Runtime

- [x] 4.1 实现 `ModelResolver`，按房间快照绑定、数据库 Runtime 默认值、迁移期环境变量的确定顺序返回解密后的运行配置
- [x] 4.2 为显式绑定停用/缺失/不可解密时禁止回退，以及旧房间空绑定使用默认/环境兜底补充解析测试
- [x] 4.3 调整房间创建流程，把 Agent 显式绑定或当时 Runtime 默认值解析为具体 Profile ID写入 `room_agents`，并验证后续 Agent 改绑不影响旧房间
- [x] 4.4 重构 Go `LLMAgentRuntime`，在每次 Agent 调用时按房间快照解析 Profile并创建对应 OpenAI-compatible 客户端
- [x] 4.5 实现动态 Go 默认客户端，使焦点提取和会议纪要每次调用当前 Go 默认 Profile，同时保留环境配置兜底
- [x] 4.6 在 `agent_runs` 中记录实际 Profile ID、配置来源和模型名，并验证活动/错误响应不包含 API Key

## 5. DeepAgent Runtime 配置注入

> 协调说明：5.1–5.3 记录的是已经完成的 local 子进程过渡实现。目标 Python gRPC 单次执行上下文由 `extract-python-grpc-agent-runtime` 继续跟踪，这些历史完成项不重新打开，也不得被解释为远程失败后的自动回退。

- [x] 5.1 调整 Go `DeepAgentRuntime`，在每次执行时解析 DeepAgent Profile并仅向子进程追加 `MODEL_PROTOCOL`、`MODEL_BASE_URL`、`MODEL_NAME`、`MODEL_API_KEY`
- [x] 5.2 保持 Python 非空进程环境优先于 `.env`/TOML 的行为，并增加测试证明数据库注入值不写入配置、报告或事件日志
- [x] 5.3 为 DeepAgent 专用绑定、DeepAgent 默认值、环境兜底、停用绑定、并发等待和超时路径补充 Go/Python 集成测试

## 6. 模型配置页面

- [x] 6.1 在前端 API Client 中增加模型 Profile 列表、创建、更新、默认切换、停用、删除和连接测试方法
- [x] 6.2 新增 `/models` 路由和模型配置管理页面，按 Go/DeepAgent 分组展示 Profile、默认状态、启用状态和密钥配置状态
- [x] 6.3 实现模型 Profile 编辑表单，包括名称、Runtime scope、API Base URL、模型 ID、API Key 密码输入、默认/启用状态和掩码保留语义
- [x] 6.4 实现已保存与草稿配置的“测试连接”交互，展示请求中、成功、延迟和已清洗失败状态，并提示测试会产生真实模型请求
- [x] 6.5 实现设为默认、替换/清除密钥、停用和受引用删除失败的确认与反馈交互
- [x] 6.6 更新管理工作台导航，使 Agent 配置与模型配置页面可相互切换，并沿用现有 Mantine 与响应式样式规范

## 7. Agent 配置页面

- [x] 7.1 扩展 Agent 管理页表单，提供 Runtime 选择和“使用 Runtime 默认模型”/兼容 Profile 选择
- [x] 7.2 在 Runtime 切换时清理或要求替换不兼容绑定，并确保后端错误仍以可理解方式展示
- [x] 7.3 在 Agent 列表和编辑器中显示实际绑定或默认继承状态，不暴露任何模型密钥信息
- [x] 7.4 增加前端测试覆盖 Go/DeepAgent Profile 过滤、默认继承、Runtime 切换和 Agent 保存 payload

## 8. 配置迁移、部署与文档

- [x] 8.1 在 `.env.example`、Docker Compose 和启动文档中加入 `MODEL_CONFIG_ENCRYPTION_KEY`，说明生成、备份和丢失后的影响
- [x] 8.2 保留并测试 `LLM_*` 与 DeepAgent `MODEL_*` 迁移兜底，在模型配置页无数据库默认值时显示非敏感环境兜底状态
- [x] 8.3 更新根 README、架构文档和 DeepAgent README，说明统一模型管理、两类 Runtime、优先级、密钥边界及不再要求模型密钥写入 DeepAgent 文件
- [x] 8.4 明确 Docker/生产环境中的 DeepAgent Python Runtime 可用性和配置注入方式，避免页面可配置但容器无执行器

## 9. 验证与回归

- [x] 9.1 运行 `gofmt`、`go -C backend test ./...`、`go -C backend vet ./...` 和 `go -C backend build ./cmd/server`，修复模型管理相关失败
- [x] 9.2 运行 DeepAgent `uv run pytest`，验证环境注入、配置优先级和密钥不落盘
- [x] 9.3 运行相关 Node 前端测试和 `npm --prefix frontend run build`，验证两个管理页面、路由与 API Client
- [ ] 9.4 执行端到端手工验收：创建并测试两个 Runtime 的多个 Profile、设置默认、绑定不同 Agent、创建房间、轮换密钥并验证旧房间使用原 Profile ID的新连接内容
- [x] 9.5 执行安全回归：检查 API 响应、应用日志、Agent 活动、DeepAgent argv/事件/报告和数据库非密钥字段均不出现测试 API Key 明文
