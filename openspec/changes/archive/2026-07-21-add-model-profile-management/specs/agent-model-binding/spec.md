## ADDED Requirements

### Requirement: 每个 Agent 可以绑定兼容的模型 Profile
系统 SHALL 允许管理员为 Agent 显式选择一个与 Agent Runtime 匹配的模型 Profile，也 SHALL 允许保持未绑定以继承对应运行时默认值。

#### Scenario: Go Agent 绑定 Go Profile
- **WHEN** 管理员为 `llm` Runtime Agent 选择一个启用的 `go` scope Profile并保存
- **THEN** 系统保存该 Profile ID 作为 Agent 的显式模型绑定

#### Scenario: DeepAgent 绑定 Go Profile
- **WHEN** 管理员尝试为 `deepagent` Runtime Agent 选择 `go` scope Profile
- **THEN** 系统拒绝保存并返回 Runtime 不匹配错误

#### Scenario: Agent 不选择专用模型
- **WHEN** 管理员把 Agent 模型选择设为“使用运行时默认模型”
- **THEN** 系统将全局 Agent 的 `model_profile_id` 保存为空

### Requirement: Agent 配置页面展示运行时兼容模型
Agent 配置页面 SHALL 展示 Agent 当前 Runtime、模型绑定状态和兼容 Profile 选择，并且 MUST NOT 向用户提供不兼容 Profile 作为可选项。

#### Scenario: 编辑普通 Go Agent
- **WHEN** 管理员编辑 `llm` Runtime Agent
- **THEN** 模型选择器显示“使用 Go 默认模型”和所有启用的 Go Profile

#### Scenario: 编辑 DeepAgent
- **WHEN** 管理员编辑 `deepagent` Runtime Agent
- **THEN** 模型选择器显示“使用 DeepAgent 默认模型”和所有启用的 DeepAgent Profile

#### Scenario: 切换 Agent Runtime
- **WHEN** 管理员切换 Agent Runtime且当前绑定与新 Runtime 不兼容
- **THEN** 页面要求清除绑定或选择兼容 Profile
- **THEN** 后端仍验证最终提交的 Runtime 与 Profile 一致性

### Requirement: 房间创建时快照具体模型选择
系统 MUST 在创建房间时把每个 Agent 当时显式绑定的 Profile，或当时对应运行时默认 Profile，解析为具体 Profile ID并写入房间 Agent 快照。

#### Scenario: 使用默认模型的 Agent 进入新房间
- **WHEN** 未显式绑定模型的 Agent 被加入房间且对应 Runtime 存在数据库默认 Profile
- **THEN** `room_agents` 快照保存该默认 Profile 的具体 ID

#### Scenario: 建房后更换全局 Agent 绑定
- **WHEN** 管理员在房间创建后修改同一全局 Agent 的模型绑定
- **THEN** 已有房间快照中的 Profile ID保持不变
- **THEN** 后续新房间使用更新后的绑定

#### Scenario: 建房后更新 Profile 连接内容
- **WHEN** 管理员修改房间快照所引用 Profile 的模型名、Base URL 或 API Key
- **THEN** 已有房间下一次调用读取更新后的 Profile 内容

### Requirement: 显式快照绑定失效时不得静默回退
系统 MUST 在房间 Agent 快照引用的 Profile 缺失、停用、Runtime 不匹配或无法解密时返回明确配置错误，并且 MUST NOT 自动改用默认 Profile。

#### Scenario: 房间绑定 Profile 被停用
- **WHEN** 房间中的 Agent 被触发且其快照 Profile 已停用
- **THEN** Agent 运行以模型配置错误结束
- **THEN** 系统不调用该 Runtime 的默认 Profile
