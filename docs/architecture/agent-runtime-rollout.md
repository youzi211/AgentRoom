# Agent Runtime 灰度与回滚

## 传输选择

`AGENT_RUNTIME_TRANSPORT` 只接受 `local` 或 `grpc`。配置由 Go 在启动时读取；远程流出现 `UNAVAILABLE`、连接重置或协议错误时，当前 `run_id` 只进入失败/中断终态，不自动交给本地 Runtime 重做。

`COLLABORATION_RUNTIME_MODE` 独立控制房间协作主链，只接受 `legacy` 或 `remote`。`legacy` 保留 Go Runner 的 `mention_fanout` / `guided_dialogue` 路径；`remote` 使用 Python `CollaborationRuntimeService`，并由 Go 继续创建 `collaboration_runs`、验证事件和提交最终消息。远程协作已经可能开始模型调用后，不得用 Native/AutoGen/legacy 重新执行同一个 collaboration run。

Compose 开发栈显式使用内部 `agent-runtime:50051` gRPC 服务和 plaintext；直接在主机启动 backend 时默认保持 `local` / `legacy`，便于迁移期间回滚。生产环境应设置 `*_GRPC_INSECURE=false`，配置 CA 和可选客户端证书，并使用服务身份校验。

## 灰度清单

| 阶段 | 操作 | 观察项 | 负责人 |
| --- | --- | --- | --- |
| 预检查 | 确认 Python Agent Runtime 与 Collaboration Runtime Health 为 `SERVING`，MySQL 正常，生成代码检查通过 | `/api/ready`、`/api/admin/collaboration-runtime`、启动日志 | Backend on-call |
| 单 Agent 小流量 | 只为新 Agent Run 设 `AGENT_RUNTIME_TRANSPORT=grpc`，不迁移活动 local Run | gRPC 状态、失败率、排队、执行耗时、artifact 超限 | Backend on-call |
| 远程 Native | 设置 `COLLABORATION_RUNTIME_MODE=remote`、`COLLABORATION_ENGINE_ALLOWLIST=native`、`COLLABORATION_DEFAULT_TRIGGER_MODE=mention_only`，只让 allowlist 房间创建远程 Native collaboration run | 选角顺序、停止原因、取消收敛、重复消息、`collaboration_runs` 终态 | Agent Runtime owner |
| Fake 影子对比 | 使用 Fake 模型对 Native/AutoGen 运行 shadow comparison，不接入真实流量 | speaker 差异、事件差异、停止原因差异 | Agent Runtime owner |
| 真实模型隔离评估 | 在隔离环境运行 Native/AutoGen 真实模型用例 | 首响应延迟、总延迟、selector token、费用、成功率 | Release owner |
| AutoGen 灰度 | 设置 `COLLABORATION_ENGINE_ALLOWLIST=native,autogen` 且 `COLLABORATION_AUTOGEN_ENABLED=true`，只为灰度房间选择 `autogen + automatic` | 额外 selector 调用、费用、模型错误、内部消息隔离、checkpoint 兼容 | Release owner |
| 稳定期 | 停止为新协作写入旧 `dialogue_runs`，保留历史读取；观察后再隔离重复 Go fanout/guided 主链 | 无旧 Runtime 调用、无重复消息、历史查询兼容 | Release owner |

## 回滚演练

1. 停止把新房间或新 run 路由到 AutoGen：将 `COLLABORATION_AUTOGEN_ENABLED=false`，并从 `COLLABORATION_ENGINE_ALLOWLIST` 移除 `autogen`。
2. 若 AutoGen 之外的远程协作异常，保持 `COLLABORATION_RUNTIME_MODE=remote`，但把新房间默认切回 `COLLABORATION_DEFAULT_ENGINE=native` 与 `COLLABORATION_DEFAULT_TRIGGER_MODE=mention_only`。
3. 若 Python Collaboration Runtime 整体不可用，将 `COLLABORATION_RUNTIME_MODE=legacy` 并滚动重启 Go；已经进入终态或可能已调用模型的 collaboration run 不重做。
4. 如单 Agent Runtime 也需要回滚，停止把新 Agent Run 路由到 `grpc`，将 `AGENT_RUNTIME_TRANSPORT=local` 并滚动重启 Go。
5. 在切换前取消或等待活动远程 Run；不要把同一 `run_id` 或 `collaboration_run_id` 再提交给另一个 Runtime。
6. 调用 `/api/ready` 和 `/api/admin/collaboration-runtime` 确认 Go/MySQL/Runtime 可用；检查活动表中没有永久 `running` Run。
7. 核对已完成远程 Run 的 `messages.agent_run_id` 唯一关联和 `collaboration_runs` 终态，确认没有重复 Agent 消息。
8. 保留 Python 服务、数据库兼容字段和旧 `dialogue_runs` 历史读取，待确认无活动调用后再停止或隔离旧路径。

该演练只影响后续新 Run；已经成功提交或已进入终态的远程 Run 不会重做。旧 Runtime 删除前必须保留一个可部署的回滚版本。

## 观察期后的架构清理

观察期结束且灰度指标达标后，`remote` 模式下的房间协作主链只保留 Go 控制面到 Python `CollaborationRuntimeService` 的路径。Go 内原 `mention_fanout` / `guided_dialogue` 主链不得参与新远程协作，只能作为 `COLLABORATION_RUNTIME_MODE=legacy` 下显式选择的开发或部署回滚路径存在。

清理顺序应优先隔离再删除：先确认新协作不再写入旧 `dialogue_runs`，再将 Go fanout/guided 入口限制在 legacy 模式和测试夹具中；删除前必须保留可部署回滚版本，并继续保证任何已经开始的 `collaboration_run_id` 不会被 Native、AutoGen 或 legacy 路径重放。
