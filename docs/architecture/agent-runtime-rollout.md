# Agent Runtime 灰度与回滚

## 传输选择

`AGENT_RUNTIME_TRANSPORT` 只接受 `local` 或 `grpc`。配置由 Go 在启动时读取；远程流出现 `UNAVAILABLE`、连接重置或协议错误时，当前 `run_id` 只进入失败/中断终态，不自动交给本地 Runtime 重做。

Compose 开发栈显式使用内部 `agent-runtime:50051` gRPC 服务和 plaintext；直接在主机启动 backend 时默认保持 `local`，便于迁移期间回滚。生产环境应设置 `AGENT_RUNTIME_GRPC_INSECURE=false`，配置 CA 和可选客户端证书，并使用服务身份校验。

## 灰度清单

| 阶段 | 操作 | 观察项 | 负责人 |
| --- | --- | --- | --- |
| 预检查 | 确认 Python Health 为 `SERVING`，MySQL 正常，生成代码检查通过 | `/api/ready`、启动日志 | Backend on-call |
| 小流量 | 只为新 Agent Run 设 `grpc`，不迁移活动 local Run | gRPC 状态、失败率、排队、执行耗时、artifact 超限 | Backend on-call |
| 扩大 | 保持数据库兼容列和旧 Runtime，逐步扩大 Agent/房间范围 | Mention Fanout、Guided Dialogue、取消和长任务 | Agent Runtime owner |
| 稳定期 | 连续观察后再删除旧进程 Adapter 配置 | 无旧 Runtime 调用、无重复消息 | Release owner |

## 回滚演练

1. 停止把新 Run 路由到 `grpc`，将 `AGENT_RUNTIME_TRANSPORT` 改为 `local` 并滚动重启 Go。
2. 在切换前取消或等待活动远程 Run；不要把同一 `run_id` 再提交给 local Runtime。
3. 调用 `/api/ready` 确认 Go/MySQL 可用；检查活动表中没有永久 `running` Run。
4. 核对已完成远程 Run 的 `messages.agent_run_id` 唯一关联，确认没有重复 Agent 消息。
5. 保留 Python 服务和数据库兼容字段，待确认无活动调用后再停止 Python。

该演练只影响后续新 Run；已经成功提交或已进入终态的远程 Run 不会重做。旧 Runtime 删除前必须保留一个可部署的回滚版本。
