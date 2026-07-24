## MODIFIED Requirements

### Requirement: 管理员可以管理运行时分组的模型 Profile
系统 SHALL 允许管理员创建、查看、编辑、启用、停用和删除模型 Profile，并且每个 Profile MUST 保留 `go` 或 `deepagent` Runtime Scope 以兼容现有绑定。Profile 的 `protocol` MUST 来自 Model Gateway 当前声明的受支持协议列表；不支持的协议不得保存。

#### Scenario: 创建 Go 模型 Profile
- **WHEN** 管理员提交名称、`go` scope、受支持协议、API Base URL、模型 ID 和 API Key
- **THEN** 系统创建一个可用于 Go 系统能力和 Go Agent 兼容路径的模型 Profile
- **THEN** 返回结果不包含明文或密文 API Key

#### Scenario: 创建 DeepAgent 模型 Profile
- **WHEN** 管理员提交名称、`deepagent` scope、受支持协议、API Base URL、模型 ID 和 API Key
- **THEN** 系统创建一个可用于 DeepAgent 的模型 Profile
- **THEN** Profile 协议被保留并在后续解析中传递给 Model Gateway

#### Scenario: 使用不支持的协议
- **WHEN** 管理员提交未被网关能力列表声明的协议
- **THEN** 系统拒绝保存并返回稳定的协议不支持错误

#### Scenario: 普通用户访问模型配置 API
- **WHEN** 未通过管理员鉴权的请求访问模型 Profile 列表或写操作
- **THEN** 系统拒绝该请求

### Requirement: 管理员可以测试 Profile 连接
系统 SHALL 允许管理员使用已保存 Profile 或未保存草稿执行一次 Model Gateway `Probe`，并返回成功状态、延迟、响应模型或脱敏错误；连接测试 MUST 使用 Profile 的实际协议，不得由 Go 手工固定请求 OpenAI Chat Completions。

#### Scenario: 测试已保存 Profile 成功
- **WHEN** 管理员测试一个配置正确且启用的 Profile
- **THEN** 后端通过 Model Gateway Probe 使用该 Profile 的协议发起最小真实请求
- **THEN** 返回成功状态、延迟和非敏感响应模型信息

#### Scenario: 测试草稿 Profile 失败
- **WHEN** 管理员使用错误 Base URL、模型 ID、协议或 API Key 测试未保存草稿
- **THEN** 系统返回已清洗的连接、认证、协议或模型错误
- **THEN** 日志和响应不包含草稿 API Key

### Requirement: 模型配置页面区分两个运行时分组并展示协议
前端 SHALL 提供独立模型配置页面，按 Go 与 DeepAgent Runtime Scope 展示 Profile、默认状态、启用状态、密钥配置状态和协议；创建 Profile 时协议选项 MUST 来自后端网关能力列表，编辑已有 Profile 时不得直接切换协议。

#### Scenario: 管理员打开模型配置页面
- **WHEN** 管理员访问模型配置页面
- **THEN** 页面分别展示 Go 与 DeepAgent Profile
- **THEN** 页面展示每个 Profile 的协议和支持的新建、编辑、测试、设为默认、停用、删除操作

#### Scenario: 创建 Profile 选择协议
- **WHEN** 管理员打开新建 Profile 表单
- **THEN** 页面加载网关声明的受支持协议选项
- **THEN** 保存 payload 包含选中的 protocol

#### Scenario: 编辑已有 Profile
- **WHEN** 管理员编辑已保存 Profile
- **THEN** 页面展示当前协议但禁用协议切换
- **THEN** 若需更换协议，管理员必须创建新的 Profile
