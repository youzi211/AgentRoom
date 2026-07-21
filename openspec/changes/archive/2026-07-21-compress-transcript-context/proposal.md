## Why

当前 Agent Runner 拼装提示词时，会把房间最近 30 条历史消息（`runner.go:61` `contextLimit=30`）原文全部塞入 `transcriptBlock`（`prompt_composer.go:165-183`），既不限制单条消息的长度，也没有任何摘要或降级机制。相比之下，知识片段（`knowledgeBlock`）已经有明确上限（最多 6 条、每条不超过 1200 字符）。这使得 transcript 成为当前提示词拼装中最主要的上下文膨胀风险点：在 `guided_dialogue` 模式下，连续多轮 Agent 的长回复会被原文反复带入后续每一轮提示词，直到被 30 条滑动窗口挤出，期间提示词体积、Token 成本和延迟随对话轮次线性增长，且没有任何兜底上限。现在处理是最小改动即可见效的窗口，越晚处理，未来接入更长会话或更多 Agent 轮次时的成本和延迟风险越大。

## What Changes

- 为 `transcriptBlock` 中每条历史消息的正文引入独立的字符长度上限：超出上限的消息内容将被截断，并附加明确的省略标记，让 Agent 知晓该消息被截断而非产生误导。
- 引入"新旧分层"的呈现策略：滑动窗口内较新的消息保持现有全文粒度（受单条长度上限约束），较早的消息降级为更简短的呈现形式（如：发言人 + 内容摘要行），从而在不增加调用成本的前提下压缩整体体积。
- 新增一个可配置的 transcript 总字符预算（对应大致的 Token 预算），当拼装出的 transcript 超出预算时，按"从最旧消息开始进一步压缩/丢弃"的顺序回收空间，直至满足预算或到达最小保留条数。
- 不引入任何额外的 LLM 调用；本次改动全部是确定性的字符串处理逻辑，可在 `agent` 包内实现和单测覆盖。
- **实现范围覆盖两条独立的提示词拼装实现**：`llm` Agent Runtime 当前有 Go 本地执行路径（`backend/internal/agent/prompt_composer.go`，架构上定性为迁移期回滚保留）和 Python gRPC 生产执行路径（`deepagent/src/agent_runtime/prompt.py`，当前实际执行 `llm` Agent 的路径）两套独立实现，通过 `prompt_golden.json` 做跨语言一致性校验。本次压缩逻辑需要在两侧同步实现等价行为，否则要么生产路径不受益，要么破坏两侧黄金测试的一致性保障。
- **不在本次范围**：真正的 LLM 摘要（用一次额外模型调用把旧对话浓缩为自然语言摘要）作为未来可选演进方向在 design.md 中记录，不在本次实现。

## Capabilities

### New Capabilities
- `transcript-context-compression`: 定义会议历史消息拼入 Agent 提示词时的长度与体积压缩规则，包括单条消息截断、新旧分层呈现和总量预算回收策略。

### Modified Capabilities
（无：本次不修改任何已验收的既有能力需求，`transcriptBlock` 当前没有对应的 spec 文件，本次改动作为新能力引入。）

## Impact

- 受影响代码：`backend/internal/agent/prompt_composer.go`（`formatTranscriptBlock` 及相关格式化函数）、`backend/internal/agent/prompt_context.go`（`PromptContext.Transcript` 的准备逻辑，可能需要携带额外的压缩元数据）、`backend/internal/agent/runner.go` 与 `backend/internal/agent/dialogue.go`（`contextLimit` 相关调用点，视方案是否需要调整取消息条数的上游行为）；同时需要改动 `deepagent/src/agent_runtime/prompt.py`（`format_transcript` 及相关格式化函数）以实现等价压缩逻辑。
- 需要更新 `proto/agent_runtime/v1/testdata/prompt_golden.json` 增补长会话/超长消息用例，并同时验证 `backend/internal/tests/agent/prompt_golden_test.go` 与 `deepagent/tests/test_prompt_golden.py` 两侧黄金测试通过。
- 不涉及数据库 schema、REST API 契约或前端变更；`model.Message` 结构本身不变。
- 不影响知识检索（`knowledgeBlock`）相关逻辑；二者是彼此独立的问题域。
