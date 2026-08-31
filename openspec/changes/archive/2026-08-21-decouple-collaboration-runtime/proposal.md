## Why

当前房间协作由 Go `Runner` 分别实现 `mention_fanout` 与 `guided_dialogue`，普通人类消息只有显式 mention 才能触发 Agent，单 Agent 执行、多人选角、交接、终止和审计也分散在不同路径。随着 AutoGen 等多 Agent 框架进入候选方案，继续把协作策略写死在 Go 业务层会扩大耦合，因此需要先建立框架中立的 Collaboration Runtime，让 AgentRoom 保持房间与数据所有权，同时允许协作引擎独立演进和替换。

## What Changes

- 新增统一的房间协作生命周期：每条符合房间策略的人类消息创建一个 collaboration run，显式 mention 仅作为优先选角信号；无 mention 时也可由协作引擎选择首位 Agent。
- 新增版本化的 Collaboration Runtime gRPC 合约，以不可变房间、Agent、消息、知识和策略快照启动一次多轮协作，并以有序事件返回选角、Agent turn、工具、artifact、交接、检查点和终态。
- 在 Go 中建立框架中立的 Collaboration Runtime 端口、客户端与运行治理；Go 继续负责权限、房间生命周期、持久化、幂等提交、WebSocket 映射和取消。
- 在 Python 常驻服务中新增 Collaboration Engine Registry，使 Native Engine 与 AutoGen Engine 通过同一接口执行，且不得直接访问 AgentRoom MySQL。
- 新增隔离的 AutoGen Engine 适配器，将 AgentRoom Agent 快照、协作策略、模型接口和事件映射到 AutoGen Team；AutoGen 类型、状态和依赖不得泄漏到 Go 或公共 API。
- 引入每房间单活动 collaboration run、人类消息优先中断、有界轮次、重复输出防护、deadline、容量和稳定终止语义。
- 以 MySQL 消息和运行审计作为业务事实源；框架状态仅作为带引擎名和版本的可选 opaque checkpoint，丢失或不兼容时必须能够从权威快照重建。
- 保留现有单 Agent Runtime 合约和 Native Engine 作为迁移与回滚路径；远程执行错误发生后不得自动换引擎重复同一 run。
- 与 `unify-model-gateway` 保持独立：本变更定义 Collaboration Engine 的模型端口，AutoGen 生产流量切换必须通过统一 Model Gateway 或等价的框架中立适配器，不新增 Provider 直连旁路。

## Capabilities

### New Capabilities

- `room-collaboration-orchestration`: 定义普通人类消息、显式 mention、选角、Agent 交接、人类中断、轮次限制和 collaboration run 审计的统一房间协作语义。
- `grpc-collaboration-runtime-contract`: 定义 Go 与 Python 之间版本化、多轮、服务端流式的 Collaboration Runtime 请求、事件、检查点、取消、错误和兼容要求。
- `collaboration-engine-service`: 定义 Python Collaboration Runtime、Engine Registry、Native Engine、状态隔离、容量治理、生命周期和框架中立边界。
- `autogen-collaboration-engine`: 定义 AutoGen Team 适配、Agent/消息/终止条件映射、事件转换、状态检查点、模型端口和依赖隔离要求。

### Modified Capabilities

- `remote-agent-run-governance`: 将“Go 必须亲自选择每一位发言 Agent”调整为“Go 保持业务和提交所有权，并可把受约束的多轮选角委托给 Collaboration Runtime”。
- `python-agent-runtime-service`: 从仅托管单 Agent Executor 扩展为同时托管逻辑独立的 Collaboration Runtime，并共享安全、容量、健康检查和优雅关闭基础设施。
- `realtime-message-delivery`: 在人类消息持久化并广播后异步启动或中断 collaboration run，确保协作规划、模型调用和运行切换均不阻塞原始消息交付。

## Impact

- Protobuf：新增版本化 Collaboration Runtime 服务、快照、策略、事件、检查点和错误类型；现有 `AgentRuntimeService.ExecuteAgent` 保持兼容。
- Go 后端：影响 `internal/agent`、`internal/service`、`internal/room`、`internal/realtime`、API 合约、运行取消治理、Store 接口和组合根。
- Python 服务：影响 `agent_runtime` Server、配置、健康检查、容量、日志、Executor/Engine Registry，并新增隔离的 AutoGen 依赖与适配模块。
- 数据：新增或扩展 collaboration run、turn、engine/checkpoint 审计字段；不把 AutoGen 内部消息或对象作为业务事实直接持久化。
- 前端：增加协作运行、选角、交接和中断活动展示，并调整建房/房间策略配置；普通消息协议保持兼容。
- 部署：Python Runtime 继续作为独立服务，可在同一进程注册 Agent Runtime 与 Collaboration Runtime；需要新增引擎选择、版本、容量和灰度配置。
- 测试：新增 Go/Python 跨语言契约、Native/AutoGen Engine 共享契约、每房间串行化、人类中断、幂等提交、状态恢复、敏感信息和回滚测试。
- 依赖：Python 新增固定版本的 Microsoft AutoGen 包；依赖必须封装在 AutoGen Engine 模块，其他生产模块不得直接导入。
