## 1. 后端统计契约

- [x] 1.1 定义入口摘要响应 contract，包含 activeRooms、todayRooms、knowledgeDocuments、enabledAgents。
- [x] 1.2 在 service/store 边界增加入口摘要查询模型和方法。
- [x] 1.3 明确 todayRooms 使用服务器本地日期边界，并在测试中固定时间输入。

## 2. 后端实现

- [x] 2.1 在 MySQL store 中实现 rooms、knowledge documents、enabled agents 的轻量聚合查询。
- [x] 2.2 在 teststore 中实现同等统计口径，供 API/service 测试使用。
- [x] 2.3 新增 `GET /api/entry-summary` 非 admin 路由。
- [x] 2.4 增加 API/service 测试，覆盖统计字段、统计口径和无需 admin key 访问。

## 3. 前端接入

- [x] 3.1 在 `roomClient.js` 中新增 `getEntrySummary` 或等价 API helper。
- [x] 3.2 移除 `JoinScreen.jsx` 中 `ENTRY_STATS` 的硬编码业务数字。
- [x] 3.3 让统计卡片展示后端摘要真实值，并在加载中显示占位。
- [x] 3.4 让统计加载失败时显示明确不可用状态，不回退到假数据。
- [x] 3.5 增加前端测试，断言入口统计使用摘要接口且不保留 `6/12/24/18` 假数据。

## 4. 验证

- [x] 4.1 运行 `go -C backend test ./...`。
- [x] 4.2 运行 `go -C backend vet ./...`。
- [x] 4.3 运行入口页相关前端 node tests。
- [x] 4.4 运行 `npm --prefix frontend run build`。
- [x] 4.5 运行 `openspec validate replace-entry-stats-with-real-data`。
