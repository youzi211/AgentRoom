## ADDED Requirements

### Requirement: 管理员可以管理运行时分组的模型 Profile
系统 SHALL 允许管理员创建、查看、编辑、启用、停用和删除 OpenAI-compatible 模型 Profile，并且每个 Profile MUST 属于 `go` 或 `deepagent` 运行时分组。

#### Scenario: 创建 Go 模型 Profile
- **WHEN** 管理员提交名称、`go` scope、API Base URL、模型 ID 和 API Key
- **THEN** 系统创建一个可用于 Go Agent 的模型 Profile
- **THEN** 返回结果不包含明文或密文 API Key

#### Scenario: 普通用户访问模型配置 API
- **WHEN** 未通过管理员鉴权的请求访问模型 Profile 列表或写操作
- **THEN** 系统拒绝该请求

#### Scenario: 使用不支持的协议
- **WHEN** 管理员提交首版不支持的模型协议
- **THEN** 系统拒绝保存并说明仅支持 OpenAI-compatible Chat Completions

### Requirement: 每个运行时分组具有唯一默认 Profile
系统 SHALL 为 `go` 和 `deepagent` 分组分别维护至多一个启用的默认 Profile，并在该分组存在可用数据库 Profile 时提供一个明确默认值。

#### Scenario: 创建分组中的第一个启用 Profile
- **WHEN** 某运行时分组尚无默认 Profile且管理员创建第一个启用 Profile
- **THEN** 系统自动将新 Profile 设为该分组默认值

#### Scenario: 更换默认 Profile
- **WHEN** 管理员把同一运行时分组中的另一个启用 Profile 设为默认值
- **THEN** 系统在同一事务中取消旧默认值并设置新默认值

#### Scenario: 停用当前默认 Profile
- **WHEN** 管理员尝试停用某运行时分组当前唯一默认 Profile且未指定替代默认值
- **THEN** 系统拒绝操作并要求先设置替代默认 Profile

### Requirement: API Key 加密保存且不可读回
系统 MUST 使用服务端主密钥对模型 API Key 进行认证加密，并且任何读取 API、日志、错误响应或活动事件 MUST NOT 返回明文密钥或可解密密文。

#### Scenario: 保存新 API Key
- **WHEN** 管理员创建 Profile 或替换已有 API Key
- **THEN** 系统使用随机 nonce 加密密钥后保存
- **THEN** API 仅返回 `hasAPIKey` 和掩码提示

#### Scenario: 编辑时不填写 API Key
- **WHEN** 管理员编辑已有 Profile并省略 API Key 字段
- **THEN** 系统保留原加密密钥不变

#### Scenario: 主密钥未配置
- **WHEN** 服务端没有有效加密主密钥且管理员尝试保存新的数据库 API Key
- **THEN** 系统拒绝操作并返回不包含密钥内容的配置错误

### Requirement: 管理员可以测试 Profile 连接
系统 SHALL 允许管理员使用已保存 Profile或未保存草稿执行一次后端发起的最小 Chat Completions 连接测试。

#### Scenario: 测试已保存 Profile 成功
- **WHEN** 管理员测试一个配置正确且启用的 Profile
- **THEN** 后端向其 `/v1/chat/completions` 端点发送最小请求
- **THEN** 返回成功状态、延迟和响应模型信息而不返回响应凭据

#### Scenario: 测试草稿 Profile 失败
- **WHEN** 管理员用错误 Base URL、模型 ID 或 API Key 测试未保存草稿
- **THEN** 系统返回已清洗的连接、鉴权或模型错误
- **THEN** 日志和响应不包含草稿 API Key

### Requirement: Profile 删除受到引用约束
系统 MUST 阻止删除仍被默认槽位、全局 Agent 或房间 Agent 快照引用的模型 Profile。

#### Scenario: 删除未引用 Profile
- **WHEN** 管理员删除一个非默认且未被任何 Agent 或房间快照引用的 Profile
- **THEN** 系统删除该 Profile

#### Scenario: 删除被房间快照引用的 Profile
- **WHEN** 管理员尝试删除仍被任一 `room_agents` 快照引用的 Profile
- **THEN** 系统拒绝删除并返回引用冲突

### Requirement: 模型配置页面区分两个运行时分组
前端 SHALL 提供独立模型配置页面，并按 Go 模型和 DeepAgent 模型分组展示 Profile、默认状态、启用状态和密钥配置状态。

#### Scenario: 管理员打开模型配置页面
- **WHEN** 管理员访问 `/models`
- **THEN** 页面分别展示 Go 与 DeepAgent Profile
- **THEN** 页面提供新增、编辑、测试、设为默认、停用和删除操作

#### Scenario: 编辑已有密钥
- **WHEN** 管理员打开已有 Profile 编辑器
- **THEN** 页面显示密钥已配置及掩码提示
- **THEN** 页面不把原始 API Key 回填到浏览器
