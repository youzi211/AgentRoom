# Collaboration Runtime 真实模型隔离评估

## 评估环境

- 评估时间：2026-08-21T15:58:26+08:00（Asia/Shanghai）
- 网关：`https://aiagent.lakala.com/model-gateway/v1/chat/completions`
- 模型：`TC-gemini-3.5-flash`
- 执行方式：在仓库本地临时评估脚本中直接实例化 `NativeCollaborationEngine` / `AutoGenCollaborationEngine`，使用内存房间快照和单轮策略；不连接 MySQL、不启动生产服务、不写入真实房间历史。
- 凭据处理：API key 仅通过进程环境变量注入；本文档、任务文件和日志记录不包含 key。
- 首响应口径：当前评估适配器使用 OpenAI-compatible 非流式 Chat Completions；因此“首响应延迟”记录为首个可见 `agent_message_completed` 事件的到达时间，和总延迟基本一致。

## 网关连通性

| 项目 | 结果 |
| --- | --- |
| Smoke 成功 | 是 |
| Smoke 延迟 | 1940.9 ms |
| Smoke token | 68 total（input 8 / output 60） |
| Smoke 费用 | 未返回金额（gateway_response_no_cost_field） |

## 用例结果

| 用例 | Engine | trigger mode | 首响应延迟 ms | 总延迟 ms | participant token | selector token | 费用 | 结果 | 终态 |
| --- | --- | --- | ---: | ---: | ---: | ---: | --- | --- | --- |
| native_mention_only_explicit | native | mention_only | 2040.8 | 2040.9 | 124 | 0 | 未返回金额（gateway_response_no_cost_field） | 成功 | completed |
| native_automatic | native | automatic | 1917.9 | 1918.1 | 124 | 0 | 未返回金额（gateway_response_no_cost_field） | 成功 | completed |
| autogen_automatic | autogen | automatic | 2042.6 | 2042.7 | 124 | 0 | 未返回金额（gateway_response_no_cost_field） | 成功 | completed |

## 汇总

- 成功率：3/3 = 100%
- 平均首响应延迟：2000.4 ms
- 平均总延迟：2000.6 ms
- participant token 合计：372
- selector token 合计：0
- AutoGen 相对 Native 的额外 selector token：0
- 费用：本次网关响应返回 usage，但没有 `cost` / `charge` / `fee` 金额字段；由于没有内部单价，本次只记录 token 计费输入，金额费用标记为 N/A。

## 结论

Native 显式 mention、Native automatic、AutoGen automatic 三个隔离真实模型用例均完成并产出 Agent 最终消息。当前 AutoGen 适配路径沿用确定性首发选择，没有额外 selector 模型调用；因此额外 selector token 为 0。正式灰度时仍需在监控中保留 selector token 与费用字段，以便未来启用模型选角后捕捉增量成本。
