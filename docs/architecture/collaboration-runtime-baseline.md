# 协作运行时迁移基线

本文记录 `decouple-collaboration-runtime` 开始实施前的行为基线。它只描述当前实现，用于保护迁移过程，不代表目标 Collaboration Runtime 已经上线。

## Go 协作行为

| 基线 | 证据 |
| --- | --- |
| `mention_fanout` 只响应人类显式 mention，并保持文本顺序与去重 | `backend/internal/tests/agent/mention_fanout_followup_test.go`、`mention_test.go` |
| Agent-to-Agent mention 受自治总轮数、单 Agent 轮数和自跟进策略限制 | `backend/internal/tests/agent/mention_fanout_followup_test.go` |
| `guided_dialogue` 保持选角顺序、父消息链和统一 `dialogue_run` 审计 | `backend/internal/tests/agent/dialogue_phase2_test.go` |
| 空输出、重复输出、轮次上限、Provider 失败和冷却取消产生稳定停止状态 | `backend/internal/tests/agent/dialogue_phase2_test.go` |
| 无 mention 的普通人类消息在两种当前模式下均不触发 Agent | `backend/internal/tests/agent/mention_fanout_followup_test.go` |
| 人类消息先持久化和广播，再显式进入 Agent/Focus 后台队列 | `backend/internal/tests/service/room_service_order_test.go`、`realtime_message_delivery_test.go` |
| Agent 成功消息与 `agent_run` 终态原子提交，提交失败不广播 Agent 消息 | `backend/internal/tests/service/agent_run_commit_test.go`、`backend/internal/tests/agent/activity_events_test.go` |

## Go/Python Runtime 治理

| 基线 | 证据 |
| --- | --- |
| Go 远程 Runtime 传播 deadline、取消房间活动调用并拒绝重复 run | `backend/internal/tests/agent/runtime_remote_test.go` |
| Go 本地 DeepAgent Runtime 在容量等待期间观察取消，并对错误和报告脱敏 | `backend/internal/tests/agent/runtime_deepagent_test.go` |
| Python Runtime 覆盖 deadline、取消、重复 run、容量、优雅关闭与关闭后拒绝新调用 | `deepagent/tests/test_agent_runtime_service.py` |
| Python 事件写入器限制 artifact 大小且不静默截断 | `deepagent/tests/test_agent_runtime_events.py`、`test_deepagent_executor.py` |
| Prompt、Authorization 和请求级凭据不进入结构化日志、错误或 artifact | `deepagent/tests/test_agent_runtime_security.py`、`test_agent_runtime_service.py`、`test_deepagent_executor.py` |

## 统一模型执行边界不变量（ADR-001）

ADR-001 实施后新增以下基线不变量：

| 不变量 | 证据 |
| --- | --- |
| `credential_ref` 只出现在 `ModelConfigResolver` 和 `CredentialResolver`，不进入 Engine、事件、checkpoint 或日志 | `deepagent/tests/test_collaboration_dependency_boundaries.py` |
| `profile:<id>` 在无 Secret Provider Adapter 时 preparation 阶段失败，不回退环境凭据 | `deepagent/tests/test_collaboration_model_execution_integration.py` |
| `environment:go` 凭据解析后正确传递 Authorization 和 model_name 到 HTTP server | `deepagent/tests/test_collaboration_model_execution_integration.py` |
| API key 不出现在事件流、序列化 checkpoint 或日志中 | `deepagent/tests/test_collaboration_model_execution_integration.py` |
| 准备阶段失败产生稳定错误码（`model_not_configured` / `model_authentication_failed` / `engine_unavailable`） | `deepagent/tests/test_collaboration_agent_executor.py`、`backend/internal/tests/collaboration/validator_test.go` |
| Native/AutoGen Engine 不 import `agent_runtime.model_config` | `deepagent/tests/test_collaboration_dependency_boundaries.py` |
| 协作运行配置失败不回退 Legacy 引擎 | `backend/internal/tests/service/collaboration_scheduler_test.go` |

## 基线验证命令

```powershell
go -C backend test ./internal/tests/agent ./internal/tests/service
deepagent\.venv\Scripts\python.exe -m pytest deepagent/tests/test_agent_runtime_service.py deepagent/tests/test_agent_runtime_events.py deepagent/tests/test_agent_runtime_security.py deepagent/tests/test_deepagent_executor.py -q
```

迁移后应继续运行这些用例，并用 Native Engine 共享场景对照旧 Go 路径的发言顺序、提交边界和终态。
