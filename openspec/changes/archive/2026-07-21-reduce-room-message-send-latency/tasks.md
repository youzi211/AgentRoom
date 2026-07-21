## 1. 锁定消息交付边界

- [x] 1.1 在服务层测试中加入可阻塞的焦点 LLM double 和可观测的房间事件收集器，覆盖消息持久化、广播与焦点调用的先后顺序
- [x] 1.2 明确消息持久化失败时不追加房间状态、不广播 `message` 事件且不创建焦点任务的测试断言

## 2. 拆分用户消息关键路径

- [x] 2.1 调整 `HandleHumanMessage`，保留输入校验、持久化和房间追加，将焦点分析从同步返回值路径移除
- [x] 2.2 调整 `PostRealtimeMessage`，在消息持久化成功后立即广播现有 `message` 事件，再把消息交给独立焦点调度器
- [x] 2.3 保持现有错误映射、`focus_update` 事件格式和 Agent 回复队列行为不变，并补充消息先于焦点完成事件到达的回归测试

## 3. 实现有界焦点后台调度

- [x] 3.1 为焦点分析增加固定 worker、有限队列和按房间去重状态；调度操作不得等待队列容量或启动无界 goroutine
- [x] 3.2 让同一房间在 queued/in-flight 期间合并新增消息，并为每次任务捕获不可变的消息快照与 `targetCount`
- [x] 3.3 使用脱离 WebSocket 请求取消但带有限超时的后台 context，确保客户端断开不会取消已接收消息的派生分析
- [x] 3.4 在有效非空结果完成后由后台 worker 发布现有 `focus_update` 事件，且不向客户端传播后台模型错误

## 4. 修正分析游标与并发规则

- [x] 4.1 让成功、模型错误、超时、响应解析失败和空结果都清理 in-flight 状态并将 `lastAnalyzed` 单调推进到 `targetCount`
- [x] 4.2 防止旧 `targetCount` 的迟到结果回退游标或覆盖较新的焦点状态，并覆盖分析期间新增消息的后续阈值触发
- [x] 4.3 增加失败/空结果后新增消息不足阈值不重复分析同一批消息的回归测试
- [x] 4.4 增加队列达到容量时消息仍能完成广播、且同一房间不会产生重复并发分析的回归测试

## 5. 验证与交付

- [x] 5.1 运行焦点服务、房间服务和 WebSocket API 的针对性 Go 测试
- [x] 5.2 运行 `go -C backend test ./...`、`go -C backend vet ./...` 和 `go -C backend build ./cmd/server`
- [x] 5.3 运行 `openspec validate reduce-room-message-send-latency --strict` 并检查变更仅包含本次 OpenSpec 提案文件
