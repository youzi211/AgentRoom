## ADDED Requirements

### Requirement: Agent Runtime 使用版本化服务端流式协议
系统 SHALL 通过版本化 Protobuf 服务定义 `ExecuteAgent` 服务端流式 RPC；Go 客户端发送一个不可变的单 Agent turn 请求，Python 服务按顺序返回零个或多个进度事件以及一个终态结果。

#### Scenario: 正常执行返回有序事件流
- **WHEN** Go 使用受支持的协议版本和唯一 `run_id` 调用 `ExecuteAgent`
- **THEN** Python 首先返回运行已接受或已开始事件
- **THEN** 后续事件保持该次运行中的发送顺序
- **THEN** 流以一个成功或失败终态结束

#### Scenario: 请求使用不受支持的协议版本
- **WHEN** 请求声明的协议版本不受 Python 服务支持
- **THEN** 服务在启动模型或工具前以稳定的协议错误拒绝请求

### Requirement: 执行事件具有稳定类型和关联标识
每个 Agent 执行事件 MUST 包含 `run_id`、事件序号和事件类型，并 SHALL 使用稳定结构表达运行、模型、工具、输出、artifact 和终态信息。

#### Scenario: 工具执行产生进度
- **WHEN** Executor 在一次运行中调用工具
- **THEN** 流返回带相同 `run_id` 的工具开始和工具完成或失败事件
- **THEN** 每个后续事件的序号大于前一个事件

#### Scenario: 执行成功
- **WHEN** Executor 成功产生 Agent 回复
- **THEN** 最后一个应用事件包含完整最终内容、非敏感模型元数据和 artifact 列表
- **THEN** 服务不在成功终态后发送其他应用事件

#### Scenario: 执行发生可描述的应用错误
- **WHEN** 模型或工具在请求已被接受后失败
- **THEN** 服务返回包含稳定错误码和安全错误描述的失败终态
- **THEN** 失败终态不得包含凭据或未脱敏 Provider 响应

### Requirement: Deadline 和取消贯穿远程执行
Go 客户端 SHALL 为每次调用设置 deadline，并 MUST 通过 gRPC 调用上下文传播取消；Python 服务 SHALL 在模型调用、工具调用、容量等待和事件发送期间观察取消状态。

#### Scenario: 调用超过 deadline
- **WHEN** Agent 执行超过 Go 调用设置的 deadline
- **THEN** Go 取消流并把运行映射为超时状态
- **THEN** Python 停止继续启动新的模型或工具工作

#### Scenario: Go 主动取消运行
- **WHEN** 房间关闭、服务停机或运行治理逻辑取消活动 `run_id`
- **THEN** 对应 gRPC 上下文被取消
- **THEN** Python 结束对应执行并释放容量

#### Scenario: 等待容量时取消
- **WHEN** 请求仍在等待 Python 并发容量时调用被取消
- **THEN** 请求不启动 Executor
- **THEN** 服务以取消语义结束该调用

### Requirement: 传输错误使用规范 gRPC 状态语义
协议层、认证层、容量层和基础设施错误 MUST 使用规范 gRPC 状态码；已接受运行中的模型或工具业务失败 SHALL 使用失败终态事件，避免 Go 解析自由文本错误。

#### Scenario: 请求字段无效
- **WHEN** 请求缺少 `run_id`、Agent 定义或必要执行上下文
- **THEN** 服务以 `INVALID_ARGUMENT` 拒绝请求且不启动 Executor

#### Scenario: 服务容量已满且不能等待
- **WHEN** 服务达到硬容量或请求无法进入受控等待队列
- **THEN** 服务以 `RESOURCE_EXHAUSTED` 结束调用

#### Scenario: Python 服务不可用
- **WHEN** Go 无法建立连接或现有连接中断
- **THEN** Go 将传输错误映射为稳定的 Runtime 不可用错误
- **THEN** Go 不把该错误误判为 Agent 生成的失败内容

### Requirement: Agent Runtime 提供标准健康检查和优雅停机
Python 服务 SHALL 实现标准 gRPC Health Checking 服务，并 MUST 区分进程存活与接受新执行的就绪状态；停机时 SHALL 先停止接收新运行，再在受限宽限期内结束或取消活动运行。

#### Scenario: 服务启动但尚未就绪
- **WHEN** Python 进程已启动但 Executor Registry 尚未初始化完成
- **THEN** 健康检查报告不接受 Agent 执行

#### Scenario: 服务进入优雅停机
- **WHEN** Python 服务收到停机信号
- **THEN** 健康状态先切换为不接受新执行
- **THEN** 已接受运行在配置的宽限期内完成或被取消

### Requirement: 协议演进保持跨语言兼容
Protobuf 合约 MUST 使用稳定包版本、不得复用已删除字段编号，并 SHALL 通过 Go/Python 跨语言契约测试验证生成代码、事件解码和旧客户端兼容行为。

#### Scenario: 新服务增加可选字段
- **WHEN** Python 服务发送旧 Go 客户端未知的可选字段
- **THEN** 旧客户端仍能处理已知字段并完成运行

#### Scenario: 请求或事件超过大小限制
- **WHEN** 请求、单个事件或内联 artifact 超过明确配置的应用限制
- **THEN** 系统在执行或持久化超大载荷前返回稳定的资源超限错误

### Requirement: gRPC 传输保护执行凭据
模型凭据 MUST 仅存在于本次执行请求和 Python 运行内存中；非本机开发部署 SHALL 使用加密传输和服务身份校验，日志、追踪、事件和 artifact 不得记录明文凭据。

#### Scenario: Go 向 Python 发送解密后的模型凭据
- **WHEN** Go 为一次执行解析出数据库 Model Profile
- **THEN** 凭据只通过受保护的内部 gRPC 调用交给 Python
- **THEN** Python 返回的任何事件都不包含该凭据

#### Scenario: 开发环境显式使用非加密内部连接
- **WHEN** 本机开发配置选择非加密 gRPC
- **THEN** 配置必须显式启用不安全模式
- **THEN** 服务不得默认监听非受控外部网络接口
