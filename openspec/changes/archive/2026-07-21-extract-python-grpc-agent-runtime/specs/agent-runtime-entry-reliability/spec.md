## ADDED Requirements

### Requirement: 远程 DeepAgent 请求内容与控制字段隔离
系统 SHALL 把用户控制的 DeepAgent 问题作为 Protobuf 内容字段传递，并 MUST 由 Python Executor 以数据而非命令行选项解释该字段。

#### Scenario: 问题以选项样式文本开头
- **WHEN** DeepAgent 问题以 `--` 或其他类似控制参数的文本开头
- **THEN** 该文本保持为问题内容
- **THEN** 它不能改变 Python 服务启动参数、Executor 选择或运行限制

### Requirement: 远程 DeepAgent 执行并发受服务容量约束
系统 SHALL 在 Python 常驻 Runtime 中限制 DeepAgent 并发，并 MUST 让等待容量的调用观察 gRPC deadline 和取消。

#### Scenario: 多个 DeepAgent 请求超过容量
- **WHEN** 活动 DeepAgent 数达到配置上限
- **THEN** 超额请求在有界队列等待或收到资源耗尽错误
- **THEN** Python 不启动超过配置上限的 DeepAgent 执行

#### Scenario: 容量等待期间调用取消
- **WHEN** 等待 DeepAgent 容量的 gRPC 调用被取消或超时
- **THEN** 该请求退出等待
- **THEN** 该请求不再启动 DeepAgent Executor

## REMOVED Requirements

### Requirement: DeepAgent question text is isolated from CLI options
**Reason**: DeepAgent 不再由 Go 为每次运行构造 CLI 参数，用户问题改为 Protobuf 请求中的普通内容字段。

**Migration**: 使用新增的“远程 DeepAgent 请求内容与控制字段隔离”要求，并通过跨语言契约测试验证选项样式文本不会进入服务启动或 Executor 控制字段。

### Requirement: DeepAgent subprocess execution is concurrency bounded
**Reason**: Go 后端不再启动 DeepAgent 子进程，原 Go 进程内信号量不能代表 Python 常驻服务的真实容量。

**Migration**: 使用新增的“远程 DeepAgent 执行并发受服务容量约束”要求，将并发限制、等待和取消转移到 Python Runtime，同时保留 Go 侧有界调用调度。
