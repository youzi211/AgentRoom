## Context

`backend/internal/agent/runner.go:61` 将 `contextLimit` 固定为 30，`backend/internal/room/room.go:296` 的 `RecentMessages(limit)` 仅按条数截取最近消息，不检查也不限制单条 `Content` 的长度。`backend/internal/agent/prompt_composer.go:165-183` 的 `formatTranscriptBlock` 把这最多 30 条消息的 `SenderName`、`SenderType`、`Content` 原文逐行拼接进提示词的 Human 消息段。

对照组是同一文件里的 `formatKnowledgeBlock`（185-201 行）：知识片段在 `knowledge_service.go` 里已经有 `chunkSizeRunes = 1200` 和 `maxKnowledgeChunks = 6` 的硬上限，总量可预期、可控。transcript 没有对应机制，是当前提示词拼装里体积最不可控的部分，尤其在 `guided_dialogue` 模式下（`dialogue.go:155-156`），一次触发可能连续产生多个 Agent 长回复，这些长回复会被后续每一轮 `NewGuidedPromptContext`/`NewMentionPromptContext` 调用原文重新拼入提示词，直到被 30 条窗口挤出。

**重要补充（实现阶段发现，扩大了原定范围）**：`llm` Agent Runtime 当前有两套彼此独立的提示词拼装实现，而不是一套：
- `backend/internal/agent/prompt_composer.go`（`formatTranscriptBlock`）—— 对应 `AGENT_RUNTIME_TRANSPORT=local` 时的 Go 内嵌执行路径（`LLMAgentRuntime`），当前架构文档将其定性为"迁移期回滚保留"的旧实现。
- `deepagent/src/agent_runtime/prompt.py`（`format_transcript`）—— 对应 `AGENT_RUNTIME_TRANSPORT=grpc` 时的 Python 常驻 gRPC 执行面（`LLMExecutor`），这是**当前生产环境实际执行 `llm` Agent 的路径**。

两者通过 `proto/agent_runtime/v1/testdata/prompt_golden.json` 做跨语言黄金测试（`backend/internal/tests/agent/prompt_golden_test.go` + `deepagent/tests/test_prompt_golden.py`），要求对同一输入产出逐字节一致的提示词文本，以保证"回滚到 Go local 执行时 Agent 行为不变"这一保障成立。

据此，本次变更的实现范围扩大为：**在 Go 和 Python 两侧的执行层提示词拼装步骤中都实现等价的压缩逻辑**，而不是只改 Go 一侧。理由：
- 只改 Go 侧：线上真正生效的 Python 路径完全不受影响，无法解决 transcript 无上限增长的实际问题。
- 只改 Python 侧：会打破两侧黄金测试的一致性假设，且让"回滚到 local"这条路径重新出现本次要修复的问题。
- 两侧都改，是唯一能同时保证"生产路径修复生效"和"回滚路径行为一致"的选项。

约束：
- 不引入额外 LLM 调用（即不做"用模型做摘要"），本次改动必须是确定性、可单测的字符串处理逻辑，Go 和 Python 两侧行为需保持等价（同一输入产出同一渲染结果，误差范围内可接受由语言差异导致的字符串处理惯用法不同，但截断阈值、分层边界、预算数值必须一致）。
- `model.Message` 结构体和 `RecentMessages` 的调用签名、以及 `agent_runtime.proto` 的消息定义尽量保持稳定，避免影响 `runtime_remote.go`、`agentproto` 等下游序列化路径。
- 改动只作用于提示词拼装阶段的呈现层，不能影响存储在 MySQL / 内存 `Room.messages` 中的原始消息内容。

## Goals / Non-Goals

**Goals:**
- 为 transcript 中每条历史消息的正文设定确定性的单条字符长度上限，超长内容被截断并附带明确的省略标记。
- 引入"新旧分层"呈现：滑动窗口内较新的消息保留现有全文粒度（受单条上限约束），较早消息降级为更简短的呈现形式。
- 引入 transcript 整体的字符预算，超预算时按从最旧消息开始的顺序进一步压缩，直至满足预算或触达最小保留条数下限。
- 所有压缩逻辑在 Go `agent` 包和 Python `agent_runtime.prompt` 模块内分别以纯函数实现，便于用 `prompt_golden_test.go`/`test_prompt_golden.py` 一类的黄金测试和单元测试锁定行为。
- 两侧使用相同的截断阈值、分层边界、预算数值（作为各自语言的包内常量维护），保证跨语言黄金测试可以继续对同一输入断言逐字节一致的输出。

**Non-Goals:**
- 不实现基于 LLM 调用的语义摘要（用一次额外模型请求把旧对话浓缩为自然语言摘要）。这是更彻底但更贵的方案，本次只在下面的"未来演进方向"中记录，不实现、不预留调用位。
- 不改变 `RecentMessages(limit)` 取消息条数的存储层行为（仍取最近 30 条），本次只处理这 30 条被拼入提示词时的呈现体积。
- 不改动知识片段检索（`knowledgeBlock`）相关逻辑；该问题域已确认与本次变更彼此独立。
- 不改动 REST API 契约、数据库 schema 或前端。

## Decisions

### 1. 压缩发生在提示词拼装层，而不是 `Room.messages` 存储层
`formatTranscriptBlock` 是压缩逻辑的自然落点：它是唯一消费 `PromptContext.Transcript` 并生成最终文本的地方，压缩后的字符串只影响提示词渲染，不影响 `Room.messages`、MySQL 里持久化的消息原文，也不影响前端展示的历史记录。
- 备选方案：在 `RecentMessages` 或 `visibleTranscript` 阶段就做裁剪。放弃原因：这两处的返回值 `[]model.Message` 还会被 `runtime_remote.go` 序列化后发往 Python gRPC Agent Runtime（`RecentMessages` proto 字段），过早裁剪会让远程 Runtime 侧拿到的历史信息比 Go 侧记录的更少，且压缩逻辑会散落在多个消费点。

### 2. 新旧分层："全文层" + "摘要行层"，而非统一截断
把最近 30 条按"新→旧"分成两段：
- **全文层**（窗口内最新的若干条，具体条数由预算动态决定）：保留现有格式，仅对单条 `Content` 施加字符长度上限（超出则截断 + 追加 `...[内容过长，已截断]` 一类的省略标记）。
- **摘要行层**（更早的消息）：降级为更短的单行呈现，例如 `- SenderName (type): 前 N 字 + 省略号`，不再保留完整正文。

放弃"对所有 30 条统一施加同一个字符上限"的更简单方案，原因：guided_dialogue 场景下，Agent 最需要完整读到的是**刚发生的**上下文（用于承接话题、避免重复/矛盾），而较早的消息更多起"防止跑题"的背景作用，分层能在同样的总预算下让近期上下文保真度更高。

### 3. 总量预算作为压缩强度的最终控制阀，而非只按固定条数分层
除了单条上限和分层降级，再引入一个 transcript 总字符预算（可配置常量，例如与知识片段预算同一量级）。拼装完成后如果仍超预算，从最旧的一条开始继续压缩（摘要行进一步截短）或整条丢弃，直至满足预算或到达一个"最小保留条数"下限（避免压缩到 Agent 完全看不到最近对话）。
- 备选方案：只做分层，不做总预算兜底。放弃原因：极端情况下（例如很多条中等长度消息）分层本身不能保证总量有硬上限，仍可能出现体积失控；总预算兜底提供确定性的最坏情况保证。

### 4. 省略标记必须让 Agent 能感知内容被截断，而不是无声丢字
截断和降级都要拼入明确的标记文本（如省略号/说明性后缀），依据是 `fixedOutputContract()`（`prompt_composer.go:217-224`）已经要求 Agent "信息不足就说不确定，不要编造"——如果截断是无声的，Agent 会把截断后的片段当作完整信息，产生更隐蔽的错误回答。

### 5. 配置以代码内常量形式引入，不做成运行时可调的环境变量
沿用现有 `chunkSizeRunes`、`maxKnowledgeChunks` 这类"包内常量"的既有约定（而不是引入新的 `.env` 配置项），保持改动范围小、评审面窄。如果后续需要按房间/按部署环境调参，可在验证效果后单独提出配置化的变更。

### 6. Go 与 Python 各自独立实现，但共享同一组阈值常量的"设计意图"
不引入跨语言共享的配置文件或代码生成机制来强制两侧数值一致（例如没有必要为 4 个常量新建一个 YAML 加双语言读取器），而是在两侧代码中以注释显式标注"必须与对方保持一致"，并通过共享的 `prompt_golden.json` 用例作为跨语言一致性的可执行验证。放弃引入共享配置源的原因：常量数量很少（4 个）、变化频率低，专门的跨语言配置机制的维护成本超过收益；黄金测试已经是足够强的一致性保障。

## Risks / Trade-offs

- **[风险] 摘要行层丢失细节可能导致 Agent 引用较早消息时出错或产生幻觉** → 缓解：摘要行仍保留发言人和内容前缀，且输出约束要求 Agent 在信息不足时明确说不确定；后续如证明不够，可将"摘要行"升级为真正的 LLM 摘要（记录在 Open Questions）。
- **[风险] 单条截断可能切断一条对 Agent 决策关键的长消息（如详细的技术说明）** → 缓解：优先截断更早的消息、保留窗口内最新消息更宽松的长度上限；同时截断标记提示 Agent 该信息不完整，鼓励其在需要时向人类确认而非编造。
- **[风险] 新旧分层的条数/字符阈值凭经验设定，可能对不同房间的对话风格（短平快 vs 长篇分析）不是最优** → 缓解：以代码常量形式实现、集中在少数几个可读的函数里，方便后续按实际使用情况调整数值，不需要重新设计机制。
- **[风险] Go 和 Python 两侧实现出现数值或边界条件漂移（例如四舍五入、rune 与 Unicode 码点计数方式不同）导致黄金测试隐性失效或长期不一致** → 缓解：黄金测试新增覆盖"超长消息""长会话触发分层/预算回收"的用例，要求两侧逐字节比对；Python 侧用 Unicode 码点长度、Go 侧用 rune 长度，对中文等场景需要额外验证两者语义等价。
- **[权衡] 双侧实现使本次改动的工作量比最初评估的"只改 Go"大约一倍** → 权衡是必要的：只改一侧要么不解决生产问题，要么破坏回滚路径的一致性保障，双侧实现是唯一能同时满足两个约束的方案。

## Migration Plan

- 纯代码变更，无数据库 schema、无 API 契约变化，无需数据迁移。
- 分步实现：先在 Go 侧实现单条截断 → 分层 → 预算回收并补单测；再在 Python 侧实现等价逻辑并补 `pytest` 单测；最后更新 `prompt_golden.json` 增补长会话/超长消息用例，跑通两侧黄金测试确认一致。
- 部署无特殊步骤，随正常发布流程上线；如上线后发现压缩强度不合适，回滚只需回退该次代码提交（无状态迁移需要回滚），Go 和 Python 两侧需一起回滚以保持一致。

## Open Questions

- 单条消息长度上限、总体预算、最小保留条数这几个具体数值，是否需要参考实际生产对话长度分布再定，还是先用保守估计值上线、后续按观察调整？
- 是否需要在 `prompt_golden_test.go` 之外，新增一个针对"长对话场景"的专项集成测试，模拟 30 条超长消息验证总预算兜底确实生效？
- 未来如果决定引入 LLM 语义摘要，是复用本次的"分层"框架（把摘要行层替换为模型摘要），还是作为完全独立的新能力单独提案？本次不需要现在回答，但设计上应避免让当前实现难以被替换。
